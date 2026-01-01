package queue

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func EnqueueCrawl(ctx context.Context, rdb *redis.Client, targetURL string) (*uuid.UUID, error) {
	jobID := uuid.New()

	err := rdb.HSet(ctx, "job:"+jobID.String(), map[string]interface{}{
		"url":        targetURL,
		"status":     "pending",
		"created_at": time.Now().Format(time.RFC3339),
	}).Err()
	if err != nil {
		return nil, err
	}

	err = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "crawl_stream",
		Values: map[string]interface{}{
			"job_id":     jobID.String(),
			"target_url": targetURL,
		},
	}).Err()

	return &jobID, err
}
