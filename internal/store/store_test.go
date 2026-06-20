package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/google/uuid"
)

func newTestStore(t *testing.T) *ArchiveStore {
	t.Helper()

	dbPath := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	s.db.SetMaxOpenConns(5)
	s.db.SetMaxIdleConns(1)

	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})

	if err := s.RunMigrations(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	return s
}

func TestInsertAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	firstID := uuid.New()
	secondID := uuid.New()

	first := models.Archive{
		ID:          firstID,
		Name:        "First Archive",
		Filename:    "first.wacz",
		Description: "first archive",
		SourceURL:   "https://example.com/first",
		Tags:        []string{"news", "2026"},
		CreatedAt:   time.Date(2026, 3, 29, 10, 0, 0, 0, time.UTC),
		SizeBytes:   1024,
	}

	second := models.Archive{
		ID:          secondID,
		Name:        "Second Archive",
		Filename:    "second.wacz",
		Description: "second archive",
		SourceURL:   "https://example.com/second",
		Tags:        []string{"tech"},
		CreatedAt:   time.Date(2026, 3, 29, 11, 0, 0, 0, time.UTC),
		SizeBytes:   2048,
	}

	if err := s.Insert(ctx, first); err != nil {
		t.Fatalf("insert first archive: %v", err)
	}

	if err := s.Insert(ctx, second); err != nil {
		t.Fatalf("insert second archive: %v", err)
	}

	archives, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}

	if len(archives) != 2 {
		t.Fatalf("expected 2 archives, got %d", len(archives))
	}

	byName := make(map[string]models.Archive, len(archives))
	for _, archive := range archives {
		byName[archive.Name] = archive
	}

	if got := byName[first.Name]; got.ID != first.ID {
		t.Fatalf("first archive id mismatch: got %s, want %s", got.ID, first.ID)
	}

	if got := byName[first.Name]; len(got.Tags) != 2 {
		t.Fatalf("first archive tags length mismatch: got %d, want 2", len(got.Tags))
	}

	if got := byName[first.Name]; got.SizeBytes != first.SizeBytes {
		t.Fatalf("first archive size mismatch: got %d, want %d", got.SizeBytes, first.SizeBytes)
	}

	if got := byName[first.Name]; got.Filename != first.Filename {
		t.Fatalf("first archive filename mismatch: got %q, want %q", got.Filename, first.Filename)
	}

	if got := byName[second.Name]; got.ID != second.ID {
		t.Fatalf("second archive id mismatch: got %s, want %s", got.ID, second.ID)
	}

	if got := byName[second.Name]; len(got.Tags) != 1 || got.Tags[0] != "tech" {
		t.Fatalf("unexpected second archive tags: %#v", got.Tags)
	}

	if got := byName[second.Name]; got.SizeBytes != second.SizeBytes {
		t.Fatalf("second archive size mismatch: got %d, want %d", got.SizeBytes, second.SizeBytes)
	}

	if got := byName[second.Name]; got.Filename != second.Filename {
		t.Fatalf("second archive filename mismatch: got %q, want %q", got.Filename, second.Filename)
	}
}

func TestInsertReturnsNameConflictOnDuplicateName(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	createdAt := time.Date(2026, 3, 29, 10, 0, 0, 0, time.UTC)

	if err := s.Insert(ctx, models.Archive{
		ID:          uuid.New(),
		Name:        "Duplicate Title",
		Filename:    "dup-one.wacz",
		Description: "first",
		SourceURL:   "https://example.com/one",
		Tags:        []string{"a"},
		CreatedAt:   createdAt,
		SizeBytes:   128,
	}); err != nil {
		t.Fatalf("insert first archive: %v", err)
	}

	err := s.Insert(ctx, models.Archive{
		ID:          uuid.New(),
		Name:        "Duplicate Title",
		Filename:    "dup-two.wacz",
		Description: "second",
		SourceURL:   "https://example.com/two",
		Tags:        []string{"b"},
		CreatedAt:   createdAt.Add(time.Minute),
		SizeBytes:   256,
	})

	if !errors.Is(err, ErrArchiveNameConflict) {
		t.Fatalf("expected ErrArchiveNameConflict, got %v", err)
	}
}

