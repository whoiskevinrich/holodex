-- Down for 0022 (F43 name-identity spine). Restores the pre-F43 SCHEMA: drops the
-- canonical nameKey unique indexes, recreates person_aliases + its FTS, copies the
-- person rows back out of the shared store, then drops the shared spine tables.
--
-- The one-time fold of exact-case canonical duplicates is NOT reversible — merged
-- associations and dropped duplicate rows cannot be un-merged (golang-migrate has no
-- auto-rollback; this matches the repo's merge migrations). A down leaves the folded
-- library folded; only the schema is reverted.

DROP INDEX IF EXISTS ux_people_namekey;
DROP INDEX IF EXISTS ux_studios_namekey;
DROP INDEX IF EXISTS ux_tags_namekey;

-- Recreate the F23 person_aliases table + FTS mirror (migration 0007 shape).
CREATE TABLE person_aliases (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    person_id INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    alias     TEXT    NOT NULL COLLATE NOCASE,
    UNIQUE (person_id, alias)
);
CREATE INDEX idx_person_aliases_person ON person_aliases(person_id);
CREATE INDEX idx_person_aliases_alias  ON person_aliases(alias);

CREATE VIRTUAL TABLE person_aliases_fts USING fts5(
    alias,
    content='person_aliases',
    content_rowid='id',
    tokenize="unicode61 remove_diacritics 2"
);
CREATE TRIGGER person_aliases_ai AFTER INSERT ON person_aliases BEGIN
    INSERT INTO person_aliases_fts(rowid, alias) VALUES (new.id, new.alias);
END;
CREATE TRIGGER person_aliases_ad AFTER DELETE ON person_aliases BEGIN
    INSERT INTO person_aliases_fts(person_aliases_fts, rowid, alias) VALUES('delete', old.id, old.alias);
END;
CREATE TRIGGER person_aliases_au AFTER UPDATE ON person_aliases BEGIN
    INSERT INTO person_aliases_fts(person_aliases_fts, rowid, alias) VALUES('delete', old.id, old.alias);
    INSERT INTO person_aliases_fts(rowid, alias) VALUES (new.id, new.alias);
END;

-- Copy person aliases back (the trigger repopulates person_aliases_fts).
INSERT OR IGNORE INTO person_aliases (person_id, alias)
    SELECT entity_id, alias FROM entity_aliases WHERE entity_type = 'person';

-- Drop the entity-delete cleanup triggers before entity_aliases (they reference it).
DROP TRIGGER IF EXISTS people_ad_aliases;
DROP TRIGGER IF EXISTS studios_ad_aliases;
DROP TRIGGER IF EXISTS tags_ad_aliases;

DROP TABLE identity_review_queue;
DROP TABLE entity_keep_separate;
DROP TABLE entity_aliases_fts;
DROP TRIGGER IF EXISTS entity_aliases_ai;
DROP TRIGGER IF EXISTS entity_aliases_ad;
DROP TRIGGER IF EXISTS entity_aliases_au;
DROP TABLE entity_aliases;
