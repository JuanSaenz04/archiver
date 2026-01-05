package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/labstack/echo/v4"
)

func (handler *Handler) HandleGetArchives(c echo.Context) error {
	pattern := filepath.Join(handler.archivesDir, "*.wacz")

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
	archiveName := c.Param("archiveName")
	archiveName = filepath.Base(archiveName)

	path := filepath.Join(handler.archivesDir, archiveName)

	err := c.File(path)
	if err != nil {
		return respondWithError(http.StatusNotFound, "Archive not found", c)
	}

	return nil
}

func (handler *Handler) HandleDeleteArchive(c echo.Context) error {
	archiveName := c.Param("archiveName")
	archiveName = filepath.Base(archiveName)

	path := filepath.Join(handler.archivesDir, archiveName)

	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return respondWithError(http.StatusNotFound, "Archive not found", c)
		}

		return respondWithError(http.StatusInternalServerError, "Internal server error", c)
	}

	return c.NoContent(http.StatusNoContent)
}

func (handler *Handler) HandleModifyArchiveName(c echo.Context) error {
	newArchive := &models.Archive{}

	if err := c.Bind(newArchive); err != nil {
		return respondWithError(http.StatusBadRequest, "Malformed request", c)
	}

	newName := newArchive.Name
	newName = filepath.Base(newName)
	newName = strings.ReplaceAll(newName, " ", "-")
	if !strings.HasSuffix(newName, ".wacz") {
		newName += ".wacz"
	}

	archiveName := c.Param("archiveName")
	archiveName = filepath.Base(archiveName)

	path := filepath.Join(handler.archivesDir, archiveName)

	newPath := filepath.Join(handler.archivesDir, newName)

	if _, err := os.Stat(newPath); err == nil {
		return respondWithError(http.StatusConflict, fmt.Sprintf("Archive with name %s already exists", newName), c)
	}

	if err := os.Rename(path, newPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return respondWithError(http.StatusNotFound, "Archive not found", c)
		}

		return respondWithError(http.StatusInternalServerError, "Internal server error", c)
	}

	return c.NoContent(http.StatusNoContent)
}
