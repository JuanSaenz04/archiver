package main

import (
	"context"
	"log"
	"os"
	"os/signal"
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

	queue.StartWorker(ctx, rdb, crawler.Run)
}