package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

const testConsumerName = "test-consumer-1"

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client, context.Context) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	t.Cleanup(func() {
		rdb.Close()
		mr.Close()
	})
	return mr, rdb, ctx
}

func startWorker(t *testing.T, ctx context.Context, rdb *redis.Client, process Processor) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- StartWorker(ctx, rdb, testConsumerName, process)
	}()
	return done
}

func enqueueMessage(t *testing.T, ctx context.Context, rdb *redis.Client, values map[string]any) {
	t.Helper()
	_, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: values,
	}).Result()
	if err != nil {
		t.Fatalf("failed to XAdd: %v", err)
	}
}

func enqueueValidMessage(t *testing.T, ctx context.Context, rdb *redis.Client, jobID string, msg CrawlMessage) {
	t.Helper()
	payloadBytes, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal message: %v", err)
	}
	enqueueMessage(t, ctx, rdb, map[string]any{
		"job_id":  jobID,
		"payload": string(payloadBytes),
	})
}

func createGroup(t *testing.T, ctx context.Context, rdb *redis.Client) {
	t.Helper()
	err := rdb.XGroupCreateMkStream(ctx, streamName, groupName, "0").Err()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		t.Fatalf("failed to create consumer group: %v", err)
	}
}

func waitForNoPending(t *testing.T, ctx context.Context, rdb *redis.Client, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pending, err := rdb.XPending(ctx, streamName, groupName).Result()
		if err == nil && pending.Count == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for no pending messages")
}

