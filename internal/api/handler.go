package api

import (
	"github.com/JuanSaenz04/archiver/internal/queue"
	"github.com/JuanSaenz04/archiver/internal/store"
)

type Handler struct {
	jobs         *queue.JobService
	archivesDir  string
	archiveStore *store.ArchiveStore
}

func NewHandler(jobs *queue.JobService, archivesDir string, archiveStore *store.ArchiveStore) *Handler {
	return &Handler{
		jobs:         jobs,
		archivesDir:  archivesDir,
		archiveStore: archiveStore,
	}
}
