package crawler

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"uuid"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/JuanSaenz04/archiver/internal/store"
	"github.com/stretchr/testify/assert"
)

// Helper function to create an in-memory test store
func newTestStore(t *testing.T) *store.ArchiveStore {
	t.Helper()

	dbPath := "file:" + uuid.New().String() + "?mode=memory&cache=shared"
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
	crawler := NewCrawler(Config{TimeoutInSeconds: 30, AnubisMode: AnubisModeOff}, archiveStore)

	// Setup temporary directories for collections (source) and archives (destination)
	tempDir := t.TempDir()
	collectionsDir := filepath.Join(tempDir, "collections")
	archivesDir := filepath.Join(tempDir, "archives")

	crawler.archivesDir = archivesDir
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
	crawler := NewCrawler(Config{TimeoutInSeconds: 30, AnubisMode: AnubisModeOff}, archiveStore)

	tempDir := t.TempDir()
	crawler.archivesDir = filepath.Join(tempDir, "archives")

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

func TestCrawlerRun_DuplicateNamePreservesExistingArchive(t *testing.T) {
	archiveStore := newTestStore(t)
	crawler := NewCrawler(Config{TimeoutInSeconds: 30, AnubisMode: AnubisModeOff}, archiveStore)

	tempDir := t.TempDir()
	collectionsDir := filepath.Join(tempDir, "collections")
	archivesDir := filepath.Join(tempDir, "archives")

	crawler.archivesDir = archivesDir
	crawler.collectionsDir = collectionsDir
	if err := os.MkdirAll(archivesDir, 0755); err != nil {
		t.Fatalf("create archives directory: %v", err)
	}

	existingArchive := models.Archive{
		ID:          uuid.New(),
		Name:        "Duplicate Name",
		Filename:    "Duplicate-Name.wacz",
		Description: "some description",
		SourceURL:   "https://example.com/original",
	}

	ctx := context.Background()
	err := archiveStore.Insert(ctx, existingArchive)
	assert.NoError(t, err)
	existingPath := filepath.Join(archivesDir, existingArchive.Filename)
	assert.NoError(t, os.WriteFile(existingPath, []byte("original archive"), 0644))

	jobID := uuid.New().String()
	archive := models.Archive{
		ID:        uuid.MustParse(jobID),
		Name:      "Duplicate Name",
		SourceURL: "https://example.com/duplicate",
	}

	crawler.runCmd = func(cmd *exec.Cmd) error {
		srcPath := filepath.Join(collectionsDir, jobID, jobID+".wacz")
		if err := os.MkdirAll(filepath.Dir(srcPath), 0755); err != nil {
			return err
		}
		return os.WriteFile(srcPath, []byte("some wacz content"), 0644)
	}

	assert.NoError(t, crawler.Run(ctx, jobID, archive, models.CrawlOptions{}))

	existingBytes, err := os.ReadFile(existingPath)
	assert.NoError(t, err)
	assert.Equal(t, "original archive", string(existingBytes))

	newPath := filepath.Join(archivesDir, "Duplicate-Name-1.wacz")
	newBytes, err := os.ReadFile(newPath)
	assert.NoError(t, err)
	assert.Equal(t, "some wacz content", string(newBytes))

	records, err := archiveStore.List(ctx)
	assert.NoError(t, err)
	assert.Len(t, records, 2)

	filenames := make(map[uuid.UUID]string, len(records))
	for _, record := range records {
		filenames[record.ID] = record.Filename
	}
	assert.Equal(t, "Duplicate-Name.wacz", filenames[existingArchive.ID])
	assert.Equal(t, "Duplicate-Name-1.wacz", filenames[archive.ID])
}

func TestAnubisDetector(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "challenge script",
			body: `<script id="anubis_challenge" type="application/json">{}</script>`,
			want: true,
		},
		{
			name: "Anubis error page",
			body: `<img src="/.within.website/x/cmd/anubis/static/img/reject.webp"><a href="https://github.com/TecharoHQ/anubis">Anubis</a>`,
			want: true,
		},
		{
			name: "unrelated page",
			body: `<html><title>Example</title></html>`,
		},
		{
			name: "Anubis link without served asset",
			body: `<a href="https://github.com/TecharoHQ/anubis">Read about Anubis</a>`,
		},
		{
			name: "marker beyond detection limit",
			body: strings.Repeat("x", anubisDetectionLimit) + `<script id="anubis_challenge"></script>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detected, err := responseHasAnubis(strings.NewReader(tt.body))
			assert.NoError(t, err)
			assert.Equal(t, tt.want, detected)
		})
	}
}

func TestNewAnubisDetectionRequest(t *testing.T) {
	req, err := newAnubisDetectionRequest(context.Background(), "https://example.com/")
	assert.NoError(t, err)

	assert.Equal(t, anubisDetectionAgent, req.Header.Get("User-Agent"))
	assert.Contains(t, req.Header.Get("Accept"), "text/html")
	assert.Equal(t, "en-US,en;q=0.9", req.Header.Get("Accept-Language"))
	assert.Equal(t, "document", req.Header.Get("Sec-Fetch-Dest"))
	assert.Equal(t, "navigate", req.Header.Get("Sec-Fetch-Mode"))
	assert.Equal(t, "none", req.Header.Get("Sec-Fetch-Site"))
	assert.Equal(t, "?1", req.Header.Get("Sec-Fetch-User"))
	assert.Equal(t, "1", req.Header.Get("Upgrade-Insecure-Requests"))
}

func TestCrawlerRun_AnubisDriverModes(t *testing.T) {
	tests := []struct {
		name          string
		mode          AnubisMode
		detected      bool
		detectionErr  error
		wantDriver    bool
		wantDetection bool
	}{
		{name: "auto detected", mode: AnubisModeAuto, detected: true, wantDriver: true, wantDetection: true},
		{name: "auto not detected", mode: AnubisModeAuto, wantDetection: true},
		{name: "auto detection failure", mode: AnubisModeAuto, detectionErr: errors.New("detection unavailable"), wantDetection: true},
		{name: "always", mode: AnubisModeAlways, wantDriver: true},
		{name: "off", mode: AnubisModeOff},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archiveStore := newTestStore(t)
			crawler := NewCrawler(Config{TimeoutInSeconds: 30, AnubisMode: tt.mode}, archiveStore)

			detectionCalled := false
			crawler.detectAnubis = func(context.Context, string) (bool, error) {
				detectionCalled = true
				return tt.detected, tt.detectionErr
			}

			var capturedCmd *exec.Cmd
			crawler.runCmd = func(cmd *exec.Cmd) error {
				capturedCmd = cmd
				return nil
			}
			err := crawler.Run(context.Background(), uuid.New().String(), models.Archive{
				Name:      "Anubis Test",
				SourceURL: "https://example.com/",
			}, models.CrawlOptions{})
			assert.NoError(t, err)
			assert.Equal(t, tt.wantDetection, detectionCalled)
			assert.NotNil(t, capturedCmd)

			joinedArgs := strings.Join(capturedCmd.Args, " ")
			if tt.wantDriver {
				assert.Contains(t, joinedArgs, "--driver "+anubisDriverPath)
			} else {
				assert.NotContains(t, joinedArgs, "--driver")
			}
		})
	}
}

func TestCrawlerRun_ExistingArchiveSkipsCrawl(t *testing.T) {
	s := newTestStore(t)
	dir := t.TempDir()
	a := models.Archive{ID: uuid.New(), Name: "saved", Filename: "saved.wacz"}
	assert.NoError(t, s.Insert(context.Background(), a))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, a.Filename), []byte("saved"), 0644))
	c := NewCrawler(Config{ArchivesDir: dir, AnubisMode: AnubisModeOff}, s)
	c.runCmd = func(*exec.Cmd) error { t.Fatal("existing archive crawled again"); return nil }
	assert.NoError(t, c.Run(context.Background(), a.ID.String(), a, models.CrawlOptions{}))
}
