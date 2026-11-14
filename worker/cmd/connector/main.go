package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pranavb1234/neonCDC/worker/internal/config"
	"github.com/pranavb1234/neonCDC/worker/internal/logging"
	"github.com/pranavb1234/neonCDC/worker/internal/postgres"
	rediscache "github.com/pranavb1234/neonCDC/worker/internal/redis"
	"github.com/pranavb1234/neonCDC/worker/internal/replication"
)

func main() {
	logger := logging.New()
	if err := run(logger); err != nil {
		logger.Error("connector stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := postgres.Connect(connectCtx, cfg.SourceDBURL)
	if err != nil {
		return err
	}
	defer db.Close(context.Background())

	redisClient, err := rediscache.Connect(connectCtx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	logger.Info("connector connected", "postgres_host", cfg.PostgresHost, "redis_host", cfg.RedisHost)
	if err := replication.Run(ctx, db, replication.Config{
		SourceDBURL:     cfg.SourceDBURL,
		SlotName:        cfg.ReplicationSlot,
		PublicationName: cfg.PublicationName,
	}, logger); err != nil {
		return err
	}

	logger.Info("connector shutting down")
	return nil
}
