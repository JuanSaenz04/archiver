package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
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
	appPublicURL, err := publicOriginFromEnv("APP_PUBLIC_URL")
	if err != nil {
		return err
	}
	replayPublicURL, err := publicOriginFromEnv("REPLAY_PUBLIC_URL")
	if err != nil {
		return err
	}
	if appPublicURL == replayPublicURL {
		return errors.New("APP_PUBLIC_URL and REPLAY_PUBLIC_URL must use different origins")
	}
	routeConfig := api.RouteConfig{
		AppPublicURL:    appPublicURL,
		ReplayPublicURL: replayPublicURL,
	}

	mainServer := echo.New()
	replayServer := echo.New()

	mainServer.IPExtractor = api.GetIPExtractorFromEnv()
	replayServer.IPExtractor = api.GetIPExtractorFromEnv()

	handler.SetMainRoutes(mainServer, routeConfig)
	handler.SetReplayRoutes(replayServer, routeConfig)

	mainConfig := echo.StartConfig{
		Address:         ":1080",
		GracefulTimeout: 10 * time.Second,
	}
	replayConfig := echo.StartConfig{
		Address:         ":1081",
		GracefulTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 2)
	go func() {
		slog.Info("starting api server", "addr", ":1080", "public_url", appPublicURL, "archives_dir", archivesDir, "sqlite_dir", sqliteDir)
		if err := mainConfig.Start(ctx, mainServer); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("start api server: %w", err)
		}
	}()
	go func() {
		slog.Info("starting replay server", "addr", ":1081", "public_url", replayPublicURL)
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

func publicOriginFromEnv(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("environment variable %s not set", name)
	}

	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("environment variable %s must be an HTTP(S) origin without a path", name)
	}

	hostname := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		port = ""
	}
	host := hostname
	if strings.Contains(hostname, ":") || port != "" {
		host = net.JoinHostPort(hostname, port)
		if port == "" {
			host = "[" + hostname + "]"
		}
	}

	return parsed.Scheme + "://" + host, nil
}
