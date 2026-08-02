CREATE VIRTUAL TABLE archive_search USING fts5(
    content,
    tokenize = 'unicode61 remove_diacritics 2'
);

INSERT INTO archive_search(rowid, content)
SELECT
    a.rowid,
    a.name || ' ' ||
    a.description || ' ' ||
    a.source_url || ' ' ||
    COALESCE((
        SELECT group_concat(t.tag, ' ')
        FROM tags t
        WHERE t.archive_id = a.id
    ), '')
FROM archives a;

CREATE TRIGGER archives_search_insert
AFTER INSERT ON archives
BEGIN
    INSERT INTO archive_search(rowid, content)
    VALUES (
        NEW.rowid,
        NEW.name || ' ' || NEW.description || ' ' || NEW.source_url
    );
END;

CREATE TRIGGER archives_search_update
AFTER UPDATE OF name, description, source_url ON archives
BEGIN
    UPDATE archive_search
    SET content =
        NEW.name || ' ' ||
        NEW.description || ' ' ||
        NEW.source_url || ' ' ||
        COALESCE((
            SELECT group_concat(tag, ' ')
            FROM tags
            WHERE archive_id = NEW.id
        ), '')
    WHERE rowid = NEW.rowid;
END;

CREATE TRIGGER archives_search_delete
AFTER DELETE ON archives
BEGIN
    DELETE FROM archive_search WHERE rowid = OLD.rowid;
END;

CREATE TRIGGER tags_search_insert
AFTER INSERT ON tags
BEGIN
    UPDATE archive_search
    SET content = (
        SELECT
            a.name || ' ' ||
            a.description || ' ' ||
            a.source_url || ' ' ||
            COALESCE(group_concat(t.tag, ' '), '')
        FROM archives a
        LEFT JOIN tags t ON t.archive_id = a.id
        WHERE a.id = NEW.archive_id
        GROUP BY a.id
    )
    WHERE rowid = (
        SELECT rowid FROM archives WHERE id = NEW.archive_id
    );
END;

CREATE TRIGGER tags_search_delete
AFTER DELETE ON tags
BEGIN
    UPDATE archive_search
    SET content = (
        SELECT
            a.name || ' ' ||
            a.description || ' ' ||
            a.source_url || ' ' ||
            COALESCE(group_concat(t.tag, ' '), '')
        FROM archives a
        LEFT JOIN tags t ON t.archive_id = a.id
        WHERE a.id = OLD.archive_id
        GROUP BY a.id
    )
    WHERE rowid = (
        SELECT rowid FROM archives WHERE id = OLD.archive_id
    );
END;

CREATE TRIGGER tags_search_update
AFTER UPDATE OF archive_id, tag ON tags
BEGIN
    UPDATE archive_search
    SET content = (
        SELECT
            a.name || ' ' ||
            a.description || ' ' ||
            a.source_url || ' ' ||
            COALESCE(group_concat(t.tag, ' '), '')
        FROM archives a
        LEFT JOIN tags t ON t.archive_id = a.id
        WHERE a.id = OLD.archive_id
        GROUP BY a.id
    )
    WHERE rowid = (
        SELECT rowid FROM archives WHERE id = OLD.archive_id
    );

    UPDATE archive_search
    SET content = (
        SELECT
            a.name || ' ' ||
            a.description || ' ' ||
            a.source_url || ' ' ||
            COALESCE(group_concat(t.tag, ' '), '')
        FROM archives a
        LEFT JOIN tags t ON t.archive_id = a.id
        WHERE a.id = NEW.archive_id
        GROUP BY a.id
    )
    WHERE rowid = (
        SELECT rowid FROM archives WHERE id = NEW.archive_id
    );
END;
