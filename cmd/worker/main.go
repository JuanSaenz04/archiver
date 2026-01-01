package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var opts *redis.Options
	var err error

	redisURL := os.Getenv("REDIS_URL")
	opts, err = redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Invalid REDIS_URL: %v", err)
	}

	rdb := redis.NewClient(opts)

	rdb.XGroupCreateMkStream(ctx, "crawl_stream", "worker_group", "$")

	log.Println("Worker started, waiting for jobs...")

	for {
		streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    "worker_group",
			Consumer: "worker-1",
			Streams:  []string{"crawl_stream", ">"},
			Count:    1,
			Block:    1 * time.Second,
		}).Result()

		if err != nil {
			if err == context.Canceled {
				log.Println("Worker shutting down...")
				break
			}
			if err == redis.Nil {
				continue
			}
			log.Printf("Error reading stream: %v", err)
			continue
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				jobID := message.Values["job_id"].(string)

				// Use background context for processing to ensure we can 
				// update status and ack even if shutdown signal is received.
				processCtx := context.Background()

				rdb.HSet(processCtx, "job:"+jobID, "status", "running")

				err := runCrawl(jobID)

				if err != nil {
					rdb.HSet(processCtx, "job:"+jobID, "status", "failed", "error", err.Error())
				} else {
					rdb.HSet(processCtx, "job:"+jobID, "status", "completed")
				}

				rdb.XAck(processCtx, "crawl_stream", "worker_group", message.ID)
			}
		}
	}

	log.Println("Worker stopped gracefully")
}

func runCrawl(jobID string) error {
	fmt.Printf("Recieved job with ID %s\n", jobID)

	return nil
}