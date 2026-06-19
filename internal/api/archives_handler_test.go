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
	"github.com/labstack/echo/v5"
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
		Filename:    "archive1.wacz",
		Description: "first",
		SourceURL:   "https://one.example",
		Tags:        []string{"news"},
		CreatedAt:   time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC),
		SizeBytes:   1536,
	})
	insertArchiveFixture(t, archiveStore, models.Archive{
		ID:          uuid.New(),
		Name:        "archive2.wacz",
		Filename:    "archive2.wacz",
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
		"INSERT INTO archives (id, name, filename, description, source_url, created_at, size_bytes) VALUES (?, ?, ?, ?, ?, ?, ?);",
		archiveID,
		"seeded.wacz",
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

func TestHandleGetArchivesIncludesFilenameDistinctFromDisplayName(t *testing.T) {
	archiveStore, dbPath := openArchiveStore(t)
	db := openArchiveDBWithFilenameColumn(t, dbPath)
	defer db.Close()

	archiveID := uuid.New()
	seedArchiveWithFilename(t, db, archiveID, "Human Readable Title", "human-readable-title.wacz")

	handler := &Handler{archiveStore: archiveStore}
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/api/archives", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if assert.NoError(t, handler.HandleGetArchives(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)

		var response struct {
			Archives []map[string]any `json:"archives"`
		}
		if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response)) && assert.Len(t, response.Archives, 1) {
			archive := response.Archives[0]
			assert.Equal(t, archiveID.String(), archive["id"])
			assert.Equal(t, "Human Readable Title", archive["name"])
			assert.Equal(t, "human-readable-title.wacz", archive["filename"])
		}
	}
}

