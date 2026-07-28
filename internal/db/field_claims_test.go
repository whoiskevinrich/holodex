package db_test

import "testing"

// Migration 0029 (ADR-074) is append-only with a manual down. The property worth pinning
// is the PRIMARY KEY grain: it carries `provider`, deliberately wider than
// field_promotions', so one provider's spelling of a key can be claimed while another
// provider's identically-named key is not. A field_promotions-shaped PK would reject the
// second row, which is exactly the shape option B was rejected for.
func TestMigration0029FieldClaimsProviderGrain(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(29); err != nil {
		t.Fatalf("migrate to 29: %v", err)
	}

	// Same entity type, same field key, two providers — two different assertions.
	mustExec(t, db, `INSERT INTO field_claims (entity_type, provider, field_key, canonical, created_at, updated_at)
	                 VALUES ('video','provA','rating','content_rating','2026-07-28T00:00:00Z','2026-07-28T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO field_claims (entity_type, provider, field_key, canonical, created_at, updated_at)
	                 VALUES ('video','provB','rating','user_score','2026-07-28T00:00:00Z','2026-07-28T00:00:00Z')`)
	if n := count(t, db, `SELECT COUNT(*) FROM field_claims WHERE field_key = 'rating'`); n != 2 {
		t.Fatalf("same key on two providers = %d rows, want 2 — the PK must carry provider", n)
	}
	// The same (entity_type, provider, field_key) is one row, re-pointable by upsert.
	mustExec(t, db, `INSERT INTO field_claims (entity_type, provider, field_key, canonical, created_at, updated_at)
	                 VALUES ('video','provA','rating','certification','2026-07-28T00:00:00Z','2026-07-28T00:01:00Z')
	                 ON CONFLICT(entity_type, provider, field_key) DO UPDATE SET
	                     canonical = excluded.canonical, updated_at = excluded.updated_at`)
	if n := count(t, db, `SELECT COUNT(*) FROM field_claims
	                      WHERE provider = 'provA' AND canonical = 'certification'`); n != 1 {
		t.Fatal("re-pointing a claim must upsert in place, not insert a second row")
	}

	// Down drops the table; up re-creates it empty and usable.
	if err := m.Migrate(28); err != nil {
		t.Fatalf("migrate down to 28: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'field_claims'`); n != 0 {
		t.Error("field_claims survived the down migration")
	}
	if err := m.Migrate(29); err != nil {
		t.Fatalf("re-migrate to 29: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM field_claims`); n != 0 {
		t.Errorf("re-applied migration should leave an empty table, got %d rows", n)
	}
}
