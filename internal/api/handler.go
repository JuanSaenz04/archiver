package api

import (
	"github.com/JuanSaenz04/archiver/internal/queue"
	"github.com/JuanSaenz04/archiver/internal/store"
)

type Handler struct {
	logSlots     chan struct{}
	jobs         *queue.JobService
	archivesDir  string
	archiveStore *store.ArchiveStore
}

func NewHandler(jobs *queue.JobService, archivesDir string, archiveStore *store.ArchiveStore) *Handler {
	return &Handler{
		logSlots:     make(chan struct{}, 32),
		jobs:         jobs,
		archivesDir:  archivesDir,
		archiveStore: archiveStore,
	}
}
