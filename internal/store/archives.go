package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/google/uuid"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var ErrArchiveNotFound = errors.New("archive not found")
var ErrArchiveNameConflict = errors.New("archive name conflict")

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
INSERT OR IGNORE INTO archives (id, name, description, source_url, created_at, size_bytes)
VALUES (?, ?, '', '', ?, ?);
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

		if _, err := stmt.ExecContext(ctx, uuid.New(), file.Name(), fileInfo.ModTime(), fileInfo.Size()); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *ArchiveStore) List(ctx context.Context) ([]models.Archive, error) {
	const query = `
SELECT a.id, a.name, a.description, a.source_url, a.created_at, a.size_bytes, t.tag
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
			sizeBytes                    int64
		)

		if err := rows.Scan(&id, &name, &description, &sourceURL, &createdAt, &sizeBytes, &tag); err != nil {
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
				SizeBytes:   sizeBytes,
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

	archiveQuery := `
INSERT INTO archives (id, name, description, source_url, created_at, size_bytes) VALUES (?, ?, ?, ?, ?, ?);
	`
	archiveArgs := []any{a.ID, a.Name, a.Description, a.SourceURL, a.CreatedAt, a.SizeBytes}
	if a.CreatedAt.IsZero() {
		archiveQuery = `
INSERT INTO archives (id, name, description, source_url, size_bytes) VALUES (?, ?, ?, ?, ?);
		`
		archiveArgs = []any{a.ID, a.Name, a.Description, a.SourceURL, a.SizeBytes}
	}

	if _, err := tx.ExecContext(ctx, archiveQuery, archiveArgs...); err != nil {
		if isUniqueConstraint(err) {
			return ErrArchiveNameConflict
		} else {
			return err
		}
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
	const query = `
UPDATE archives SET name = ?
WHERE name = ?;
	`

	if res, err := s.db.ExecContext(ctx, query, newName, oldName); err != nil {
		if isUniqueConstraint(err) {
			return ErrArchiveNameConflict
		}

		return err
	} else {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrArchiveNotFound
		}
	}

	return nil
}

func (s *ArchiveStore) UpdateMetadata(ctx context.Context, name, description string, tags []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const getIDQuery = `
SELECT id
FROM archives
WHERE name = ?;
	`

	var id uuid.UUID
	if err := tx.QueryRowContext(ctx, getIDQuery, name).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrArchiveNotFound
		}

		return err
	}

	const changeDescriptionQuery = `
UPDATE archives SET description = ?
WHERE name = ?;
	`

	if res, err := tx.ExecContext(ctx, changeDescriptionQuery, description, name); err != nil {
		return err
	} else {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrArchiveNotFound
		}
	}

	const deleteOldTagsQuery = `
DELETE FROM tags
WHERE archive_id = ?;
	`
	if _, err := tx.ExecContext(ctx, deleteOldTagsQuery, id); err != nil {
		return err
	}

	const addNewTagQuery = `
INSERT INTO tags (archive_id, tag) VALUES (?, ?);
	`

	stmt, err := tx.PrepareContext(ctx, addNewTagQuery)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, tag := range tags {
		if _, err := stmt.ExecContext(ctx, id, tag); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *ArchiveStore) Delete(ctx context.Context, name string) error {
	const deleteQuery = `
DELETE FROM archives
WHERE name = ?;
	`

	if res, err := s.db.ExecContext(ctx, deleteQuery, name); err != nil {
		return err
	} else {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrArchiveNotFound
		}
	}

	return nil
}

func isUniqueConstraint(err error) bool {
	var sqlErr *sqlite.Error
	if !errors.As(err, &sqlErr) {
		return false
	}

	if sqlErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return true
	}

	if sqlErr.Code() == sqlite3.SQLITE_CONSTRAINT {
		return true
	}

	return false
}
