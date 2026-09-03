package db_test

import (
	"database/sql"
	"testing"

	"github.com/golang-migrate/migrate/v4"
)

// seedAliasCollapse migrates to 43 (one before the collapse) and seeds the pre-migration
// shapes the assertions below read: three people, one pre-existing owner alias, and the
// metadata_curation rows the retired 'aliases' field could hold.
func seedAliasCollapse(t *testing.T) (*sql.DB, *migrate.Migrate) {
	t.Helper()
	db, m := openAt(t)
	if err := m.Migrate(43); err != nil { // one before the alias collapse
		t.Fatalf("migrate to 43: %v", err)
	}

	mustExec(t, db, `INSERT INTO people (id, name) VALUES
		(1,'Jennifer Lawrence'),(2,'Hayao Miyazaki'),(3,'Someone Else')`)

	// A pre-existing owner-authored alias. Part 1's DEFAULT has to backfill it correctly
	// with no UPDATE, and its identity must survive the migration untouched.
	mustExec(t, db, `INSERT INTO entity_aliases (entity_type, entity_id, alias) VALUES
		('person', 1, 'J Law')`)

	// The curation the old display-only "Also known as" row accumulated.
	mustExec(t, db, `INSERT INTO metadata_curation
		(entity_type, entity_id, field_key, norm_value, value, action, source, created_at) VALUES
		-- promoted to an owner alias row
		('person', 2, 'aliases', 'miyazaki hayao', 'Miyazaki Hayao', 'add',      'manual', '2026-01-01T00:00:00Z'),
		-- promoted to a suppression row
		('person', 2, 'aliases', 'hayao m',        'Hayao M.',       'suppress', 'manual', '2026-01-01T00:00:00Z'),
		-- collides with person 1's existing 'J Law' on the global UNIQUE (entity_type,
		-- alias_key); must be skipped without aborting the migration
		('person', 3, 'aliases', 'j law',          'j law',          'add',      'manual', '2026-01-01T00:00:00Z'),
		-- 'nowrite' has no meaning once the field leaves the registry; deleted, not promoted
		('person', 2, 'aliases', 'nowrite one',    'Nowrite One',    'nowrite',  'manual', '2026-01-01T00:00:00Z'),
		-- entity_aliases has no 'video' entity; deleted, but never promoted into the spine
		('video',  9, 'aliases', 'stray',          'Stray',          'add',      'manual', '2026-01-01T00:00:00Z'),
		-- a different field must be left completely alone
		('person', 2, 'other',   'x',              'X',              'add',      'manual', '2026-01-01T00:00:00Z')`)
	return db, m
}

