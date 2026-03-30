package store

import (
	"context"
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
		Name:        "first.wacz",
		Description: "first archive",
		SourceURL:   "https://example.com/first",
		Tags:        []string{"news", "2026"},
		CreatedAt:   time.Date(2026, 3, 29, 10, 0, 0, 0, time.UTC),
	}

	second := models.Archive{
		ID:          secondID,
		Name:        "second.wacz",
		Description: "second archive",
		SourceURL:   "https://example.com/second",
		Tags:        []string{"tech"},
		CreatedAt:   time.Date(2026, 3, 29, 11, 0, 0, 0, time.UTC),
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

	if got := byName[second.Name]; got.ID != second.ID {
		t.Fatalf("second archive id mismatch: got %s, want %s", got.ID, second.ID)
	}

	if got := byName[second.Name]; len(got.Tags) != 1 || got.Tags[0] != "tech" {
		t.Fatalf("unexpected second archive tags: %#v", got.Tags)
	}
}

func TestInsertReturnsNameConflictOnDuplicateName(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	createdAt := time.Date(2026, 3, 29, 10, 0, 0, 0, time.UTC)

	if err := s.Insert(ctx, models.Archive{
		ID:          uuid.New(),
		Name:        "dup.wacz",
		Description: "first",
		SourceURL:   "https://example.com/one",
		Tags:        []string{"a"},
		CreatedAt:   createdAt,
	}); err != nil {
		t.Fatalf("insert first archive: %v", err)
	}

	err := s.Insert(ctx, models.Archive{
		ID:          uuid.New(),
		Name:        "dup.wacz",
		Description: "second",
		SourceURL:   "https://example.com/two",
		Tags:        []string{"b"},
		CreatedAt:   createdAt.Add(time.Minute),
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
		Name:        "default-created-at.wacz",
		Description: "created with sqlite default",
		SourceURL:   "https://example.com/default",
		Tags:        []string{"default"},
	}); err != nil {
		t.Fatalf("insert archive with zero created_at: %v", err)
	}

	after := time.Now().UTC().Add(time.Second)

	var createdAt time.Time
	if err := s.db.QueryRowContext(ctx, "SELECT created_at FROM archives WHERE name = ?;", "default-created-at.wacz").Scan(&createdAt); err != nil {
		t.Fatalf("read created_at: %v", err)
	}

	if createdAt.IsZero() {
		t.Fatal("expected sqlite to assign created_at, got zero value")
	}

	if createdAt.Before(before) || createdAt.After(after) {
		t.Fatalf("expected created_at between %s and %s, got %s", before, after, createdAt)
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
	if err := s.db.QueryRowContext(ctx, "SELECT created_at FROM archives WHERE name = ?;", "a.wacz").Scan(&createdAt); err != nil {
		t.Fatalf("read created_at for a.wacz: %v", err)
	}

	if !createdAt.Equal(mtime) {
		t.Fatalf("created_at mismatch: got %s, want %s", createdAt.UTC(), mtime.UTC())
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
	existingName := "existing.wacz"
	newName := "new.wacz"

	existingPath := filepath.Join(archivesDir, existingName)
	newPath := filepath.Join(archivesDir, newName)

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
		Description: "already in db",
		SourceURL:   "https://example.com/existing",
		Tags:        []string{"keep"},
		CreatedAt:   existingCreatedAt,
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
	if err := s.db.QueryRowContext(ctx, "SELECT id, description, created_at FROM archives WHERE name = ?;", existingName).Scan(&gotID, &gotDescription, &gotCreatedAt); err != nil {
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
}

func TestRename(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	createdAt := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)

	if err := s.Insert(ctx, models.Archive{
		ID:          uuid.New(),
		Name:        "old.wacz",
		Description: "to rename",
		SourceURL:   "https://example.com/old",
		Tags:        []string{"one", "two"},
		CreatedAt:   createdAt,
	}); err != nil {
		t.Fatalf("insert old archive: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		if err := s.Rename(ctx, "old.wacz", "new.wacz"); err != nil {
			t.Fatalf("rename archive: %v", err)
		}

		var oldCount int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM archives WHERE name = ?;", "old.wacz").Scan(&oldCount); err != nil {
			t.Fatalf("count old name: %v", err)
		}

		if oldCount != 0 {
			t.Fatalf("expected old name to be removed, got count=%d", oldCount)
		}

		var newCount int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM archives WHERE name = ?;", "new.wacz").Scan(&newCount); err != nil {
			t.Fatalf("count new name: %v", err)
		}

		if newCount != 1 {
			t.Fatalf("expected new name to exist once, got count=%d", newCount)
		}
	})

	t.Run("conflict", func(t *testing.T) {
		if err := s.Insert(ctx, models.Archive{
			ID:          uuid.New(),
			Name:        "other.wacz",
			Description: "existing target",
			SourceURL:   "https://example.com/other",
			Tags:        []string{},
			CreatedAt:   createdAt,
		}); err != nil {
			t.Fatalf("insert other archive: %v", err)
		}

		err := s.Rename(ctx, "new.wacz", "other.wacz")
		if !errors.Is(err, ErrArchiveNameConflict) {
			t.Fatalf("expected ErrArchiveNameConflict, got %v", err)
		}
	})

	t.Run("missing_source", func(t *testing.T) {
		err := s.Rename(ctx, "missing.wacz", "unused.wacz")
		if !errors.Is(err, ErrArchiveNotFound) {
			t.Fatalf("expected ErrArchiveNotFound, got %v", err)
		}

		var count int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM archives WHERE name = ?;", "unused.wacz").Scan(&count); err != nil {
			t.Fatalf("count unused name: %v", err)
		}

		if count != 0 {
			t.Fatalf("expected no row for unused name, got count=%d", count)
		}
	})
}

