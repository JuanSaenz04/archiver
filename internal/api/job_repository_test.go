package api

import (
	"context"
	"testing"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	t.Cleanup(func() {
		rdb.Close()
		mr.Close()
	})

	return mr, rdb
}

func TestJobRepository_GetAllJobs_Empty(t *testing.T) {
	_, rdb := newTestRedis(t)
	repo := NewJobRepository(rdb)

	ctx := context.Background()
	jobs, err := repo.GetAllJobs(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, jobs)
	assert.Empty(t, jobs, "Should return an empty slice when there are no jobs in the index")
}

func TestJobRepository_GetAllJobs_Success(t *testing.T) {
	mr, rdb := newTestRedis(t)
	repo := NewJobRepository(rdb)

	ctx := context.Background()

	// Setup 2 valid jobs in miniredis
	jobID1 := uuid.New()
	jobID2 := uuid.New()

	// Seed jobs:index set
	mr.SAdd("jobs:index", jobID1.String())
	mr.SAdd("jobs:index", jobID2.String())

	// Seed hashes for each job
	mr.HSet("job:"+jobID1.String(), "url", "https://example.com/1", "status", "pending", "created_at", "2026-06-19T21:00:00Z")
	mr.HSet("job:"+jobID2.String(), "url", "https://example.com/2", "status", "completed", "created_at", "2026-06-19T22:00:00Z")

	jobs, err := repo.GetAllJobs(ctx)
	assert.NoError(t, err)
	assert.Len(t, jobs, 2)

	// Build a map for easy validation (SMembers order is arbitrary)
	jobMap := make(map[uuid.UUID]models.Job)
	for _, j := range jobs {
		jobMap[j.ID] = j
	}

	// Validate job 1
	j1, exists := jobMap[jobID1]
	assert.True(t, exists)
	assert.Equal(t, "https://example.com/1", j1.URL)
	assert.Equal(t, "pending", j1.Status)
	assert.Equal(t, "2026-06-19T21:00:00Z", j1.CreatedAt)

	// Validate job 2
	j2, exists := jobMap[jobID2]
	assert.True(t, exists)
	assert.Equal(t, "https://example.com/2", j2.URL)
	assert.Equal(t, "completed", j2.Status)
	assert.Equal(t, "2026-06-19T22:00:00Z", j2.CreatedAt)
}

func TestJobRepository_GetAllJobs_MixedMalformedAndMissing(t *testing.T) {
	mr, rdb := newTestRedis(t)
	repo := NewJobRepository(rdb)

	ctx := context.Background()

	// Seed database with:
	// 1. One valid job ID with valid hash
	validJobID := uuid.New()
	mr.SAdd("jobs:index", validJobID.String())
	mr.HSet("job:"+validJobID.String(), "url", "https://example.com/valid", "status", "pending", "created_at", "2026-06-19T23:00:00Z")

	// 2. An orphaned job ID (in set but hash is missing/empty)
	missingJobID := uuid.New()
	mr.SAdd("jobs:index", missingJobID.String())

	// 3. An invalid UUID string in the index set
	mr.SAdd("jobs:index", "this-is-not-a-valid-uuid")

	jobs, err := repo.GetAllJobs(ctx)
	assert.NoError(t, err)
	assert.Len(t, jobs, 1, "Should filter out the invalid UUID and the missing hash, returning only the valid job")

	assert.Equal(t, validJobID, jobs[0].ID)
	assert.Equal(t, "https://example.com/valid", jobs[0].URL)
	assert.Equal(t, "pending", jobs[0].Status)
}

func TestJobRepository_GetAllJobs_RedisError(t *testing.T) {
	_, rdb := newTestRedis(t)
	repo := NewJobRepository(rdb)

	// Close client connection to simulate connection error
	rdb.Close()

	ctx := context.Background()
	jobs, err := repo.GetAllJobs(ctx)
	assert.Error(t, err)
	assert.Nil(t, jobs)
}
