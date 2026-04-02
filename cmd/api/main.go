package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/JuanSaenz04/archiver/internal/api"
	"github.com/JuanSaenz04/archiver/internal/store"
	"github.com/labstack/echo/v5"
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
		slog.Error("api server failed", "error", err)
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

	if err := rdb.XGroupCreateMkStream(ctx, "crawl_stream", "worker_group", "$").Err(); err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		return fmt.Errorf("ensure redis stream/group: %w", err)
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

	if err := archiveStore.SyncFromDisk(ctx, archivesDir); err != nil {
		return fmt.Errorf("sync sqlite database from disk: %w", err)
	}

	handler := api.NewHandler(rdb, archivesDir, archiveStore)

	e := echo.New()

	handler.SetRoutes(e)

	sc := echo.StartConfig{
		Address:         ":1080",
		GracefulTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("starting api server", "addr", ":1080", "archives_dir", archivesDir, "sqlite_dir", sqliteDir)
		if err := sc.Start(ctx, e); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("start api server: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("server stopped gracefully")
	return nil
}
