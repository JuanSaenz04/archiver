package queue

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/redis/go-redis/v9"
)

// Processor is a function that processes a job.
type Processor func(ctx context.Context, jobID, targetURL string, options models.CrawlOptions) error

// StartWorker starts the worker loop to consume jobs from Redis.
func StartWorker(ctx context.Context, rdb *redis.Client, process Processor) {
	err := rdb.XGroupCreateMkStream(ctx, "crawl_stream", "worker_group", "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		log.Printf("Warning: Failed to create consumer group: %v", err)
	}

	log.Println("Worker started, waiting for jobs...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker shutting down...")
			return
		default:
		}

		streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    "worker_group",
			Consumer: "worker-1", // TODO: Make this unique if running multiple workers
			Streams:  []string{"crawl_stream", ">"},
			Count:    1,
			Block:    1 * time.Second,
		}).Result()

		if err != nil {
			if err == context.Canceled {
				log.Println("Worker shutting down...")
				return
			}
			if err == redis.Nil {
				continue
			}
			log.Printf("Error reading stream: %v", err)
			continue
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				jobID, ok := message.Values["job_id"].(string)
				if !ok {
					log.Printf("Error: job_id not found or not a string in message %s", message.ID)
					rdb.XAck(ctx, "crawl_stream", "worker_group", message.ID)
					continue
				}
				targetURL, ok := message.Values["target_url"].(string)
				if !ok {
					log.Printf("Error: target_url not found or not a string in message %s", message.ID)
					rdb.XAck(ctx, "crawl_stream", "worker_group", message.ID)
					continue
				}

				var opts models.CrawlOptions
				if optsStr, ok := message.Values["options"].(string); ok {
					if err := json.Unmarshal([]byte(optsStr), &opts); err != nil {
						log.Printf("Error unmarshaling options for job %s: %v", jobID, err)
						// Proceed with default/empty options or fail? Proceeding for now.
					}
				}

				rdb.HSet(ctx, "job:"+jobID, "status", "running")

				err := process(ctx, jobID, targetURL, opts)

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
