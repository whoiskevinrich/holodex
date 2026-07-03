package api

import (
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// TestStudioExternalIDsFromRows covers the ADR-054 side-map parse: only
// _studio_external_ids rows contribute; each value "<external_id> <name>" splits on
// the first space (names keep their internal spaces); malformed entries are skipped;
// no sidecar rows → nil map (name-only resolve).
func TestStudioExternalIDsFromRows(t *testing.T) {
	rows := []repo.EnrichmentRow{
		{Provider: "tmdb", FieldKey: "studio", Values: []string{"Warner Bros. Pictures", "Legendary"}},
		// Well-formed pairs (names keep internal spaces/dots), then three malformed
		// entries that must be skipped: blank name after the id, no separator, nothing.
		{Provider: "tmdb", FieldKey: model.StudioExternalIDsField, Values: []string{
			"tmdb:174 Warner Bros. Pictures",
			"tmdb:923 Legendary",
			"tmdb:5 ",
			"noSpaceToken",
			" ",
		}},
		{Provider: "tmdb", FieldKey: "overview", Values: []string{"unrelated"}},
	}

	got := studioExternalIDsFromRows(rows)
	want := map[string]string{
		"Warner Bros. Pictures": "tmdb:174",
		"Legendary":             "tmdb:923",
	}
	if len(got) != len(want) {
		t.Fatalf("map = %v, want %v", got, want)
	}
	for name, id := range want {
		if got[name] != id {
			t.Errorf("map[%q] = %q, want %q", name, got[name], id)
		}
	}

	// No sidecar rows → nil (so ReconcileVideoStudios resolves by name only).
	if m := studioExternalIDsFromRows([]repo.EnrichmentRow{
		{Provider: "tmdb", FieldKey: "studio", Values: []string{"Acme"}},
	}); m != nil {
		t.Errorf("no-sidecar map = %v, want nil", m)
	}
}
