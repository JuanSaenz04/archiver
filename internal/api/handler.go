package api

import "github.com/redis/go-redis/v9"

type Handler struct {
	rdb     *redis.Client
	jobRepo *JobRepository
}

func NewHandler(rdb *redis.Client) *Handler {
	return &Handler{
		rdb:     rdb,
		jobRepo: NewJobRepository(rdb),
	}
}