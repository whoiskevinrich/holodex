package api

import (
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// TestPersonExternalIDsFromRows covers the F32/ADR-055 side-map parse — the person
// analogue of TestStudioExternalIDsFromRows (studios_internal_test.go). The shared
// parse (externalIDsFromRows) is already exercised there for malformed entries; this
// only needs to confirm personExternalIDsFromRows reads the right field key
// (_person_external_ids, not _studio_external_ids) and ignores unrelated rows.
func TestPersonExternalIDsFromRows(t *testing.T) {
	rows := []repo.EnrichmentRow{
		{Provider: "tmdb", FieldKey: "actors", Values: []string{"Brad Pitt", "Edward Norton"}},
		{Provider: "tmdb", FieldKey: model.PersonExternalIDsField, Values: []string{
			"tmdb:287 Brad Pitt",
			"tmdb:7467 David Fincher",
		}},
		// A studio sidecar row on the same video must not leak into the person map.
		{Provider: "tmdb", FieldKey: model.StudioExternalIDsField, Values: []string{"tmdb:174 Warner Bros."}},
	}

	got := personExternalIDsFromRows(rows)
	want := map[string]string{
		"Brad Pitt":     "tmdb:287",
		"David Fincher": "tmdb:7467",
	}
	if len(got) != len(want) {
		t.Fatalf("map = %v, want %v", got, want)
	}
	for name, id := range want {
		if got[name] != id {
			t.Errorf("map[%q] = %q, want %q", name, got[name], id)
		}
	}

	// No sidecar rows → nil (so ReconcileVideoPeople resolves by name only).
	if m := personExternalIDsFromRows([]repo.EnrichmentRow{
		{Provider: "tmdb", FieldKey: "actors", Values: []string{"Someone"}},
	}); m != nil {
		t.Errorf("no-sidecar map = %v, want nil", m)
	}
}
