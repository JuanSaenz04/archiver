CREATE TABLE archives_new (
    id          TEXT     PRIMARY KEY,
    name        TEXT     NOT NULL,
    filename    TEXT,
    description TEXT     NOT NULL DEFAULT '',
    source_url  TEXT     NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    size_bytes  INTEGER  NOT NULL DEFAULT 0
);

CREATE TABLE tags_new (
    archive_id TEXT NOT NULL REFERENCES archives_new(id) ON DELETE CASCADE,
    tag        TEXT NOT NULL,
    PRIMARY KEY (archive_id, tag)
);

INSERT INTO archives_new (id, name, filename, description, source_url, created_at, size_bytes)
SELECT id, name, filename, description, source_url, created_at, size_bytes FROM archives;

INSERT INTO tags_new (archive_id, tag)
SELECT archive_id, tag FROM tags;

DROP TABLE tags;
DROP TABLE archives;

ALTER TABLE archives_new RENAME TO archives;
ALTER TABLE tags_new RENAME TO tags;

CREATE INDEX idx_tags_archive_id ON tags(archive_id);
CREATE INDEX idx_tags_tag ON tags(tag);
CREATE UNIQUE INDEX idx_archives_filename_unique ON archives(filename);
