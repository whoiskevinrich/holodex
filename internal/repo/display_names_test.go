package repo_test

import (
	"context"
	"slices"
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

// TestListPeopleDisplayNames pins HOLODEX-461 on the People index: each row carries
// its Displayed As spelling, and the name sort orders by that spelling (case-folded,
// like the SQL NOCASE it replaces) so the A–Z bar lands where the labels say.
func TestListPeopleDisplayNames(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	for _, n := range []string{"Alpha", "bravo", "Charlie"} {
		vid, err := r.UpsertVideo(ctx, sampleVideo("/m/"+n+".mkv", n, []string{n}, nil), nil)
		if err != nil {
			t.Fatal(err)
		}
		linkPeople(t, r, vid, n)
	}
	alpha, _, err := r.PersonIDByName(ctx, "Alpha")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetDecision(ctx, model.EnrichEntityPerson, alpha, "name", "manual", "zulu"); err != nil {
		t.Fatal(err)
	}

	got, err := r.ListPeople(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, p := range got {
		label := p.Name
		if p.DisplayName != "" {
			label = p.DisplayName
		}
		order = append(order, label)
	}
	if want := []string{"bravo", "Charlie", "zulu"}; !slices.Equal(order, want) {
		t.Errorf("ListPeople labels = %v, want %v", order, want)
	}
	if got[2].Name != "Alpha" {
		t.Errorf("decided row name = %q, want canonical Alpha", got[2].Name)
	}
}

// TestCastDisplayNames pins HOLODEX-461: both Cast grids (the video detail and a
// film's inherited cast) carry the Displayed As spelling, while `name` stays
// canonical — it is what the grid's detach sends back for linking.
func TestCastDisplayNames(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	vid, pid := seedVideoAndPerson(t, r)
	if err := r.SetDecision(ctx, model.EnrichEntityPerson, pid, "name", "manual", "Miyazaki-sensei"); err != nil {
		t.Fatal(err)
	}

	check := func(surface string, people []model.Person) {
		t.Helper()
		if len(people) != 1 {
			t.Fatalf("%s people = %v, want one", surface, people)
		}
		if p := people[0]; p.DisplayName != "Miyazaki-sensei" || p.Name != "Hayao Miyazaki" {
			t.Errorf("%s person = {name %q, display %q}, want canonical name + decided display", surface, p.Name, p.DisplayName)
		}
	}

	v, _, err := r.GetVideo(ctx, vid)
	if err != nil {
		t.Fatal(err)
	}
	check("GetVideo", v.People)

	rel, err := r.Related(ctx, vid, 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Person == nil || rel.Person.DisplayName != "Miyazaki-sensei" || rel.Person.Name != "Hayao Miyazaki" {
		t.Errorf("Related person shelf = %+v, want canonical name + decided display", rel.Person)
	}

	fid, err := r.CreateFilm(ctx, "Spirited Away", 2001)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.AttachFilmVideo(ctx, fid, vid, nil, true); err != nil {
		t.Fatal(err)
	}
	cast, err := r.FilmCast(ctx, fid)
	if err != nil {
		t.Fatal(err)
	}
	check("FilmCast", cast)
}
