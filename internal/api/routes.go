package api

import (
	"github.com/labstack/echo/v4"
)

func (handler *Handler) SetRoutes(e *echo.Echo) {
	e.POST("/jobs", handler.HandleNewJob)
}
