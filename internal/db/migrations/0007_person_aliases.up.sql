-- F23 (ADR-036): owner-curated alternate names for a person (stage names,
-- nicknames, romanizations). Kept in their own table — never a column on `people`
-- — so the scanner's write path (which upserts people.name) never touches user
-- data, mirroring the entity_enrichment separation. Each alias is indexed in its
-- own external-content FTS5 mirror so global search matches any alias (ADR-017).
--
-- alias is COLLATE NOCASE so uniqueness is per-person case-insensitive (matching
-- the people.name collation intent); the stored value keeps its original casing
-- for display and FTS. ON DELETE CASCADE means an alias never outlives its person.
CREATE TABLE person_aliases (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    person_id INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    alias     TEXT    NOT NULL COLLATE NOCASE,
    UNIQUE (person_id, alias)
);
CREATE INDEX idx_person_aliases_person ON person_aliases(person_id);
-- alias lookup is on the scanner write path (name→canonical resolution, ADR-036)
-- and the collision check; index it. COLLATE NOCASE is inherited from the column.
CREATE INDEX idx_person_aliases_alias  ON person_aliases(alias);

-- External-content FTS mirror of person_aliases.alias, kept in sync by triggers —
-- identical in shape to people_fts (migration 0001). unicode61 + diacritic folding
-- so "beyonce" matches an alias "Beyoncé".
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