func TestInsertUsesSQLiteDefaultCreatedAtWhenZero(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	before := time.Now().UTC().Add(-time.Second)

	if err := s.Insert(ctx, models.Archive{
		ID:          uuid.New(),
		Name:        "Default Created At",
		Filename:    "default-created-at.wacz",
		Description: "created with sqlite default",
		SourceURL:   "https://example.com/default",
		Tags:        []string{"default"},
		SizeBytes:   4096,
	}); err != nil {
		t.Fatalf("insert archive with zero created_at: %v", err)
	}

	after := time.Now().UTC().Add(time.Second)

	var createdAt time.Time
	if err := s.db.QueryRowContext(ctx, "SELECT created_at FROM archives WHERE name = ?;", "Default Created At").Scan(&createdAt); err != nil {
		t.Fatalf("read created_at: %v", err)
	}

	if createdAt.IsZero() {
		t.Fatal("expected sqlite to assign created_at, got zero value")
	}

	if createdAt.Before(before) || createdAt.After(after) {
		t.Fatalf("expected created_at between %s and %s, got %s", before, after, createdAt)
	}

	var sizeBytes int64
	if err := s.db.QueryRowContext(ctx, "SELECT size_bytes FROM archives WHERE name = ?;", "Default Created At").Scan(&sizeBytes); err != nil {
		t.Fatalf("read size_bytes: %v", err)
	}

	if sizeBytes != 4096 {
		t.Fatalf("expected size_bytes=%d, got %d", int64(4096), sizeBytes)
	}
}

func TestSyncFromDiskInsertsMissingWACZFiles(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	archivesDir := t.TempDir()

	firstPath := filepath.Join(archivesDir, "a.wacz")
	secondPath := filepath.Join(archivesDir, "b.WACZ")
	ignoredPath := filepath.Join(archivesDir, "ignore.txt")

	if err := os.WriteFile(firstPath, []byte("one"), 0644); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte("two"), 0644); err != nil {
		t.Fatalf("write second file: %v", err)
	}
	if err := os.WriteFile(ignoredPath, []byte("ignored"), 0644); err != nil {
		t.Fatalf("write ignored file: %v", err)
	}

	mtime := time.Date(2025, 12, 15, 8, 30, 0, 0, time.UTC)
	if err := os.Chtimes(firstPath, mtime, mtime); err != nil {
		t.Fatalf("set file time: %v", err)
	}

	if err := s.SyncFromDisk(ctx, archivesDir); err != nil {
		t.Fatalf("sync from disk: %v", err)
	}

	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM archives;").Scan(&count); err != nil {
		t.Fatalf("count archives: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 archives after sync, got %d", count)
	}

	var createdAt time.Time
	var sizeBytes int64
	if err := s.db.QueryRowContext(ctx, "SELECT created_at, size_bytes FROM archives WHERE name = ?;", "a").Scan(&createdAt, &sizeBytes); err != nil {
		t.Fatalf("read created_at for a: %v", err)
	}

	if !createdAt.Equal(mtime) {
		t.Fatalf("created_at mismatch: got %s, want %s", createdAt.UTC(), mtime.UTC())
	}

	if sizeBytes != int64(len("one")) {
		t.Fatalf("size_bytes mismatch for a.wacz: got %d, want %d", sizeBytes, len("one"))
	}

	if err := s.db.QueryRowContext(ctx, "SELECT size_bytes FROM archives WHERE name = ?;", "b").Scan(&sizeBytes); err != nil {
		t.Fatalf("read size_bytes for b: %v", err)
	}

	if sizeBytes != int64(len("two")) {
		t.Fatalf("size_bytes mismatch for b.WACZ: got %d, want %d", sizeBytes, len("two"))
	}

	if err := s.SyncFromDisk(ctx, archivesDir); err != nil {
		t.Fatalf("sync from disk second run: %v", err)
	}

	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM archives;").Scan(&count); err != nil {
		t.Fatalf("count archives second run: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 archives after second sync, got %d", count)
	}
}

