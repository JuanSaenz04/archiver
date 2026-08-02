package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/JuanSaenz04/archiver/internal/store"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const (
	errArchiveNotFound     = "Archive not found"
	errInternalServerError = "Internal server error"
	errInvalidId           = "Invalid archive ID"
	errInvalidArchiveQuery = "Invalid archive query"
	defaultArchivePageSize = 30
	maxArchivePageSize     = 100
)

func (handler *Handler) HandleGetArchives(c *echo.Context) error {
	options, err := archiveListOptions(c.Request())
	if err != nil {
		return respondWithError(http.StatusBadRequest, errInvalidArchiveQuery, c)
	}

	page, err := handler.archiveStore.ListArchives(c.Request().Context(), options)
	if err != nil {
		slog.Error("failed to list archives", "error", err)
		return respondWithError(http.StatusInternalServerError, errInternalServerError, c)
	}

	var nextCursor string
	if page.NextCursor != nil {
		nextCursor, err = encodeArchiveCursor(*page.NextCursor)
		if err != nil {
			slog.Error("failed to encode archive cursor", "error", err)
			return respondWithError(http.StatusInternalServerError, errInternalServerError, c)
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"archives":    page.Archives,
		"next_cursor": nextCursor,
	})
}

func (handler *Handler) HandleGetArchiveTags(c *echo.Context) error {
	tags, err := handler.archiveStore.ListTags(c.Request().Context())
	if err != nil {
		slog.Error("failed to list archive tags", "error", err)
		return respondWithError(http.StatusInternalServerError, errInternalServerError, c)
	}
	return c.JSON(http.StatusOK, map[string]any{"tags": tags})
}

func archiveListOptions(request *http.Request) (store.ListArchivesOptions, error) {
	query := request.URL.Query()
	options := store.ListArchivesOptions{
		Tags:   uniqueNonEmpty(query["tag"]),
		Search: strings.TrimSpace(query.Get("q")),
	}

	fromValue, toValue := query.Get("from"), query.Get("to")
	if (fromValue == "") != (toValue == "") {
		return options, errors.New("from and to must be provided together")
	}
	if fromValue != "" {
		from, err := time.Parse(time.RFC3339Nano, fromValue)
		if err != nil {
			return options, err
		}
		to, err := time.Parse(time.RFC3339Nano, toValue)
		if err != nil || !from.Before(to) {
			return options, errors.New("invalid archive range")
		}
		if query.Get("cursor") != "" || query.Get("limit") != "" {
			return options, errors.New("range queries cannot be paginated")
		}
		options.CreatedFrom = &from
		options.CreatedBefore = &to
		return options, nil
	}

	limit := defaultArchivePageSize
	if value := query.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > maxArchivePageSize {
			return options, errors.New("invalid limit")
		}
		limit = parsed
	}
	options.Limit = limit

	if value := query.Get("cursor"); value != "" {
		cursor, err := decodeArchiveCursor(value)
		if err != nil {
			return options, err
		}
		options.Cursor = &cursor
	}
	return options, nil
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func encodeArchiveCursor(cursor store.ArchiveCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeArchiveCursor(value string) (store.ArchiveCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return store.ArchiveCursor{}, err
	}
	var cursor store.ArchiveCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return store.ArchiveCursor{}, err
	}
	if cursor.CreatedAt.IsZero() || cursor.ID == uuid.Nil {
		return store.ArchiveCursor{}, errors.New("invalid cursor")
	}
	return cursor, nil
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
	if err != nil {
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
		if errors.Is(err, store.ErrArchiveNotFound) {
			return respondWithError(http.StatusNotFound, errArchiveNotFound, c)
		}

		slog.Error("failed to rename archive metadata", "archive_id", archiveId, "new_name", newArchive.Name, "error", err)
		return respondWithError(http.StatusInternalServerError, errInternalServerError, c)
	}

	slog.Info("archive renamed", "archive_id", archiveId, "new_name", newArchive.Name)

	return c.NoContent(http.StatusNoContent)
}
