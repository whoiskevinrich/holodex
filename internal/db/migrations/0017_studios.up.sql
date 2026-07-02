-- F38 (ADR-053): promote `studio` from a resolved video field to a first-class
-- entity. The shape is the 0001 people/tags block verbatim — a name-keyed entity
-- table, a composite-PK join, and an external-content FTS5 mirror kept in sync by
-- triggers (ADR-017).
--
-- Unlike video_people (derived from raw file extraction at scan time), video_studios
-- is a DERIVED INDEX over the RESOLVED `studio` field: RelinkVideoStudios reconciles
-- it on scan/enrich/decision/curation, with prune-on-empty (ADR-053 §2). No column
-- on `videos` — a join keeps headroom for a multi-valued studio mapping (RD2).
CREATE TABLE studios (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE video_studios (
    video_id  INTEGER NOT NULL REFERENCES videos(id)  ON DELETE CASCADE,
    studio_id INTEGER NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    PRIMARY KEY (video_id, studio_id)
);
CREATE INDEX idx_video_studios_studio ON video_studios(studio_id);

-- External-content FTS mirror of studios.name, kept in sync by triggers — identical
-- in shape to people_fts / tags_fts (migration 0001). unicode61 + diacritic folding
-- so "cinematheque" matches "Cinémathèque".
CREATE VIRTUAL TABLE studios_fts USING fts5(
    name,
    content='studios',
    content_rowid='id',
    tokenize="unicode61 remove_diacritics 2"
);
CREATE TRIGGER studios_ai AFTER INSERT ON studios BEGIN
    INSERT INTO studios_fts(rowid, name) VALUES (new.id, new.name);
END;
CREATE TRIGGER studios_ad AFTER DELETE ON studios BEGIN
    INSERT INTO studios_fts(studios_fts, rowid, name) VALUES('delete', old.id, old.name);
END;
CREATE TRIGGER studios_au AFTER UPDATE ON studios BEGIN
    INSERT INTO studios_fts(studios_fts, rowid, name) VALUES('delete', old.id, old.name);
    INSERT INTO studios_fts(rowid, name) VALUES (new.id, new.name);
END;
