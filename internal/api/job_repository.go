package api

import (
	"context"
	"fmt"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type JobRepository struct {
	rdb *redis.Client
}

func NewJobRepository(rdb *redis.Client) *JobRepository {
	return &JobRepository{rdb: rdb}
}

func (repo *JobRepository) GetAllJobs(ctx context.Context) ([]models.Job, error) {
	jobIDs, err := repo.rdb.SMembers(ctx, "jobs:index").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get job IDs: %w", err)
	}

	if len(jobIDs) == 0 {
		return []models.Job{}, nil
	}

	// Use a pipeline to fetch all jobs efficiently
	pipe := repo.rdb.Pipeline()
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
		if err != nil {
			// For now, we skip malformed entries
			continue
		}

		if len(result) == 0 {
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
