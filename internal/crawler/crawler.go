package crawler

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// Run executes the crawler for a specific job.
func Run(ctx context.Context, jobID, targetURL string) error {
	fmt.Printf("Received job with ID %s\n", jobID)

	cmd := exec.CommandContext(
		ctx,
		"node", "/app/dist/main.js", "crawl",
		"--url", targetURL,
		"--generateWACZ",
		"--collection", "test",
		"--ignoreRobots",
		"--text",
		"--workers", "2",
		"--scopeType", "prefix",
		"--limit", "1000",
		"--sizeLimit", "104857600",
		"--depth", "0")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Printf("Crawl failed for %s: %v", targetURL, err)
		return err
	}

	archivesDir := os.Getenv("ARCHIVES_DIR")
	if archivesDir == "" {
		return nil
	}

	// Ensure archives directory exists
	if info, err := os.Stat(archivesDir); err != nil || !info.IsDir() {
		// We might want to log this in the future?
		return nil
	}

	srcPath := "collections/test/test.wacz"
	dstPath := filepath.Join(archivesDir, jobID+".wacz")

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
