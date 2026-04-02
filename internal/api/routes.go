package api

import (
	"embed"
	"errors"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

//go:embed dist
var frontendDist embed.FS

func (handler *Handler) SetRoutes(e *echo.Echo) {
	// Enable Gzip compression for frontend assets and JSON API,
	// but skip it for archive downloads to support HTTP Range requests.
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Skipper: func(c *echo.Context) bool {
			return strings.HasPrefix(c.Request().URL.Path, "/api/archives/")
		},
	}))

	apiGroup := e.Group("/api")

	apiGroup.POST("/jobs", handler.HandleNewJob)
	apiGroup.GET("/jobs", handler.HandleGetJobs)
	apiGroup.GET("/archives", handler.HandleGetArchives)
	apiGroup.GET("/archives/:archiveName", handler.HandleGetArchive)
	apiGroup.DELETE("/archives/:archiveName", handler.HandleDeleteArchive)
	apiGroup.PUT("/archives/:archiveName", handler.HandleModifyArchiveMetadata)

	dist := echo.MustSubFS(frontendDist, "dist")

	e.GET("/*", func(c *echo.Context) error {
		path := c.Request().URL.Path

		// API requests should not fallback to index.html
		if strings.HasPrefix(path, "/api") {
			return echo.ErrNotFound
		}

		cleanPath := strings.TrimPrefix(path, "/")

		if cleanPath == "" {
			cleanPath = "index.html"
		}

		if err := c.FileFS(cleanPath, dist); err != nil {
			if errors.Is(err, echo.ErrNotFound) {
				// Fallback to index.html
				return c.FileFS("index.html", dist)
			}
			return err
		}

		return nil
	})
}
