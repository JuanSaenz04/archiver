package api

import (
	"github.com/JuanSaenz04/archiver/internal/store"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	rdb          *redis.Client
	jobRepo      *JobRepository
	archivesDir  string
	archiveStore *store.ArchiveStore
}

func NewHandler(rdb *redis.Client, archivesDir string, archiveStore *store.ArchiveStore) *Handler {
	return &Handler{
		rdb:          rdb,
		jobRepo:      NewJobRepository(rdb),
		archivesDir:  archivesDir,
		archiveStore: archiveStore,
	}
}
