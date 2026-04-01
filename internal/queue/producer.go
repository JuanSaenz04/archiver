package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func EnqueueCrawl(ctx context.Context, rdb *redis.Client, request models.CrawlRequest) (*uuid.UUID, error) {
	jobID := uuid.New()

	err := rdb.HSet(ctx, "job:"+jobID.String(), map[string]interface{}{
		"url":        request.URL,
		"status":     "pending",
		"created_at": time.Now().Format(time.RFC3339),
	}).Err()
	if err != nil {
		return nil, err
	}

	// Index the job so we can list them later
	if err := rdb.SAdd(ctx, "jobs:index", jobID.String()).Err(); err != nil {
		return nil, err
	}

	archive := models.Archive{
		ID:          jobID,
		Name:        request.Name,
		Description: request.Description,
		SourceURL:   request.URL,
		Tags:        request.Tags,
	}

	msg := CrawlMessage{
		JobID:   jobID.String(),
		Options: request.Options,
		Archive: archive,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	err = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "crawl_stream",
		Values: map[string]interface{}{
			"job_id":  jobID.String(),
			"payload": string(msgBytes),
		},
	}).Err()

	return &jobID, err
}