func TestHandleGetArchive(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tempDir := t.TempDir()
		archiveStore, _ := openArchiveStore(t)
		archiveID := uuid.New()
		filename := "stored-file.wacz"
		content := []byte("dummy wacz content")

		if err := os.WriteFile(filepath.Join(tempDir, filename), content, 0644); err != nil {
			t.Fatalf("write archive file: %v", err)
		}
		insertArchiveFixture(t, archiveStore, models.Archive{
			ID:          archiveID,
			Name:        "Display Name",
			Filename:    filename,
			Description: "stored file",
			SourceURL:   "https://example.com/archive",
			CreatedAt:   time.Now().UTC(),
		})

		handler := &Handler{archivesDir: tempDir, archiveStore: archiveStore}
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/archives/"+archiveID.String(), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues([]echo.PathValue{{Name: "archiveId", Value: archiveID.String()}})

		if assert.NoError(t, handler.HandleGetArchive(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, content, rec.Body.Bytes())
		}
	})

	t.Run("InvalidID", func(t *testing.T) {
		handler := &Handler{}
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/archives/not-a-uuid", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues([]echo.PathValue{{Name: "archiveId", Value: "not-a-uuid"}})

		if assert.NoError(t, handler.HandleGetArchive(c)) {
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("MetadataNotFound", func(t *testing.T) {
		tempDir := t.TempDir()
		archiveStore, _ := openArchiveStore(t)
		archiveID := uuid.New()
		handler := &Handler{archivesDir: tempDir, archiveStore: archiveStore}
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/archives/"+archiveID.String(), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues([]echo.PathValue{{Name: "archiveId", Value: archiveID.String()}})

		if assert.NoError(t, handler.HandleGetArchive(c)) {
			assert.Equal(t, http.StatusNotFound, rec.Code)
		}
	})

	t.Run("FileNotFound", func(t *testing.T) {
		tempDir := t.TempDir()
		archiveStore, _ := openArchiveStore(t)
		archiveID := uuid.New()
		insertArchiveFixture(t, archiveStore, models.Archive{
			ID:          archiveID,
			Name:        "Missing File",
			Filename:    "missing-file.wacz",
			Description: "metadata exists",
			SourceURL:   "https://example.com/missing",
			CreatedAt:   time.Now().UTC(),
		})

		handler := &Handler{archivesDir: tempDir, archiveStore: archiveStore}
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/archives/"+archiveID.String(), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues([]echo.PathValue{{Name: "archiveId", Value: archiveID.String()}})

		if assert.NoError(t, handler.HandleGetArchive(c)) {
			assert.Equal(t, http.StatusNotFound, rec.Code)
		}
	})
}

func TestHandleDeleteArchive(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tempDir := t.TempDir()
		archiveStore, _ := openArchiveStore(t)
		archiveID := uuid.New()
		filename := "delete-me.wacz"
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("write archive file: %v", err)
		}
		insertArchiveFixture(t, archiveStore, models.Archive{
			ID:          archiveID,
			Name:        "Delete Me",
			Filename:    filename,
			Description: "to delete",
			SourceURL:   "https://delete.example",
			Tags:        []string{"tag"},
			CreatedAt:   time.Now().UTC(),
		})

		handler := &Handler{archivesDir: tempDir, archiveStore: archiveStore}
		e := echo.New()
		req := httptest.NewRequest(http.MethodDelete, "/api/archives/"+archiveID.String(), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues([]echo.PathValue{{Name: "archiveId", Value: archiveID.String()}})

		if assert.NoError(t, handler.HandleDeleteArchive(c)) {
			assert.Equal(t, http.StatusNoContent, rec.Code)

			_, statErr := os.Stat(filePath)
			assert.True(t, errors.Is(statErr, os.ErrNotExist), "file should be deleted")
			assert.Equal(t, 0, countArchiveByName(t, archiveStore, "Delete Me"))
		}
	})

	t.Run("InvalidID", func(t *testing.T) {
		handler := &Handler{}
		e := echo.New()
		req := httptest.NewRequest(http.MethodDelete, "/api/archives/not-a-uuid", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues([]echo.PathValue{{Name: "archiveId", Value: "not-a-uuid"}})

		if assert.NoError(t, handler.HandleDeleteArchive(c)) {
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("MetadataNotFound", func(t *testing.T) {
		tempDir := t.TempDir()
		archiveStore, _ := openArchiveStore(t)
		archiveID := uuid.New()
		handler := &Handler{archivesDir: tempDir, archiveStore: archiveStore}
		e := echo.New()
		req := httptest.NewRequest(http.MethodDelete, "/api/archives/"+archiveID.String(), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues([]echo.PathValue{{Name: "archiveId", Value: archiveID.String()}})

		if assert.NoError(t, handler.HandleDeleteArchive(c)) {
			assert.Equal(t, http.StatusNotFound, rec.Code)
		}
	})

	t.Run("FileNotFound", func(t *testing.T) {
		tempDir := t.TempDir()
		archiveStore, _ := openArchiveStore(t)
		archiveID := uuid.New()
		insertArchiveFixture(t, archiveStore, models.Archive{
			ID:          archiveID,
			Name:        "Missing File",
			Filename:    "missing-delete.wacz",
			Description: "metadata exists",
			SourceURL:   "https://delete.example/missing",
			CreatedAt:   time.Now().UTC(),
		})

		handler := &Handler{archivesDir: tempDir, archiveStore: archiveStore}
		e := echo.New()
		req := httptest.NewRequest(http.MethodDelete, "/api/archives/"+archiveID.String(), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues([]echo.PathValue{{Name: "archiveId", Value: archiveID.String()}})

		if assert.NoError(t, handler.HandleDeleteArchive(c)) {
			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.Equal(t, 1, countArchiveByName(t, archiveStore, "Missing File"))
		}
	})

	t.Run("DatabaseErrorRollsBackFile", func(t *testing.T) {
		tempDir := t.TempDir()
		archiveStore, dbPath := openArchiveStore(t)
		db := openArchiveDBWithFilenameColumn(t, dbPath)
		defer db.Close()

		archiveID := uuid.New()
		filename := "rollback-delete.wacz"
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("write archive file: %v", err)
		}
		seedArchiveWithFilename(t, db, archiveID, "Rollback Delete", filename)
		if _, err := db.ExecContext(context.Background(), `CREATE TRIGGER fail_archive_delete BEFORE DELETE ON archives BEGIN SELECT RAISE(ABORT, 'forced delete failure'); END;`); err != nil {
			t.Fatalf("create failing delete trigger: %v", err)
		}

		handler := &Handler{archivesDir: tempDir, archiveStore: archiveStore}
		e := echo.New()
		req := httptest.NewRequest(http.MethodDelete, "/api/archives/"+archiveID.String(), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues([]echo.PathValue{{Name: "archiveId", Value: archiveID.String()}})

		if assert.NoError(t, handler.HandleDeleteArchive(c)) {
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
			_, statErr := os.Stat(filePath)
			assert.NoError(t, statErr, "file should be restored on DB delete failure")

			var count int
			if assert.NoError(t, db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM archives WHERE id = ?;", archiveID).Scan(&count)) {
				assert.Equal(t, 1, count)
			}
		}
	})
}

func TestHandleModifyArchiveMetadata(t *testing.T) {
	t.Run("SuccessUpdatesMetadataByIDWithoutRenamingFile", func(t *testing.T) {
		tempDir := t.TempDir()
		archiveStore, dbPath := openArchiveStore(t)
		db := openArchiveDBWithFilenameColumn(t, dbPath)
		defer db.Close()

		archiveID := uuid.New()
		filename := "stable-file.wacz"
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("write archive file: %v", err)
		}
		seedArchiveWithFilename(t, db, archiveID, "Original Title", filename)
		if _, err := db.ExecContext(context.Background(), "INSERT INTO tags (archive_id, tag) VALUES (?, ?);", archiveID, "old"); err != nil {
			t.Fatalf("seed tag: %v", err)
		}

		handler := &Handler{archivesDir: tempDir, archiveStore: archiveStore}
		e := echo.New()
		body, _ := json.Marshal(map[string]any{
			"name":        "Renamed Title",
			"description": "updated description",
			"tags":        []string{"new", "metadata"},
		})
		req := httptest.NewRequest(http.MethodPut, "/api/archives/"+archiveID.String(), strings.NewReader(string(body)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues([]echo.PathValue{{Name: "archiveId", Value: archiveID.String()}})

		if assert.NoError(t, handler.HandleModifyArchiveMetadata(c)) {
			assert.Equal(t, http.StatusNoContent, rec.Code)

			_, err := os.Stat(filePath)
			assert.NoError(t, err, "metadata update should not rename or remove the physical file")

			_, err = os.Stat(filepath.Join(tempDir, "Renamed-Title.wacz"))
			assert.True(t, errors.Is(err, os.ErrNotExist), "display name should not become a new filename")

			var name, description, gotFilename string
			if assert.NoError(t, db.QueryRowContext(context.Background(), "SELECT name, description, filename FROM archives WHERE id = ?;", archiveID).Scan(&name, &description, &gotFilename)) {
				assert.Equal(t, "Renamed Title", name)
				assert.Equal(t, "updated description", description)
				assert.Equal(t, filename, gotFilename)
			}

			rows, err := db.QueryContext(context.Background(), "SELECT tag FROM tags WHERE archive_id = ? ORDER BY tag;", archiveID)
			if assert.NoError(t, err) {
				defer rows.Close()
				tags := make([]string, 0)
				for rows.Next() {
					var tag string
					if err := rows.Scan(&tag); err != nil {
						t.Fatalf("scan tag: %v", err)
					}
					tags = append(tags, tag)
				}
				assert.NoError(t, rows.Err())
				assert.Equal(t, []string{"metadata", "new"}, tags)
			}
		}
	})

	t.Run("PreservesUnnormalizedDisplayName", func(t *testing.T) {
		archiveStore, _ := openArchiveStore(t)
		archiveID := uuid.New()
		insertArchiveFixture(t, archiveStore, models.Archive{
			ID:          archiveID,
			Name:        "Original Name",
			Filename:    "original-name.wacz",
			Description: "before",
			SourceURL:   "https://metadata.example",
			Tags:        []string{},
			CreatedAt:   time.Now().UTC(),
		})

		handler := &Handler{archiveStore: archiveStore}
		e := echo.New()
		body, _ := json.Marshal(map[string]any{"name": "my new name", "description": "after", "tags": []string{"san"}})
		req := httptest.NewRequest(http.MethodPut, "/api/archives/"+archiveID.String(), strings.NewReader(string(body)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues([]echo.PathValue{{Name: "archiveId", Value: archiveID.String()}})

		if assert.NoError(t, handler.HandleModifyArchiveMetadata(c)) {
			assert.Equal(t, http.StatusNoContent, rec.Code)

			archives, err := archiveStore.List(context.Background())
			if assert.NoError(t, err) && assert.Len(t, archives, 1) {
				assert.Equal(t, "my new name", archives[0].Name)
				assert.Equal(t, "original-name.wacz", archives[0].Filename)
				assert.Equal(t, "after", archives[0].Description)
				assert.ElementsMatch(t, []string{"san"}, archives[0].Tags)
			}
		}
	})

	t.Run("InvalidID", func(t *testing.T) {
		handler := &Handler{}
		e := echo.New()
		body, _ := json.Marshal(map[string]any{"name": "unused", "description": "unused", "tags": []string{}})
		req := httptest.NewRequest(http.MethodPut, "/api/archives/not-a-uuid", strings.NewReader(string(body)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues([]echo.PathValue{{Name: "archiveId", Value: "not-a-uuid"}})

		if assert.NoError(t, handler.HandleModifyArchiveMetadata(c)) {
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		archiveStore, _ := openArchiveStore(t)
		archiveID := uuid.New()
		handler := &Handler{archiveStore: archiveStore}
		e := echo.New()
		body, _ := json.Marshal(map[string]any{"name": "missing", "description": "missing", "tags": []string{"x"}})
		req := httptest.NewRequest(http.MethodPut, "/api/archives/"+archiveID.String(), strings.NewReader(string(body)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues([]echo.PathValue{{Name: "archiveId", Value: archiveID.String()}})

		if assert.NoError(t, handler.HandleModifyArchiveMetadata(c)) {
			assert.Equal(t, http.StatusNotFound, rec.Code)
		}
	})

	t.Run("ConflictByExistingName", func(t *testing.T) {
		tempDir := t.TempDir()
		archiveStore, _ := openArchiveStore(t)
		archiveID := uuid.New()
		filename := "source-file.wacz"
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("write source file: %v", err)
		}

		insertArchiveFixture(t, archiveStore, models.Archive{
			ID:          archiveID,
			Name:        "Source Title",
			Filename:    filename,
			Description: "source",
			SourceURL:   "https://source.example",
			Tags:        []string{},
			CreatedAt:   time.Now().UTC(),
		})
		insertArchiveFixture(t, archiveStore, models.Archive{
			ID:          uuid.New(),
			Name:        "Existing Title",
			Filename:    "existing-file.wacz",
			Description: "existing",
			SourceURL:   "https://existing.example",
			Tags:        []string{},
			CreatedAt:   time.Now().UTC(),
		})

		handler := &Handler{archivesDir: tempDir, archiveStore: archiveStore}
		e := echo.New()
		body, _ := json.Marshal(map[string]any{"name": "Existing Title", "description": "should fail", "tags": []string{"x"}})
		req := httptest.NewRequest(http.MethodPut, "/api/archives/"+archiveID.String(), strings.NewReader(string(body)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues([]echo.PathValue{{Name: "archiveId", Value: archiveID.String()}})

		if assert.NoError(t, handler.HandleModifyArchiveMetadata(c)) {
			assert.Equal(t, http.StatusConflict, rec.Code)

			_, err := os.Stat(filePath)
			assert.NoError(t, err, "metadata conflict should not affect the physical file")
			assert.Equal(t, 1, countArchiveByName(t, archiveStore, "Source Title"))
		}
	})
}

func TestArchiveStoreNameConflictTypeIsMatched(t *testing.T) {
	err := store.ErrArchiveNameConflict
	assert.True(t, errors.Is(err, store.ErrArchiveNameConflict))
	assert.False(t, errors.Is(err, sql.ErrNoRows))
}

func openArchiveDBWithFilenameColumn(t *testing.T, dbPath string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open direct sqlite handle: %v", err)
	}

	if !apiArchiveColumnExists(t, db, "filename") {
		if _, err := db.ExecContext(context.Background(), "ALTER TABLE archives ADD COLUMN filename TEXT;"); err != nil {
			db.Close()
			t.Fatalf("add filename column fixture: %v", err)
		}
	}

	return db
}

func seedArchiveWithFilename(t *testing.T, db *sql.DB, id uuid.UUID, name, filename string) {
	t.Helper()

	if _, err := db.ExecContext(
		context.Background(),
		"INSERT INTO archives (id, name, filename, description, source_url, created_at, size_bytes) VALUES (?, ?, ?, ?, ?, ?, ?);",
		id,
		name,
		filename,
		"description",
		"https://example.com/archive",
		time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		int64(42),
	); err != nil {
		t.Fatalf("seed archive row: %v", err)
	}
}

func apiArchiveColumnExists(t *testing.T, db *sql.DB, column string) bool {
	t.Helper()

	rows, err := db.QueryContext(context.Background(), "PRAGMA table_info(archives);")
	if err != nil {
		t.Fatalf("read archives table info: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			t.Fatalf("scan archives table info: %v", err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate archives table info: %v", err)
	}

	return false
}
