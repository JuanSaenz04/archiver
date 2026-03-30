package models

import (
	"github.com/google/uuid"
)

type Job struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	Status    string    `json:"status"`
	CreatedAt string    `json:"created_at"`
}

type CrawlRequest struct {
	URL         string       `json:"url"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Tags        []string     `json:"tags"`
	Options     CrawlOptions `json:"crawl_options"`
}
