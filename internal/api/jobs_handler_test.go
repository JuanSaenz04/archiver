package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestHandleNewJob(t *testing.T) {
	// 1. Setup miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	// 2. Setup Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// 3. Initialize Handler
	handler := NewHandler(rdb, t.TempDir())
	e := echo.New()

	t.Run("Success", func(t *testing.T) {
		jobReq := models.CrawlRequest{
			URL: "https://example.com",
			Options: models.CrawlOptions{
				Depth: 1,
			},
		}
		body, _ := json.Marshal(jobReq)

		req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(string(body)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, handler.HandleNewJob(c)) {
			assert.Equal(t, http.StatusCreated, rec.Code)

			var response map[string]interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			assert.NoError(t, err)

			assert.Contains(t, response, "job_id")
			assert.Equal(t, "pending", response["status"])

			// Verify Redis state
			// Check stream
			streamEntries, err := rdb.XRange(c.Request().Context(), "crawl_stream", "-", "+").Result()
			assert.NoError(t, err)
			assert.Len(t, streamEntries, 1)
			assert.Equal(t, "https://example.com", streamEntries[0].Values["target_url"])
		}
	})

	t.Run("BadRequest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader("invalid json"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, handler.HandleNewJob(c)) {
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		}
	})
}

func TestHandleGetJobs(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	handler := NewHandler(rdb, t.TempDir())
	e := echo.New()

	t.Run("Empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, handler.HandleGetJobs(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)

			var jobs []models.Job
			err := json.Unmarshal(rec.Body.Bytes(), &jobs)
			assert.NoError(t, err)
			assert.Empty(t, jobs)
		}
	})

	t.Run("WithJobs", func(t *testing.T) {
		// Manually seed data into miniredis to simulate existing jobs
		jobID1 := "550e8400-e29b-41d4-a716-446655440000"
		jobID2 := "550e8400-e29b-41d4-a716-446655440001"

		mr.SAdd("jobs:index", jobID1, jobID2)

		mr.HSet("job:"+jobID1, "url", "https://site1.com")
		mr.HSet("job:"+jobID1, "status", "pending")
		mr.HSet("job:"+jobID1, "created_at", "2023-01-01T00:00:00Z")

		mr.HSet("job:"+jobID2, "url", "https://site2.com")
		mr.HSet("job:"+jobID2, "status", "completed")
		mr.HSet("job:"+jobID2, "created_at", "2023-01-02T00:00:00Z")

		req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, handler.HandleGetJobs(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)

			var jobs []models.Job
			err := json.Unmarshal(rec.Body.Bytes(), &jobs)
			assert.NoError(t, err)
			assert.Len(t, jobs, 2)

			// We need to verify contents, but map iteration order is random
			// so we just check that we have the right data present
			foundSite1 := false
			foundSite2 := false
			for _, j := range jobs {
				if j.URL == "https://site1.com" {
					foundSite1 = true
				}
				if j.URL == "https://site2.com" {
					foundSite2 = true
				}
			}
			assert.True(t, foundSite1)
			assert.True(t, foundSite2)
		}
	})
}
