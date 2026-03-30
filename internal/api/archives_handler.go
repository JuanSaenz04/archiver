package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/JuanSaenz04/archiver/internal/store"
	"github.com/labstack/echo/v4"
)

func (handler *Handler) HandleGetArchives(c echo.Context) error {
	archives, err := handler.archiveStore.List(c.Request().Context())
	if err != nil {
		// Log errors in the future
		return respondWithError(http.StatusInternalServerError, "Internal Server Error", c)
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

	tmpDir, err := os.MkdirTemp("", "archiver")
	if err != nil {
		return respondWithError(http.StatusInternalServerError, "Internal Server Error", c)
	}

	tempArchiveName := filepath.Join(tmpDir, archiveName)

	if err := os.Rename(path, tempArchiveName); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return respondWithError(http.StatusNotFound, "Archive not found", c)
		}

		return respondWithError(http.StatusInternalServerError, "Internal server error", c)
	}

	err = handler.archiveStore.Delete(c.Request().Context(), archiveName)
	if err != nil {
		// Attempt rollback. Log errors in the future.
		os.Rename(tempArchiveName, path)

		if errors.Is(err, store.ErrArchiveNotFound) {
			return respondWithError(http.StatusNotFound, "Archive not found", c)
		}

		return respondWithError(http.StatusInternalServerError, "Internal server error", c)
	}

	// Remove temp file permanently
	// Log errors in the future.
	os.RemoveAll(tmpDir)

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

	err := handler.archiveStore.Rename(c.Request().Context(), archiveName, newName)
	if err != nil {
		// Best-effort rollback
		// Log errors in the future
		os.Rename(newPath, path)

		if errors.Is(err, store.ErrArchiveNameConflict) {
			return respondWithError(http.StatusConflict, fmt.Sprintf("Archive with name %s already exists", newName), c)
		}

		if errors.Is(err, store.ErrArchiveNotFound) {
			return respondWithError(http.StatusNotFound, "Archive not found", c)
		}

		return respondWithError(http.StatusInternalServerError, "Internal server error", c)
	}

	return c.NoContent(http.StatusNoContent)
}