func TestSyncFromDiskSkipsAlreadyInsertedFiles(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	archivesDir := t.TempDir()
	existingName := "existing"
	existingFilename := "existing.wacz"
	newName := "new"
	newFilename := "new.wacz"

	existingPath := filepath.Join(archivesDir, existingFilename)
	newPath := filepath.Join(archivesDir, newFilename)

	if err := os.WriteFile(existingPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0644); err != nil {
		t.Fatalf("write new file: %v", err)
	}

	existingID := uuid.New()
	existingCreatedAt := time.Date(2024, 7, 10, 9, 0, 0, 0, time.UTC)

	if err := s.Insert(ctx, models.Archive{
		ID:          existingID,
		Name:        existingName,
		Filename:    existingFilename,
		Description: "already in db",
		SourceURL:   "https://example.com/existing",
		Tags:        []string{"keep"},
		CreatedAt:   existingCreatedAt,
		SizeBytes:   777,
	}); err != nil {
		t.Fatalf("insert existing archive: %v", err)
	}

	if err := s.SyncFromDisk(ctx, archivesDir); err != nil {
		t.Fatalf("sync from disk: %v", err)
	}

	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM archives;").Scan(&count); err != nil {
		t.Fatalf("count archives: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 archives after sync, got %d", count)
	}

	var gotID uuid.UUID
	var gotDescription string
	var gotCreatedAt time.Time
	var gotSizeBytes int64
	if err := s.db.QueryRowContext(ctx, "SELECT id, description, created_at, size_bytes FROM archives WHERE name = ?;", existingName).Scan(&gotID, &gotDescription, &gotCreatedAt, &gotSizeBytes); err != nil {
		t.Fatalf("read existing archive row: %v", err)
	}

	if gotID != existingID {
		t.Fatalf("existing archive id changed: got %s, want %s", gotID, existingID)
	}
	if gotDescription != "already in db" {
		t.Fatalf("existing archive description changed: got %q", gotDescription)
	}
	if !gotCreatedAt.Equal(existingCreatedAt) {
		t.Fatalf("existing archive created_at changed: got %s, want %s", gotCreatedAt.UTC(), existingCreatedAt.UTC())
	}
	if gotSizeBytes != 777 {
		t.Fatalf("existing archive size_bytes changed: got %d, want %d", gotSizeBytes, 777)
	}

	var newArchiveSizeBytes int64
	if err := s.db.QueryRowContext(ctx, "SELECT size_bytes FROM archives WHERE name = ?;", newName).Scan(&newArchiveSizeBytes); err != nil {
		t.Fatalf("read new archive size_bytes: %v", err)
	}
	if newArchiveSizeBytes != int64(len("new")) {
		t.Fatalf("new archive size_bytes mismatch: got %d, want %d", newArchiveSizeBytes, len("new"))
	}
}

