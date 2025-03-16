package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/keystore"
	"github.com/theblitlabs/parity-server/internal/api"
	"github.com/theblitlabs/parity-server/internal/api/handlers"
	"github.com/theblitlabs/parity-server/internal/config"
	"github.com/theblitlabs/parity-server/internal/database"
	"github.com/theblitlabs/parity-server/internal/database/repositories"
	"github.com/theblitlabs/parity-server/internal/services"

	"github.com/go-co-op/gocron"
)

func verifyPortAvailable(host string, port string) error {
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port number: %w", err)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, portNum))
	if err != nil {
		return fmt.Errorf("port %s is not available: %w", port, err)
	}
	ln.Close()
	return nil
}

func RunServer() {
	log := gologger.Get()

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Error().Err(err).Msg("Failed to load config")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	log.Info().Msg("Successfully connected to database")

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()

	taskRepo := repositories.NewTaskRepository(db)
	runnerRepo := repositories.NewRunnerRepository(db)

	rewardCalculator := services.NewRewardCalculator()

	rewardClient := services.NewEthereumRewardClient(cfg)

	runnerService := services.NewRunnerService(runnerRepo)
	taskService := services.NewTaskService(taskRepo, rewardCalculator.(*services.RewardCalculator), runnerService)
	taskService.SetRewardClient(rewardClient)

	scheduler := gocron.NewScheduler(time.UTC)

	if _, err := scheduler.Every(15).Minutes().Do(func() {
		taskService.MonitorTasks()
	}); err != nil {
		log.Error().Err(err).Msg("Failed to schedule task monitoring")
	}

	scheduler.StartAsync()

	webhookService := services.NewWebhookService(*taskService)
	s3Service, err := services.NewS3Service(cfg.AWS.BucketName)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize S3 service")
		return
	}
	taskHandler := handlers.NewTaskHandler(taskService, webhookService, runnerService, s3Service)

	internalStopCh := make(chan struct{})
	go func() {
		<-shutdownCtx.Done()
		close(internalStopCh)
	}()
	taskHandler.SetStopChannel(internalStopCh)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get home directory")
	}

	ks, err := keystore.NewKeystore(keystore.Config{
		DirPath:  filepath.Join(homeDir, KeystoreDirName),
		FileName: KeystoreFileName,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create keystore")
	}

	privateKey, err := ks.LoadPrivateKey()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get private key - please authenticate first")
	}

	// Create wallet client
	walletClient, err := walletsdk.NewClient(walletsdk.ClientConfig{
		RPCURL:       cfg.Ethereum.RPC,
		ChainID:      cfg.Ethereum.ChainID,
		TokenAddress: common.HexToAddress(cfg.Ethereum.TokenAddress),
		PrivateKey:   common.Bytes2Hex(crypto.FromECDSA(privateKey)),
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create wallet client")
	}

	// Create stake wallet instance
	stakeWallet, err := walletsdk.NewStakeWallet(
		walletClient,
		common.HexToAddress(cfg.Ethereum.StakeWalletAddress),
		common.HexToAddress(cfg.Ethereum.TokenAddress),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create stake wallet")
	}

	taskHandler.SetStakeWallet(stakeWallet)

	router := api.NewRouter(
		taskHandler,
		cfg.Server.Endpoint,
	)

	if err := verifyPortAvailable(cfg.Server.Host, cfg.Server.Port); err != nil {
		log.Fatal().
			Err(err).
			Str("host", cfg.Server.Host).
			Str("port", cfg.Server.Port).
			Msg("Server port is not available")
	}

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
	}

	go func() {
		<-stopChan
		log.Info().
			Msg("Shutdown signal received, gracefully shutting down...")
		shutdownCancel()
	}()

	go func() {
		log.Info().
			Str("address", fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)).
			Msg("Server starting")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	<-shutdownCtx.Done()

	serverShutdownCtx, serverShutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer serverShutdownCancel()

	log.Info().
		Int("shutdown_timeout_seconds", 15).
		Msg("Initiating server shutdown sequence")

	shutdownStart := time.Now()
	if err := server.Shutdown(serverShutdownCtx); err != nil {
		log.Error().
			Err(err).
			Msg("Server shutdown error")
		if err == context.DeadlineExceeded {
			log.Warn().
				Msg("Server shutdown deadline exceeded, forcing immediate shutdown")
		}
	} else {
		shutdownDuration := time.Since(shutdownStart)
		log.Info().
			Dur("duration_ms", shutdownDuration).
			Msg("Server HTTP connections gracefully closed")
	}

	log.Info().Msg("Starting webhook resource cleanup...")
	cleanupStart := time.Now()
	taskHandler.CleanupResources()
	log.Info().
		Dur("duration_ms", time.Since(cleanupStart)).
		Msg("Webhook resources cleanup completed")

	dbCloseStart := time.Now()

	sqlDB, err := db.DB()
	if err != nil {
		log.Error().Err(err).Msg("Error getting underlying *sql.DB instance")
	} else {
		if err := sqlDB.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing database connection")
		} else {
			log.Info().
				Dur("duration_ms", time.Since(dbCloseStart)).
				Msg("Database connection closed successfully")
		}
	}

	log.Info().Msg("Shutdown complete")
}
