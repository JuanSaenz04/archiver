package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/JuanSaenz04/archiver/internal/crawler"
	"github.com/JuanSaenz04/archiver/internal/queue"
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

	crawler := crawler.NewCrawler(timeoutSeconds)

	queue.StartWorker(ctx, rdb, crawler.Run)
}
