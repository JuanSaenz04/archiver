package api

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/JuanSaenz04/archiver/internal/joblogs"
	"github.com/JuanSaenz04/archiver/internal/queue"
	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func logServer(t *testing.T) (*Handler, *redis.Client, *httptest.Server, string) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	h := NewHandler(queue.NewJobService(rdb), t.TempDir(), nil)
	e := echo.New()
	h.SetMainRoutes(e, RouteConfig{})
	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)
	id := uuid.New().String()
	require.NoError(t, rdb.HSet(context.Background(), "job:"+id, "status", "running").Err())
	return h, rdb, srv, id
}

func TestLogEndpointValidation(t *testing.T) {
	h, _, srv, id := logServer(t)
	for _, tc := range []struct {
		path, cursor string
		code         int
	}{
		{"invalid", "", 400}, {uuid.New().String(), "", 404}, {id, "bad", 400}, {id, "18446744073709551616-0", 400},
	} {
		req, err := http.NewRequest("GET", srv.URL+"/api/jobs/"+tc.path+"/logs", nil)
		require.NoError(t, err)
		req.Header.Set("Last-Event-ID", tc.cursor)
		res, err := srv.Client().Do(req)
		require.NoError(t, err)
		res.Body.Close()
		require.Equal(t, tc.code, res.StatusCode)
	}
	for range cap(h.logSlots) {
		h.logSlots <- struct{}{}
	}
	res, err := srv.Client().Get(srv.URL + "/api/jobs/" + id + "/logs")
	require.NoError(t, err)
	res.Body.Close()
	require.Equal(t, 429, res.StatusCode)
	for range cap(h.logSlots) {
		<-h.logSlots
	}
}

func TestLogEndpointHistoryResumeAndCompletion(t *testing.T) {
	_, rdb, srv, id := logServer(t)
	logs := joblogs.NewStore(rdb)
	session := logs.Start(id)
	session.Message("first\n<script>alert(1)</script>")
	session.Message("last")
	session.Close(false)
	require.NoError(t, rdb.HSet(context.Background(), "job:"+id, "status", "completed").Err())
	entries, err := logs.Read(context.Background(), id, "")
	require.NoError(t, err)
	for _, cursor := range []string{"", entries[0].ID, "1-0"} {
		req, err := http.NewRequest("GET", srv.URL+"/api/jobs/"+id+"/logs", nil)
		require.NoError(t, err)
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Last-Event-ID", cursor)
		res, err := srv.Client().Do(req)
		require.NoError(t, err)
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		require.NoError(t, err)
		require.Equal(t, "text/event-stream", res.Header.Get("Content-Type"))
		require.Empty(t, res.Header.Get("Content-Encoding"))
		require.Equal(t, "no", res.Header.Get("X-Accel-Buffering"))
		require.Contains(t, string(body), "event: done")
		require.Contains(t, string(body), "last")
		if cursor == entries[0].ID {
			require.NotContains(t, string(body), "first")
		}
		if cursor == "1-0" {
			require.Contains(t, string(body), "event: gap")
		}
	}
}

func TestLogEndpointLiveAndDisconnect(t *testing.T) {
	h, rdb, srv, id := logServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/jobs/"+id+"/logs", nil)
	require.NoError(t, err)
	res, err := srv.Client().Do(req)
	require.NoError(t, err)
	session := joblogs.NewStore(rdb).Start(id)
	session.Message("live output")
	session.Close(false)
	scanner := bufio.NewScanner(res.Body)
	found := false
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "live output") {
			found = true
			break
		}
	}
	require.True(t, found)
	res.Body.Close()
	cancel()
	require.Eventually(t, func() bool { return len(h.logSlots) == 0 }, time.Second, 10*time.Millisecond)
}

func TestLogEndpointExpired(t *testing.T) {
	_, rdb, srv, id := logServer(t)
	require.NoError(t, rdb.HSet(context.Background(), "job:"+id, "status", "failed").Err())
	res, err := srv.Client().Get(srv.URL + "/api/jobs/" + id + "/logs")
	require.NoError(t, err)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `"unavailable":true`)
}

func TestLogEndpointIndependentViewers(t *testing.T) {
	h, rdb, srv, id := logServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var responses []*http.Response
	for range 2 {
		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/jobs/"+id+"/logs", nil)
		require.NoError(t, err)
		res, err := srv.Client().Do(req)
		require.NoError(t, err)
		responses = append(responses, res)
		defer res.Body.Close()
	}
	session := joblogs.NewStore(rdb).Start(id)
	session.Message("shared output")
	session.Close(false)
	require.NoError(t, rdb.HSet(ctx, "job:"+id, "status", "completed").Err())
	for _, res := range responses {
		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "shared output")
		require.Contains(t, string(body), "event: done")
	}
	require.Eventually(t, func() bool { return len(h.logSlots) == 0 }, time.Second, 10*time.Millisecond)
}

func TestLogEndpointUnavailableRedis(t *testing.T) {
	h, rdb, srv, id := logServer(t)
	require.NoError(t, rdb.Close())
	res, err := srv.Client().Get(srv.URL + "/api/jobs/" + id + "/logs")
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, res.StatusCode)
	require.Empty(t, h.logSlots)
}
