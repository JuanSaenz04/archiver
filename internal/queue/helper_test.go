package queue

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

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

func TestMain(m *testing.M) {
	retryInterval = 10 * time.Millisecond
	readBlock = 50 * time.Millisecond
	os.Exit(m.Run())
}