// TestMigration0044PromotesAliasCuration covers the data-transforming half of the
// HOLODEX-306 collapse (ADR-088 D6, spec F58 P0-1): metadata_curation rows against the
// retired 'aliases' field carry real owner intent, so the migration converts them into
// first-class identity rows rather than dropping them. Modelled on
// TestMigration0022FoldsCaseDuplicates, the repo's other data-transforming migration test.
func TestMigration0044PromotesAliasCuration(t *testing.T) {
	db, m := seedAliasCollapse(t)
	if err := m.Migrate(44); err != nil {
		t.Fatalf("migrate to 44 (alias collapse): %v", err)
	}

	// An 'add' becomes an owner alias row — source empty, not the provider namespace,
	// because the owner authored it.
	if n := count(t, db, `SELECT COUNT(*) FROM entity_aliases
		WHERE entity_type='person' AND entity_id=2 AND alias='Miyazaki Hayao' AND source=''`); n != 1 {
		t.Fatalf("curated 'add' not promoted to an owner alias row")
	}

	// The payoff: a promoted alias is searchable, because the FTS triggers fire on insert.
	// An owner who curated the old display-only row now has a name that actually finds them.
	if n := count(t, db, `SELECT COUNT(*) FROM entity_aliases_fts WHERE entity_aliases_fts MATCH 'Miyazaki'`); n == 0 {
		t.Fatalf("promoted alias never reached entity_aliases_fts")
	}

	// A 'suppress' becomes a suppression row, keyed with entity_aliases.alias_key's own
	// fold (lower+trim for a person) rather than curationNorm's.
	if n := count(t, db, `SELECT COUNT(*) FROM entity_alias_suppressions
		WHERE entity_type='person' AND entity_id=2 AND alias_key='hayao m.'`); n != 1 {
		t.Fatalf("curated 'suppress' not promoted to a suppression row")
	}

	// The collision is the case INSERT OR IGNORE exists for: person 3's curated name is
	// already person 1's alias, and that one row must not take the whole upgrade with it.
	// Reaching this line at all proves the migration completed.
	if n := count(t, db, `SELECT COUNT(*) FROM entity_aliases WHERE entity_type='person' AND entity_id=3`); n != 0 {
		t.Fatalf("colliding curated name was promoted; it belongs to person 1")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM entity_aliases WHERE entity_type='person' AND alias_key='j law'`); n != 1 {
		t.Fatalf("collision produced a duplicate alias_key")
	}

	// 'nowrite' and the non-spine entity type are deleted but never promoted.
	if n := count(t, db, `SELECT COUNT(*) FROM entity_aliases WHERE alias='Nowrite One'`); n != 0 {
		t.Fatalf("'nowrite' curation was promoted; it has no meaning after the field is retired")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM entity_aliases WHERE entity_type='video'`); n != 0 {
		t.Fatalf("a 'video' curation row was promoted into the identity spine")
	}

	// Every 'aliases' curation row is gone; other fields are untouched.
	if n := count(t, db, `SELECT COUNT(*) FROM metadata_curation WHERE field_key='aliases'`); n != 0 {
		t.Fatalf("'aliases' curation rows survived the promotion")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM metadata_curation WHERE field_key='other'`); n != 1 {
		t.Fatalf("promotion deleted curation for an unrelated field")
	}

	// The pre-existing alias is byte-identical: same id, and source backfilled by the
	// column DEFAULT rather than by an UPDATE that could have touched anything else.
	if n := count(t, db, `SELECT COUNT(*) FROM entity_aliases
		WHERE id=1 AND entity_type='person' AND entity_id=1 AND alias='J Law' AND source=''`); n != 1 {
		t.Fatalf("pre-existing alias row was altered by the migration")
	}
}

// TestMigration0044SuppressionsDieWithTheirEntity pins the invariant 0022 established for
// entity_aliases ("an alias never outlives its entity") across the new suppressions table.
// Both are polymorphic, so no FK can express it and nothing else would ever prune the rows.
func TestMigration0044SuppressionsDieWithTheirEntity(t *testing.T) {
	db, m := seedAliasCollapse(t)
	if err := m.Migrate(44); err != nil {
		t.Fatalf("migrate to 44: %v", err)
	}
	mustExec(t, db, `INSERT INTO studios (id, name) VALUES (1,'Ghibli')`)
	mustExec(t, db, `INSERT INTO entity_alias_suppressions (entity_type, entity_id, alias_key)
		VALUES ('studio', 1, 'studio ghibli')`)

	// Person 2 carries a promoted suppression; deleting the person must take it.
	mustExec(t, db, `DELETE FROM people WHERE id = 2`)
	if n := count(t, db, `SELECT COUNT(*) FROM entity_alias_suppressions WHERE entity_type='person' AND entity_id=2`); n != 0 {
		t.Fatalf("suppression outlived its person")
	}
	// Scoped: the studio's own suppression is untouched by a person delete.
	if n := count(t, db, `SELECT COUNT(*) FROM entity_alias_suppressions WHERE entity_type='studio'`); n != 1 {
		t.Fatalf("person delete removed a studio's suppression")
	}
	mustExec(t, db, `DELETE FROM studios WHERE id = 1`)
	if n := count(t, db, `SELECT COUNT(*) FROM entity_alias_suppressions`); n != 0 {
		t.Fatalf("suppression outlived its studio")
	}
}

// TestMigration0044Down reverses the schema and deliberately does not un-promote the data —
// once promoted an alias is indistinguishable from any other owner-authored row, so
// reconstructing metadata_curation from it would have to guess. Asserted so the lossiness
// stays a recorded decision (spec F58 P0-1) rather than a surprise.
func TestMigration0044Down(t *testing.T) {
	db, m := seedAliasCollapse(t)
	if err := m.Migrate(44); err != nil {
		t.Fatalf("migrate to 44: %v", err)
	}
	if err := m.Migrate(43); err != nil {
		t.Fatalf("migrate down to 43: %v", err)
	}

	// Schema is back: no suppressions table, no source column.
	if _, err := db.Exec(`SELECT 1 FROM entity_alias_suppressions`); err == nil {
		t.Fatalf("entity_alias_suppressions survived the down migration")
	}
	if _, err := db.Exec(`SELECT source FROM entity_aliases`); err == nil {
		t.Fatalf("entity_aliases.source survived the down migration")
	}

	// The cleanup triggers went with it — they live on people/studios/tags, so dropping the
	// table would not have removed them and this DELETE would fail on a missing table.
	mustExec(t, db, `DELETE FROM people WHERE id = 3`)

	// The promoted alias stays, and the aliases the owner already had are unharmed.
	if n := count(t, db, `SELECT COUNT(*) FROM entity_aliases WHERE alias='Miyazaki Hayao'`); n != 1 {
		t.Fatalf("down migration removed a promoted alias")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM entity_aliases WHERE alias='J Law'`); n != 1 {
		t.Fatalf("down migration disturbed a pre-existing alias")
	}
}

