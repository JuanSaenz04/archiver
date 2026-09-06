package queue

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/redis/go-redis/v9"
)

const (
	streamName          = "crawl_stream"
	groupName           = "worker_group"
	retryInterval       = 5 * time.Second
	reclaimInterval     = 10 * time.Second
	finalizationTimeout = 30 * time.Second
)

type Processor func(ctx context.Context, jobID string, archive models.Archive, options models.CrawlOptions) error

func ensureStreamAndGroup(ctx context.Context, rdb *redis.Client) error {
	err := rdb.XGroupCreateMkStream(ctx, streamName, groupName, "0").Err()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		return err
	}
	return nil
}

// StartWorker recovers abandoned deliveries between jobs. jobTimeout bounds the
// entire processor; the additional reclaim margin allows shutdown and finalization.
func StartWorker(ctx context.Context, rdb *redis.Client, consumerName string, jobTimeout time.Duration, process Processor) error {
	if jobTimeout <= 0 {
		return fmt.Errorf("job timeout must be positive")
	}
	cursor := "0-0"
	var nextClaim time.Time
	for ctx.Err() == nil {
		if err := ensureStreamAndGroup(ctx, rdb); err != nil {
			slog.Error("create consumer group", "error", err)
			if !waitRetry(ctx, retryInterval) {
				break
			}
			continue
		}
		for ctx.Err() == nil {
			var messages []redis.XMessage
			var err error
			if !time.Now().Before(nextClaim) {
				var nextCursor string
				messages, nextCursor, err = rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
					Stream: streamName, Group: groupName, Consumer: consumerName,
					MinIdle: jobTimeout + 2*time.Minute, Start: cursor, Count: 1,
				}).Result()
				if err == nil {
					cursor = nextCursor
				}
				nextClaim = time.Now().Add(reclaimInterval)
			}
			if err == nil && len(messages) == 0 {
				var streams []redis.XStream
				streams, err = rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
					Group: groupName, Consumer: consumerName, Streams: []string{streamName, ">"}, Count: 1, Block: time.Second,
				}).Result()
				for _, stream := range streams {
					messages = append(messages, stream.Messages...)
				}
			}
			if errors.Is(err, redis.Nil) {
				continue
			}
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				slog.Error("read crawl stream", "consumer", consumerName, "error", err)
				if redis.HasErrorPrefix(err, "NOGROUP") {
					cursor = "0-0"
					nextClaim = time.Time{}
					break
				}
				if !waitRetry(ctx, retryInterval) {
					return nil
				}
				continue
			}
			for _, message := range messages {
				if err := handleMessage(ctx, rdb, message, jobTimeout, process); err != nil && ctx.Err() == nil {
					slog.Error("job left pending for recovery", "message_id", message.ID, "error", err)
				}
			}
		}
	}
	return nil
}

func handleMessage(ctx context.Context, rdb *redis.Client, message redis.XMessage, jobTimeout time.Duration, process Processor) error {
	jobID, _ := message.Values["job_id"].(string)
	if _, err := uuid.Parse(jobID); err != nil {
		slog.Warn("invalid job ID", "message_id", message.ID)
		return finalize(ctx, rdb, message.ID, "", "", "")
	}
	// A previous delivery may have saved its result but lost the acknowledgment.
	status, err := rdb.HGet(ctx, "job:"+jobID, "status").Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if status == "completed" || status == "failed" {
		return finalize(ctx, rdb, message.ID, "", "", "")
	}
	payload, _ := message.Values["payload"].(string)
	var msg CrawlMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil || msg.JobID != jobID || msg.Archive.ID.String() != jobID || msg.Archive.SourceURL == "" {
		return finalize(ctx, rdb, message.ID, jobID, "failed", "invalid crawl message")
	}
	if err := rdb.HSet(ctx, "job:"+jobID, "status", "running", "error", "").Err(); err != nil {
		return err
	}
	jobCtx, cancel := context.WithTimeout(ctx, jobTimeout)
	err = process(jobCtx, jobID, msg.Archive, msg.Options)
	jobErr := jobCtx.Err()
	cancel()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err == nil {
		err = jobErr
	}
	status, detail := "completed", ""
	if err != nil {
		status, detail = "failed", err.Error()
		slog.Error("crawl job failed", "job_id", jobID, "error", err)
	} else {
		slog.Info("crawl job completed", "job_id", jobID)
	}
	return finalize(ctx, rdb, message.ID, jobID, status, detail)
}

func finalize(ctx context.Context, rdb *redis.Client, messageID, jobID, status, detail string) error {
	ctx, cancel := context.WithTimeout(ctx, finalizationTimeout)
	defer cancel()
	if jobID != "" {
		if err := retryRedis(ctx, func() error {
			return rdb.HSet(ctx, "job:"+jobID, "status", status, "error", detail).Err()
		}); err != nil {
			return err
		}
	}
	if err := retryRedis(ctx, func() error { return rdb.XAck(ctx, streamName, groupName, messageID).Err() }); err != nil {
		return err
	}
	// This stream has one consumer group. Delete only acknowledged entries;
	// trimming by length could discard jobs that have not finished yet.
	if err := retryRedis(ctx, func() error { return rdb.XDel(ctx, streamName, messageID).Err() }); err != nil {
		slog.Warn("acknowledged stream entry could not be deleted", "message_id", messageID, "error", err)
	}
	return nil
}

func retryRedis(ctx context.Context, operation func() error) error {
	for ctx.Err() == nil {
		if err := operation(); err == nil {
			return nil
		} else {
			slog.Warn("retrying job finalization", "error", err)
		}
		if !waitRetry(ctx, retryInterval) {
			break
		}
	}
	return ctx.Err()
}

func waitRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