func waitForJobStatus(t *testing.T, ctx context.Context, rdb *redis.Client, jobID, expectedStatus string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status := rdb.HGet(ctx, "job:"+jobID, "status").Val()
		if status == expectedStatus {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	status := rdb.HGet(ctx, "job:"+jobID, "status").Val()
	t.Fatalf("timed out waiting for job %s status %q, got %q", jobID, expectedStatus, status)
}

func waitForProcessorCall(ch <-chan struct{}, timeout time.Duration) bool {
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

func makeTestCrawlMessage(jobID string) CrawlMessage {
	return CrawlMessage{
		JobID: jobID,
		Archive: models.Archive{
			ID:        uuid.MustParse(jobID),
			Name:      "Test Archive",
			SourceURL: "https://example.com",
			Tags:      []string{"test"},
		},
		Options: models.CrawlOptions{
			ScopeType: models.Page,
			Depth:     1,
		},
	}
}

func TestStartWorker_ProcessesExistingJobWhenGroupDoesNotExistYet(t *testing.T) {
	_, rdb, ctx := newTestRedis(t)

	jobID := uuid.New().String()
	msg := makeTestCrawlMessage(jobID)

	enqueueValidMessage(t, ctx, rdb, jobID, msg)

	called := make(chan struct{}, 1)
	process := func(_ context.Context, gotJobID string, _ models.Archive, _ models.CrawlOptions) error {
		if gotJobID == jobID {
			called <- struct{}{}
		}
		return nil
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	_ = startWorker(t, workerCtx, rdb, process)

	if !waitForProcessorCall(called, 2*time.Second) {
		t.Fatal("processor was not called for existing job")
	}
	status := rdb.HGet(ctx, "job:"+jobID, "status").Val()
	assert.Equal(t, "completed", status)
}

func TestStartWorker_ProcessesValidMessageAndMarksCompleted(t *testing.T) {
	_, rdb, ctx := newTestRedis(t)
	createGroup(t, ctx, rdb)

	jobID := uuid.New().String()
	msg := makeTestCrawlMessage(jobID)

	var gotJobID string
	var gotArchive models.Archive
	var gotOptions models.CrawlOptions
	called := make(chan struct{}, 1)

	process := func(pCtx context.Context, pJobID string, pArchive models.Archive, pOptions models.CrawlOptions) error {
		status := rdb.HGet(pCtx, "job:"+pJobID, "status").Val()
		assert.Equal(t, "running", status, "job should be marked running before processor is called")

		gotJobID = pJobID
		gotArchive = pArchive
		gotOptions = pOptions
		called <- struct{}{}
		return nil
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	_ = startWorker(t, workerCtx, rdb, process)

	enqueueValidMessage(t, ctx, rdb, jobID, msg)

	if !waitForProcessorCall(called, 2*time.Second) {
		t.Fatal("processor was not called")
	}

	assert.Equal(t, jobID, gotJobID)
	assert.Equal(t, msg.Archive.SourceURL, gotArchive.SourceURL)
	assert.Equal(t, msg.Archive.Name, gotArchive.Name)
	assert.Equal(t, msg.Options.Depth, gotOptions.Depth)
	assert.Equal(t, msg.Options.ScopeType, gotOptions.ScopeType)

	waitForJobStatus(t, ctx, rdb, jobID, "completed", 2*time.Second)
	waitForNoPending(t, ctx, rdb, 2*time.Second)
}

func TestStartWorker_MarksJobFailedAndStoresError(t *testing.T) {
	_, rdb, ctx := newTestRedis(t)
	createGroup(t, ctx, rdb)

	jobID := uuid.New().String()
	msg := makeTestCrawlMessage(jobID)

	called := make(chan struct{}, 1)
	sentinelErr := errors.New("crawl failed")

	process := func(_ context.Context, _ string, _ models.Archive, _ models.CrawlOptions) error {
		called <- struct{}{}
		return sentinelErr
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	_ = startWorker(t, workerCtx, rdb, process)

	enqueueValidMessage(t, ctx, rdb, jobID, msg)

	if !waitForProcessorCall(called, 2*time.Second) {
		t.Fatal("processor was not called")
	}

	waitForJobStatus(t, ctx, rdb, jobID, "failed", 2*time.Second)

	errVal := rdb.HGet(ctx, "job:"+jobID, "error").Val()
	assert.Equal(t, "crawl failed", errVal)

	waitForNoPending(t, ctx, rdb, 2*time.Second)
}

// Note: "non-string" field cases are not included because Redis stores all stream
// values as strings, so go-redis always returns string types from XReadGroup.
// The .(string) type assertions in consumer.go will always succeed with real Redis data.
func TestStartWorker_AcksMalformedMessagesWithoutCallingProcessor(t *testing.T) {
	cases := []struct {
		name   string
		values map[string]any
	}{
		{
			name:   "missing job_id",
			values: map[string]any{"payload": `{"job_id":"1"}`},
		},
		{
			name:   "missing payload",
			values: map[string]any{"job_id": "test-job"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, rdb, ctx := newTestRedis(t)
			createGroup(t, ctx, rdb)

			processorCalled := make(chan struct{}, 1)
			process := func(_ context.Context, _ string, _ models.Archive, _ models.CrawlOptions) error {
				processorCalled <- struct{}{}
				return nil
			}

			workerCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			_ = startWorker(t, workerCtx, rdb, process)

			enqueueMessage(t, ctx, rdb, tc.values)

			didCall := waitForProcessorCall(processorCalled, 2*time.Second)
			assert.False(t, didCall, "processor should not be called for malformed message")

			waitForNoPending(t, ctx, rdb, 2*time.Second)
		})
	}
}

// Regression check: invalid JSON payloads are acknowledged so they do not
// stay pending in the stream after a failed unmarshal.
func TestStartWorker_AcksInvalidJSONPayloadWithoutCallingProcessor(t *testing.T) {
	_, rdb, ctx := newTestRedis(t)
	createGroup(t, ctx, rdb)

	processorCalled := make(chan struct{}, 1)
	process := func(_ context.Context, _ string, _ models.Archive, _ models.CrawlOptions) error {
		processorCalled <- struct{}{}
		return nil
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	_ = startWorker(t, workerCtx, rdb, process)

	jobID := uuid.New().String()
	enqueueMessage(t, ctx, rdb, map[string]any{
		"job_id":  jobID,
		"payload": "{invalid json",
	})

	didCall := waitForProcessorCall(processorCalled, 2*time.Second)
	assert.False(t, didCall, "processor should not be called for invalid JSON payload")

	waitForNoPending(t, ctx, rdb, 2*time.Second)
}

func TestStartWorker_StopsWhenContextIsCanceled(t *testing.T) {
	_, rdb, _ := newTestRedis(t)

	bgCtx := context.Background()
	createGroup(t, bgCtx, rdb)

	workerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	process := func(_ context.Context, _ string, _ models.Archive, _ models.CrawlOptions) error {
		return nil
	}

	done := startWorker(t, workerCtx, rdb, process)

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err, "worker should exit cleanly on context cancellation")
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not stop after context cancellation")
	}
}