func TestUpdateMetadata(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	archiveID := uuid.New()
	createdAt := time.Date(2026, 3, 29, 14, 0, 0, 0, time.UTC)
	sizeBytes := int64(2048)

	if err := s.Insert(ctx, models.Archive{
		ID:          archiveID,
		Name:        "Old Title",
		Filename:    "old.wacz",
		Description: "old description",
		SourceURL:   "https://example.com/meta",
		Tags:        []string{"old", "tags"},
		CreatedAt:   createdAt,
		SizeBytes:   sizeBytes,
	}); err != nil {
		t.Fatalf("insert archive: %v", err)
	}

	t.Run("updates_name_description_and_replaces_tags", func(t *testing.T) {
		if err := s.UpdateMetadata(ctx, archiveID, "New Title", "new description", []string{"news", "2026"}); err != nil {
			t.Fatalf("update metadata: %v", err)
		}

		var oldCount int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM archives WHERE name = ?;", "Old Title").Scan(&oldCount); err != nil {
			t.Fatalf("count old name: %v", err)
		}
		if oldCount != 0 {
			t.Fatalf("expected old name to be removed, got count=%d", oldCount)
		}

		var gotID uuid.UUID
		var description, filename string
		var gotCreatedAt time.Time
		var gotSizeBytes int64
		if err := s.db.QueryRowContext(ctx, "SELECT id, description, filename, created_at, size_bytes FROM archives WHERE name = ?;", "New Title").Scan(&gotID, &description, &filename, &gotCreatedAt, &gotSizeBytes); err != nil {
			t.Fatalf("read updated archive: %v", err)
		}

		if gotID != archiveID {
			t.Fatalf("archive id changed: got %s, want %s", gotID, archiveID)
		}
		if description != "new description" {
			t.Fatalf("description mismatch: got %q, want %q", description, "new description")
		}
		if filename != "old.wacz" {
			t.Fatalf("filename changed: got %q, want %q", filename, "old.wacz")
		}
		if !gotCreatedAt.Equal(createdAt) {
			t.Fatalf("created_at changed: got %s, want %s", gotCreatedAt.UTC(), createdAt.UTC())
		}
		if gotSizeBytes != sizeBytes {
			t.Fatalf("size_bytes changed: got %d, want %d", gotSizeBytes, sizeBytes)
		}

		rows, err := s.db.QueryContext(ctx, "SELECT tag FROM tags WHERE archive_id = ?;", archiveID)
		if err != nil {
			t.Fatalf("query tags: %v", err)
		}
		defer rows.Close()

		gotTags := make(map[string]struct{})
		for rows.Next() {
			var tag string
			if err := rows.Scan(&tag); err != nil {
				t.Fatalf("scan tag: %v", err)
			}
			gotTags[tag] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate tags: %v", err)
		}

		if len(gotTags) != 2 {
			t.Fatalf("expected 2 tags, got %d", len(gotTags))
		}
		if _, ok := gotTags["news"]; !ok {
			t.Fatalf("missing tag %q", "news")
		}
		if _, ok := gotTags["2026"]; !ok {
			t.Fatalf("missing tag %q", "2026")
		}
	})

	t.Run("same_name_updates_description_and_replaces_tags", func(t *testing.T) {
		if err := s.UpdateMetadata(ctx, archiveID, "New Title", "desc with same name", []string{"updated"}); err != nil {
			t.Fatalf("update metadata with same name: %v", err)
		}

		var description string
		if err := s.db.QueryRowContext(ctx, "SELECT description FROM archives WHERE name = ?;", "New Title").Scan(&description); err != nil {
			t.Fatalf("read description: %v", err)
		}
		if description != "desc with same name" {
			t.Fatalf("description mismatch: got %q, want %q", description, "desc with same name")
		}

		rows, err := s.db.QueryContext(ctx, "SELECT tag FROM tags WHERE archive_id = ?;", archiveID)
		if err != nil {
			t.Fatalf("query tags: %v", err)
		}
		defer rows.Close()

		gotTags := make(map[string]struct{})
		for rows.Next() {
			var tag string
			if err := rows.Scan(&tag); err != nil {
				t.Fatalf("scan tag: %v", err)
			}
			gotTags[tag] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate tags: %v", err)
		}
		if len(gotTags) != 1 {
			t.Fatalf("expected 1 tag, got %d", len(gotTags))
		}
		if _, ok := gotTags["updated"]; !ok {
			t.Fatalf("missing tag %q", "updated")
		}
	})

	t.Run("empty_tags_clears_existing_tags", func(t *testing.T) {
		if err := s.UpdateMetadata(ctx, archiveID, "New Title", "desc without tags", []string{}); err != nil {
			t.Fatalf("update metadata with empty tags: %v", err)
		}

		var count int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tags WHERE archive_id = ?;", archiveID).Scan(&count); err != nil {
			t.Fatalf("count tags: %v", err)
		}

		if count != 0 {
			t.Fatalf("expected 0 tags after clearing, got %d", count)
		}
	})

	t.Run("conflict_returns_name_conflict", func(t *testing.T) {
		if err := s.Insert(ctx, models.Archive{
			ID:          uuid.New(),
			Name:        "Other Title",
			Filename:    "other.wacz",
			Description: "existing target",
			SourceURL:   "https://example.com/other",
			Tags:        []string{},
			CreatedAt:   createdAt,
		}); err != nil {
			t.Fatalf("insert other archive: %v", err)
		}

		err := s.UpdateMetadata(ctx, archiveID, "Other Title", "should fail", []string{"x"})
		if !errors.Is(err, ErrArchiveNameConflict) {
			t.Fatalf("expected ErrArchiveNameConflict, got %v", err)
		}
	})

	t.Run("missing_archive_returns_not_found", func(t *testing.T) {
		err := s.UpdateMetadata(ctx, uuid.New(), "Unused Title", "irrelevant", []string{"x"})
		if !errors.Is(err, ErrArchiveNotFound) {
			t.Fatalf("expected ErrArchiveNotFound, got %v", err)
		}
	})
}

