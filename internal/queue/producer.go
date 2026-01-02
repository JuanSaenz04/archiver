package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func EnqueueCrawl(ctx context.Context, rdb *redis.Client, targetURL string, options models.CrawlOptions) (*uuid.UUID, error) {
	jobID := uuid.New()

	err := rdb.HSet(ctx, "job:"+jobID.String(), map[string]interface{}{
		"url":        targetURL,
		"status":     "pending",
		"created_at": time.Now().Format(time.RFC3339),
	}).Err()
	if err != nil {
		return nil, err
	}

	optsBytes, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}

	err = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "crawl_stream",
		Values: map[string]interface{}{
			"job_id":     jobID.String(),
			"target_url": targetURL,
			"options":    string(optsBytes),
		},
	}).Err()

	return &jobID, err
}
