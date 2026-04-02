package api

import "github.com/labstack/echo/v5"

func respondWithError(code int, message string, c *echo.Context) error {
	return c.JSON(code, map[string]string{"error": message})
}