func TestUpdateMetadata(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	archiveID := uuid.New()
	createdAt := time.Date(2026, 3, 29, 14, 0, 0, 0, time.UTC)

	if err := s.Insert(ctx, models.Archive{
		ID:          archiveID,
		Name:        "meta.wacz",
		Description: "old description",
		SourceURL:   "https://example.com/meta",
		Tags:        []string{"old", "tags"},
		CreatedAt:   createdAt,
	}); err != nil {
		t.Fatalf("insert archive: %v", err)
	}

	t.Run("updates_description_and_replaces_tags", func(t *testing.T) {
		if err := s.UpdateMetadata(ctx, "meta.wacz", "new description", []string{"news", "2026"}); err != nil {
			t.Fatalf("update metadata: %v", err)
		}

		var description string
		if err := s.db.QueryRowContext(ctx, "SELECT description FROM archives WHERE name = ?;", "meta.wacz").Scan(&description); err != nil {
			t.Fatalf("read description: %v", err)
		}

		if description != "new description" {
			t.Fatalf("description mismatch: got %q, want %q", description, "new description")
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

	t.Run("empty_tags_clears_existing_tags", func(t *testing.T) {
		if err := s.UpdateMetadata(ctx, "meta.wacz", "desc without tags", []string{}); err != nil {
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

	t.Run("missing_archive_returns_not_found", func(t *testing.T) {
		err := s.UpdateMetadata(ctx, "missing.wacz", "irrelevant", []string{"x"})
		if !errors.Is(err, ErrArchiveNotFound) {
			t.Fatalf("expected ErrArchiveNotFound, got %v", err)
		}
	})
}

func TestDeleteErrors(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	t.Run("missing_archive_returns_not_found", func(t *testing.T) {
		err := s.Delete(ctx, "missing.wacz")
		if !errors.Is(err, ErrArchiveNotFound) {
			t.Fatalf("expected ErrArchiveNotFound, got %v", err)
		}
	})

	t.Run("existing_archive_deletes_successfully", func(t *testing.T) {
		archiveID := uuid.New()
		if err := s.Insert(ctx, models.Archive{
			ID:          archiveID,
			Name:        "delete-me.wacz",
			Description: "to delete",
			SourceURL:   "https://example.com/delete",
			Tags:        []string{"x", "y"},
			CreatedAt:   time.Date(2026, 3, 29, 16, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("insert archive: %v", err)
		}

		if err := s.Delete(ctx, "delete-me.wacz"); err != nil {
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
