package joblogs

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func testStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), ContextTimeoutEnabled: true, MaxRetries: -1, DialTimeout: 100 * time.Millisecond})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewStore(rdb), mr
}

func TestSessionLinesAndRetention(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			s, mr := testStore(t)
			l := s.Start("job")
			_, err := io.WriteString(l.Stdout(), "hel")
			require.NoError(t, err)
			_, err = io.WriteString(l.Stdout(), "lo\r\n\npartial")
			require.NoError(t, err)
			_, err = io.WriteString(l.Stderr(), "failure\n")
			require.NoError(t, err)
			l.Close(failed)
			l.Close(failed)
			entries, err := s.Read(context.Background(), "job", "")
			require.NoError(t, err)
			require.Len(t, entries, 4)
			require.Equal(t, "hello", entries[0].Text)
			require.Empty(t, entries[1].Text)
			require.Equal(t, "stderr", entries[2].Source)
			require.Equal(t, "partial", entries[3].Text)
			ttl := SuccessTTL
			if failed {
				ttl = SafetyTTL
			}
			require.Equal(t, ttl, mr.TTL(Key("job")))
			mr.FastForward(ttl)
			require.False(t, mr.Exists(Key("job")))
		})
	}
}

func TestWriterBoundsAndConcurrentWrites(t *testing.T) {
	s, _ := testStore(t)
	l := s.Start("job")
	_, err := io.WriteString(l.Stdout(), strings.Repeat("x", MaxLineBytes*10)+"\nnext\n")
	require.NoError(t, err)
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			for range 10 {
				_, _ = io.WriteString(l.Stderr(), "concurrent\n")
			}
		})
	}
	wg.Wait()
	l.Close(false)
	entries, err := s.Read(context.Background(), "job", "")
	require.NoError(t, err)
	require.Len(t, entries, 102)
	require.Equal(t, strings.Repeat("x", MaxLineBytes)+" [truncated]", entries[0].Text)
	require.Equal(t, "next", entries[1].Text)
}

func TestQueueOverflowNeverBlocksAndReportsDrops(t *testing.T) {
	s, _ := testStore(t)
	l := &Session{store: s, jobID: "job", queue: make(chan Entry, 1), done: make(chan struct{})}
	l.ctx, l.cancel = context.WithCancel(context.Background())
	defer l.cancel()
	l.enqueue("stdout", "first")
	for range 10000 {
		l.enqueue("stdout", "dropped")
	}
	require.EqualValues(t, 10000, l.dropped.Load())
	close(l.queue)
	l.publish()
	entries, err := s.Read(context.Background(), "job", "")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "10000 log lines omitted", entries[1].Text)
}

func TestStoreTrimmingCursorsAndSafetyTTL(t *testing.T) {
	s, mr := testStore(t)
	ctx := context.Background()
	for range 21 {
		entries := make([]Entry, 100)
		for i := range entries {
			entries[i] = Entry{Source: "stdout", Text: "line"}
		}
		require.NoError(t, s.append(ctx, "job", entries))
	}
	require.EqualValues(t, MaxEntries, s.client.XLen(ctx, Key("job")).Val())
	require.Equal(t, SafetyTTL, mr.TTL(Key("job")))
	tail, err := s.Read(ctx, "job", "")
	require.NoError(t, err)
	require.Len(t, tail, 500)
	missing, err := s.HistoryMissing(ctx, "job", "1-0")
	require.NoError(t, err)
	require.True(t, missing)
	missing, err = s.HistoryMissing(ctx, "job", tail[0].ID)
	require.NoError(t, err)
	require.False(t, missing)
	next, err := s.Read(ctx, "job", tail[0].ID)
	require.NoError(t, err)
	require.Equal(t, tail[1:101], next)
	next, err = s.Read(ctx, "job", tail[len(tail)-1].ID)
	require.NoError(t, err)
	require.Empty(t, next)
	other, err := s.Read(ctx, "other", "")
	require.NoError(t, err)
	require.Empty(t, other)
}

func TestPublishingWithoutViewerAndRedisOutage(t *testing.T) {
	s, mr := testStore(t)
	l := s.Start("job")
	l.Message("live")
	require.Eventually(t, func() bool { return s.client.XLen(context.Background(), Key("job")).Val() == 1 }, time.Second, 10*time.Millisecond)
	mr.Close()
	start := time.Now()
	for range 2000 {
		_, err := io.WriteString(l.Stdout(), "output\n")
		require.NoError(t, err)
	}
	require.Less(t, time.Since(start), time.Second)
	l.Close(true)
	require.Less(t, time.Since(start), 4*time.Second)
}

func TestRedisRecoveryPreservesDropCount(t *testing.T) {
	s, mr := testStore(t)
	mr.SetError("ERR unavailable")
	l := s.Start("job")
	l.Message("lost one")
	l.Message("lost two")
	require.Eventually(t, func() bool { return l.dropped.Load() == 2 }, time.Second, 10*time.Millisecond)
	// Allow another failed flush of the omission notice itself.
	time.Sleep(250 * time.Millisecond)
	require.EqualValues(t, 2, l.dropped.Load())
	mr.SetError("")
	l.Close(true)
	entries, err := s.Read(context.Background(), "job", "")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "2 log lines omitted", entries[0].Text)
}
