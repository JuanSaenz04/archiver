package crawler

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/JuanSaenz04/archiver/internal/store"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Helper function to create an in-memory test store
func newTestStore(t *testing.T) *store.ArchiveStore {
	t.Helper()

	dbPath := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

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

func TestCrawlerRun_Success(t *testing.T) {
	archiveStore := newTestStore(t)
	crawler := NewCrawler(30, archiveStore)

	// Setup temporary directories for collections (source) and archives (destination)
	tempDir := t.TempDir()
	collectionsDir := filepath.Join(tempDir, "collections")
	archivesDir := filepath.Join(tempDir, "archives")

	t.Setenv("ARCHIVES_DIR", archivesDir)
	crawler.collectionsDir = collectionsDir

	jobID := uuid.New().String()
	archive := models.Archive{
		ID:          uuid.MustParse(jobID),
		Name:        "Test Archive Site",
		Description: "Crawler unit test site description",
		SourceURL:   "https://example.com/blog",
		Tags:        []string{"blog", "test"},
	}
	options := models.CrawlOptions{
		ScopeType: models.Page,
		Depth:     2,
		PageLimit: 50,
		SizeLimit: 5, // 5MB
	}

	// Fake/mock command execution callback
	var capturedCmd *exec.Cmd
	crawler.runCmd = func(cmd *exec.Cmd) error {
		capturedCmd = cmd

		// Write a fake source .wacz file so the Copy operation succeeds
		srcPath := filepath.Join(collectionsDir, jobID, jobID+".wacz")
		if err := os.MkdirAll(filepath.Dir(srcPath), 0755); err != nil {
			return err
		}
		fakeContent := []byte("fake wacz zip content bytes")
		return os.WriteFile(srcPath, fakeContent, 0644)
	}

	ctx := context.Background()
	err := crawler.Run(ctx, jobID, archive, options)
	assert.NoError(t, err)

	// 1. Assert command arguments were set correctly
	assert.NotNil(t, capturedCmd)
	args := capturedCmd.Args
	assert.Contains(t, args, "xvfb-run")
	assert.Contains(t, args, "node")
	assert.Contains(t, args, "/app/dist/main.js")
	assert.Contains(t, args, "crawl")
	assert.Contains(t, args, "--url")
	assert.Contains(t, args, "https://example.com/blog")
	assert.Contains(t, args, "--collection")
	assert.Contains(t, args, jobID)
	assert.Contains(t, args, "--scopeType")
	assert.Contains(t, args, "page")
	assert.Contains(t, args, "--depth")
	assert.Contains(t, args, "2")
	assert.Contains(t, args, "--limit")
	assert.Contains(t, args, "50")
	assert.Contains(t, args, "--sizeLimit")
	assert.Contains(t, args, "5242880") // 5 * 1024 * 1024

	// 2. Assert destination file is copied correctly with normalized name
	expectedFilename := "Test-Archive-Site.wacz"
	dstPath := filepath.Join(archivesDir, expectedFilename)
	assert.FileExists(t, dstPath)

	dstBytes, err := os.ReadFile(dstPath)
	assert.NoError(t, err)
	assert.Equal(t, "fake wacz zip content bytes", string(dstBytes))

	// 3. Assert database record has been inserted with correct metadata
	records, err := archiveStore.List(ctx)
	assert.NoError(t, err)
	assert.Len(t, records, 1)

	rec := records[0]
	assert.Equal(t, archive.ID, rec.ID)
	assert.Equal(t, archive.Name, rec.Name)
	assert.Equal(t, expectedFilename, rec.Filename)
	assert.Equal(t, archive.Description, rec.Description)
	assert.Equal(t, archive.SourceURL, rec.SourceURL)
	assert.Equal(t, int64(len("fake wacz zip content bytes")), rec.SizeBytes)
	assert.ElementsMatch(t, archive.Tags, rec.Tags)
}

func TestCrawlerRun_CrawlCommandFailure(t *testing.T) {
	archiveStore := newTestStore(t)
	crawler := NewCrawler(30, archiveStore)

	tempDir := t.TempDir()
	t.Setenv("ARCHIVES_DIR", filepath.Join(tempDir, "archives"))

	crawler.runCmd = func(cmd *exec.Cmd) error {
		return errors.New("xvfb-run crashed")
	}

	jobID := uuid.New().String()
	archive := models.Archive{
		ID:        uuid.MustParse(jobID),
		Name:      "Crashed Site",
		SourceURL: "https://example.com/crash",
	}

	ctx := context.Background()
	err := crawler.Run(ctx, jobID, archive, models.CrawlOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "xvfb-run crashed")

	// Ensure nothing is written to DB
	records, err := archiveStore.List(ctx)
	assert.NoError(t, err)
	assert.Empty(t, records)
}

func TestCrawlerRun_StoreInsertFailureCleanup(t *testing.T) {
	archiveStore := newTestStore(t)
	crawler := NewCrawler(30, archiveStore)

	tempDir := t.TempDir()
	collectionsDir := filepath.Join(tempDir, "collections")
	archivesDir := filepath.Join(tempDir, "archives")

	t.Setenv("ARCHIVES_DIR", archivesDir)
	crawler.collectionsDir = collectionsDir

	// Pre-insert an archive with the same name to cause a unique key constraint violation on name
	existingArchive := models.Archive{
		ID:          uuid.New(),
		Name:        "Duplicate Name",
		Filename:    "existing.wacz",
		Description: "some description",
		SourceURL:   "https://example.com/original",
	}

	ctx := context.Background()
	err := archiveStore.Insert(ctx, existingArchive)
	assert.NoError(t, err)

	jobID := uuid.New().String()
	archive := models.Archive{
		ID:        uuid.MustParse(jobID),
		Name:      "Duplicate Name", // Same name as existingArchive
		SourceURL: "https://example.com/duplicate",
	}

	crawler.runCmd = func(cmd *exec.Cmd) error {
		srcPath := filepath.Join(collectionsDir, jobID, jobID+".wacz")
		if err := os.MkdirAll(filepath.Dir(srcPath), 0755); err != nil {
			return err
		}
		return os.WriteFile(srcPath, []byte("some wacz content"), 0644)
	}

	err = crawler.Run(ctx, jobID, archive, models.CrawlOptions{})
	// Expect database insert to fail (UNIQUE constraint failed: archives.name)
	assert.Error(t, err)

	// Ensure the copied destination file was deleted/rolled back on database error
	expectedFilename := "duplicate-name.wacz"
	dstPath := filepath.Join(archivesDir, expectedFilename)
	assert.NoFileExists(t, dstPath, "copied archive should be cleaned up on database insert failure")

	// Verify only the original archive exists in database
	records, err := archiveStore.List(ctx)
	assert.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, existingArchive.ID, records[0].ID)
}
