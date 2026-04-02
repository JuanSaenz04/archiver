package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/JuanSaenz04/archiver/internal/archiveutil"
	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/JuanSaenz04/archiver/internal/store"
	"github.com/labstack/echo/v5"
)

func (handler *Handler) HandleGetArchives(c *echo.Context) error {
	archives, err := handler.archiveStore.List(c.Request().Context())
	if err != nil {
		slog.Error("failed to list archives", "error", err)
		return respondWithError(http.StatusInternalServerError, "Internal Server Error", c)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"archives": archives,
	})
}

func (handler *Handler) HandleGetArchive(c *echo.Context) error {
	archiveName, ok := archiveutil.NormalizeArchiveName(c.Param("archiveName"))
	if !ok {
		return respondWithError(http.StatusNotFound, "Archive not found", c)
	}

	err := c.FileFS(archiveName, echo.NewDefaultFS(handler.archivesDir))
	if err != nil {
		return respondWithError(http.StatusNotFound, "Archive not found", c)
	}

	return nil
}

func (handler *Handler) HandleDeleteArchive(c *echo.Context) error {
	archiveName, ok := archiveutil.NormalizeArchiveName(c.Param("archiveName"))
	if !ok {
		return respondWithError(http.StatusNotFound, "Archive not found", c)
	}

	path := filepath.Join(handler.archivesDir, archiveName)

	tmpDir, err := os.MkdirTemp(handler.archivesDir, "archiver")
	if err != nil {
		slog.Error("failed to create temporary directory for delete", "archive_name", archiveName, "error", err)
		return respondWithError(http.StatusInternalServerError, "Internal Server Error", c)
	}

	tempArchiveName := filepath.Join(tmpDir, archiveName)

	if err := os.Rename(path, tempArchiveName); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return respondWithError(http.StatusNotFound, "Archive not found", c)
		}

		slog.Error("failed to move archive to temporary location", "archive_name", archiveName, "path", path, "temp_path", tempArchiveName, "error", err)
		return respondWithError(http.StatusInternalServerError, "Internal server error", c)
	}

	err = handler.archiveStore.Delete(c.Request().Context(), archiveName)
	if err != nil {
		if rollbackErr := os.Rename(tempArchiveName, path); rollbackErr != nil {
			slog.Error("failed to rollback archive file after delete error", "archive_name", archiveName, "temp_path", tempArchiveName, "path", path, "error", rollbackErr)
		}

		if errors.Is(err, store.ErrArchiveNotFound) {
			return respondWithError(http.StatusNotFound, "Archive not found", c)
		}

		slog.Error("failed to delete archive metadata", "archive_name", archiveName, "error", err)
		return respondWithError(http.StatusInternalServerError, "Internal server error", c)
	}

	if err := os.RemoveAll(tmpDir); err != nil {
		slog.Warn("failed to remove temporary directory after delete", "archive_name", archiveName, "tmp_dir", tmpDir, "error", err)
	}

	slog.Info("archive deleted", "archive_name", archiveName)

	return c.NoContent(http.StatusNoContent)
}

func (handler *Handler) HandleModifyArchiveMetadata(c *echo.Context) error {
	newArchive := &models.Archive{}

	if err := c.Bind(newArchive); err != nil {
		return respondWithError(http.StatusBadRequest, "Malformed request", c)
	}

	newName, ok := archiveutil.NormalizeArchiveName(newArchive.Name)
	if !ok {
		return respondWithError(http.StatusBadRequest, "Malformed request", c)
	}

	archiveName, ok := archiveutil.NormalizeArchiveName(c.Param("archiveName"))
	if !ok {
		return respondWithError(http.StatusNotFound, "Archive not found", c)
	}

	path := filepath.Join(handler.archivesDir, archiveName)

	newPath := filepath.Join(handler.archivesDir, newName)

	if archiveName != newName {
		if _, err := os.Stat(newPath); err == nil {
			return respondWithError(http.StatusConflict, fmt.Sprintf("Archive with name %s already exists", newName), c)
		}

		if err := os.Rename(path, newPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return respondWithError(http.StatusNotFound, "Archive not found", c)
			}

			slog.Error("failed to rename archive file", "old_name", archiveName, "new_name", newName, "path", path, "new_path", newPath, "error", err)
			return respondWithError(http.StatusInternalServerError, "Internal server error", c)
		}
	}

	err := handler.archiveStore.UpdateMetadata(c.Request().Context(), archiveName, newName, newArchive.Description, newArchive.Tags)
	if err != nil {
		if rollbackErr := os.Rename(newPath, path); rollbackErr != nil {
			slog.Error("failed to rollback archive file rename", "old_name", archiveName, "new_name", newName, "path", path, "new_path", newPath, "error", rollbackErr)
		}

		if errors.Is(err, store.ErrArchiveNameConflict) {
			return respondWithError(http.StatusConflict, fmt.Sprintf("Archive with name %s already exists", newName), c)
		}

		if errors.Is(err, store.ErrArchiveNotFound) {
			return respondWithError(http.StatusNotFound, "Archive not found", c)
		}

		slog.Error("failed to rename archive metadata", "old_name", archiveName, "new_name", newName, "error", err)
		return respondWithError(http.StatusInternalServerError, "Internal server error", c)
	}

	slog.Info("archive renamed", "old_name", archiveName, "new_name", newName)

	return c.NoContent(http.StatusNoContent)
}
