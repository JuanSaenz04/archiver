package queue

import (
	"context"
	"encoding/json/v2"
	"errors"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/alicebob/miniredis/v2/server"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

const testConsumerName = "test-consumer-1"

func startWorker(t *testing.T, ctx context.Context, rdb *redis.Client, process Processor) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- StartWorker(ctx, rdb, testConsumerName, time.Hour, process)
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
	waitForJobStatus(t, ctx, rdb, jobID, "completed", 2*time.Second)
	waitForNoPending(t, ctx, rdb, 2*time.Second)
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

func TestStartWorker_RecoversAbandonedDelivery(t *testing.T) {
	mr, rdb, ctx := newTestRedis(t)
	now := time.Now()
	mr.SetTime(now)
	createGroup(t, ctx, rdb)
	jobID := uuid.New().String()
	enqueueValidMessage(t, ctx, rdb, jobID, makeTestCrawlMessage(jobID))
	_, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{Group: groupName, Consumer: "old-worker", Streams: []string{streamName, ">"}, Count: 1}).Result()
	assert.NoError(t, err)
	mr.SetTime(now.Add(2 * time.Hour))
	called := make(chan struct{}, 1)
	workerCtx, cancel := context.WithCancel(ctx)
	done := startWorker(t, workerCtx, rdb, func(context.Context, string, models.Archive, models.CrawlOptions) error {
		called <- struct{}{}
		return nil
	})
	defer func() { cancel(); <-done }()
	assert.True(t, waitForProcessorCall(called, 2*time.Second))
	waitForJobStatus(t, ctx, rdb, jobID, "completed", 2*time.Second)
	waitForNoPending(t, ctx, rdb, 2*time.Second)
}

func TestStartWorker_DoesNotReclaimRecentDelivery(t *testing.T) {
	_, rdb, ctx := newTestRedis(t)
	activeID, _ := pendingTestMessage(t, rdb, ctx)
	newID := uuid.New().String()
	enqueueValidMessage(t, ctx, rdb, newID, makeTestCrawlMessage(newID))
	workerCtx, cancel := context.WithCancel(ctx)
	called := make(chan string, 2)
	done := make(chan error, 1)
	go func() {
		done <- StartWorker(workerCtx, rdb, "another-worker", time.Hour, func(_ context.Context, id string, _ models.Archive, _ models.CrawlOptions) error {
			called <- id
			return nil
		})
	}()
	defer func() { cancel(); <-done }()
	select {
	case id := <-called:
		assert.Equal(t, newID, id)
	case <-time.After(2 * time.Second):
		t.Fatal("new job was not processed")
	}
	waitForJobStatus(t, ctx, rdb, newID, "completed", time.Second)
	pending, err := rdb.XPendingExt(ctx, &redis.XPendingExtArgs{Stream: streamName, Group: groupName, Start: "-", End: "+", Count: 10}).Result()
	assert.NoError(t, err)
	var originalFound bool
	for _, delivery := range pending {
		if delivery.Consumer == testConsumerName {
			originalFound = true
		}
	}
	assert.True(t, originalFound, "active job %s should remain owned by the original worker", activeID)
}

func pendingTestMessage(t *testing.T, rdb *redis.Client, ctx context.Context) (string, redis.XMessage) {
	t.Helper()
	createGroup(t, ctx, rdb)
	id := uuid.New().String()
	enqueueValidMessage(t, ctx, rdb, id, makeTestCrawlMessage(id))
	streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{Group: groupName, Consumer: testConsumerName, Streams: []string{streamName, ">"}, Count: 1}).Result()
	if err != nil {
		t.Fatal(err)
	}
	return id, streams[0].Messages[0]
}

