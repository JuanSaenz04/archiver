package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JuanSaenz04/archiver/internal/api"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	var opts *redis.Options
	var err error

	redisURL := os.Getenv("REDIS_URL")
	opts, err = redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Invalid REDIS_URL: %v", err)
	}

	rdb := redis.NewClient(opts)

	rdb.XGroupCreateMkStream(ctx, "crawl_stream", "worker_group", "$")

	handler := api.NewHandler(rdb)

	e := echo.New()

	handler.SetRoutes(e)

	go func() {
		if err := e.Start(":1080"); err != nil {
			e.Logger.Info("shutting down server")
		}
	}()

	<-ctx.Done()
	cancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		e.Logger.Fatal(err)
	}

	log.Println("Server stopped gracefully")
}
