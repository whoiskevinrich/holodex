package db_test

import (
	"database/sql"
	"regexp"
	"sort"
	"strconv"
	"testing"

	"holodex/internal/db/migrations"
)

// Migration 0048 (F65, ADR-099 D4): completeness_dirty is fed by triggers on
// every table resolver.Complete reads from. This test is the enumeration the
// ADR calls for — one write per input table, each of which must leave a dirty
// row — and it cross-checks the table list against the migration text, so a
// new input table cannot be added without extending both the trigger set and
// this list.

// completenessInputs is the exhaustive list: table → a write against it plus
// the (entity_type, entity_id) the write must dirty. Ids refer to the fixture
// seeded by seedCompletenessFixture (video 1, person 1, studio 1, tag 1, film 1).
var completenessInputs = []struct {
	table      string
	write      string
	entityType string
	entityID   int64
}{
	{"videos", `UPDATE videos SET title = 'renamed' WHERE id = 1`, "video", 1},
	{"people", `INSERT INTO people (name) VALUES ('new person')`, "person", 2},
	{"studios", `INSERT INTO studios (name) VALUES ('new studio')`, "studio", 2},
	{"video_metadata", `INSERT INTO video_metadata (video_id, source_key, value) VALUES (1, 'Publisher', 'Acme')`, "video", 1},
	{"entity_enrichment", `INSERT INTO entity_enrichment (entity_type, entity_id, provider, field_key, value, fetched_at) VALUES ('person', 1, 'tmdb', 'bio', 'x', '2026-09-18T00:00:00Z')`, "person", 1},
	{"metadata_curation", `INSERT INTO metadata_curation (entity_type, entity_id, field_key, norm_value, value, action, created_at) VALUES ('video', 1, 'genres', 'x', 'x', 'add', '2026-09-18T00:00:00Z')`, "video", 1},
	{"field_source_decisions", `INSERT INTO field_source_decisions (entity_type, entity_id, field_key, source, created_at) VALUES ('studio', 1, 'description', 'manual', '2026-09-18T00:00:00Z')`, "studio", 1},
	{"facet_not_applicable", `INSERT INTO facet_not_applicable (entity_type, entity_id, canonical_field, created_at) VALUES ('video', 1, 'external_provider_id', '2026-09-18T00:00:00Z')`, "video", 1},
	{"entity_aliases", `INSERT INTO entity_aliases (entity_type, entity_id, alias) VALUES ('person', 1, 'Also Known As')`, "person", 1},
	{"video_people", `INSERT INTO video_people (video_id, person_id, role) VALUES (1, 1, 'actor')`, "video", 1},
	{"video_studios", `INSERT INTO video_studios (video_id, studio_id) VALUES (1, 1)`, "video", 1},
	{"video_tags", `INSERT INTO video_tags (video_id, tag_id) VALUES (1, 1)`, "video", 1},
	{"film_videos", `INSERT INTO film_videos (film_id, video_id, created_at) VALUES (1, 1, '2026-09-18T00:00:00Z')`, "video", 1},
	{"person_images", `INSERT INTO person_images (person_id, role, source, width, height, byte_size, created_at) VALUES (1, 'headshot', 'upload', 1, 1, 1, '2026-09-18T00:00:00Z')`, "person", 1},
	{"studio_images", `INSERT INTO studio_images (studio_id, role, source, width, height, byte_size, created_at) VALUES (1, 'logo', 'upload', 1, 1, 1, '2026-09-18T00:00:00Z')`, "studio", 1},
	{"tags", `UPDATE tags SET writeback_enabled = 0 WHERE id = 1`, "video", 1},
	{"denied_tags", `INSERT INTO denied_tags (term_key, term, created_at) VALUES ('gnome', 'Gnome', '2026-09-18T00:00:00Z')`, "video", 1},
	{"field_promotions", `INSERT INTO field_promotions (entity_type, field_key, created_at, updated_at) VALUES ('person', 'height', '2026-09-18T00:00:00Z', '2026-09-18T00:00:00Z')`, "person", 1},
	{"field_claims", `INSERT INTO field_claims (entity_type, provider, field_key, canonical, created_at, updated_at) VALUES ('studio', 'tmdb', 'hq', 'country', '2026-09-18T00:00:00Z', '2026-09-18T00:00:00Z')`, "studio", 1},
	{"provider_field_hints", `INSERT INTO provider_field_hints (provider, field_key, updated_at) VALUES ('tmdb', 'height', '2026-09-18T00:00:00Z')`, "video", 1},
}

func seedCompletenessFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	mustExec(t, db, `INSERT INTO videos (id, file_path, file_size, title, duration_sec, width, height, indexed_at, file_mtime, active) VALUES (1, '/m/a.mkv', 1, 'A', 1, 1, 1, '2026-09-18T00:00:00Z', '2026-09-18T00:00:00Z', 1)`)
	mustExec(t, db, `INSERT INTO people (id, name) VALUES (1, 'Ada')`)
	mustExec(t, db, `INSERT INTO studios (id, name) VALUES (1, 'Acme')`)
	mustExec(t, db, `INSERT INTO tags (id, name) VALUES (1, 'drama')`)
	mustExec(t, db, `INSERT INTO films (id, name) VALUES (1, 'Film')`)
	mustExec(t, db, `DELETE FROM completeness_dirty`)
}