func TestHandleMessage_TerminalDeliveryIsNotProcessed(t *testing.T) {
	for _, status := range []string{"completed", "failed"} {
		t.Run(status, func(t *testing.T) {
			_, rdb, ctx := newTestRedis(t)
			id, message := pendingTestMessage(t, rdb, ctx)
			assert.NoError(t, rdb.HSet(ctx, "job:"+id, "status", status).Err())
			err := handleMessage(ctx, rdb, message, time.Hour, func(context.Context, string, models.Archive, models.CrawlOptions) error {
				t.Fatal("terminal job processed")
				return nil
			})
			assert.NoError(t, err)
			waitForNoPending(t, ctx, rdb, time.Second)
		})
	}
}

func TestHandleMessage_CancellationLeavesPending(t *testing.T) {
	_, rdb, ctx := newTestRedis(t)
	id, message := pendingTestMessage(t, rdb, ctx)
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	err := handleMessage(workerCtx, rdb, message, time.Hour, func(context.Context, string, models.Archive, models.CrawlOptions) error {
		cancel()
		return errors.New("signal: killed")
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, "running", rdb.HGet(ctx, "job:"+id, "status").Val())
	assert.EqualValues(t, 1, rdb.XPending(ctx, streamName, groupName).Val().Count)
}

func TestHandleMessage_JobDeadlineIsFailure(t *testing.T) {
	_, rdb, ctx := newTestRedis(t)
	id, message := pendingTestMessage(t, rdb, ctx)
	err := handleMessage(ctx, rdb, message, 10*time.Millisecond, func(ctx context.Context, _ string, _ models.Archive, _ models.CrawlOptions) error {
		<-ctx.Done()
		return ctx.Err()
	})
	assert.NoError(t, err)
	assert.Equal(t, "failed", rdb.HGet(ctx, "job:"+id, "status").Val())
	waitForNoPending(t, ctx, rdb, time.Second)
}

func TestHandleMessage_RetriesStatusWithoutRecrawling(t *testing.T) {
	mr, rdb, ctx := newTestRedis(t)
	id, message := pendingTestMessage(t, rdb, ctx)
	calls := 0
	restored := make(chan struct{})
	err := handleMessage(ctx, rdb, message, time.Hour, func(context.Context, string, models.Archive, models.CrawlOptions) error {
		calls++
		mr.SetError("ERR temporarily unavailable")
		go func() {
			defer close(restored)
			time.Sleep(100 * time.Millisecond)
			mr.SetError("")
		}()
		return nil
	})
	<-restored
	assert.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, "completed", rdb.HGet(ctx, "job:"+id, "status").Val())
	waitForNoPending(t, ctx, rdb, time.Second)
}

func TestHandleMessage_RetriesAcknowledgmentWithoutRecrawling(t *testing.T) {
	mr, rdb, ctx := newTestRedis(t)
	id, message := pendingTestMessage(t, rdb, ctx)
	var acks atomic.Int32
	mr.Server().SetPreHook(func(peer *server.Peer, cmd string, args ...string) bool {
		if cmd == "XACK" && acks.Add(1) == 1 {
			peer.WriteError("ERR temporarily unavailable")
			return true
		}
		return false
	})
	calls := 0
	err := handleMessage(ctx, rdb, message, time.Hour, func(context.Context, string, models.Archive, models.CrawlOptions) error { calls++; return nil })
	assert.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.GreaterOrEqual(t, acks.Load(), int32(2))
	assert.Equal(t, "completed", rdb.HGet(ctx, "job:"+id, "status").Val())
	waitForNoPending(t, ctx, rdb, time.Second)
	assert.Zero(t, rdb.XLen(ctx, streamName).Val())
}

func TestHandleMessage_StatusFailureDoesNotAcknowledge(t *testing.T) {
	mr, rdb, ctx := newTestRedis(t)
	_, message := pendingTestMessage(t, rdb, ctx)
	workerCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	err := handleMessage(workerCtx, rdb, message, time.Hour, func(context.Context, string, models.Archive, models.CrawlOptions) error {
		mr.SetError("ERR temporarily unavailable")
		return nil
	})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	mr.SetError("")
	assert.EqualValues(t, 1, rdb.XPending(ctx, streamName, groupName).Val().Count)
	assert.EqualValues(t, 1, rdb.XLen(ctx, streamName).Val())
}
