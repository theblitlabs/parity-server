package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/app"
	"github.com/theblitlabs/parity-server/internal/core/config"
)

// RunServer starts the parity server
func RunServer() {
	log := gologger.Get()

	// Load application configuration
	cfg, err := config.GetConfigManager().GetConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Create cancellable context for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()

	// Create channel to receive OS signals
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Create the server builder which handles all initialization
	serverBuilder := app.NewServerBuilder(cfg)

	// Initialize the services, repositories and other components
	server, err := serverBuilder.
		InitDatabase().
		InitRepositories().
		InitServices().
		InitWallet().
		InitTaskMonitoring().
		InitHeartbeatService().
		InitRouter().
		Build()

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize server")
	}

	// Start the server in a separate goroutine
	go func() {
		serverAddr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
		log.Info().Str("address", serverAddr).Msg("Server starting")

		if err := server.HttpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	// Wait for shutdown signal
	<-stopChan
	log.Info().Msg("Shutdown signal received, gracefully shutting down...")

	shutdownTimeoutCtx, cancel := context.WithTimeout(shutdownCtx, 20*time.Second)
	defer cancel()

	signal.Stop(stopChan)

	// Setup additional signal handler for force shutdown during graceful shutdown
	forceStopChan := make(chan os.Signal, 1)
	signal.Notify(forceStopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Force shutdown if additional signals are received during graceful shutdown
	go func() {
		<-forceStopChan
		log.Warn().Msg("Forced shutdown requested, terminating immediately")
		os.Exit(1)
	}()

	// Trigger graceful shutdown with timeout
	server.Shutdown(shutdownTimeoutCtx)

	// Normal shutdown completed
	log.Info().Msg("Shutdown completed successfully, exiting")

	// Exit with success code
	os.Exit(0)
}
