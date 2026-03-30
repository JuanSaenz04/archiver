package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/JuanSaenz04/archiver/internal/crawler"
	"github.com/JuanSaenz04/archiver/internal/queue"
	"github.com/JuanSaenz04/archiver/internal/store"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	redisURL := os.Getenv("REDIS_URL")
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Invalid REDIS_URL: %v", err)
	}

	rdb := redis.NewClient(opts)

	// Close redis client on exit
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Printf("Error closing redis client: %v", err)
		}
	}()

	timeoutEnv := os.Getenv("CRAWLER_TIMEOUT")

	timeoutSeconds, err := strconv.Atoi(timeoutEnv)

	if err != nil {
		timeoutSeconds = 30
	}

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

	crawler := crawler.NewCrawler(timeoutSeconds, archiveStore)

	queue.StartWorker(ctx, rdb, crawler.Run)
}
