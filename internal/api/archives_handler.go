package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/labstack/echo/v4"
)

func (handler *Handler) HandleGetArchives(c echo.Context) error {
	archivesDir := os.Getenv("ARCHIVES_DIR")

	pattern := filepath.Join(archivesDir, "*.wacz")

	files, err := filepath.Glob(pattern)
	if err != nil {
		return respondWithError(http.StatusInternalServerError, "Internal server error", c)
	}

	archives := make([]models.Archive, len(files))

	for i, path := range files {
		archives[i].Name = filepath.Base(path)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"archives": archives,
	})
}

func (handler *Handler) HandleGetArchive(c echo.Context) error {
	archivesDir := os.Getenv("ARCHIVES_DIR")

	archiveName := c.Param("archiveName")
	archiveName = filepath.Base(archiveName)

	path := filepath.Join(archivesDir, archiveName)

	err := c.File(path)
	if err != nil {
		return respondWithError(http.StatusNotFound, "Archive not found", c)
	}

	return nil
}
