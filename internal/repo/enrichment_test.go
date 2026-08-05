package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
)

// TestEnrichmentShadowStore covers the F22 shadow layer (ADR-033): upsert,
// multi-value split, match-id persistence, the cardinal "re-scan never touches
// enrichment" invariant, overwrite-on-refetch, and clear-by-provider.
func TestEnrichmentShadowStore(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	idA, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", []string{"Hayao Miyazaki"}, nil), nil)
	if err != nil {
		t.Fatalf("seed person: %v", err)
	}
	linkPeople(t, r, idA, "Hayao Miyazaki")
	pid, ok, err := r.PersonIDByName(ctx, "Hayao Miyazaki")
	if err != nil || !ok {
		t.Fatalf("person id: ok=%v err=%v", ok, err)
	}

	fields := map[string][]string{"bio": {"a filmmaker"}, "aliases": {"宮崎駿", "Miyazaki Hayao"}}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityPerson, pid, "tmdb", "tmdb:608", fields); err != nil {
		t.Fatalf("upsert enrichment: %v", err)
	}

	rows, err := r.EnrichmentForEntity(ctx, model.EnrichEntityPerson, pid)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	for _, row := range rows {
		if row.FieldKey == "aliases" && len(row.Values) != 2 {
			t.Errorf("aliases not split: %v", row.Values)
		}
	}

	if id, ok, _ := r.MatchExternalID(ctx, model.EnrichEntityPerson, pid, "tmdb"); !ok || id != "tmdb:608" {
		t.Errorf("match = %q ok=%v, want tmdb:608", id, ok)
	}

	// Cardinal invariant: re-scanning the file must not disturb the shadow rows.
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T2", []string{"Hayao Miyazaki"}, nil), nil); err != nil {
		t.Fatalf("rescan: %v", err)
	}
	if rows2, _ := r.EnrichmentForEntity(ctx, model.EnrichEntityPerson, pid); len(rows2) != 2 {
		t.Errorf("enrichment lost on re-scan: %d rows", len(rows2))
	}

	// Re-fetch overwrites by canonical key.
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityPerson, pid, "tmdb", "tmdb:608", map[string][]string{"bio": {"updated"}}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	rows3, _ := r.EnrichmentForEntity(ctx, model.EnrichEntityPerson, pid)
	for _, row := range rows3 {
		if row.FieldKey == "bio" && (len(row.Values) != 1 || row.Values[0] != "updated") {
			t.Errorf("bio not overwritten: %v", row.Values)
		}
	}

	// Clear by provider removes only that provider's rows.
	if n, err := r.DeleteEnrichmentByProvider(ctx, model.EnrichEntityPerson, pid, "tmdb"); err != nil || n == 0 {
		t.Fatalf("delete: n=%d err=%v", n, err)
	}
	if rows4, _ := r.EnrichmentForEntity(ctx, model.EnrichEntityPerson, pid); len(rows4) != 0 {
		t.Errorf("rows after clear = %d, want 0", len(rows4))
	}
}
