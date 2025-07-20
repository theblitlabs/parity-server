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

func RunServer() error {
	log := gologger.Get()

	cfg, err := config.GetConfigManager().GetConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load configuration")
		return err
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
		log.Error().Err(err).Msg("Failed to initialize server")
		return err
	}

	serverErrChan := make(chan error, 1)
	go func() {
		serverAddr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
		log.Info().Str("address", serverAddr).Msg("Server starting")

		if err := server.HttpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Server failed to start")
			serverErrChan <- err
		}
	}()

	select {
	case <-stopChan:
		log.Info().Msg("Shutdown signal received, gracefully shutting down...")
	case err := <-serverErrChan:
		log.Error().Err(err).Msg("Server error occurred")
		return err
	}

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

	return nil
}
