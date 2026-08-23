package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"uuid"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/redis/go-redis/v9"
)

type JobService struct {
	rdb *redis.Client
}

func NewJobService(rdb *redis.Client) *JobService {
	return &JobService{rdb: rdb}
}

func (service *JobService) EnqueueCrawl(ctx context.Context, request models.CrawlRequest) (*uuid.UUID, error) {
	jobID := uuid.New()
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
		return nil, fmt.Errorf("marshal crawl message: %w", err)
	}

	_, err = service.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, "job:"+jobID.String(), map[string]interface{}{
			"url":        request.URL,
			"status":     "pending",
			"created_at": time.Now().Format(time.RFC3339),
		})
		pipe.SAdd(ctx, "jobs:index", jobID.String())
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: "crawl_stream",
			Values: map[string]interface{}{
				"job_id":  jobID.String(),
				"payload": string(msgBytes),
			},
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("enqueue crawl job: %w", err)
	}

	return &jobID, nil
}

func (service *JobService) GetAllJobs(ctx context.Context) ([]models.Job, error) {
	jobIDs, err := service.rdb.SMembers(ctx, "jobs:index").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get job IDs: %w", err)
	}

	if len(jobIDs) == 0 {
		return []models.Job{}, nil
	}

	pipe := service.rdb.Pipeline()
	cmds := make(map[string]*redis.MapStringStringCmd)

	for _, id := range jobIDs {
		cmds[id] = pipe.HGetAll(ctx, "job:"+id)
	}

	_, err = pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to execute pipeline: %w", err)
	}

	var jobs []models.Job
	for id, cmd := range cmds {
		result, err := cmd.Result()
		if err != nil || len(result) == 0 {
			continue
		}

		uid, err := uuid.Parse(id)
		if err != nil {
			continue
		}

		jobs = append(jobs, models.Job{
			ID:        uid,
			URL:       result["url"],
			Status:    result["status"],
			CreatedAt: result["created_at"],
		})
	}

	return jobs, nil
}