// TestMigration0045ReviewQueueDetail covers the column the Aliases panel's review line
// depends on (F58 P0-8): a provider-alias collision's skipped name exists nowhere else
// once the enrich pass ends, since F58 stopped storing provider aliases in the shadow
// layer. The DEFAULT has to backfill every existing near-miss row without an UPDATE.
func TestMigration0045ReviewQueueDetail(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(44); err != nil { // one before the detail column
		t.Fatalf("migrate to 44: %v", err)
	}
	mustExec(t, db, `INSERT INTO people (id, name) VALUES (1,'Fox'),(2,'Foxx')`)
	mustExec(t, db, `INSERT INTO identity_review_queue (entity_type, id_lo, id_hi, variation)
		VALUES ('person', 1, 2, 'punctuation')`)

	if err := m.Migrate(45); err != nil {
		t.Fatalf("migrate to 45: %v", err)
	}
	// The pre-existing near-miss survives with an empty detail — it has no name to
	// carry, and nothing should have rewritten it.
	if n := count(t, db, `SELECT COUNT(*) FROM identity_review_queue WHERE detail = ''`); n != 1 {
		t.Fatalf("existing near-miss did not take the column default")
	}
	// And a provider-alias row can now carry one.
	mustExec(t, db, `INSERT INTO people (id, name) VALUES (3,'Foxy')`)
	mustExec(t, db, `INSERT INTO identity_review_queue (entity_type, id_lo, id_hi, variation, detail)
		VALUES ('person', 1, 3, 'provider-alias', 'The Fox')`)
	if n := count(t, db, `SELECT COUNT(*) FROM identity_review_queue WHERE detail = 'The Fox'`); n != 1 {
		t.Fatalf("provider-alias detail did not round-trip")
	}

	if err := m.Migrate(44); err != nil {
		t.Fatalf("migrate down to 44: %v", err)
	}
	if _, err := db.Exec(`SELECT detail FROM identity_review_queue`); err == nil {
		t.Fatalf("detail column survived the down migration")
	}
}
