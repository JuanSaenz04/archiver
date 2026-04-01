package queue

import "github.com/JuanSaenz04/archiver/internal/models"

type CrawlMessage struct {
	JobID   string              `json:"job_id"`
	Options models.CrawlOptions `json:"options"`
	Archive models.Archive      `json:"archive"`
}
