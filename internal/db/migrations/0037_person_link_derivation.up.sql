-- F40 (ADR-072): video_people migrates from scan-time raw-extraction to resolved-value
-- derivation (RelinkVideoPeople, generalized into RelinkVideoEntity alongside studio).
-- Two changes:
--   1. video_people gains `role`, derived from the source person-typed field ('actor',
--      'director', or '' for a role-less person-typed field — the unset sentinel).
--      SQLite treats NULL as distinct in a composite PK (two (v,p,NULL) rows would
--      coexist), so unset MUST be the empty string, never NULL. PK becomes
--      (video_id, person_id, role) so one person can hold two roles on one video.
--      SQLite has no ALTER TABLE ... ADD CONSTRAINT for a PK change, so the table is
--      rebuilt (existing rows carry the pre-derivation role-flat default '').
--   2. people.orphaned_at: a person whose last link is removed is stamped, not deleted
--      immediately (30-day grace + authored-identity guard, ADR-072 §4) — unlike
--      studio's immediate prune, since person identity is authored (aliases, merge
--      history, curated images), not purely derived.
CREATE TABLE video_people_new (
    video_id  INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    person_id INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    role      TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (video_id, person_id, role)
);
INSERT INTO video_people_new (video_id, person_id, role)
SELECT video_id, person_id, '' FROM video_people;
DROP TABLE video_people;
ALTER TABLE video_people_new RENAME TO video_people;
CREATE INDEX idx_video_people_person ON video_people(person_id);

ALTER TABLE people ADD COLUMN orphaned_at TEXT;
