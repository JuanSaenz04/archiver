package queue

import (
	"encoding/json/v2"
	"testing"
	"time"
	"uuid"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestJobService_EnqueueCrawl_Success(t *testing.T) {
	// Setup miniredis and the client via the package-level helper
	_, rdb, ctx := newTestRedis(t)
	service := NewJobService(rdb)

	request := models.CrawlRequest{
		URL:         "https://example.com/test-page",
		Name:        "Test Archive Name",
		Description: "This is a test description",
		Tags:        []string{"tag1", "tag2"},
		Options: models.CrawlOptions{
			ScopeType: models.Page,
			Depth:     3,
		},
	}

	// Act: Enqueue the crawl job
	jobID, err := service.EnqueueCrawl(ctx, request)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, jobID)
	assert.NotEqual(t, uuid.Nil(), *jobID)

	// 1. Verify that the job details are stored in a Hash at "job:<jobID>"
	jobKey := "job:" + jobID.String()
	jobData, err := rdb.HGetAll(ctx, jobKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, request.URL, jobData["url"])
	assert.Equal(t, "pending", jobData["status"])

	createdAtStr, exists := jobData["created_at"]
	assert.True(t, exists, "created_at field should exist in job hash")
	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	assert.NoError(t, err, "created_at should be a valid RFC3339 timestamp")
	// Make sure the timestamp is reasonably close to now (within 5 seconds)
	assert.WithinDuration(t, time.Now(), createdAt, 5*time.Second)

	// 2. Verify that the job ID is added to the "jobs:index" Set
	isIndexed, err := rdb.SIsMember(ctx, "jobs:index", jobID.String()).Result()
	assert.NoError(t, err)
	assert.True(t, isIndexed, "jobID should be indexed in jobs:index")

	// 3. Verify that a message was added to the "crawl_stream" Stream
	streamMessages, err := rdb.XRead(ctx, &redis.XReadArgs{
		Streams: []string{"crawl_stream", "0"},
		Count:   1,
	}).Result()
	assert.NoError(t, err)
	assert.Len(t, streamMessages, 1)
	assert.Len(t, streamMessages[0].Messages, 1)

	msg := streamMessages[0].Messages[0]
	assert.Equal(t, jobID.String(), msg.Values["job_id"])

	payloadStr, ok := msg.Values["payload"].(string)
	assert.True(t, ok, "payload in stream message must be a string")

	// Decode the payload to verify the CrawlMessage contents
	var crawlMsg CrawlMessage
	err = json.Unmarshal([]byte(payloadStr), &crawlMsg)
	assert.NoError(t, err)

	// Assert fields inside the marshaled message match expectations
	assert.Equal(t, jobID.String(), crawlMsg.JobID)
	assert.Equal(t, request.Options.ScopeType, crawlMsg.Options.ScopeType)
	assert.Equal(t, request.Options.Depth, crawlMsg.Options.Depth)

	assert.Equal(t, *jobID, crawlMsg.Archive.ID)
	assert.Equal(t, request.Name, crawlMsg.Archive.Name)
	assert.Equal(t, request.Description, crawlMsg.Archive.Description)
	assert.Equal(t, request.URL, crawlMsg.Archive.SourceURL)
	assert.Equal(t, request.Tags, crawlMsg.Archive.Tags)
}

func TestJobService_EnqueueCrawl_RedisError(t *testing.T) {
	_, rdb, ctx := newTestRedis(t)
	service := NewJobService(rdb)

	// Close the connection client to simulate a connection/Redis error
	rdb.Close()

	request := models.CrawlRequest{
		URL: "https://example.com/fail-test",
	}

	jobID, err := service.EnqueueCrawl(ctx, request)
	assert.Error(t, err)
	assert.Nil(t, jobID)
}

func TestJobService_GetAllJobs_Empty(t *testing.T) {
	_, rdb, ctx := newTestRedis(t)
	service := NewJobService(rdb)

	jobs, err := service.GetAllJobs(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, jobs)
	assert.Empty(t, jobs, "Should return an empty slice when there are no jobs in the index")
}

func TestJobService_GetAllJobs_Success(t *testing.T) {
	mr, rdb, ctx := newTestRedis(t)
	service := NewJobService(rdb)

	jobID1 := uuid.New()
	jobID2 := uuid.New()

	mr.SAdd("jobs:index", jobID1.String())
	mr.SAdd("jobs:index", jobID2.String())
	mr.HSet("job:"+jobID1.String(), "url", "https://example.com/1", "status", "pending", "created_at", "2026-06-19T21:00:00Z")
	mr.HSet("job:"+jobID2.String(), "url", "https://example.com/2", "status", "completed", "created_at", "2026-06-19T22:00:00Z")

	jobs, err := service.GetAllJobs(ctx)
	assert.NoError(t, err)
	assert.Len(t, jobs, 2)

	jobMap := make(map[uuid.UUID]models.Job)
	for _, job := range jobs {
		jobMap[job.ID] = job
	}

	assert.Equal(t, "https://example.com/1", jobMap[jobID1].URL)
	assert.Equal(t, "pending", jobMap[jobID1].Status)
	assert.Equal(t, "2026-06-19T21:00:00Z", jobMap[jobID1].CreatedAt)
	assert.Equal(t, "https://example.com/2", jobMap[jobID2].URL)
	assert.Equal(t, "completed", jobMap[jobID2].Status)
	assert.Equal(t, "2026-06-19T22:00:00Z", jobMap[jobID2].CreatedAt)
}

func TestJobService_GetAllJobs_MixedMalformedAndMissing(t *testing.T) {
	mr, rdb, ctx := newTestRedis(t)
	service := NewJobService(rdb)

	validJobID := uuid.New()
	mr.SAdd("jobs:index", validJobID.String())
	mr.HSet("job:"+validJobID.String(), "url", "https://example.com/valid", "status", "pending", "created_at", "2026-06-19T23:00:00Z")
	mr.SAdd("jobs:index", uuid.New().String())
	mr.SAdd("jobs:index", "this-is-not-a-valid-uuid")

	jobs, err := service.GetAllJobs(ctx)
	assert.NoError(t, err)
	assert.Len(t, jobs, 1)
	assert.Equal(t, validJobID, jobs[0].ID)
	assert.Equal(t, "https://example.com/valid", jobs[0].URL)
	assert.Equal(t, "pending", jobs[0].Status)
}

func TestJobService_GetAllJobs_RedisError(t *testing.T) {
	_, rdb, ctx := newTestRedis(t)
	service := NewJobService(rdb)

	rdb.Close()

	jobs, err := service.GetAllJobs(ctx)
	assert.Error(t, err)
	assert.Nil(t, jobs)
}
