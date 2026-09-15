package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
)

// TestDisplayNames pins the SQL mirror of the resolver's decided-replace rule for
// `name` (F60 RD9, HOLODEX-378): manual → the literal; provider → that provider's
// stored spelling under the kind's key; a provider with no stored spelling and a
// `file` decision both resolve to canonical and are therefore omitted; film reads
// the sidecar's `title` key.
func TestDisplayNames(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	seed := func(name string) int64 { return seedPerson(t, r, name) }
	manual := seed("Manual")
	provider := seed("Provider")
	unmatched := seed("Unmatched")
	file := seed("File")
	undecided := seed("Undecided")

	if err := r.SetDecision(ctx, model.EnrichEntityPerson, manual, "name", "manual", "  Manual Spelling "); err != nil {
		t.Fatal(err)
	}
	if err := r.SetDecision(ctx, model.EnrichEntityPerson, provider, "name", "provider:tmdb", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityPerson, provider, "tmdb", "x", map[string][]string{"name": {"Provider Spelling"}}); err != nil {
		t.Fatal(err)
	}
	// Another provider's spelling must not leak into a tmdb decision.
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityPerson, unmatched, "other", "y", map[string][]string{"name": {"Wrong Provider"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetDecision(ctx, model.EnrichEntityPerson, unmatched, "name", "provider:tmdb", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.SetDecision(ctx, model.EnrichEntityPerson, file, "name", "file", ""); err != nil {
		t.Fatal(err)
	}
	// A decision on another field of an undecided-name person is not a name decision.
	if err := r.SetDecision(ctx, model.EnrichEntityPerson, undecided, "bio", "manual", "Bio"); err != nil {
		t.Fatal(err)
	}

	got, err := r.DisplayNames(ctx, model.EnrichEntityPerson, "name")
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64]string{manual: "Manual Spelling", provider: "Provider Spelling"}
	if len(got) != len(want) {
		t.Fatalf("DisplayNames = %v, want %v", got, want)
	}
	for id, v := range want {
		if got[id] != v {
			t.Errorf("DisplayNames[%d] = %q, want %q", id, got[id], v)
		}
	}

	// Film: the provider spelling lives under `title` (ADR-086 §3), so the person
	// key finds nothing and the film key finds it.
	fid, err := r.CreateFilm(ctx, "Dune", 1984)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityFilm, fid, "tmdb", "z", map[string][]string{"title": {"Dune: Part One"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetDecision(ctx, model.EnrichEntityFilm, fid, "name", "provider:tmdb", ""); err != nil {
		t.Fatal(err)
	}
	if got, err := r.DisplayNames(ctx, model.EnrichEntityFilm, "name"); err != nil || len(got) != 0 {
		t.Errorf("film under the person key = %v (%v), want none", got, err)
	}
	if got, err := r.DisplayNames(ctx, model.EnrichEntityFilm, "title"); err != nil || got[fid] != "Dune: Part One" {
		t.Errorf("film under title = %v (%v), want the provider title", got, err)
	}
}
