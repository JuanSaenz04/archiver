package api

import (
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

//go:embed dist
var frontendDist embed.FS

type RouteConfig struct {
	AppPublicURL    string
	ReplayPublicURL string
}

func (handler *Handler) SetMainRoutes(e *echo.Echo, config RouteConfig) {
	dist := echo.MustSubFS(frontendDist, "dist")
	handler.setMainRoutes(e, config, dist)
}

func (handler *Handler) setMainRoutes(e *echo.Echo, config RouteConfig, dist fs.FS) {
	e.Use(middleware.Gzip())

	apiGroup := e.Group("/api")
	apiGroup.Use(requestLogger())
	apiGroup.Use(requireTrustedOrigin(config.AppPublicURL))

	apiGroup.GET("/config", func(c *echo.Context) error {
		c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
		return c.JSON(http.StatusOK, map[string]string{
			"replay_origin": config.ReplayPublicURL,
		})
	})
	apiGroup.POST("/jobs", handler.HandleNewJob)
	apiGroup.GET("/jobs", handler.HandleGetJobs)
	apiGroup.GET("/archives", handler.HandleGetArchives)
	apiGroup.GET("/archives/tags", handler.HandleGetArchiveTags)
	apiGroup.DELETE("/archives/:archiveId", handler.HandleDeleteArchive)
	apiGroup.PUT("/archives/:archiveId", handler.HandleModifyArchiveMetadata)

	e.GET("/*", func(c *echo.Context) error {
		path := c.Request().URL.Path

		// API requests and replay assets must never fall back to the frontend.
		if strings.HasPrefix(path, "/api") || path == "/viewer.html" || path == "/replay" || strings.HasPrefix(path, "/replay/") {
			return echo.ErrNotFound
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		if err := c.FileFS(cleanPath, dist); err != nil {
			if errors.Is(err, echo.ErrNotFound) {
				return c.FileFS("index.html", dist)
			}
			return err
		}

		return nil
	})
}

func (handler *Handler) SetReplayRoutes(e *echo.Echo, config RouteConfig) {
	dist := echo.MustSubFS(frontendDist, "dist")
	handler.setReplayRoutes(e, config, dist)
}

func (handler *Handler) setReplayRoutes(e *echo.Echo, config RouteConfig, dist fs.FS) {
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Skipper: func(c *echo.Context) bool {
			return strings.HasPrefix(c.Request().URL.Path, "/archives/")
		},
	}))

	e.GET("/viewer.html", func(c *echo.Context) error {
		c.Response().Header().Set("Content-Security-Policy", "frame-ancestors "+config.AppPublicURL)
		c.Response().Header().Set("Referrer-Policy", "no-referrer")
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")
		return c.FileFS("viewer.html", dist)
	})
	e.GET("/replay/*", func(c *echo.Context) error {
		cleanPath := strings.TrimPrefix(c.Request().URL.Path, "/")
		return c.FileFS(cleanPath, dist)
	})
	e.GET("/archives/:archiveId", handler.HandleGetArchive)
	e.HEAD("/archives/:archiveId", handler.HandleGetArchive)
}

func requestLogger() echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:   true,
		LogURI:      true,
		LogRemoteIP: true,
		LogMethod:   true,
		LogLatency:  true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			log := slog.Info
			if v.Status >= 500 {
				log = slog.Error
			} else if v.Status >= 400 {
				log = slog.Warn
			}
			log("request",
				"method", v.Method,
				"uri", v.URI,
				"status", v.Status,
				"remote_ip", v.RemoteIP,
				"latency", v.Latency.String(),
			)
			return nil
		},
	})
}

func requireTrustedOrigin(appPublicURL string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			switch c.Request().Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				return next(c)
			}

			if c.Request().Header.Get(echo.HeaderOrigin) != appPublicURL {
				return echo.NewHTTPError(http.StatusForbidden, "request origin not allowed")
			}

			return next(c)
		}
	}
}
