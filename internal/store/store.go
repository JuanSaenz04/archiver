package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const latestSchemaVersion = 2

const migrationV1 = `
CREATE TABLE IF NOT EXISTS archives (
	id          TEXT     PRIMARY KEY,
	name        TEXT     NOT NULL UNIQUE,
	description TEXT     NOT NULL DEFAULT '',
	source_url  TEXT     NOT NULL DEFAULT '',
	created_at  DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE IF NOT EXISTS tags (
	archive_id  TEXT NOT NULL REFERENCES archives(id) ON DELETE CASCADE,
	tag         TEXT NOT NULL,
	PRIMARY KEY (archive_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_tags_archive_id ON tags(archive_id);
CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);
`

const migrationV2 = `
ALTER TABLE archives ADD COLUMN size_bytes INTEGER NOT NULL DEFAULT 0;
`

type ArchiveStore struct {
	db *sql.DB
}

func Open(path string) (*ArchiveStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("PRAGMA foreign_keys=ON;")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("PRAGMA synchronous=NORMAL;")
	if err != nil {
		return nil, err
	}

	return &ArchiveStore{
		db: db,
	}, nil
}

func (s *ArchiveStore) RunMigrations() error {
	version, err := s.userVersion()
	if err != nil {
		return err
	}

	if version > latestSchemaVersion {
		return fmt.Errorf("unsupported schema version: %d", version)
	}

	if version < 1 {
		if err := s.migrateV1(); err != nil {
			return err
		}
	}

	if version < 2 {
		if err := s.migrateV2(); err != nil {
			return err
		}
	}

	return nil
}

func (s *ArchiveStore) userVersion() (int, error) {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version;").Scan(&version); err != nil {
		return 0, err
	}

	return version, nil
}

func (s *ArchiveStore) migrateV1() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(migrationV1); err != nil {
		return err
	}

	if _, err = tx.Exec("PRAGMA user_version = 1;"); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *ArchiveStore) migrateV2() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.Exec(migrationV2); err != nil {
		return err
	}

	if _, err := tx.Exec("PRAGMA user_version = 2;"); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *ArchiveStore) Close() error {
	return s.db.Close()
}
