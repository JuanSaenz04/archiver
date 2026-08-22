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
	"syscall"

	"github.com/JuanSaenz04/archiver/internal/api"
	"github.com/JuanSaenz04/archiver/internal/store"
	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api server failed", "error", err)
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

	if err := rdb.XGroupCreateMkStream(ctx, "crawl_stream", "worker_group", "$").Err(); err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		return fmt.Errorf("ensure redis stream/group: %w", err)
	}

	archivesDir := cfg.ArchivesDir
	sqliteDir := cfg.SQLiteDir

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
	routeConfig := api.RouteConfig{
		AppPublicURL:    cfg.AppPublicURL,
		ReplayPublicURL: cfg.ReplayPublicURL,
	}

	mainServer := echo.New()
	replayServer := echo.New()

	mainServer.IPExtractor = api.GetIPExtractor(cfg.TrustedProxies)
	replayServer.IPExtractor = api.GetIPExtractor(cfg.TrustedProxies)

	handler.SetMainRoutes(mainServer, routeConfig)
	handler.SetReplayRoutes(replayServer, routeConfig)

	mainConfig := echo.StartConfig{
		Address:         cfg.AppAddress,
		GracefulTimeout: cfg.GracefulTimeout,
	}
	replayConfig := echo.StartConfig{
		Address:         cfg.ReplayAddress,
		GracefulTimeout: cfg.GracefulTimeout,
	}

	errCh := make(chan error, 2)
	go func() {
		slog.Info("starting api server", "addr", cfg.AppAddress, "public_url", cfg.AppPublicURL, "archives_dir", archivesDir, "sqlite_dir", sqliteDir)
		if err := mainConfig.Start(ctx, mainServer); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("start api server: %w", err)
		}
	}()
	go func() {
		slog.Info("starting replay server", "addr", cfg.ReplayAddress, "public_url", cfg.ReplayPublicURL)
		if err := replayConfig.Start(ctx, replayServer); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("start replay server: %w", err)
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

func configureLogger(level slog.Level) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}
