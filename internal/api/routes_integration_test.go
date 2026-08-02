package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainAndReplayServersOverHTTP(t *testing.T) {
	archivesDir := t.TempDir()
	archiveStore, _ := openArchiveStore(t)
	archiveID := uuid.New()
	archiveContent := bytes.Repeat([]byte("0123456789"), 32)
	archiveFilename := "integration-test.wacz"

	require.NoError(t, os.WriteFile(filepath.Join(archivesDir, archiveFilename), archiveContent, 0o644))
	insertArchiveFixture(t, archiveStore, models.Archive{
		ID:        archiveID,
		Name:      "Integration test",
		Filename:  archiveFilename,
		SourceURL: "https://example.com",
		CreatedAt: time.Now().UTC(),
		SizeBytes: int64(len(archiveContent)),
	})

	handler := &Handler{archivesDir: archivesDir, archiveStore: archiveStore}
	mainEcho := echo.New()
	replayEcho := echo.New()
	mainServer := httptest.NewUnstartedServer(mainEcho)
	replayServer := httptest.NewUnstartedServer(replayEcho)
	config := RouteConfig{
		AppPublicURL:    testServerURL(mainServer),
		ReplayPublicURL: testServerURL(replayServer),
	}
	handler.setMainRoutes(mainEcho, config, testFrontendFS)
	handler.setReplayRoutes(replayEcho, config, testFrontendFS)
	mainServer.Start()
	replayServer.Start()
	t.Cleanup(mainServer.Close)
	t.Cleanup(replayServer.Close)

	mainURL, err := url.Parse(mainServer.URL)
	require.NoError(t, err)
	replayURL, err := url.Parse(replayServer.URL)
	require.NoError(t, err)
	assert.NotEqual(t, mainURL.Host, replayURL.Host, "test servers must use different origins")

	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}

	t.Run("runtime config points to replay server", func(t *testing.T) {
		response, body := integrationRequest(t, client, http.MethodGet, mainServer.URL+"/api/config", nil)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "no-store", response.Header.Get("Cache-Control"))
		assert.Empty(t, response.Header.Get("Access-Control-Allow-Origin"))

		var runtimeConfig map[string]string
		require.NoError(t, json.Unmarshal(body, &runtimeConfig))
		assert.Equal(t, replayServer.URL, runtimeConfig["replay_origin"])
	})

	t.Run("origins expose only their intended routes", func(t *testing.T) {
		checks := []struct {
			name       string
			url        string
			wantStatus int
		}{
			{name: "main frontend", url: mainServer.URL + "/", wantStatus: http.StatusOK},
			{name: "main viewer blocked", url: mainServer.URL + "/viewer.html", wantStatus: http.StatusNotFound},
			{name: "main replay UI blocked", url: mainServer.URL + "/replay/ui.js", wantStatus: http.StatusNotFound},
			{name: "main replay worker blocked", url: mainServer.URL + "/replay/sw.js", wantStatus: http.StatusNotFound},
			{name: "old main archive route blocked", url: mainServer.URL + "/api/archives/" + archiveID.String(), wantStatus: http.StatusNotFound},
			{name: "replay root blocked", url: replayServer.URL + "/", wantStatus: http.StatusNotFound},
			{name: "replay API blocked", url: replayServer.URL + "/api/config", wantStatus: http.StatusNotFound},
			{name: "replay viewer", url: replayServer.URL + "/viewer.html", wantStatus: http.StatusOK},
			{name: "replay UI", url: replayServer.URL + "/replay/ui.js", wantStatus: http.StatusOK},
		}

		for _, check := range checks {
			t.Run(check.name, func(t *testing.T) {
				response, _ := integrationRequest(t, client, http.MethodGet, check.url, nil)
				assert.Equal(t, check.wantStatus, response.StatusCode)
			})
		}

		response, _ := integrationRequest(t, client, http.MethodGet, replayServer.URL+"/viewer.html", nil)
		assert.Equal(t, "frame-ancestors "+mainServer.URL, response.Header.Get("Content-Security-Policy"))
	})

	t.Run("archive supports range and head requests without gzip", func(t *testing.T) {
		archiveURL := replayServer.URL + "/archives/" + archiveID.String()
		response, body := integrationRequest(t, client, http.MethodGet, archiveURL, map[string]string{
			"Accept-Encoding": "gzip",
			"Range":           "bytes=10-24",
		})
		assert.Equal(t, http.StatusPartialContent, response.StatusCode)
		assert.Equal(t, archiveContent[10:25], body)
		assert.Equal(t, "bytes 10-24/"+strconv.Itoa(len(archiveContent)), response.Header.Get("Content-Range"))
		assert.Equal(t, "bytes", response.Header.Get("Accept-Ranges"))
		assert.Empty(t, response.Header.Get("Content-Encoding"))

		headResponse, headBody := integrationRequest(t, client, http.MethodHead, archiveURL, nil)
		assert.Equal(t, http.StatusOK, headResponse.StatusCode)
		assert.Empty(t, headBody)
		assert.Equal(t, strconv.Itoa(len(archiveContent)), headResponse.Header.Get("Content-Length"))
	})

	t.Run("mutation origin is enforced over HTTP", func(t *testing.T) {
		mutationURL := mainServer.URL + "/api/archives/not-a-uuid"

		response, _ := integrationRequest(t, client, http.MethodDelete, mutationURL, map[string]string{
			"Origin": replayServer.URL,
		})
		assert.Equal(t, http.StatusForbidden, response.StatusCode)

		response, _ = integrationRequest(t, client, http.MethodDelete, mutationURL, map[string]string{
			"Origin": mainServer.URL,
		})
		assert.Equal(t, http.StatusBadRequest, response.StatusCode, "trusted origin should reach the handler")

		response, _ = integrationRequest(t, client, http.MethodDelete, replayServer.URL+"/archives/"+archiveID.String(), nil)
		assert.Equal(t, http.StatusMethodNotAllowed, response.StatusCode)
	})
}

func testServerURL(server *httptest.Server) string {
	return "http://" + server.Listener.Addr().String()
}

func integrationRequest(t *testing.T, client *http.Client, method, target string, headers map[string]string) (*http.Response, []byte) {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), method, target, nil)
	require.NoError(t, err)
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	response, err := client.Do(request)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	return response, body
}