func TestMigration0048_EveryCompletenessInputDirtiesItsEntity(t *testing.T) {
	db, m := openAt(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	seedCompletenessFixture(t, db)

	for _, tc := range completenessInputs {
		t.Run(tc.table, func(t *testing.T) {
			mustExec(t, db, `DELETE FROM completeness_dirty`)
			mustExec(t, db, tc.write)
			q := `SELECT COUNT(*) FROM completeness_dirty WHERE entity_type = '` + tc.entityType + `' AND entity_id = ` + strconv.FormatInt(tc.entityID, 10)
			if n := count(t, db, q); n != 1 {
				t.Errorf("write to %s left %d dirty rows for %s/%d, want 1", tc.table, n, tc.entityType, tc.entityID)
			}
		})
	}
}

// The scanner rewrites every videos row on every scan (UpsertVideo's ON CONFLICT
// DO UPDATE lists title among its SET columns even when unchanged); the videos
// update trigger must fire only on a real change to a column Complete reads.
func TestMigration0048_VideoUpdateDirtiesOnlyOnScoredColumnChange(t *testing.T) {
	db, m := openAt(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	seedCompletenessFixture(t, db)

	mustExec(t, db, `UPDATE videos SET title = title, file_size = 2, duration_sec = 9 WHERE id = 1`)
	if n := count(t, db, `SELECT COUNT(*) FROM completeness_dirty`); n != 0 {
		t.Errorf("a scan-shaped rewrite (same title, other columns changed) dirtied %d rows, want 0", n)
	}
	mustExec(t, db, `UPDATE videos SET deleted_at = '2026-09-18T00:00:00Z' WHERE id = 1`)
	if n := count(t, db, `SELECT COUNT(*) FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = 1`); n != 1 {
		t.Errorf("soft-delete dirtied %d rows, want 1", n)
	}
}

// A conflict clause on the firing statement overrides the one inside a trigger
// body in SQLite, which is why 0048 uses INSERT ... WHERE NOT EXISTS rather than
// INSERT OR IGNORE: an already-dirty entity written again via an upsert must
// neither fail nor duplicate.
func TestMigration0048_TriggersSurviveUpsertConflictPolicy(t *testing.T) {
	db, m := openAt(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	seedCompletenessFixture(t, db)
	mustExec(t, db, `INSERT INTO completeness_dirty (entity_type, entity_id) VALUES ('video', 1)`)
	mustExec(t, db, `INSERT OR REPLACE INTO video_metadata (id, video_id, source_key, value) VALUES (1, 1, 'Publisher', 'Acme')`)
	mustExec(t, db, `INSERT INTO facet_not_applicable (entity_type, entity_id, canonical_field, created_at) VALUES ('video', 1, 'poster_url', 'x')
		ON CONFLICT (entity_type, entity_id, canonical_field) DO UPDATE SET created_at = excluded.created_at`)
	if n := count(t, db, `SELECT COUNT(*) FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = 1`); n != 1 {
		t.Errorf("dirty rows for video 1 = %d, want exactly 1", n)
	}
}

// Deleting an entity row must drop every derived row, not leave a dirty
// tombstone the drain can never resolve.
func TestMigration0048_EntityDeleteDropsDerivedRows(t *testing.T) {
	db, m := openAt(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	seedCompletenessFixture(t, db)
	mustExec(t, db, `INSERT INTO entity_completeness (entity_type, entity_id, required, extras, computed_at) VALUES ('person', 1, 0, 50, 'x')`)
	mustExec(t, db, `INSERT INTO entity_completeness_missing (entity_type, entity_id, canonical, band) VALUES ('person', 1, 'photo', 'critical')`)
	mustExec(t, db, `INSERT INTO completeness_dirty (entity_type, entity_id) VALUES ('person', 1)`)
	mustExec(t, db, `DELETE FROM people WHERE id = 1`)
	for _, tbl := range []string{"entity_completeness", "entity_completeness_missing", "completeness_dirty"} {
		if n := count(t, db, `SELECT COUNT(*) FROM `+tbl+` WHERE entity_type = 'person' AND entity_id = 1`); n != 0 {
			t.Errorf("%s still has %d rows for the deleted person", tbl, n)
		}
	}
}

// The test list above and the migration's trigger set must name the same
// tables: this is what makes "the migration is the list" enforceable.
func TestMigration0048_TriggerSetMatchesEnumeratedInputs(t *testing.T) {
	src, err := migrations.FS.ReadFile("0048_entity_completeness.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?m)^CREATE TRIGGER cd_\w+ AFTER (?:INSERT|UPDATE(?: OF [\w, ]+)?|DELETE) ON (\w+)`)
	inMigration := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		inMigration[m[1]] = true
	}
	enumerated := map[string]bool{}
	for _, tc := range completenessInputs {
		enumerated[tc.table] = true
	}
	if diff := setDiff(inMigration, enumerated); len(diff) > 0 {
		t.Errorf("tables with a 0048 trigger but no entry in completenessInputs: %v", diff)
	}
	if diff := setDiff(enumerated, inMigration); len(diff) > 0 {
		t.Errorf("tables enumerated in completenessInputs with no 0048 trigger: %v", diff)
	}
}

func setDiff(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
