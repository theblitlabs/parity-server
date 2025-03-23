package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/keystore"
	"github.com/theblitlabs/parity-server/internal/api"
	"github.com/theblitlabs/parity-server/internal/api/handlers"
	"github.com/theblitlabs/parity-server/internal/core/config"
	"github.com/theblitlabs/parity-server/internal/services"
	"github.com/theblitlabs/parity-server/internal/storage/db"
	"github.com/theblitlabs/parity-server/internal/utils"

)


func RunServer() {
	log := gologger.Get()

	cfg, err := config.GetConfigManager().GetConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load config")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbManager := db.GetDBManager()
	if err := dbManager.Connect(ctx, cfg.Database.URL); err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	
	gormDB := dbManager.GetDB()
	db.InitRepositoryFactory(gormDB)
	repoFactory := db.GetRepositoryFactory()

	log.Info().Msg("Successfully connected to database")

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()

	taskRepo := repoFactory.TaskRepository()
	runnerRepo := repoFactory.RunnerRepository()

	rewardCalculator := services.NewRewardCalculator()

	rewardClient := services.NewEthereumRewardClient(cfg)

	runnerService := services.NewRunnerService(runnerRepo)
	taskService := services.NewTaskService(taskRepo, rewardCalculator.(*services.RewardCalculator), runnerService)
	taskService.SetRewardClient(rewardClient)
	runnerService.SetTaskService(taskService)

	heartbeatTimeoutMinutes := cfg.Scheduler.Interval
	if heartbeatTimeoutMinutes <= 0 {
		heartbeatTimeoutMinutes = 5
		log.Warn().
			Int("default_timeout_minutes", heartbeatTimeoutMinutes).
			Msg("Heartbeat timeout not specified in config, using default")
	}

	heartbeatService := services.NewHeartbeatService(runnerService)
	heartbeatService.SetHeartbeatTimeout(time.Duration(heartbeatTimeoutMinutes) * time.Minute)
	heartbeatService.SetCheckInterval(1 * time.Minute)

	webhookService := services.NewWebhookService(taskService)
	s3Service, err := services.NewS3Service(cfg.AWS.BucketName)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize S3 service")
		return
	}
	taskHandler := handlers.NewTaskHandler(taskService, webhookService, runnerService, s3Service)

	if err := heartbeatService.Start(); err != nil {
		log.Error().Err(err).Msg("Failed to start heartbeat monitoring service")
	}

	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	defer monitorCancel()
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				taskService.MonitorTasks()
			}
		}
	}()

	internalStopCh := make(chan struct{})
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

	if err := utils.VerifyPortAvailable(cfg.Server.Host, cfg.Server.Port); err != nil {
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

	log.Info().
		Str("address", fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)).
		Msg("Server starting")

	go func() {
		<-stopChan
		log.Info().Msg("Shutdown signal received, gracefully shutting down...")

		close(internalStopCh)
		monitorCancel()

		heartbeatService.Stop()
		log.Info().Msg("Stopped heartbeat monitoring service")

		if runnerService != nil {
			if err := runnerService.StopTaskMonitor(); err != nil {
				log.Warn().Err(err).Msg("Error stopping task monitor")
			} else {
				log.Info().Msg("Stopped task monitor")
			}
		}

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
		if err := dbManager.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing database connection")
		} else {
			log.Info().
				Dur("duration_ms", time.Since(dbCloseStart)).
				Msg("Database connection closed successfully")
		}

		log.Info().Msg("Shutdown complete")
		shutdownCancel()
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Server failed to start")
	}

	<-shutdownCtx.Done()
}
