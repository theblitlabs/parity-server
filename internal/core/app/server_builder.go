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
	"github.com/theblitlabs/parity-server/internal/core/ports"
	"github.com/theblitlabs/parity-server/internal/core/services"
	"github.com/theblitlabs/parity-server/internal/database"
	"github.com/theblitlabs/parity-server/internal/database/repositories"
	"github.com/theblitlabs/parity-server/internal/utils"
	"gorm.io/gorm"
)

const (
	KeystoreDirName  = ".parity"
	KeystoreFileName = "keystore.json"
)

type Server struct {
	Config                  *config.Config
	HttpServer              *http.Server
	DB                      *gorm.DB
	TaskService             *services.TaskService
	RunnerService           *services.RunnerService
	ReputationService       *services.ReputationService
	RunnerMonitoringService *services.RunnerMonitoringService
	HeartbeatService        *services.HeartbeatService
	TaskQueue               *services.TaskQueue
	TaskHandler             *handlers.TaskHandler
	RunnerHandler           *handlers.RunnerHandler
	WebhookHandler          *handlers.WebhookHandler
	StopChannel             chan struct{}
	monitorCancel           context.CancelFunc
	monitorWg               *sync.WaitGroup
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

	// Stop task queue processor
	if s.TaskQueue != nil {
		s.TaskQueue.Stop()
		log.Info().Msg("Stopped task queue processor")
	}

	// Stop reputation monitoring service
	if s.RunnerMonitoringService != nil {
		if err := s.RunnerMonitoringService.Stop(); err != nil {
			log.Warn().Err(err).Msg("Error stopping reputation monitoring service")
		} else {
			log.Info().Msg("Stopped reputation monitoring service")
		}
	}

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
	s.WebhookHandler.CleanupResources()
	log.Info().Dur("duration_ms", time.Since(cleanupStart)).Msg("Webhook resources cleanup completed")

	dbCloseStart := time.Now()
	if sqlDB, err := s.DB.DB(); err != nil {
		log.Error().Err(err).Msg("Error getting underlying *sql.DB")
	} else if err := sqlDB.Close(); err != nil {
		log.Error().Err(err).Msg("Error closing database connection")
	} else {
		log.Info().Dur("duration_ms", time.Since(dbCloseStart)).Msg("Database connection closed successfully")
	}

	log.Info().Msg("Shutdown complete")
}

type ServerBuilder struct {
	config                      *config.Config
	DB                          *gorm.DB
	taskRepo                    *repositories.TaskRepository
	runnerRepo                  *repositories.RunnerRepository
	reputationRepo              *repositories.ReputationRepository
	promptRepo                  ports.PromptRepository
	billingRepo                 ports.BillingRepository
	flSessionRepo               ports.FLSessionRepository
	flRoundRepo                 ports.FLRoundRepository
	flParticipantRepo           ports.FLParticipantRepository
	taskService                 *services.TaskService
	runnerService               *services.RunnerService
	reputationService           *services.ReputationService
	reputationBlockchainService *services.ReputationBlockchainService
	runnerMonitoringService     *services.RunnerMonitoringService
	llmService                  *services.LLMService
	taskQueue                   *services.TaskQueue
	heartbeatService            *services.HeartbeatService
	webhookService              *services.WebhookService
	storageService              services.StorageService
	verificationService         *services.VerificationService
	federatedLearningService    *services.FederatedLearningService
	flRewardService             *services.FLRewardService
	stakeWallet                 *walletsdk.StakeWallet
	taskHandler                 *handlers.TaskHandler
	runnerHandler               *handlers.RunnerHandler
	reputationHandler           *handlers.ReputationHandler
	webhookHandler              *handlers.WebhookHandler
	llmHandler                  *handlers.LLMHandler
	federatedLearningHandler    *handlers.FederatedLearningHandler
	httpServer                  *http.Server
	stopChannel                 chan struct{}
	monitorCtx                  context.Context
	monitorCancel               context.CancelFunc
	monitorWg                   *sync.WaitGroup
	err                         error
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

	db, err := database.Connect(ctx, URL)
	if err != nil {
		sb.err = fmt.Errorf("failed to connect to database: %w", err)
		return sb
	}

	sb.DB = db
	log.Info().Msg("Successfully connected to database")
	return sb
}

func (sb *ServerBuilder) InitRepositories() *ServerBuilder {
	if sb.err != nil {
		return sb
	}

	sb.taskRepo = repositories.NewTaskRepository(sb.DB)
	sb.runnerRepo = repositories.NewRunnerRepository(sb.DB)
	sb.reputationRepo = repositories.NewReputationRepository(sb.DB)
	sb.promptRepo = repositories.NewPromptRepository(sb.DB)
	sb.billingRepo = repositories.NewBillingRepository(sb.DB)
	sb.flSessionRepo = repositories.NewFLSessionRepository(sb.DB)
	sb.flRoundRepo = repositories.NewFLRoundRepository(sb.DB)
	sb.flParticipantRepo = repositories.NewFLParticipantRepository(sb.DB)

	return sb
}

