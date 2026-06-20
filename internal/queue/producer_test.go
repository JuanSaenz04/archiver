package queue

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestEnqueueCrawl_Success(t *testing.T) {
	// Setup miniredis and the client via the package-level helper
	_, rdb, ctx := newTestRedis(t)

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
	jobID, err := EnqueueCrawl(ctx, rdb, request)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, jobID)
	assert.NotEqual(t, uuid.Nil, *jobID)

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

func TestEnqueueCrawl_RedisHSetError(t *testing.T) {
	_, rdb, ctx := newTestRedis(t)

	// Close the connection client to simulate a connection/Redis error
	rdb.Close()

	request := models.CrawlRequest{
		URL: "https://example.com/fail-test",
	}

	jobID, err := EnqueueCrawl(ctx, rdb, request)
	assert.Error(t, err)
	assert.Nil(t, jobID)
}
