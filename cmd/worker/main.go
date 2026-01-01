package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var opts *redis.Options
	var err error

	redisURL := os.Getenv("REDIS_URL")
	opts, err = redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Invalid REDIS_URL: %v", err)
	}

	rdb := redis.NewClient(opts)

	rdb.XGroupCreateMkStream(ctx, "crawl_stream", "worker_group", "$")

	log.Println("Worker started, waiting for jobs...")

	for {
		streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    "worker_group",
			Consumer: "worker-1",
			Streams:  []string{"crawl_stream", ">"},
			Count:    1,
			Block:    1 * time.Second,
		}).Result()

		if err != nil {
			if err == context.Canceled {
				log.Println("Worker shutting down...")
				break
			}
			if err == redis.Nil {
				continue
			}
			log.Printf("Error reading stream: %v", err)
			continue
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				jobID := message.Values["job_id"].(string)
				targetURL := message.Values["target_url"].(string)

				// Use background context for processing to ensure we can
				// update status and ack even if shutdown signal is received.
				processCtx := context.Background()

				rdb.HSet(processCtx, "job:"+jobID, "status", "running")

				err := runCrawl(jobID, targetURL)

				if err != nil {
					rdb.HSet(processCtx, "job:"+jobID, "status", "failed", "error", err.Error())
				} else {
					rdb.HSet(processCtx, "job:"+jobID, "status", "completed")
				}

				rdb.XAck(processCtx, "crawl_stream", "worker_group", message.ID)
			}
		}
	}

	log.Println("Worker stopped gracefully")
}

func runCrawl(jobID, targetURL string) error {
	fmt.Printf("Recieved job with ID %s\n", jobID)

	cmd := exec.Command(
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

	if info, err := os.Stat(archivesDir); err != nil || !info.IsDir() {
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
