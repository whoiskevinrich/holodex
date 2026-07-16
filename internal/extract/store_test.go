package extract_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"holodex/internal/db"
	"holodex/internal/extract"
	"holodex/internal/mapping"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

func newRepo(t *testing.T) *repo.Repo {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return repo.New(database)
}

// TestStore_RoundTripsThroughEntityEnrichment proves F48.2a: no migration is
// needed for storage — the existing entity_enrichment table (migration 0005)
// accepts the "filename" provider exactly like "tmdb", with no schema change.
func TestStore_RoundTripsThroughEntityEnrichment(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)

	fields := map[string][]string{
		"title":        {"Big Movie"},
		"people":       {"Alice Smith", "Bob Jones"},
		"studio":       {"Acme Studios"},
		"release_date": {"2020"},
	}
	if err := extract.Store(ctx, r, "video", 42, fields); err != nil {
		t.Fatalf("Store: %v", err)
	}

	rows, err := r.EnrichmentForEntity(ctx, "video", 42)
	if err != nil {
		t.Fatalf("EnrichmentForEntity: %v", err)
	}

	got := make(map[string][]string, len(rows))
	for _, row := range rows {
		if row.Provider != extract.Provider {
			t.Errorf("row provider = %q, want %q", row.Provider, extract.Provider)
		}
		got[row.FieldKey] = row.Values
	}
	// UpsertEnrichment/EnrichmentForEntity round-trip multi-values in order
	// (newline-joined then split), so the stored fields must match the input
	// exactly — no sorting needed.
	if !reflect.DeepEqual(got, fields) {
		t.Fatalf("stored fields = %#v, want %#v", got, fields)
	}
}

// TestStore_EmptyFieldsIsNoop mirrors UpsertEnrichment's own no-op contract for
// an empty map (e.g. a filename that matched no pattern, F48.1b).
func TestStore_EmptyFieldsIsNoop(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)

	if err := extract.Store(ctx, r, "video", 1, nil); err != nil {
		t.Fatalf("Store: %v", err)
	}
	rows, err := r.EnrichmentForEntity(ctx, "video", 1)
	if err != nil {
		t.Fatalf("EnrichmentForEntity: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("EnrichmentForEntity = %d rows, want 0", len(rows))
	}
}

// TestFilenameSourceResolvesWithNoResolverChange is the F48.2b regression
// guard: a mapping.Field configured with a "filename:<field>" source resolves
// through the existing resolver.orderedSources iteration with zero resolver
// code changes — it's treated exactly like any other provider namespace
// (tmdb, …), which the resolver already handles generically.
func TestFilenameSourceResolvesWithNoResolverChange(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)

	if err := extract.Store(ctx, r, "video", 7, map[string][]string{"title": {"Extracted Title"}}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	rows, err := r.EnrichmentForEntity(ctx, "video", 7)
	if err != nil {
		t.Fatalf("EnrichmentForEntity: %v", err)
	}
	enrichment := resolver.Enrichment{}
	for _, row := range rows {
		if enrichment[row.Provider] == nil {
			enrichment[row.Provider] = map[string][]string{}
		}
		enrichment[row.Provider][row.FieldKey] = row.Values
	}

	field := mapping.Field{
		Canonical:     "title",
		ParsedSources: []mapping.Source{{Namespace: "filename", Key: "title"}},
	}
	resolved := resolver.ResolveFields(resolver.NewVideoBaseline(nil, nil), enrichment, nil, []mapping.Field{field}, resolver.Options{})
	if len(resolved) != 1 || len(resolved[0].Values) != 1 || resolved[0].Values[0] != "Extracted Title" {
		t.Fatalf("ResolveFields = %#v, want a single \"Extracted Title\" value", resolved)
	}
	if resolved[0].WinningSource != "filename:title" {
		t.Fatalf("WinningSource = %q, want %q", resolved[0].WinningSource, "filename:title")
	}
}
