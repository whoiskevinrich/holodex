-- HOLODEX-102/HOLODEX-125 (F32, ADR-055 person conformance): give a person entity a
-- stable provider identifier so cast/crew credits ingested from a video's people[]
-- resolve deterministically instead of by name only -- mirrors studio_external_ids
-- (migration 0018, ADR-054).
--
-- A join table, never a column on `people`: a person may carry ids from multiple
-- providers (e.g. tmdb + imdb). external_id is namespace-qualified ("tmdb:6384") and
-- is the PRIMARY KEY, so it is GLOBALLY UNIQUE -- one provider person id maps to
-- exactly one Person, which is the de-dup guarantee. resolveOrCreateByName consults
-- it BEFORE name/alias lookup (mirroring studio's ADR-054 §4 order), so a person
-- enriched twice under slightly different name spellings but the same provider id
-- converges to one row.
--
-- ON DELETE CASCADE ties id rows to their person: unlike studio's immediate
-- prune-on-empty, a person is only removed after ADR-072's 30-day orphan grace sweep
-- (people.orphaned_at) -- when that eventually deletes the row, its external ids go
-- with it. video_people is still the derived link index; this table only records
-- identity, never links.
CREATE TABLE person_external_ids (
    person_id   INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    external_id TEXT    NOT NULL,
    PRIMARY KEY (external_id)
);

-- Reverse lookup (a person's ids) and the cascade path; the external_id lookup is
-- already served by the PK.
CREATE INDEX idx_person_external_ids_person ON person_external_ids(person_id);
