package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

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
		err := e.Start(":1080")
		if err != nil {
			log.Printf("Error: %s", err.Error())
		} else {
			log.Printf("Info: stopping gracefully...")
		}
	}()

	<-ctx.Done()
	cancel()
	e.Close()
}
