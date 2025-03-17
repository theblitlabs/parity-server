package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/theblitlabs/parity-server/internal/models"
	"github.com/theblitlabs/parity-server/internal/services"

	"github.com/go-co-op/gocron"
	"github.com/google/uuid"
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
	runnerService.SetTaskService(taskService)

	// Initialize scheduler with task monitoring
	scheduler := gocron.NewScheduler(time.UTC)
	if _, err := scheduler.Every(30).Seconds().Do(func() {
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

	// Set up internal stop channel for task handler
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

	// Start server in main goroutine
	log.Info().
		Str("address", fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)).
		Msg("Server starting")

	go func() {
		<-stopChan
		log.Info().Msg("Shutdown signal received, gracefully shutting down...")

		log.Info().Msg("Stopping scheduler...")
		scheduler.Stop()

		close(internalStopCh)

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

		// Clean up resources
		log.Info().Msg("Starting webhook resource cleanup...")
		cleanupStart := time.Now()
		taskHandler.CleanupResources()
		log.Info().
			Dur("duration_ms", time.Since(cleanupStart)).
			Msg("Webhook resources cleanup completed")

		// Close database connection
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
		shutdownCancel()
	}()

	// Start server and handle errors
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Server failed to start")
	}

	<-shutdownCtx.Done()
}

func pushTaskToRunner(taskID string, runnerID string, cfg *config.Config) error {
	log := gologger.WithComponent("task_push")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("error getting *sql.DB: %w", err)
	}
	defer sqlDB.Close()

	// Get repositories and services
	taskRepo := repositories.NewTaskRepository(db)
	runnerRepo := repositories.NewRunnerRepository(db)
	runnerService := services.NewRunnerService(runnerRepo)

	// Get the task
	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		return fmt.Errorf("invalid task ID: %w", err)
	}

	task, err := taskRepo.Get(ctx, taskUUID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	// Get the runner
	runner, err := runnerService.GetRunner(ctx, runnerID)
	if err != nil {
		return fmt.Errorf("failed to get runner: %w", err)
	}

	if runner.Webhook == "" {
		return fmt.Errorf("runner has no webhook URL")
	}

	// Prepare the webhook message
	type WebhookMessage struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}

	tasks := []*models.Task{task}
	tasksJSON, err := json.Marshal(tasks)
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	message := WebhookMessage{
		Type:    "available_tasks",
		Payload: tasksJSON,
	}

	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Send the webhook
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(runner.Webhook, "application/json", bytes.NewBuffer(messageJSON))
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Info().
		Str("task_id", taskID).
		Str("runner_id", runnerID).
		Str("webhook", runner.Webhook).
		Msg("Successfully pushed task to runner")

	return nil
}
