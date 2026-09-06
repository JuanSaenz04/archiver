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
	"time"

	"github.com/JuanSaenz04/archiver/internal/archiveutil"
	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/JuanSaenz04/archiver/internal/store"
)

type AnubisMode string

const (
	AnubisModeAuto   AnubisMode = "auto"
	AnubisModeAlways AnubisMode = "always"
	AnubisModeOff    AnubisMode = "off"
)

type Config struct {
	TimeoutInSeconds int
	AnubisMode       AnubisMode
	ArchivesDir      string
}

type Crawler struct {
	timeoutInSeconds int
	anubisMode       AnubisMode
	archiveStore     *store.ArchiveStore
	archivesDir      string
	collectionsDir   string
	runCmd           func(cmd *exec.Cmd) error
	detectAnubis     func(context.Context, string) (bool, error)
}

func NewCrawler(config Config, archiveStore *store.ArchiveStore) *Crawler {
	if config.AnubisMode == "" {
		config.AnubisMode = AnubisModeAuto
	}

	return &Crawler{
		timeoutInSeconds: config.TimeoutInSeconds,
		anubisMode:       config.AnubisMode,
		archiveStore:     archiveStore,
		archivesDir:      config.ArchivesDir,
		collectionsDir:   "collections",
		runCmd:           func(cmd *exec.Cmd) error { return runProcess(cmd, 10*time.Second) },
		detectAnubis:     newAnubisDetector(),
	}
}

// Run executes the crawler for a specific job.
func (crawler *Crawler) Run(ctx context.Context, jobID string, archive models.Archive, options models.CrawlOptions) error {
	if crawler.archivesDir != "" {
		for {
			filename, err := crawler.archiveStore.GetFilename(ctx, archive.ID)
			if errors.Is(err, store.ErrArchiveNotFound) {
				break
			}
			if err == nil {
				info, statErr := os.Stat(filepath.Join(crawler.archivesDir, filename))
				if statErr == nil && info.Mode().IsRegular() {
					return nil
				}
				if statErr == nil {
					return fmt.Errorf("existing archive is not a regular file")
				}
				err = statErr
			}
			slog.Warn("cannot verify existing archive; retrying", "job_id", jobID, "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
	setDefaultValuesIfEmpty(&options)

	slog.Info("starting crawl",
		"job_id", jobID,
		"url", archive.SourceURL,
		"archive_name", archive.Name,
	)

	args := []string{
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
		"--sizeLimit", strconv.Itoa(options.SizeLimit * 1024 * 1024),
		"--depth", strconv.Itoa(options.Depth),
		"--timeout", strconv.Itoa(crawler.timeoutInSeconds),
		"--postLoadDelay", "10",
		"--pageExtraDelay", "10",
		"--behaviorTimeout", "120",
	}

	if crawler.useAnubisDriver(ctx, archive.SourceURL) {
		args = append(args, "--driver", anubisDriverPath)
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := crawler.runCmd(cmd); err != nil {
		slog.Error("crawl command failed", "job_id", jobID, "url", archive.SourceURL, "error", err)
		return err
	}

	archivesDir := crawler.archivesDir
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

	size, err := io.Copy(dst, contextReader{ctx: ctx, reader: src})
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

func (crawler *Crawler) useAnubisDriver(ctx context.Context, targetURL string) bool {
	switch crawler.anubisMode {
	case AnubisModeAlways:
		slog.Info("Anubis driver enabled", "url", targetURL, "mode", crawler.anubisMode)
		return true
	case AnubisModeOff:
		return false
	case AnubisModeAuto:
		detected, err := crawler.detectAnubis(ctx, targetURL)
		if err != nil {
			slog.Warn("Anubis detection failed, continuing without driver", "url", targetURL, "error", err)
			return false
		}
		if detected {
			slog.Info("Anubis detected, enabling driver", "url", targetURL)
		} else {
			slog.Info("Anubis not detected, continuing without driver", "url", targetURL)
		}
		return detected
	default:
		slog.Warn("unknown Anubis mode, continuing without driver", "mode", crawler.anubisMode)
		return false
	}
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
