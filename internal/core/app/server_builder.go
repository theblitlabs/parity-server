package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/keystore"
	"github.com/theblitlabs/parity-server/internal/api"
	"github.com/theblitlabs/parity-server/internal/api/handlers"
	"github.com/theblitlabs/parity-server/internal/core/config"
	"github.com/theblitlabs/parity-server/internal/core/services"
	"github.com/theblitlabs/parity-server/internal/database/repositories"
	"github.com/theblitlabs/parity-server/internal/storage/db"
	"github.com/theblitlabs/parity-server/internal/utils"
)

const (
	KeystoreDirName  = ".parity"
	KeystoreFileName = "keystore.json"
)

type Server struct {
	Config           *config.Config
	HttpServer       *http.Server
	DBManager        *db.DBManager
	TaskService      *services.TaskService
	RunnerService    *services.RunnerService
	HeartbeatService *services.HeartbeatService
	TaskHandler      *handlers.TaskHandler
	StopChannel      chan struct{}
	monitorCancel    context.CancelFunc
	monitorWg        *sync.WaitGroup
}

func (s *Server) Shutdown(ctx context.Context) {
	log := gologger.Get()

	serverShutdownCtx, serverShutdownCancel := context.WithTimeout(ctx, 15*time.Second)
	defer serverShutdownCancel()

	close(s.StopChannel)

	if s.monitorCancel != nil {
		log.Info().Msg("Cancelling task monitoring context")
		s.monitorCancel()

		if s.monitorWg != nil {
			log.Info().Msg("Waiting for task monitoring goroutine to exit")
			shutdownWaitCh := make(chan struct{})

			go func() {
				s.monitorWg.Wait()
				close(shutdownWaitCh)
			}()

			select {
			case <-shutdownWaitCh:
				log.Info().Msg("Task monitoring goroutine exited successfully")
			case <-time.After(2 * time.Second):
				log.Warn().Msg("Timed out waiting for task monitoring goroutine to exit")
			}
		}
	}

	s.HeartbeatService.Stop()
	log.Info().Msg("Stopped heartbeat monitoring service")

	if s.RunnerService != nil {
		if err := s.RunnerService.StopTaskMonitor(); err != nil {
			log.Warn().Err(err).Msg("Error stopping task monitor")
		} else {
			log.Info().Msg("Stopped task monitor")
		}
	}

	log.Info().Int("shutdown_timeout_seconds", 15).Msg("Initiating server shutdown sequence")
	shutdownStart := time.Now()

	if err := s.HttpServer.Shutdown(serverShutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
		if err == context.DeadlineExceeded {
			log.Warn().Msg("Server shutdown deadline exceeded, forcing immediate shutdown")
		}
	} else {
		shutdownDuration := time.Since(shutdownStart)
		log.Info().Dur("duration_ms", shutdownDuration).Msg("Server HTTP connections gracefully closed")
	}

	log.Info().Msg("Starting webhook resource cleanup...")
	cleanupStart := time.Now()
	s.TaskHandler.CleanupResources()
	log.Info().Dur("duration_ms", time.Since(cleanupStart)).Msg("Webhook resources cleanup completed")

	dbCloseStart := time.Now()
	if err := s.DBManager.Close(); err != nil {
		log.Error().Err(err).Msg("Error closing database connection")
	} else {
		log.Info().Dur("duration_ms", time.Since(dbCloseStart)).Msg("Database connection closed successfully")
	}

	log.Info().Msg("Shutdown complete")
}

type ServerBuilder struct {
	config           *config.Config
	dbManager        *db.DBManager
	repoFactory      *db.RepositoryFactory
	taskRepo         *repositories.TaskRepository
	runnerRepo       *repositories.RunnerRepository
	taskService      *services.TaskService
	runnerService    *services.RunnerService
	heartbeatService *services.HeartbeatService
	webhookService   *services.WebhookService
	s3Service        *services.S3Service
	stakeWallet      *walletsdk.StakeWallet
	taskHandler      *handlers.TaskHandler
	httpServer       *http.Server
	stopChannel      chan struct{}
	monitorCtx       context.Context
	monitorCancel    context.CancelFunc
	monitorWg        *sync.WaitGroup
	err              error
}

func NewServerBuilder(cfg *config.Config) *ServerBuilder {
	return &ServerBuilder{
		config:      cfg,
		stopChannel: make(chan struct{}),
	}
}

func (sb *ServerBuilder) InitDatabase() *ServerBuilder {
	if sb.err != nil {
		return sb
	}

	log := gologger.Get()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	URL := sb.config.Database.GetConnectionURL()

	sb.dbManager = db.GetDBManager()
	if err := sb.dbManager.Connect(ctx, URL); err != nil {
		sb.err = fmt.Errorf("failed to connect to database: %w", err)
		return sb
	}

	log.Info().Msg("Successfully connected to database")
	return sb
}

func (sb *ServerBuilder) InitRepositories() *ServerBuilder {
	if sb.err != nil {
		return sb
	}

	gormDB := sb.dbManager.GetDB()
	db.InitRepositoryFactory(gormDB)
	sb.repoFactory = db.GetRepositoryFactory()

	sb.taskRepo = sb.repoFactory.TaskRepository()
	sb.runnerRepo = sb.repoFactory.RunnerRepository()

	return sb
}

