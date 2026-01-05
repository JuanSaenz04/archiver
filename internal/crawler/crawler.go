package crawler

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/JuanSaenz04/archiver/internal/models"
)

// Run executes the crawler for a specific job.
func Run(ctx context.Context, jobID, targetURL string, options models.CrawlOptions) error {
	fmt.Printf("Received job with ID %s\n", jobID)

	setDefaultValuesIfEmpty(&options)

	cmd := exec.CommandContext(
		ctx,
		"node", "/app/dist/main.js", "crawl",
		"--url", targetURL,
		"--generateWACZ",
		"--collection", jobID,
		"--ignoreRobots",
		"--text",
		"--workers", "2",
		"--scopeType", string(options.ScopeType),
		"--limit", strconv.Itoa(options.PageLimit),
		"--sizeLimit", strconv.Itoa(options.SizeLimit*1024*1024),
		"--depth", strconv.Itoa(options.Depth),
		"--timeout", "30")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Printf("Crawl failed for %s: %v", targetURL, err)
		return err
	}

	archivesDir := os.Getenv("ARCHIVES_DIR")
	if archivesDir == "" {
		log.Println("ARCHIVES_DIR directory not set, archives won't be persisted.")
		return nil
	}

	if err := os.MkdirAll(archivesDir, 0755); err != nil {
		log.Printf("Crawl failed for %s: %v", targetURL, err)
		return err
	}

	srcPath := fmt.Sprintf("collections/%s/%s.wacz", jobID, jobID)
	name := strings.ReplaceAll(options.Name, " ", "-") + ".wacz"
	if name == "" {
		name = jobID + ".wacz"
	}
	dstPath := filepath.Join(archivesDir, name)

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source wacz: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination wacz: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy wacz: %w", err)
	}

	return nil
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
		options.Depth = 0
	}
}
