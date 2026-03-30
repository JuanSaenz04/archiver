package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/JuanSaenz04/archiver/internal/store"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	_ "modernc.org/sqlite"
)

func openArchiveStore(t *testing.T) (*store.ArchiveStore, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "archiver.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	if err := s.RunMigrations(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() {
		if err := s.Close(); err != nil && !strings.Contains(err.Error(), "database is closed") {
			t.Fatalf("close store: %v", err)
		}
	})

	return s, dbPath
}

func insertArchiveFixture(t *testing.T, s *store.ArchiveStore, archive models.Archive) {
	t.Helper()

	if err := s.Insert(context.Background(), archive); err != nil {
		t.Fatalf("insert archive fixture: %v", err)
	}
}

func countArchiveByName(t *testing.T, s *store.ArchiveStore, name string) int {
	t.Helper()

	archives, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}

	count := 0
	for _, archive := range archives {
		if archive.Name == name {
			count++
		}
	}

	return count
}

func TestHandleGetArchives(t *testing.T) {
	archiveStore, _ := openArchiveStore(t)
	handler := &Handler{archiveStore: archiveStore}
	e := echo.New()

	insertArchiveFixture(t, archiveStore, models.Archive{
		ID:          uuid.New(),
		Name:        "archive1.wacz",
		Description: "first",
		SourceURL:   "https://one.example",
		Tags:        []string{"news"},
		CreatedAt:   time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC),
		SizeBytes:   1536,
	})
	insertArchiveFixture(t, archiveStore, models.Archive{
		ID:          uuid.New(),
		Name:        "archive2.wacz",
		Description: "second",
		SourceURL:   "https://two.example",
		Tags:        []string{"tech", "go"},
		CreatedAt:   time.Date(2026, 3, 30, 11, 0, 0, 0, time.UTC),
		SizeBytes:   3072,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/archives", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if assert.NoError(t, handler.HandleGetArchives(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string][]models.Archive
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)

		archives := response["archives"]
		assert.Len(t, archives, 2)

		names := make([]string, 0, len(archives))
		for _, archive := range archives {
			names = append(names, archive.Name)
		}
		assert.ElementsMatch(t, []string{"archive1.wacz", "archive2.wacz"}, names)

		byName := make(map[string]models.Archive, len(archives))
		for _, archive := range archives {
			byName[archive.Name] = archive
		}
		assert.Equal(t, int64(1536), byName["archive1.wacz"].SizeBytes)
		assert.Equal(t, int64(3072), byName["archive2.wacz"].SizeBytes)
	}
}