func (sb *ServerBuilder) InitServices() *ServerBuilder {
	if sb.err != nil {
		return sb
	}

	rewardCalculator := services.NewRewardCalculator()
	rewardClient := services.NewEthereumRewardClient(sb.config)

	sb.runnerService = services.NewRunnerService(sb.runnerRepo)
	sb.taskService = services.NewTaskService(sb.taskRepo, rewardCalculator.(*services.RewardCalculator), sb.runnerService)
	sb.taskService.SetRewardClient(rewardClient)
	sb.runnerService.SetTaskService(sb.taskService)

	sb.webhookService = services.NewWebhookService(sb.taskService)

	s3Service, err := services.NewS3Service(sb.config)
	if err != nil {
		sb.err = fmt.Errorf("failed to initialize S3 service: %w", err)
		return sb
	}
	sb.s3Service = s3Service

	return sb
}

func (sb *ServerBuilder) InitHeartbeatService() *ServerBuilder {
	if sb.err != nil {
		return sb
	}

	log := gologger.Get()

	heartbeatTimeoutMinutes := sb.config.Scheduler.Interval
	if heartbeatTimeoutMinutes <= 0 {
		heartbeatTimeoutMinutes = 5
		log.Warn().
			Int("default_timeout_minutes", heartbeatTimeoutMinutes).
			Msg("Heartbeat timeout not specified in config, using default")
	}

	sb.heartbeatService = services.NewHeartbeatService(sb.runnerService)
	sb.heartbeatService.SetHeartbeatTimeout(time.Duration(heartbeatTimeoutMinutes) * time.Minute)
	sb.heartbeatService.SetCheckInterval(1 * time.Minute)

	if err := sb.heartbeatService.Start(); err != nil {
		sb.err = fmt.Errorf("failed to start heartbeat monitoring service: %w", err)
		return sb
	}

	return sb
}

func (sb *ServerBuilder) InitTaskMonitoring() *ServerBuilder {
	log := gologger.Get()
	if sb.err != nil {
		return sb
	}

	sb.monitorCtx, sb.monitorCancel = context.WithCancel(context.Background())
	sb.monitorWg = &sync.WaitGroup{}

	log.Info().Msg("Starting task monitoring services")
	sb.taskService.StartMonitoring()

	return sb
}

func (sb *ServerBuilder) InitWallet() *ServerBuilder {
	if sb.err != nil {
		return sb
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		sb.err = fmt.Errorf("failed to get home directory: %w", err)
		return sb
	}

	ks, err := keystore.NewKeystore(keystore.Config{
		DirPath:  filepath.Join(homeDir, KeystoreDirName),
		FileName: KeystoreFileName,
	})
	if err != nil {
		sb.err = fmt.Errorf("failed to create keystore: %w", err)
		return sb
	}

	privateKey, err := ks.LoadPrivateKey()
	if err != nil {
		sb.err = fmt.Errorf("failed to get private key - please authenticate first: %w", err)
		return sb
	}

	walletClient, err := walletsdk.NewClient(walletsdk.ClientConfig{
		RPCURL:       sb.config.Ethereum.RPC,
		ChainID:      sb.config.Ethereum.ChainID,
		TokenAddress: common.HexToAddress(sb.config.Ethereum.TokenAddress),
		PrivateKey:   common.Bytes2Hex(crypto.FromECDSA(privateKey)),
	})
	if err != nil {
		sb.err = fmt.Errorf("failed to create wallet client: %w", err)
		return sb
	}

	stakeWallet, err := walletsdk.NewStakeWallet(
		walletClient,
		common.HexToAddress(sb.config.Ethereum.StakeWalletAddress),
		common.HexToAddress(sb.config.Ethereum.TokenAddress),
	)
	if err != nil {
		sb.err = fmt.Errorf("failed to create stake wallet: %w", err)
		return sb
	}
	sb.stakeWallet = stakeWallet

	return sb
}

func (sb *ServerBuilder) InitRouter() *ServerBuilder {
	if sb.err != nil {
		return sb
	}

	sb.taskHandler = handlers.NewTaskHandler(sb.taskService, sb.webhookService, sb.runnerService, sb.s3Service)
	sb.taskHandler.SetStopChannel(sb.stopChannel)
	sb.taskHandler.SetStakeWallet(sb.stakeWallet)

	router := api.NewRouter(
		sb.taskHandler,
		sb.config.Server.Endpoint,
	)

	if err := utils.VerifyPortAvailable(sb.config.Server.Host, sb.config.Server.Port); err != nil {
		sb.err = fmt.Errorf("server port is not available: %w", err)
		return sb
	}

	sb.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%s", sb.config.Server.Host, sb.config.Server.Port),
		Handler: router,
	}

	return sb
}

func (sb *ServerBuilder) Build() (*Server, error) {
	if sb.err != nil {
		return nil, sb.err
	}

	return &Server{
		Config:           sb.config,
		HttpServer:       sb.httpServer,
		DBManager:        sb.dbManager,
		TaskService:      sb.taskService,
		RunnerService:    sb.runnerService,
		HeartbeatService: sb.heartbeatService,
		TaskHandler:      sb.taskHandler,
		StopChannel:      sb.stopChannel,
		monitorCancel:    sb.monitorCancel,
		monitorWg:        sb.monitorWg,
	}, nil
}
