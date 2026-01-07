package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

//go:embed dist
var frontendDist embed.FS

func (handler *Handler) SetRoutes(e *echo.Echo) {
	// Enable Gzip compression for frontend assets and JSON API,
	// but skip it for archive downloads to support HTTP Range requests.
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Skipper: func(c echo.Context) bool {
			return strings.HasPrefix(c.Request().URL.Path, "/api/archives/")
		},
	}))

	apiGroup := e.Group("/api")

	apiGroup.POST("/jobs", handler.HandleNewJob)
	apiGroup.GET("/jobs", handler.HandleGetJobs)
	apiGroup.GET("/archives", handler.HandleGetArchives)
	apiGroup.GET("/archives/:archiveName", handler.HandleGetArchive)
	apiGroup.DELETE("/archives/:archiveName", handler.HandleDeleteArchive)
	apiGroup.PUT("/archives/:archiveName", handler.HandleModifyArchiveName)

	dist, err := fs.Sub(frontendDist, "dist")
	if err != nil {
		// This should only happen if the embed fails drastically, which is unlikely with correct build
		e.Logger.Fatal(err)
	}

	fileHandler := http.FileServer(http.FS(dist))

	e.GET("/*", func(c echo.Context) error {
		path := c.Request().URL.Path

		// API requests should not fallback to index.html
		if strings.HasPrefix(path, "/api") {
			return echo.ErrNotFound
		}

		cleanPath := strings.TrimPrefix(path, "/")

		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// Check if file exists in the embedded FS
		_, err := dist.Open(cleanPath)
		if err == nil {
			fileHandler.ServeHTTP(c.Response(), c.Request())
			return nil
		}

		// Fallback to index.html for SPA routing
		c.Request().URL.Path = "/"
		fileHandler.ServeHTTP(c.Response(), c.Request())
		return nil
	})
}
