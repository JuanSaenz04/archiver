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