func TestDeleteErrors(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	t.Run("missing_archive_returns_not_found", func(t *testing.T) {
		err := s.Delete(ctx, uuid.New())
		if !errors.Is(err, ErrArchiveNotFound) {
			t.Fatalf("expected ErrArchiveNotFound, got %v", err)
		}
	})

	t.Run("existing_archive_deletes_successfully", func(t *testing.T) {
		archiveID := uuid.New()
		if err := s.Insert(ctx, models.Archive{
			ID:          archiveID,
			Name:        "Delete Me",
			Filename:    "delete-me.wacz",
			Description: "to delete",
			SourceURL:   "https://example.com/delete",
			Tags:        []string{"x", "y"},
			CreatedAt:   time.Date(2026, 3, 29, 16, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("insert archive: %v", err)
		}

		if err := s.Delete(ctx, archiveID); err != nil {
			t.Fatalf("delete archive: %v", err)
		}

		var archiveCount int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM archives WHERE id = ?;", archiveID).Scan(&archiveCount); err != nil {
			t.Fatalf("count deleted archive rows: %v", err)
		}
		if archiveCount != 0 {
			t.Fatalf("expected archive row deleted, got count=%d", archiveCount)
		}

		var tagCount int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tags WHERE archive_id = ?;", archiveID).Scan(&tagCount); err != nil {
			t.Fatalf("count deleted tag rows: %v", err)
		}
		if tagCount != 0 {
			t.Fatalf("expected cascade delete on tags, got count=%d", tagCount)
		}
	})
}

