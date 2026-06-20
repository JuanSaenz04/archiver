package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

var migrationFilenamePattern = regexp.MustCompile(`^(\d{5})_.+\.sql$`)

type migration struct {
	version int
	name    string
	sql     string
}

type ArchiveStore struct {
	db *sql.DB
}

func Open(path string) (*ArchiveStore, error) {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	dsn := fmt.Sprintf("%s%s_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path, separator)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	return &ArchiveStore{
		db: db,
	}, nil
}

func (s *ArchiveStore) RunMigrations() error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	version, err := s.userVersion()
	if err != nil {
		return err
	}

	latestVersion := 0
	if len(migrations) > 0 {
		latestVersion = migrations[len(migrations)-1].version
	}

	if version > latestVersion {
		return fmt.Errorf("unsupported schema version: %d", version)
	}

	pendingCount := 0
	for _, migration := range migrations {
		if migration.version > version {
			pendingCount++
		}
	}

	slog.Info("checking database migrations", "current_version", version, "latest_version", latestVersion, "pending", pendingCount)
	if pendingCount == 0 {
		slog.Info("database schema is up to date", "version", version)
		return nil
	}

	for _, migration := range migrations {
		if migration.version <= version {
			continue
		}

		if err := s.applyMigration(migration); err != nil {
			return err
		}
	}

	slog.Info("database migrations complete", "from_version", version, "to_version", latestVersion, "applied", pendingCount)
	return nil
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, err
	}

	migrations := make([]migration, 0, len(entries))
	versions := make(map[int]string, len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		matches := migrationFilenamePattern.FindStringSubmatch(name)
		if matches == nil {
			slog.Warn("ignoring migration file with invalid filename", "filename", name)
			continue
		}

		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", name, err)
		}

		if existingName, ok := versions[version]; ok {
			return nil, fmt.Errorf("duplicate migration version %05d: %s and %s", version, existingName, name)
		}
		versions[version] = name

		contents, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return nil, err
		}

		migrations = append(migrations, migration{
			version: version,
			name:    name,
			sql:     string(contents),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	for i, migration := range migrations {
		expectedVersion := i + 1
		if migration.version != expectedVersion {
			return nil, fmt.Errorf("missing migration version %05d", expectedVersion)
		}
	}

	return migrations, nil
}

func (s *ArchiveStore) applyMigration(migration migration) error {
	startedAt := time.Now()
	slog.Info("applying database migration", "version", migration.version, "filename", migration.name)

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(migration.sql); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply migration %s: %w", migration.name, err)
	}

	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d;", migration.version)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("set schema version after migration %s: %w", migration.name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", migration.name, err)
	}

	slog.Info("database migration applied", "version", migration.version, "filename", migration.name, "duration", time.Since(startedAt))
	return nil
}

func (s *ArchiveStore) userVersion() (int, error) {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version;").Scan(&version); err != nil {
		return 0, err
	}

	return version, nil
}

func (s *ArchiveStore) Close() error {
	return s.db.Close()
}
