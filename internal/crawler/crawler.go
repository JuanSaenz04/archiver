package crawler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/JuanSaenz04/archiver/internal/archiveutil"
	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/JuanSaenz04/archiver/internal/store"
)

type Crawler struct {
	timeoutInSeconds int
	archiveStore     *store.ArchiveStore
	collectionsDir   string
	runCmd           func(cmd *exec.Cmd) error
}

func NewCrawler(timeoutInSeconds int, archiveStore *store.ArchiveStore) *Crawler {
	return &Crawler{
		timeoutInSeconds: timeoutInSeconds,
		archiveStore:     archiveStore,
		collectionsDir:   "collections",
		runCmd:           func(cmd *exec.Cmd) error { return cmd.Run() },
	}
}

// Run executes the crawler for a specific job.
func (crawler *Crawler) Run(ctx context.Context, jobID string, archive models.Archive, options models.CrawlOptions) error {
	setDefaultValuesIfEmpty(&options)

	slog.Info("starting crawl",
		"job_id", jobID,
		"url", archive.SourceURL,
		"archive_name", archive.Name,
	)

	cmd := exec.CommandContext(
		ctx,
		"xvfb-run", "--auto-servernum", "--server-args=-screen 0 1280x1024x24",
		"node", "/app/dist/main.js", "crawl",
		"--url", archive.SourceURL,
		"--generateWACZ",
		"--collection", jobID,
		"--ignoreRobots",
		"--text",
		"--workers", "2",
		"--scopeType", string(options.ScopeType),
		"--limit", strconv.Itoa(options.PageLimit),
		"--sizeLimit", strconv.Itoa(options.SizeLimit*1024*1024),
		"--depth", strconv.Itoa(options.Depth),
		"--timeout", strconv.Itoa(crawler.timeoutInSeconds),
		"--postLoadDelay", "10",
		"--pageExtraDelay", "10",
		"--behaviorTimeout", "120",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := crawler.runCmd(cmd); err != nil {
		slog.Error("crawl command failed", "job_id", jobID, "url", archive.SourceURL, "error", err)
		return err
	}

	archivesDir := os.Getenv("ARCHIVES_DIR")
	if archivesDir == "" {
		slog.Warn("ARCHIVES_DIR not set, archive will not be persisted", "job_id", jobID, "url", archive.SourceURL)
		return nil
	}

	if err := os.MkdirAll(archivesDir, 0755); err != nil {
		slog.Error("failed to create archives directory", "job_id", jobID, "archives_dir", archivesDir, "error", err)
		return err
	}

	srcPath := filepath.Join(crawler.collectionsDir, jobID, jobID+".wacz")
	filename, ok := archiveutil.NormalizeArchiveName(archive.Name)
	if !ok {
		filename = jobID + ".wacz"
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source wacz: %w", err)
	}
	defer src.Close()

	dst, filename, err := createArchiveFile(archivesDir, filename)
	if err != nil {
		return fmt.Errorf("failed to create destination wacz: %w", err)
	}
	dstPath := dst.Name()
	keepFile := false
	defer func() {
		_ = dst.Close()
		if !keepFile {
			_ = os.Remove(dstPath)
		}
	}()

	size, err := io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("failed to copy wacz: %w", err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("failed to close destination wacz: %w", err)
	}

	archive.Filename = filename
	archive.SizeBytes = size

	err = crawler.archiveStore.Insert(ctx, archive)
	if err != nil {
		return err
	}
	keepFile = true

	slog.Info("archive persisted",
		"job_id", jobID,
		"archive_name", archive.Name,
		"path", dstPath,
		"size_bytes", archive.SizeBytes,
	)

	return nil
}

func createArchiveFile(dir, filename string) (*os.File, string, error) {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	for suffix := 0; ; suffix++ {
		candidate := filename
		if suffix > 0 {
			candidate = fmt.Sprintf("%s-%d%s", name, suffix, ext)
		}

		file, err := os.OpenFile(filepath.Join(dir, candidate), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return file, candidate, err
	}
}

func setDefaultValuesIfEmpty(options *models.CrawlOptions) {
	if options.ScopeType == "" {
		options.ScopeType = models.Prefix
	}

	if options.PageLimit < 0 {
		options.PageLimit = 1000
	}

	if options.SizeLimit < 0 {
		options.SizeLimit = 100
	}

	if options.Depth < 0 {
		options.Depth = -1
	}
}
