package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/google/uuid"
)

func (s *ArchiveStore) SyncFromDisk(ctx context.Context, archivesDir string) error {
	files, err := os.ReadDir(archivesDir)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const insertArchiveQuery = `
INSERT OR IGNORE INTO archives (id, name, description, source_url, created_at)
VALUES (?, ?, '', '', ?);
`

	stmt, err := tx.PrepareContext(ctx, insertArchiveQuery)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}

		if file.IsDir() || !strings.EqualFold(filepath.Ext(file.Name()), ".wacz") {
			continue
		}

		fileInfo, err := file.Info()
		if err != nil {
			return err
		}

		if _, err := stmt.ExecContext(ctx, uuid.New(), file.Name(), fileInfo.ModTime()); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *ArchiveStore) List(ctx context.Context) ([]models.Archive, error) {
	const query = `
SELECT a.id, a.name, a.description, a.source_url, a.created_at, t.tag
FROM archives a
LEFT JOIN tags t ON t.archive_id = a.id;
`

	archiveIndexByID := make(map[uuid.UUID]int)
	archives := make([]models.Archive, 0)

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id                           uuid.UUID
			name, description, sourceURL string
			tag                          sql.NullString
			createdAt                    time.Time
		)

		if err := rows.Scan(&id, &name, &description, &sourceURL, &createdAt, &tag); err != nil {
			return nil, err
		}

		if index, ok := archiveIndexByID[id]; ok {
			if tag.Valid {
				archives[index].Tags = append(archives[index].Tags, tag.String)
			}
		} else {
			archive := models.Archive{
				ID:          id,
				Name:        name,
				Description: description,
				SourceURL:   sourceURL,
				Tags:        make([]string, 0),
				CreatedAt:   createdAt,
			}
			if tag.Valid {
				archive.Tags = append(archive.Tags, tag.String)
			}

			archives = append(archives, archive)
			archiveIndexByID[id] = len(archives) - 1
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return archives, nil
}

func (s *ArchiveStore) Insert(ctx context.Context, a models.Archive) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const archiveQuery = `
INSERT INTO archives (id, name, description, source_url, created_at) VALUES (?, ?, ?, ?, ?);
	`

	if _, err := tx.ExecContext(ctx, archiveQuery, a.ID, a.Name, a.Description, a.SourceURL, a.CreatedAt); err != nil {
		return err
	}

	const tagQuery = `
INSERT INTO tags (archive_id, tag) VALUES (?, ?)
	`

	for _, tag := range a.Tags {
		if _, err := tx.ExecContext(ctx, tagQuery, a.ID, tag); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *ArchiveStore) Rename(ctx context.Context, oldName, newName string) error {
	return nil
}

func (s *ArchiveStore) UpdateMetadata(ctx context.Context, name, description string, tags []string) error {
	return nil
}

func (s *ArchiveStore) Delete(ctx context.Context, name string) error {
	return nil
}
