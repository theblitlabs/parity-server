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

func RunServer() {
	log := gologger.Get()

	cfg, err := config.GetConfigManager().GetConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	serverBuilder := app.NewServerBuilder(cfg)

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

	go func() {
		serverAddr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
		log.Info().Str("address", serverAddr).Msg("Server starting")

		if err := server.HttpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	<-stopChan
	log.Info().Msg("Shutdown signal received, gracefully shutting down...")

	shutdownTimeoutCtx, cancel := context.WithTimeout(shutdownCtx, 20*time.Second)
	defer cancel()

	signal.Stop(stopChan)

	forceStopChan := make(chan os.Signal, 1)
	signal.Notify(forceStopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-forceStopChan
		log.Warn().Msg("Forced shutdown requested, terminating immediately")
		os.Exit(1)
	}()

	server.Shutdown(shutdownTimeoutCtx)

	log.Info().Msg("Shutdown completed successfully, exiting")

	os.Exit(0)
}
