package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testRouteConfig = RouteConfig{
	AppPublicURL:    "https://archiver.example.com",
	ReplayPublicURL: "https://replay.example.com",
}

var testFrontendFS = fstest.MapFS{
	"index.html":      {Data: []byte("<!doctype html><title>Archiver</title>")},
	"viewer.html":     {Data: []byte("<!doctype html><title>Archive Viewer</title>")},
	"replay/ui.js":    {Data: []byte("customElements.define('replay-test', class extends HTMLElement {});")},
	"replay/sw.js":    {Data: []byte("self.addEventListener('fetch', () => {});")},
	"placeholder.txt": {Data: []byte("test fixture")},
}

func TestMainRoutesExposeRuntimeConfigAndNotReplayContent(t *testing.T) {
	handler := &Handler{}
	e := echo.New()
	handler.setMainRoutes(e, testRouteConfig, testFrontendFS)

	t.Run("runtime config", func(t *testing.T) {
		rec := serveRequest(e, http.MethodGet, "/api/config", "")
		require.Equal(t, http.StatusOK, rec.Code)

		var response map[string]string
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, testRouteConfig.ReplayPublicURL, response["replay_origin"])
		assert.Equal(t, "no-store", rec.Header().Get(echo.HeaderCacheControl))
	})

	for _, path := range []string{
		"/viewer.html",
		"/replay/sw.js",
		"/replay/ui.js",
		"/api/archives/00000000-0000-0000-0000-000000000000",
	} {
		t.Run(path+" is unavailable", func(t *testing.T) {
			rec := serveRequest(e, http.MethodGet, path, "")
			assert.Equal(t, http.StatusNotFound, rec.Code)
		})
	}
}

func TestReplayRoutesExposeOnlyReplayContent(t *testing.T) {
	handler := &Handler{}
	e := echo.New()
	handler.setReplayRoutes(e, testRouteConfig, testFrontendFS)

	rec := serveRequest(e, http.MethodGet, "/viewer.html", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "frame-ancestors "+testRouteConfig.AppPublicURL, rec.Header().Get("Content-Security-Policy"))

	rec = serveRequest(e, http.MethodGet, "/replay/ui.js", "")
	assert.Equal(t, http.StatusOK, rec.Code)

	for _, path := range []string{"/", "/api/config", "/api/archives"} {
		rec = serveRequest(e, http.MethodGet, path, "")
		assert.Equal(t, http.StatusNotFound, rec.Code)
	}

	rec = serveRequest(e, http.MethodDelete, "/archives/00000000-0000-0000-0000-000000000000", "")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestRequireTrustedOrigin(t *testing.T) {
	e := echo.New()
	e.POST("/mutation", func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, requireTrustedOrigin(testRouteConfig.AppPublicURL))

	t.Run("allows app origin", func(t *testing.T) {
		rec := serveRequest(e, http.MethodPost, "/mutation", testRouteConfig.AppPublicURL)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("rejects replay origin", func(t *testing.T) {
		rec := serveRequest(e, http.MethodPost, "/mutation", testRouteConfig.ReplayPublicURL)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("rejects missing origin", func(t *testing.T) {
		rec := serveRequest(e, http.MethodPost, "/mutation", "")
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func serveRequest(e *echo.Echo, method, target, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if origin != "" {
		req.Header.Set(echo.HeaderOrigin, origin)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}
