DROP INDEX IF EXISTS idx_tags_archive_id;

CREATE INDEX idx_archives_created_at_id
ON archives(created_at DESC, id DESC);