func TestRunMigrationsAddsFilenameAndBackfillsExistingRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "archiver.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if _, err := db.Exec(migrationSQL(t, 1)); err != nil {
		t.Fatalf("apply migration v1 fixture: %v", err)
	}
	if _, err := db.Exec(migrationSQL(t, 2)); err != nil {
		t.Fatalf("apply migration v2 fixture: %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version = 2;"); err != nil {
		t.Fatalf("set v2 user_version fixture: %v", err)
	}

	archiveID := uuid.New()
	if _, err := db.Exec(
		"INSERT INTO archives (id, name, description, source_url, created_at, size_bytes) VALUES (?, ?, ?, ?, ?, ?);",
		archiveID,
		"legacy-name.wacz",
		"legacy description",
		"https://legacy.example",
		time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		int64(1234),
	); err != nil {
		t.Fatalf("insert legacy archive fixture: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close fixture db: %v", err)
	}

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	if err := s.RunMigrations(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	version, err := s.userVersion()
	if err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != latestMigrationVersion(t) {
		t.Fatalf("expected schema version %d after migrations, got %d", latestMigrationVersion(t), version)
	}

	if !archiveColumnExists(t, s.db, "filename") {
		t.Fatal("expected archives.filename column to exist")
	}
	if !archiveUniqueIndexExists(t, s.db, "idx_archives_filename_unique") {
		t.Fatal("expected unique index on archives.filename to exist")
	}

	var name, filename string
	if err := s.db.QueryRow("SELECT name, filename FROM archives WHERE id = ?;", archiveID).Scan(&name, &filename); err != nil {
		t.Fatalf("read migrated archive: %v", err)
	}

	if name != "legacy-name.wacz" {
		t.Fatalf("expected migration to preserve legacy display name, got %q", name)
	}
	if filename != "legacy-name.wacz" {
		t.Fatalf("expected migration to backfill filename from legacy name, got %q", filename)
	}
}

func TestSyncFromDiskDoesNotDuplicateLegacyRowsByFilename(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "archiver.db")
	archivesDir := t.TempDir()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if _, err := db.Exec(migrationSQL(t, 1)); err != nil {
		t.Fatalf("apply migration v1 fixture: %v", err)
	}
	if _, err := db.Exec(migrationSQL(t, 2)); err != nil {
		t.Fatalf("apply migration v2 fixture: %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version = 2;"); err != nil {
		t.Fatalf("set v2 user_version fixture: %v", err)
	}

	archiveID := uuid.New()
	if _, err := db.Exec(
		"INSERT INTO archives (id, name, description, source_url, created_at, size_bytes) VALUES (?, ?, ?, ?, ?, ?);",
		archiveID,
		"legacy-name.wacz",
		"legacy description",
		"https://legacy.example",
		time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		int64(1234),
	); err != nil {
		t.Fatalf("insert legacy archive fixture: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close fixture db: %v", err)
	}

	if err := os.WriteFile(filepath.Join(archivesDir, "legacy-name.wacz"), []byte("same file"), 0644); err != nil {
		t.Fatalf("write archive file: %v", err)
	}

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	if err := s.RunMigrations(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if err := s.SyncFromDisk(context.Background(), archivesDir); err != nil {
		t.Fatalf("sync from disk: %v", err)
	}

	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM archives WHERE filename = ?;", "legacy-name.wacz").Scan(&count); err != nil {
		t.Fatalf("count archives by filename: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected sync to ignore existing filename, got %d rows", count)
	}

	var name string
	if err := s.db.QueryRow("SELECT name FROM archives WHERE id = ?;", archiveID).Scan(&name); err != nil {
		t.Fatalf("read legacy row name: %v", err)
	}
	if name != "legacy-name.wacz" {
		t.Fatalf("expected legacy display name to be preserved, got %q", name)
	}
}

func TestSyncFromDiskStoresFilenameSeparatelyFromDisplayName(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	ensureFilenameColumnForExpectation(t, s.db)

	archivesDir := t.TempDir()
	archivePath := filepath.Join(archivesDir, "Disk Archive.wacz")
	if err := os.WriteFile(archivePath, []byte("archive bytes"), 0644); err != nil {
		t.Fatalf("write archive file: %v", err)
	}

	if err := s.SyncFromDisk(ctx, archivesDir); err != nil {
		t.Fatalf("sync from disk: %v", err)
	}

	var name string
	var filename sql.NullString
	if err := s.db.QueryRowContext(ctx, "SELECT name, filename FROM archives;").Scan(&name, &filename); err != nil {
		t.Fatalf("read synced archive: %v", err)
	}

	if name != "Disk Archive" {
		t.Fatalf("expected display name to be derived from filename without .wacz, got %q", name)
	}
	if !filename.Valid || filename.String != "Disk Archive.wacz" {
		t.Fatalf("expected filename to preserve the actual disk filename, got valid=%v value=%q", filename.Valid, filename.String)
	}
}

func migrationSQL(t *testing.T, version int) string {
	t.Helper()

	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	for _, migration := range migrations {
		if migration.version == version {
			return migration.sql
		}
	}

	t.Fatalf("migration version %d not found", version)
	return ""
}

func latestMigrationVersion(t *testing.T) int {
	t.Helper()

	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("expected at least one migration")
	}

	return migrations[len(migrations)-1].version
}

func ensureFilenameColumnForExpectation(t *testing.T, db *sql.DB) {
	t.Helper()

	if archiveColumnExists(t, db, "filename") {
		return
	}

	if _, err := db.Exec("ALTER TABLE archives ADD COLUMN filename TEXT;"); err != nil {
		t.Fatalf("add filename column fixture: %v", err)
	}
}

func archiveUniqueIndexExists(t *testing.T, db *sql.DB, indexName string) bool {
	t.Helper()

	rows, err := db.Query("PRAGMA index_list(archives);")
	if err != nil {
		t.Fatalf("read archives index list: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			seq     int
			name    string
			unique  int
			origin  string
			partial int
		)
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan archives index list: %v", err)
		}
		if name == indexName && unique == 1 {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate archives index list: %v", err)
	}

	return false
}

func archiveColumnExists(t *testing.T, db *sql.DB, column string) bool {
	t.Helper()

	rows, err := db.Query("PRAGMA table_info(archives);")
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
