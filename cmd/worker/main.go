package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/JuanSaenz04/archiver/internal/crawler"
	"github.com/JuanSaenz04/archiver/internal/queue"
	"github.com/JuanSaenz04/archiver/internal/store"
	"github.com/redis/go-redis/v9"
)

func main() {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	if err := run(); err != nil {
		slog.Error("worker failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	opts, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		return fmt.Errorf("invalid REDIS_URL: %w", err)
	}

	rdb := redis.NewClient(opts)

	defer func() {
		if err := rdb.Close(); err != nil {
			slog.Warn("failed to close redis client", "error", err)
		}
	}()

	timeoutEnv := os.Getenv("CRAWLER_TIMEOUT")

	timeoutSeconds, err := strconv.Atoi(timeoutEnv)

	if err != nil {
		timeoutSeconds = 30
		slog.Debug("invalid CRAWLER_TIMEOUT, using default", "value", timeoutEnv, "default", timeoutSeconds)
	}

	archivesDir := os.Getenv("ARCHIVES_DIR")
	if archivesDir == "" {
		return errors.New("environment variable ARCHIVES_DIR not set")
	}

	sqliteDir := os.Getenv("SQLITE_DIR")
	if sqliteDir == "" {
		sqliteDir = archivesDir
	}

	archiveStore, err := store.Open(filepath.Join(sqliteDir, "archive.db"))
	if err != nil {
		return fmt.Errorf("open sqlite database: %w", err)
	}
	defer func() {
		if err := archiveStore.Close(); err != nil {
			slog.Warn("failed to close sqlite database", "error", err)
		}
	}()

	if err := archiveStore.RunMigrations(); err != nil {
		return fmt.Errorf("run sqlite migrations: %w", err)
	}

	crawler := crawler.NewCrawler(timeoutSeconds, archiveStore)

	slog.Info("starting worker", "timeout_seconds", timeoutSeconds, "archives_dir", archivesDir, "sqlite_dir", sqliteDir)

	if err := queue.StartWorker(ctx, rdb, crawler.Run); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}

	slog.Info("worker stopped gracefully")
	return nil
}
