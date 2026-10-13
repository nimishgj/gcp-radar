package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nimishgj/gcp-radar/internal/collector"
	"github.com/nimishgj/gcp-radar/internal/config"
	"github.com/nimishgj/gcp-radar/internal/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	setupLogging(cfg.Logging)

	log.Info().
		Str("version", version).
		Str("project_id", cfg.GCP.ProjectID).
		Msg("Starting GCP Radar")

	if cfg.GCP.ProjectID == "" {
		log.Warn().Msg("gcp.project_id is empty; set it in config so collectors have a scope")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	orchestrator, err := collector.NewOrchestrator(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize collectors")
	}
	go orchestrator.Start(ctx)

	srv := server.New(&cfg.Server)
	go func() {
		if err := srv.Start(); err != nil {
			log.Error().Err(err).Msg("HTTP server error")
			cancel()
		}
	}()

	<-sigChan
	log.Info().Msg("Shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during server shutdown")
	}

	cancel()
	log.Info().Msg("GCP Radar stopped")
}

func setupLogging(cfg config.LoggingConfig) {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	if cfg.Format == "text" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	} else {
		log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	}
}
