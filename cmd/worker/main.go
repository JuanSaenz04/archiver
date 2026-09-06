package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JuanSaenz04/archiver/internal/crawler"
	"github.com/JuanSaenz04/archiver/internal/queue"
	"github.com/JuanSaenz04/archiver/internal/store"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configureLogger(slog.LevelInfo)

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	configureLogger(cfg.LogLevel)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rdb := redis.NewClient(cfg.RedisOptions)

	defer func() {
		if err := rdb.Close(); err != nil {
			slog.Warn("failed to close redis client", "error", err)
		}
	}()

	archivesDir := cfg.ArchivesDir
	sqliteDir := cfg.SQLiteDir

	archiveStore, err := store.Open(databasePath(cfg))
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

	crawlerConfig := crawler.Config{
		TimeoutInSeconds: cfg.CrawlerTimeout,
		AnubisMode:       cfg.AnubisMode,
		ArchivesDir:      cfg.ArchivesDir,
	}
	crawler := crawler.NewCrawler(crawlerConfig, archiveStore)

	slog.Info("starting worker", "timeout_seconds", cfg.CrawlerTimeout, "job_timeout_seconds", cfg.JobTimeout, "anubis_mode", cfg.AnubisMode, "archives_dir", archivesDir, "sqlite_dir", sqliteDir)

	if err := queue.StartWorker(ctx, rdb, cfg.ConsumerName, time.Duration(cfg.JobTimeout)*time.Second, crawler.Run); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}

	slog.Info("worker stopped gracefully")
	return nil
}

func configureLogger(level slog.Level) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}
