package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

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
			Block:    0,
		}).Result()

		if err != nil {
			log.Fatal(err)
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				jobID := message.Values["job_id"].(string)

				rdb.HSet(ctx, "job:"+jobID, "status", "running")

				err := runCrawl(jobID)

				if err != nil {
					rdb.HSet(ctx, "job:"+jobID, "status", "failed", "error", err.Error())
				} else {
					rdb.HSet(ctx, "job:"+jobID, "status", "completed")
				}

				rdb.XAck(ctx, "crawl_stream", "worker_group", message.ID)
			}
		}
	}

}

func runCrawl(jobID string) error {
	fmt.Printf("Recieved job with ID %s\n", jobID)

	return nil
}
