-- Holodex initial schema (Phase 1).
-- See ADR-003 (SQLite), ADR-013 (video_metadata), ADR-017 (FTS5).

CREATE TABLE videos (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path    TEXT    NOT NULL UNIQUE,        -- canonical absolute path (ADR-011)
    file_size    INTEGER NOT NULL DEFAULT 0,
    title        TEXT    NOT NULL DEFAULT '',
    duration_sec INTEGER NOT NULL DEFAULT 0,
    width        INTEGER NOT NULL DEFAULT 0,
    height       INTEGER NOT NULL DEFAULT 0,
    recorded_at  TEXT,                            -- ISO date, nullable
    indexed_at   TEXT    NOT NULL,
    file_mtime   TEXT    NOT NULL,                -- for incremental change detection (ADR-018)
    active       INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX idx_videos_active       ON videos(active);
CREATE INDEX idx_videos_recorded_at  ON videos(recorded_at);
CREATE INDEX idx_videos_duration     ON videos(duration_sec);
CREATE INDEX idx_videos_width        ON videos(width);
CREATE INDEX idx_videos_indexed_at   ON videos(indexed_at);

CREATE TABLE people (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE tags (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE video_people (
    video_id  INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    person_id INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    PRIMARY KEY (video_id, person_id)
);
CREATE INDEX idx_video_people_person ON video_people(person_id);

CREATE TABLE video_tags (
    video_id INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    tag_id   INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (video_id, tag_id)
);
CREATE INDEX idx_video_tags_tag ON video_tags(tag_id);

-- Extended/raw container tags captured at index time (ADR-013, F2.9).
CREATE TABLE video_metadata (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    video_id   INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    source_key TEXT    NOT NULL,
    value      TEXT    NOT NULL
);
CREATE INDEX idx_video_metadata_video ON video_metadata(video_id);
CREATE INDEX idx_video_metadata_key   ON video_metadata(source_key, value);

-- ---------------------------------------------------------------------------
-- Full-text search (ADR-017): unicode61 tokenizer with diacritic folding.
-- External-content FTS tables kept in sync with triggers.
-- ---------------------------------------------------------------------------

CREATE VIRTUAL TABLE videos_fts USING fts5(
    title,
    content='videos',
    content_rowid='id',
    tokenize="unicode61 remove_diacritics 2"
);
CREATE TRIGGER videos_ai AFTER INSERT ON videos BEGIN
    INSERT INTO videos_fts(rowid, title) VALUES (new.id, new.title);
END;
CREATE TRIGGER videos_ad AFTER DELETE ON videos BEGIN
    INSERT INTO videos_fts(videos_fts, rowid, title) VALUES('delete', old.id, old.title);
END;
CREATE TRIGGER videos_au AFTER UPDATE ON videos BEGIN
    INSERT INTO videos_fts(videos_fts, rowid, title) VALUES('delete', old.id, old.title);
    INSERT INTO videos_fts(rowid, title) VALUES (new.id, new.title);
END;

CREATE VIRTUAL TABLE people_fts USING fts5(
    name,
    content='people',
    content_rowid='id',
    tokenize="unicode61 remove_diacritics 2"
);
CREATE TRIGGER people_ai AFTER INSERT ON people BEGIN
    INSERT INTO people_fts(rowid, name) VALUES (new.id, new.name);
END;
CREATE TRIGGER people_ad AFTER DELETE ON people BEGIN
    INSERT INTO people_fts(people_fts, rowid, name) VALUES('delete', old.id, old.name);
END;
CREATE TRIGGER people_au AFTER UPDATE ON people BEGIN
    INSERT INTO people_fts(people_fts, rowid, name) VALUES('delete', old.id, old.name);
    INSERT INTO people_fts(rowid, name) VALUES (new.id, new.name);
END;

CREATE VIRTUAL TABLE tags_fts USING fts5(
    name,
    content='tags',
    content_rowid='id',
    tokenize="unicode61 remove_diacritics 2"
);
CREATE TRIGGER tags_ai AFTER INSERT ON tags BEGIN
    INSERT INTO tags_fts(rowid, name) VALUES (new.id, new.name);
END;
CREATE TRIGGER tags_ad AFTER DELETE ON tags BEGIN
    INSERT INTO tags_fts(tags_fts, rowid, name) VALUES('delete', old.id, old.name);
END;
CREATE TRIGGER tags_au AFTER UPDATE ON tags BEGIN
    INSERT INTO tags_fts(tags_fts, rowid, name) VALUES('delete', old.id, old.name);
    INSERT INTO tags_fts(rowid, name) VALUES (new.id, new.name);
END;
