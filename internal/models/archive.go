package models

import (
	"time"

	"github.com/google/uuid"
)

type Archive struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	SourceURL   string    `json:"source_url"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
}
