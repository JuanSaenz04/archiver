package queue

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/redis/go-redis/v9"
)

const (
	streamName    = "crawl_stream"
	groupName     = "worker_group"
	consumerName  = "worker-1" // TODO: Make this unique if running multiple workers
	retryInterval = 5 * time.Second
)

// Processor is a function that processes a job.
type Processor func(ctx context.Context, jobID, targetURL string, options models.CrawlOptions) error

func ensureStreamAndGroup(ctx context.Context, rdb *redis.Client) error {
	err := rdb.XGroupCreateMkStream(ctx, streamName, groupName, "$").Err()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		return err
	}
	return nil
}

// StartWorker starts the worker loop to consume jobs from Redis.
// On any error it retries after retryInterval indefinitely.
func StartWorker(ctx context.Context, rdb *redis.Client, process Processor) {
	if err := ensureStreamAndGroup(ctx, rdb); err != nil {
		log.Fatalf("Failed to create consumer group on startup: %v", err)
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
			Group:    groupName,
			Consumer: consumerName,
			Streams:  []string{streamName, ">"},
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

			// Stream or consumer group is gone. Try to re-create it.
			if redis.HasErrorPrefix(err, "NOGROUP") {
				log.Printf("Stream or consumer group missing, attempting to re-create: %v", err)
				if recreateErr := ensureStreamAndGroup(ctx, rdb); recreateErr != nil {
					log.Printf("Failed to re-create stream/group: %v", recreateErr)
				} else {
					log.Println("Stream and consumer group re-created successfully")
					continue
				}
			} else {
				log.Printf("Error reading stream: %v", err)
			}

			select {
			case <-ctx.Done():
				log.Println("Worker shutting down...")
				return
			case <-time.After(retryInterval):
			}
			continue
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				jobID, ok := message.Values["job_id"].(string)
				if !ok {
					log.Printf("Error: job_id not found or not a string in message %s", message.ID)
					rdb.XAck(ctx, streamName, groupName, message.ID)
					continue
				}
				targetURL, ok := message.Values["target_url"].(string)
				if !ok {
					log.Printf("Error: target_url not found or not a string in message %s", message.ID)
					rdb.XAck(ctx, streamName, groupName, message.ID)
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

				rdb.XAck(ctx, streamName, groupName, message.ID)
			}
		}
	}
}