func TestHandleGetArchivesIncludesSizeBytesFromStoredArchiveMetadata(t *testing.T) {
	archiveStore, dbPath := openArchiveStore(t)
	handler := &Handler{archiveStore: archiveStore}
	e := echo.New()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open direct sqlite handle: %v", err)
	}
	defer db.Close()

	archiveID := uuid.New()
	createdAt := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	if _, err := db.ExecContext(
		context.Background(),
		"INSERT INTO archives (id, name, description, source_url, created_at, size_bytes) VALUES (?, ?, ?, ?, ?, ?);",
		archiveID,
		"seeded.wacz",
		"seeded",
		"https://seeded.example",
		createdAt,
		int64(8192),
	); err != nil {
		t.Fatalf("seed archive row: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "INSERT INTO tags (archive_id, tag) VALUES (?, ?);", archiveID, "seed"); err != nil {
		t.Fatalf("seed tag row: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/archives", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if assert.NoError(t, handler.HandleGetArchives(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string][]models.Archive
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)

		archives := response["archives"]
		assert.Len(t, archives, 1)
		assert.Equal(t, "seeded.wacz", archives[0].Name)
		assert.Equal(t, int64(8192), archives[0].SizeBytes)
	}
}

func TestHandleGetArchive(t *testing.T) {
	tempDir := t.TempDir()
	archiveName := "test.wacz"
	content := []byte("dummy wacz content")

	err := os.WriteFile(filepath.Join(tempDir, archiveName), content, 0644)
	if err != nil {
		t.Fatalf("failed to create dummy archive: %v", err)
	}

	handler := &Handler{archivesDir: tempDir}
	e := echo.New()

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/archives/"+archiveName, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("archiveName")
		c.SetParamValues(archiveName)

		if assert.NoError(t, handler.HandleGetArchive(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, content, rec.Body.Bytes())
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/archives/nonexistent.wacz", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("archiveName")
		c.SetParamValues("nonexistent.wacz")

		err := handler.HandleGetArchive(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("SanitizesArchiveName", func(t *testing.T) {
		sanitizedName := "test-archive.wacz"
		sanitizedContent := []byte("sanitized archive content")
		if err := os.WriteFile(filepath.Join(tempDir, sanitizedName), sanitizedContent, 0644); err != nil {
			t.Fatalf("failed to create sanitized archive: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/archives/test%20archive", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("archiveName")
		c.SetParamValues("test archive")

		if assert.NoError(t, handler.HandleGetArchive(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, sanitizedContent, rec.Body.Bytes())
		}
	})
}

func TestHandleDeleteArchive(t *testing.T) {
	tempDir := t.TempDir()
	archiveStore, _ := openArchiveStore(t)
	handler := &Handler{archivesDir: tempDir, archiveStore: archiveStore}
	e := echo.New()

	t.Run("Success", func(t *testing.T) {
		archiveName := "to_delete.wacz"
		filePath := filepath.Join(tempDir, archiveName)
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		insertArchiveFixture(t, archiveStore, models.Archive{
			ID:          uuid.New(),
			Name:        archiveName,
			Description: "to delete",
			SourceURL:   "https://delete.example",
			Tags:        []string{"tag"},
			CreatedAt:   time.Now().UTC(),
		})

		req := httptest.NewRequest(http.MethodDelete, "/api/archives/"+archiveName, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("archiveName")
		c.SetParamValues(archiveName)

		if assert.NoError(t, handler.HandleDeleteArchive(c)) {
			assert.Equal(t, http.StatusNoContent, rec.Code)

			_, statErr := os.Stat(filePath)
			assert.True(t, errors.Is(statErr, os.ErrNotExist), "file should be deleted")

			assert.Equal(t, 0, countArchiveByName(t, archiveStore, archiveName))
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		archiveName := "non_existent.wacz"
		req := httptest.NewRequest(http.MethodDelete, "/api/archives/"+archiveName, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("archiveName")
		c.SetParamValues(archiveName)

		if assert.NoError(t, handler.HandleDeleteArchive(c)) {
			assert.Equal(t, http.StatusNotFound, rec.Code)
		}
	})

	t.Run("DBNotFoundRollsBackFile", func(t *testing.T) {
		archiveName := "file_only.wacz"
		filePath := filepath.Join(tempDir, archiveName)
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/archives/"+archiveName, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("archiveName")
		c.SetParamValues(archiveName)

		if assert.NoError(t, handler.HandleDeleteArchive(c)) {
			assert.Equal(t, http.StatusNotFound, rec.Code)

			_, statErr := os.Stat(filePath)
			assert.NoError(t, statErr, "file should be restored on DB delete failure")
		}
	})
}

func TestHandleModifyArchiveName(t *testing.T) {
	tempDir := t.TempDir()
	archiveStore, _ := openArchiveStore(t)
	handler := &Handler{archivesDir: tempDir, archiveStore: archiveStore}
	e := echo.New()

	t.Run("Success", func(t *testing.T) {
		oldName := "old.wacz"
		newName := "new.wacz"
		filePath := filepath.Join(tempDir, oldName)
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("write old file: %v", err)
		}

		insertArchiveFixture(t, archiveStore, models.Archive{
			ID:          uuid.New(),
			Name:        oldName,
			Description: "old",
			SourceURL:   "https://old.example",
			Tags:        []string{},
			CreatedAt:   time.Now().UTC(),
		})

		body, _ := json.Marshal(map[string]string{"name": "new"})
		req := httptest.NewRequest(http.MethodPut, "/api/archives/"+oldName, strings.NewReader(string(body)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("archiveName")
		c.SetParamValues(oldName)

		if assert.NoError(t, handler.HandleModifyArchiveName(c)) {
			assert.Equal(t, http.StatusNoContent, rec.Code)

			_, err := os.Stat(filepath.Join(tempDir, oldName))
			assert.True(t, errors.Is(err, os.ErrNotExist))
			_, err = os.Stat(filepath.Join(tempDir, newName))
			assert.NoError(t, err)

			assert.Equal(t, 0, countArchiveByName(t, archiveStore, oldName))
			assert.Equal(t, 1, countArchiveByName(t, archiveStore, newName))
		}
	})

	t.Run("Sanitization", func(t *testing.T) {
		oldName := "san.wacz"
		expectedName := "my-new-name.wacz"
		filePath := filepath.Join(tempDir, oldName)
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("write old file: %v", err)
		}

		insertArchiveFixture(t, archiveStore, models.Archive{
			ID:          uuid.New(),
			Name:        oldName,
			Description: "san",
			SourceURL:   "https://san.example",
			Tags:        []string{},
			CreatedAt:   time.Now().UTC(),
		})

		body, _ := json.Marshal(map[string]string{"name": "my new name"})
		req := httptest.NewRequest(http.MethodPut, "/api/archives/"+oldName, strings.NewReader(string(body)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("archiveName")
		c.SetParamValues(oldName)

		if assert.NoError(t, handler.HandleModifyArchiveName(c)) {
			assert.Equal(t, http.StatusNoContent, rec.Code)
			_, err := os.Stat(filepath.Join(tempDir, expectedName))
			assert.NoError(t, err)
			assert.Equal(t, 1, countArchiveByName(t, archiveStore, expectedName))
		}
	})

	t.Run("ConflictByExistingFile", func(t *testing.T) {
		oldName := "source.wacz"
		existingName := "existing.wacz"
		if err := os.WriteFile(filepath.Join(tempDir, oldName), []byte("content"), 0644); err != nil {
			t.Fatalf("write source file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tempDir, existingName), []byte("content"), 0644); err != nil {
			t.Fatalf("write existing file: %v", err)
		}

		insertArchiveFixture(t, archiveStore, models.Archive{
			ID:          uuid.New(),
			Name:        oldName,
			Description: "source",
			SourceURL:   "https://source.example",
			Tags:        []string{},
			CreatedAt:   time.Now().UTC(),
		})

		body, _ := json.Marshal(map[string]string{"name": "existing.wacz"})
		req := httptest.NewRequest(http.MethodPut, "/api/archives/"+oldName, strings.NewReader(string(body)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("archiveName")
		c.SetParamValues(oldName)

		if assert.NoError(t, handler.HandleModifyArchiveName(c)) {
			assert.Equal(t, http.StatusConflict, rec.Code)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"name": "really-missing.wacz"})
		req := httptest.NewRequest(http.MethodPut, "/api/archives/missing.wacz", strings.NewReader(string(body)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("archiveName")
		c.SetParamValues("missing.wacz")

		if assert.NoError(t, handler.HandleModifyArchiveName(c)) {
			assert.Equal(t, http.StatusNotFound, rec.Code)
		}
	})
}

func TestHandleModifyArchiveNameDatabaseConflictRollsBackFile(t *testing.T) {
	tempDir := t.TempDir()
	archiveStore, _ := openArchiveStore(t)
	handler := &Handler{archivesDir: tempDir, archiveStore: archiveStore}
	e := echo.New()

	oldName := "old-db-conflict.wacz"
	targetName := "target-db-conflict.wacz"

	if err := os.WriteFile(filepath.Join(tempDir, oldName), []byte("content"), 0644); err != nil {
		t.Fatalf("write old file: %v", err)
	}

	insertArchiveFixture(t, archiveStore, models.Archive{
		ID:          uuid.New(),
		Name:        oldName,
		Description: "old",
		SourceURL:   "https://old.example",
		Tags:        []string{},
		CreatedAt:   time.Now().UTC(),
	})
	insertArchiveFixture(t, archiveStore, models.Archive{
		ID:          uuid.New(),
		Name:        targetName,
		Description: "target",
		SourceURL:   "https://target.example",
		Tags:        []string{},
		CreatedAt:   time.Now().UTC(),
	})

	body, _ := json.Marshal(map[string]string{"name": targetName})
	req := httptest.NewRequest(http.MethodPut, "/api/archives/"+oldName, strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("archiveName")
	c.SetParamValues(oldName)

	if assert.NoError(t, handler.HandleModifyArchiveName(c)) {
		assert.Equal(t, http.StatusConflict, rec.Code)

		_, err := os.Stat(filepath.Join(tempDir, oldName))
		assert.NoError(t, err, "old file should be restored on DB conflict")

		_, err = os.Stat(filepath.Join(tempDir, targetName))
		assert.True(t, errors.Is(err, os.ErrNotExist), "target file should not be created on DB conflict")
	}
}

func TestHandleDeleteArchiveDatabaseErrorRollsBackFile(t *testing.T) {
	tempDir := t.TempDir()
	archiveStore, dbPath := openArchiveStore(t)
	handler := &Handler{archivesDir: tempDir, archiveStore: archiveStore}
	e := echo.New()

	archiveName := "rollback-delete.wacz"
	filePath := filepath.Join(tempDir, archiveName)
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	insertArchiveFixture(t, archiveStore, models.Archive{
		ID:          uuid.New(),
		Name:        archiveName,
		Description: "to rollback",
		SourceURL:   "https://rollback.example",
		Tags:        []string{},
		CreatedAt:   time.Now().UTC(),
	})

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open direct sqlite handle: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), "DROP TABLE archives;"); err != nil {
		t.Fatalf("drop archives table: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/archives/"+archiveName, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("archiveName")
	c.SetParamValues(archiveName)

	if assert.NoError(t, handler.HandleDeleteArchive(c)) {
		assert.Equal(t, http.StatusInternalServerError, rec.Code)

		_, statErr := os.Stat(filePath)
		assert.NoError(t, statErr, "file should be restored on DB failure")
	}
}

func TestArchiveStoreNameConflictTypeIsMatched(t *testing.T) {
	err := store.ErrArchiveNameConflict
	assert.True(t, errors.Is(err, store.ErrArchiveNameConflict))
	assert.False(t, errors.Is(err, sql.ErrNoRows))
}
