package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/JuanSaenz04/archiver/internal/store"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const (
	errArchiveNotFound     = "Archive not found"
	errInternalServerError = "Internal server error"
	errInvalidId           = "Invalid archive ID"
)

func (handler *Handler) HandleGetArchives(c *echo.Context) error {
	archives, err := handler.archiveStore.List(c.Request().Context())
	if err != nil {
		slog.Error("failed to list archives", "error", err)
		return respondWithError(http.StatusInternalServerError, errInternalServerError, c)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"archives": archives,
	})
}

func (handler *Handler) HandleGetArchive(c *echo.Context) error {
	archiveId, err := uuid.Parse(c.Param("archiveId"))
	if err != nil {
		return respondWithError(http.StatusBadRequest, errInvalidId, c)
	}

	filename, err := handler.archiveStore.GetFilename(c.Request().Context(), archiveId)
	if err != nil {
		if errors.Is(err, store.ErrArchiveNotFound) {
			return respondWithError(http.StatusNotFound, errArchiveNotFound, c)
		} else {
			return respondWithError(http.StatusInternalServerError, errInternalServerError, c)
		}
	}

	err = c.FileFS(filename, echo.NewDefaultFS(handler.archivesDir))
	if err != nil {
		return respondWithError(http.StatusNotFound, errArchiveNotFound, c)
	}

	return nil
}

func (handler *Handler) HandleDeleteArchive(c *echo.Context) error {
	archiveId, err := uuid.Parse(c.Param("archiveId"))
	if err != nil  {
		return respondWithError(http.StatusBadRequest, errInvalidId, c)
	}

	filename, err := handler.archiveStore.GetFilename(c.Request().Context(), archiveId)
	if err != nil {
		if errors.Is(err, store.ErrArchiveNotFound) {
			return respondWithError(http.StatusNotFound, errArchiveNotFound, c)
		}
		return respondWithError(http.StatusInternalServerError, errInternalServerError, c)
	}
	
	path := filepath.Join(handler.archivesDir, filename)

	tmpDir, err := os.MkdirTemp(handler.archivesDir, "archiver")
	if err != nil {
		slog.Error("failed to create temporary directory for delete", "filename", filename, "error", err)
		return respondWithError(http.StatusInternalServerError, errInternalServerError, c)
	}

	tempArchiveName := filepath.Join(tmpDir, filename)

	if err := os.Rename(path, tempArchiveName); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return respondWithError(http.StatusNotFound, errArchiveNotFound, c)
		}

		slog.Error("failed to move archive to temporary location", "filename", filename, "path", path, "temp_path", tempArchiveName, "error", err)
		return respondWithError(http.StatusInternalServerError, errInternalServerError, c)
	}

	err = handler.archiveStore.Delete(c.Request().Context(), archiveId)
	if err != nil {
		if rollbackErr := os.Rename(tempArchiveName, path); rollbackErr != nil {
			slog.Error("failed to rollback archive file after delete error", "filename", filename, "temp_path", tempArchiveName, "path", path, "error", rollbackErr)
		}

		if errors.Is(err, store.ErrArchiveNotFound) {
			return respondWithError(http.StatusNotFound, errArchiveNotFound, c)
		}

		slog.Error("failed to delete archive metadata", "filename", filename, "error", err)
		return respondWithError(http.StatusInternalServerError, errInternalServerError, c)
	}

	if err := os.RemoveAll(tmpDir); err != nil {
		slog.Warn("failed to remove temporary directory after delete", "filename", filename, "tmp_dir", tmpDir, "error", err)
	}

	slog.Info("archive deleted", "filename", filename)

	return c.NoContent(http.StatusNoContent)
}

func (handler *Handler) HandleModifyArchiveMetadata(c *echo.Context) error {
	newArchive := &models.Archive{}

	if err := c.Bind(newArchive); err != nil {
		return respondWithError(http.StatusBadRequest, "Malformed request", c)
	}

	archiveId, err := uuid.Parse(c.Param("archiveId"))
	if err != nil {
		return respondWithError(http.StatusBadRequest, errInvalidId, c)
	}

	err = handler.archiveStore.UpdateMetadata(c.Request().Context(), archiveId, newArchive.Name, newArchive.Description, newArchive.Tags)
	if err != nil {
		if errors.Is(err, store.ErrArchiveNameConflict) {
			return respondWithError(http.StatusConflict, fmt.Sprintf("Archive with name %s already exists", newArchive.Name), c)
		}

		if errors.Is(err, store.ErrArchiveNotFound) {
			return respondWithError(http.StatusNotFound, errArchiveNotFound, c)
		}

		slog.Error("failed to rename archive metadata", "archive_id", archiveId, "new_name", newArchive.Name, "error", err)
		return respondWithError(http.StatusInternalServerError, errInternalServerError, c)
	}

	slog.Info("archive renamed", "archive_id", archiveId, "new_name", newArchive.Name)

	return c.NoContent(http.StatusNoContent)
}
