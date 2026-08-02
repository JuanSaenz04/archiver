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
var ErrArchiveFilenameConflict = errors.New("archive filename conflict")

type ArchiveCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type ListArchivesOptions struct {
	Limit         int
	Cursor        *ArchiveCursor
	Tags          []string
	Search        string
	CreatedFrom   *time.Time
	CreatedBefore *time.Time
}

type ArchivePage struct {
	Archives   []models.Archive
	NextCursor *ArchiveCursor
}

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
INSERT OR IGNORE INTO archives (id, name, filename, description, source_url, created_at, size_bytes)
VALUES (?, ?, ?, '', '', ?, ?);
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

		if _, err := stmt.ExecContext(
			ctx,
			uuid.New(),
			strings.TrimSuffix(file.Name(), filepath.Ext(file.Name())),
			file.Name(),
			fileInfo.ModTime(),
			fileInfo.Size(),
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *ArchiveStore) List(ctx context.Context) ([]models.Archive, error) {
	page, err := s.ListArchives(ctx, ListArchivesOptions{})
	if err != nil {
		return nil, err
	}
	return page.Archives, nil
}

func (s *ArchiveStore) ListArchives(ctx context.Context, options ListArchivesOptions) (ArchivePage, error) {
	var where []string
	var args []any

	if options.CreatedFrom != nil {
		where = append(where, "a.created_at >= ?")
		args = append(args, *options.CreatedFrom)
	}
	if options.CreatedBefore != nil {
		where = append(where, "a.created_at < ?")
		args = append(args, *options.CreatedBefore)
	}
	if options.Cursor != nil {
		where = append(where, "(a.created_at < ? OR (a.created_at = ? AND a.id < ?))")
		args = append(args, options.Cursor.CreatedAt, options.Cursor.CreatedAt, options.Cursor.ID)
	}
	if len(options.Tags) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(options.Tags)), ",")
		where = append(where, `a.id IN (
			SELECT archive_id FROM tags
			WHERE tag IN (`+placeholders+`)
			GROUP BY archive_id
			HAVING COUNT(DISTINCT tag) = ?
		)`)
		for _, tag := range options.Tags {
			args = append(args, tag)
		}
		args = append(args, len(options.Tags))
	}
	if options.Search != "" {
		where = append(where, `a.rowid IN (
			SELECT rowid FROM archive_search
			WHERE archive_search MATCH ?
		)`)
		args = append(args, archiveSearchQuery(options.Search))
	}

	query := `
WITH filtered_archives AS (
	SELECT a.id, a.name, a.filename, a.description, a.source_url, a.created_at, a.size_bytes
	FROM archives a`
	if len(where) > 0 {
		query += "\n\tWHERE " + strings.Join(where, " AND ")
	}
	query += "\n\tORDER BY a.created_at DESC, a.id DESC"
	if options.Limit > 0 {
		query += "\n\tLIMIT ?"
		args = append(args, options.Limit+1)
	}
	query += `
)
SELECT a.id, a.name, a.filename, a.description, a.source_url, a.created_at, a.size_bytes, t.tag
FROM filtered_archives a
LEFT JOIN tags t ON t.archive_id = a.id
ORDER BY a.created_at DESC, a.id DESC, t.tag ASC;
`

	archiveIndexByID := make(map[uuid.UUID]int)
	archives := make([]models.Archive, 0)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return ArchivePage{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id                                     uuid.UUID
			name, filename, description, sourceURL string
			tag                                    sql.NullString
			createdAt                              time.Time
			sizeBytes                              int64
		)

		if err := rows.Scan(&id, &name, &filename, &description, &sourceURL, &createdAt, &sizeBytes, &tag); err != nil {
			return ArchivePage{}, err
		}

		if index, ok := archiveIndexByID[id]; ok {
			if tag.Valid {
				archives[index].Tags = append(archives[index].Tags, tag.String)
			}
		} else {
			archive := models.Archive{
				ID:          id,
				Name:        name,
				Filename:    filename,
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
		return ArchivePage{}, err
	}

	page := ArchivePage{Archives: archives}
	if options.Limit > 0 && len(archives) > options.Limit {
		page.Archives = archives[:options.Limit]
		last := page.Archives[len(page.Archives)-1]
		page.NextCursor = &ArchiveCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}

	return page, nil
}

func (s *ArchiveStore) ListTags(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT tag FROM tags ORDER BY tag;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]string, 0)
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func archiveSearchQuery(value string) string {
	return `"` + strings.ReplaceAll(strings.TrimSpace(value), `"`, `""`) + `" *`
}

func (s *ArchiveStore) Insert(ctx context.Context, a models.Archive) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	archiveQuery := `
INSERT INTO archives (id, name, filename, description, source_url, created_at, size_bytes) VALUES (?, ?, ?, ?, ?, ?, ?);
	`
	archiveArgs := []any{a.ID, a.Name, a.Filename, a.Description, a.SourceURL, a.CreatedAt, a.SizeBytes}
	if a.CreatedAt.IsZero() {
		archiveQuery = `
INSERT INTO archives (id, name, filename, description, source_url, size_bytes) VALUES (?, ?, ?, ?, ?, ?);
		`
		archiveArgs = []any{a.ID, a.Name, a.Filename, a.Description, a.SourceURL, a.SizeBytes}
	}

	if _, err := tx.ExecContext(ctx, archiveQuery, archiveArgs...); err != nil {
		if isUniqueConstraint(err) {
			return ErrArchiveFilenameConflict
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

func (s *ArchiveStore) UpdateMetadata(ctx context.Context, archiveId uuid.UUID, newName, description string, tags []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const renameQuery = `
UPDATE archives SET name = ?
WHERE id = ?;
		`

	if res, err := tx.ExecContext(ctx, renameQuery, newName, archiveId); err != nil {
		return err
	} else {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrArchiveNotFound
		}
	}

	const changeDescriptionQuery = `
UPDATE archives SET description = ?
WHERE id = ?;
	`

	if res, err := tx.ExecContext(ctx, changeDescriptionQuery, description, archiveId); err != nil {
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
	if _, err := tx.ExecContext(ctx, deleteOldTagsQuery, archiveId); err != nil {
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
		if _, err := stmt.ExecContext(ctx, archiveId, tag); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *ArchiveStore) Delete(ctx context.Context, archiveId uuid.UUID) error {
	const deleteQuery = `
DELETE FROM archives
WHERE id = ?;
	`

	if res, err := s.db.ExecContext(ctx, deleteQuery, archiveId); err != nil {
		return err
	} else {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrArchiveNotFound
		}
	}

	return nil
}

func (s *ArchiveStore) GetFilename(ctx context.Context, archiveId uuid.UUID) (string, error) {
	const getFilenameQuery = `
SELECT filename
FROM archives
WHERE id = ?;
	`

	row := s.db.QueryRowContext(ctx, getFilenameQuery, archiveId)

	var filename string
	if err := row.Scan(&filename); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrArchiveNotFound
		} else {
			return "", err
		}
	}

	return filename, nil
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
