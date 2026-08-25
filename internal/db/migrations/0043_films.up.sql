-- F56 (ADR-085): films become a first-class entity. Structurally the opposite of
-- studio (migration 0017): video_studios is a DERIVED index the resolver reconciles
-- on scan/enrich/decision/curation via RelinkVideoStudios; film_videos is an
-- ASSERTED owner link with no reconciler at all -- its only writers are the
-- attach/bulk-attach/detach endpoints. Nothing in this migration wires a relink
-- trigger, and none should ever be added (spec RD1/ADR-085 §2).
CREATE TABLE films (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    year        INTEGER,
    UNIQUE (name, year)                    -- (name, year), not bare name UNIQUE: film-name
                                            -- collisions across different releases/years are
                                            -- the common case here, unlike studio names
);

CREATE TABLE film_videos (
    film_id       INTEGER NOT NULL REFERENCES films(id)  ON DELETE CASCADE,
    video_id      INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    scene_number  INTEGER,                 -- NULL = unnumbered
    is_full_film  INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL,
    PRIMARY KEY (film_id, video_id),       -- one link per (film, video); no ranges (spec Non-Goal)
    UNIQUE (film_id, scene_number)
);
CREATE INDEX idx_film_videos_video ON film_videos(video_id);

-- UNIQUE(film_id, scene_number) deliberately relies on SQLite/ANSI SQL treating NULL as
-- distinct in a UNIQUE constraint, so any number of unnumbered scenes coexist while
-- numbered ones collide (spec RD5). This is the OPPOSITE fix from migration 0037's
-- video_people, where that same NULL-distinctness was a bug worked around with an
-- empty-string sentinel for `role` -- do not "fix" this table by copying that pattern;
-- NULL is the wanted behavior here, not a bug.

-- film_people_roles is a property of the FILM, not derived from any video: billing/role
-- data the owner enters directly on the film page. Deliberately separate from
-- video_people (migration 0037), which stays unchanged and per-video (spec RD3).
CREATE TABLE film_people_roles (
    film_id       INTEGER NOT NULL REFERENCES films(id)  ON DELETE CASCADE,
    person_id     INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    role          TEXT    NOT NULL DEFAULT '',  -- '' sentinel mirrors migration 0037's video_people.role
    billing_order INTEGER,
    PRIMARY KEY (film_id, person_id, role)
);

-- External-content FTS mirror of films.name, kept in sync by triggers -- identical in
-- shape to studios_fts (migration 0017).
CREATE VIRTUAL TABLE films_fts USING fts5(
    name,
    content='films',
    content_rowid='id',
    tokenize="unicode61 remove_diacritics 2"
);
CREATE TRIGGER films_ai AFTER INSERT ON films BEGIN
    INSERT INTO films_fts(rowid, name) VALUES (new.id, new.name);
END;
CREATE TRIGGER films_ad AFTER DELETE ON films BEGIN
    INSERT INTO films_fts(films_fts, rowid, name) VALUES('delete', old.id, old.name);
END;
CREATE TRIGGER films_au AFTER UPDATE ON films BEGIN
    INSERT INTO films_fts(films_fts, rowid, name) VALUES('delete', old.id, old.name);
    INSERT INTO films_fts(rowid, name) VALUES (new.id, new.name);
END;

-- Poster/thumb image slots, shaped after studio_images (migration 0036, ADR-079) with
-- one deliberate difference: UNIQUE is (film_id, role, source) rather than (film_id,
-- role) alone, so an uploaded poster and a provider-sourced poster can coexist as
-- distinct rows instead of the studio model's single-slot-per-role. Bytes live on disk
-- at DATA_PATH/film-images/{film_id}/{id}.jpg (ADR-014) and never in the DB; this table
-- is only the metadata index.
CREATE TABLE film_images (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    film_id     INTEGER NOT NULL REFERENCES films(id) ON DELETE CASCADE,
    role        TEXT    NOT NULL,            -- 'poster' | 'thumb'
    source      TEXT    NOT NULL,            -- 'upload' | 'provider:<name>'
    provider    TEXT    NOT NULL DEFAULT '',
    external_id TEXT    NOT NULL DEFAULT '',
    width       INTEGER NOT NULL,
    height      INTEGER NOT NULL,
    byte_size   INTEGER NOT NULL,
    created_at  TEXT    NOT NULL,
    UNIQUE (film_id, role, source)
);
CREATE INDEX idx_film_images_film ON film_images(film_id);
