package api

import (
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	rdb         *redis.Client
	jobRepo     *JobRepository
	archivesDir string
}

func NewHandler(rdb *redis.Client, archivesDir string) *Handler {
	return &Handler{
		rdb:         rdb,
		jobRepo:     NewJobRepository(rdb),
		archivesDir: archivesDir,
	}
}
