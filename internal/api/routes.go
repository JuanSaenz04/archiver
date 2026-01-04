package api

import (
	"github.com/labstack/echo/v4"
)

func (handler *Handler) SetRoutes(e *echo.Echo) {
	apiGroup := e.Group("/api")

	apiGroup.POST("/jobs", handler.HandleNewJob)
	apiGroup.GET("/jobs", handler.HandleGetJobs)
	apiGroup.GET("/archives", handler.HandleGetArchives)
	apiGroup.GET("/archives/:archiveName", handler.HandleGetArchive)
}