func (sb *ServerBuilder) InitServices() *ServerBuilder {
	if sb.err != nil {
		return sb
	}

	rewardCalculator := services.NewRewardCalculator()
	rewardClient := services.NewFilecoinRewardClient(sb.config)

	sb.runnerService = services.NewRunnerService(sb.runnerRepo)
	sb.taskService = services.NewTaskService(sb.taskRepo, rewardCalculator.(*services.RewardCalculator), sb.runnerService)
	sb.taskService.SetRewardClient(rewardClient)
	sb.runnerService.SetTaskService(sb.taskService)

	sb.webhookService = services.NewWebhookService(sb.taskService)

	storageService, err := services.NewStorageService(sb.config)
	if err != nil {
		sb.err = fmt.Errorf("failed to initialize storage service: %w", err)
		return sb
	}
	sb.storageService = storageService

	sb.verificationService = services.NewVerificationService(sb.taskRepo)

	// Initialize task queue before LLM service
	sb.taskQueue = services.NewTaskQueue(sb.promptRepo, sb.runnerRepo, sb.runnerService)

	sb.llmService = services.NewLLMService(sb.promptRepo, sb.billingRepo, sb.runnerRepo, sb.runnerService, sb.taskQueue)
	sb.federatedLearningService = services.NewFederatedLearningService(
		sb.flSessionRepo,
		sb.flRoundRepo,
		sb.flParticipantRepo,
		sb.runnerService,
		sb.taskService,
	)

	// Initialize FL reward service
	sb.flRewardService = services.NewFLRewardService(
		sb.config,
		sb.flSessionRepo,
		sb.flParticipantRepo,
		sb.runnerService,
	)
	sb.federatedLearningService.SetFLRewardService(sb.flRewardService)

	// Initialize reputation blockchain service
	filecoinService, ok := sb.storageService.(*services.FilecoinService)
	if !ok {
		// Create a minimal service for reputation blockchain
		filecoinService = nil
	}

	reputationBlockchainService, err := services.NewReputationBlockchainService(sb.config, filecoinService)
	if err != nil {
		sb.err = fmt.Errorf("failed to initialize reputation blockchain service: %w", err)
		return sb
	}
	sb.reputationBlockchainService = reputationBlockchainService

	// Initialize reputation service
	reputationService, err := services.NewReputationService(
		sb.reputationRepo,
		sb.reputationBlockchainService,
		sb.config.FilecoinNetwork.RPC,
		sb.config.SmartContract.ReputationContractAddress,
	)
	if err != nil {
		sb.err = fmt.Errorf("failed to initialize reputation service: %w", err)
		return sb
	}
	sb.reputationService = reputationService

	// Initialize runner monitoring service
	sb.runnerMonitoringService = services.NewRunnerMonitoringService(
		sb.runnerService,
		sb.reputationService,
		sb.taskService,
	)

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

	// Start task queue processor
	go sb.taskQueue.Start(sb.monitorCtx)
	log.Info().Msg("Task queue processor started")

	// Start reputation monitoring service if enabled
	if sb.config.Reputation.MonitoringEnabled && sb.runnerMonitoringService != nil {
		if err := sb.runnerMonitoringService.Start(); err != nil {
			log.Warn().Err(err).Msg("Failed to start runner monitoring service")
		} else {
			log.Info().Msg("Runner monitoring service started")
		}
	}

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
		RPCURL:       sb.config.FilecoinNetwork.RPC,
		ChainID:      sb.config.FilecoinNetwork.ChainID,
		TokenAddress: common.HexToAddress(sb.config.FilecoinNetwork.TokenAddress),
		PrivateKey:   common.Bytes2Hex(crypto.FromECDSA(privateKey)),
	})
	if err != nil {
		sb.err = fmt.Errorf("failed to create wallet client: %w", err)
		return sb
	}

	stakeWallet, err := walletsdk.NewStakeWallet(
		walletClient,
		common.HexToAddress(sb.config.FilecoinNetwork.StakeWalletAddress),
		common.HexToAddress(sb.config.FilecoinNetwork.TokenAddress),
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

	sb.taskHandler = handlers.NewTaskHandler(sb.taskService, sb.storageService, sb.verificationService)
	sb.taskHandler.SetStakeWallet(sb.stakeWallet)
	sb.taskHandler.SetWebhookService(sb.webhookService)

	// FL reward service now uses real blockchain transactions directly

	sb.runnerHandler = handlers.NewRunnerHandler(sb.taskService, sb.runnerService)
	sb.webhookHandler = handlers.NewWebhookHandler(sb.webhookService, sb.runnerService)
	sb.webhookHandler.SetStopChannel(sb.stopChannel)
	sb.llmHandler = handlers.NewLLMHandler(sb.llmService)
	sb.federatedLearningHandler = handlers.NewFederatedLearningHandler(sb.federatedLearningService)
	sb.reputationHandler = handlers.NewReputationHandler(sb.reputationService, sb.runnerMonitoringService)

	router := api.NewRouter(
		sb.taskHandler,
		sb.runnerHandler,
		sb.webhookHandler,
		sb.llmHandler,
		sb.federatedLearningHandler,
		sb.reputationHandler,
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
		Config:                  sb.config,
		HttpServer:              sb.httpServer,
		DB:                      sb.DB,
		TaskService:             sb.taskService,
		RunnerService:           sb.runnerService,
		ReputationService:       sb.reputationService,
		RunnerMonitoringService: sb.runnerMonitoringService,
		HeartbeatService:        sb.heartbeatService,
		TaskQueue:               sb.taskQueue,
		TaskHandler:             sb.taskHandler,
		RunnerHandler:           sb.runnerHandler,
		WebhookHandler:          sb.webhookHandler,
		StopChannel:             sb.stopChannel,
		monitorCancel:           sb.monitorCancel,
		monitorWg:               sb.monitorWg,
	}, nil
}
