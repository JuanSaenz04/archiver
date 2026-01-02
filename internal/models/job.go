package models

import "github.com/google/uuid"

type Job struct {
	ID     uuid.UUID `json:"id"`
	URL    string    `json:"url"`
	Status string    `json:"status"`
}

type CrawlRequest struct {
	URL     string       `json:"url"`
	Options CrawlOptions `json:"crawl_options"`
}
