package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
type Processor func(ctx context.Context, jobID string, archive models.Archive, options models.CrawlOptions) error

func ensureStreamAndGroup(ctx context.Context, rdb *redis.Client) error {
	err := rdb.XGroupCreateMkStream(ctx, streamName, groupName, "$").Err()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		return err
	}
	return nil
}

// StartWorker starts the worker loop to consume jobs from Redis.
// On any error it retries after retryInterval indefinitely.
func StartWorker(ctx context.Context, rdb *redis.Client, process Processor) error {
	if err := ensureStreamAndGroup(ctx, rdb); err != nil {
		return fmt.Errorf("create consumer group on startup: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
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
				return nil
			}
			if err == redis.Nil {
				continue
			}

			// Stream or consumer group is gone. Try to re-create it.
			if redis.HasErrorPrefix(err, "NOGROUP") {
				slog.Warn("redis stream or consumer group missing, attempting to recreate", "stream", streamName, "group", groupName, "error", err)
				if recreateErr := ensureStreamAndGroup(ctx, rdb); recreateErr != nil {
					slog.Error("failed to recreate redis stream or consumer group", "stream", streamName, "group", groupName, "error", recreateErr)
				} else {
					slog.Info("redis stream and consumer group recreated", "stream", streamName, "group", groupName)
					continue
				}
			} else {
				slog.Error("failed to read redis stream", "stream", streamName, "group", groupName, "consumer", consumerName, "error", err)
			}

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(retryInterval):
			}
			continue
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				jobID, ok := message.Values["job_id"].(string)
				if !ok {
					slog.Warn("redis message missing valid job_id", "message_id", message.ID)
					if err := rdb.XAck(ctx, streamName, groupName, message.ID).Err(); err != nil {
						slog.Error("failed to acknowledge malformed redis message", "message_id", message.ID, "error", err)
					}
					continue
				}
				payloadMsg, ok := message.Values["payload"].(string)
				if !ok {
					slog.Warn("redis message missing valid payload", "job_id", jobID, "message_id", message.ID)
					if err := rdb.XAck(ctx, streamName, groupName, message.ID).Err(); err != nil {
						slog.Error("failed to acknowledge malformed redis message", "job_id", jobID, "message_id", message.ID, "error", err)
					}
					continue
				}
				var msg CrawlMessage
				if err := json.Unmarshal([]byte(payloadMsg), &msg); err != nil {
					slog.Warn("failed to unmarshal crawl message", "job_id", jobID, "message_id", message.ID, "error", err)
					continue
				}

				slog.Info("processing crawl job", "job_id", jobID, "url", msg.Archive.SourceURL)

				if err := rdb.HSet(ctx, "job:"+jobID, "status", "running").Err(); err != nil {
					slog.Warn("failed to update job status", "job_id", jobID, "status", "running", "error", err)
				}

				err := process(ctx, jobID, msg.Archive, msg.Options)

				if err != nil {
					slog.Error("crawl job failed", "job_id", jobID, "url", msg.Archive.SourceURL, "error", err)
					if statusErr := rdb.HSet(ctx, "job:"+jobID, "status", "failed", "error", err.Error()).Err(); statusErr != nil {
						slog.Warn("failed to update job status", "job_id", jobID, "status", "failed", "error", statusErr)
					}
				} else {
					slog.Info("crawl job completed", "job_id", jobID, "url", msg.Archive.SourceURL)
					if statusErr := rdb.HSet(ctx, "job:"+jobID, "status", "completed").Err(); statusErr != nil {
						slog.Warn("failed to update job status", "job_id", jobID, "status", "completed", "error", statusErr)
					}
				}

				if err := rdb.XAck(ctx, streamName, groupName, message.ID).Err(); err != nil {
					slog.Error("failed to acknowledge redis message", "job_id", jobID, "message_id", message.ID, "error", err)
				}
			}
		}
	}
}
