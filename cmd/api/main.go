package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/JuanSaenz04/archiver/internal/api"
	"github.com/JuanSaenz04/archiver/internal/store"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

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

	var opts *redis.Options
	var err error

	redisURL := os.Getenv("REDIS_URL")
	opts, err = redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Invalid REDIS_URL: %v", err)
	}

	rdb := redis.NewClient(opts)

	rdb.XGroupCreateMkStream(ctx, "crawl_stream", "worker_group", "$")

	archivesDir := os.Getenv("ARCHIVES_DIR")
	if archivesDir == "" {
		log.Fatalln("Environment variable ARCHIVES_DIR not set")
	}

	sqliteDir := os.Getenv("SQLITE_DIR")
	if sqliteDir == "" {
		sqliteDir = archivesDir
	}

	archiveStore, err := store.Open(filepath.Join(sqliteDir, "archive.db"))
	if err != nil {
		log.Fatalln("Failed to open sqlite database")
	}
	defer archiveStore.Close()

	if err := archiveStore.RunMigrations(); err != nil {
		log.Fatalf("Failed to run sqlite migrations: %v", err)
	}

	handler := api.NewHandler(rdb, archivesDir, archiveStore)

	e := echo.New()

	handler.SetRoutes(e)

	go func() {
		slog.Info("starting api server", "addr", ":1080", "archives_dir", archivesDir, "sqlite_dir", sqliteDir)
		if err := e.Start(":1080"); err != nil {
			e.Logger.Info("shutting down server")
		}
	}()

	<-ctx.Done()
	cancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		e.Logger.Fatal(err)
	}

	slog.Info("server stopped gracefully")
}
