package enrich

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// newSourceURLService wires a Service over a real SQLite repo and an in-process
// Fake registered under the source name "tmdb" — the provider name doubles as the
// namespace its ids carry, which is exactly the equality ADR-098 D3 keys the stored
// URL on (F63 RD3).
func newSourceURLService(t *testing.T, fake *Fake) (*Service, *repo.Repo) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	store, err := NewStore(writeSources(t, `
sources:
  - name: tmdb
    base_url: http://fake:9100
    entity_types: [person, studio, video]
    enabled: true
`), nil)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServiceWithClient(store, r, log, func(Source) ProviderClient { return fake }), r
}

func sourceURLRow(t *testing.T, r *repo.Repo, entityType string, id int64) (values []string, found bool) {
	t.Helper()
	rows, err := r.EnrichmentForEntity(context.Background(), entityType, id)
	if err != nil {
		t.Fatalf("enrichment rows: %v", err)
	}
	for _, row := range rows {
		if row.FieldKey == model.SourceURLField {
			return row.Values, true
		}
	}
	return nil, false
}

// TestEnrichSourceURL_StoresFirstValidValue is ADR-098 D2's ingest shape: a hostile
// scheme is skipped, the first http(s) value is kept, and extras are dropped — the
// row is single-valued and SourceURLs reads it back keyed by provider.
func TestEnrichSourceURL_StoresFirstValidValue(t *testing.T) {
	fake := NewFake("tmdb")
	fake.People["tmdb:608"] = FakePerson{
		Label: "Hayao Miyazaki",
		Fields: map[string][]string{
			"bio":                {"Filmmaker."},
			model.SourceURLField: {"javascript:alert(1)", "https://tmdb.example/person/608", "https://tmdb.example/extra"},
		},
	}
	svc, r := newSourceURLService(t, fake)
	ctx := context.Background()

	if _, err := svc.Enrich(ctx, model.EnrichEntityPerson, 7, "tmdb", "tmdb:608", false); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	values, found := sourceURLRow(t, r, model.EnrichEntityPerson, 7)
	if !found || len(values) != 1 || values[0] != "https://tmdb.example/person/608" {
		t.Fatalf("_source_url row = %v (found=%v), want the single first-valid URL", values, found)
	}
	stored, err := svc.SourceURLs(ctx, model.EnrichEntityPerson, 7)
	if err != nil {
		t.Fatalf("source urls: %v", err)
	}
	if stored["tmdb"] != "https://tmdb.example/person/608" || len(stored) != 1 {
		t.Errorf("SourceURLs = %v, want {tmdb: https://tmdb.example/person/608}", stored)
	}
}

// TestEnrichSourceURL_OmittedKeepsMalformedClears is the raw/ok rule ADR-098 D2
// inherits from HOLODEX-258: an enrich that omits _source_url leaves the last good
// URL in place (additive shadow store), while one that sends only garbage overwrites
// the row to empty — and neither fails the enrich. Clear then removes the row with
// the provider's other rows.
func TestEnrichSourceURL_OmittedKeepsMalformedClears(t *testing.T) {
	fake := NewFake("tmdb")
	svc, r := newSourceURLService(t, fake)
	ctx := context.Background()
	enrich := func(fields map[string][]string) {
		t.Helper()
		fake.People["tmdb:608"] = FakePerson{Label: "Hayao Miyazaki", Fields: fields}
		if _, err := svc.Enrich(ctx, model.EnrichEntityPerson, 7, "tmdb", "tmdb:608", false); err != nil {
			t.Fatalf("enrich %v: %v", fields, err)
		}
	}

	enrich(map[string][]string{"bio": {"Filmmaker."}, model.SourceURLField: {"https://tmdb.example/person/608"}})
	enrich(map[string][]string{"bio": {"Filmmaker, again."}})
	if values, found := sourceURLRow(t, r, model.EnrichEntityPerson, 7); !found || len(values) != 1 || values[0] != "https://tmdb.example/person/608" {
		t.Fatalf("after omitted key: _source_url row = %v (found=%v), want the prior URL kept", values, found)
	}

	enrich(map[string][]string{"bio": {"Filmmaker."}, model.SourceURLField: {"javascript:alert(1)", "/relative/path"}})
	if values, found := sourceURLRow(t, r, model.EnrichEntityPerson, 7); !found || len(values) != 1 || values[0] != "" {
		t.Fatalf("after malformed value: _source_url row = %v (found=%v), want the stale URL cleared to an empty row", values, found)
	}
	if stored, _ := svc.SourceURLs(ctx, model.EnrichEntityPerson, 7); len(stored) != 0 {
		t.Errorf("SourceURLs after clear-to-empty = %v, want none", stored)
	}

	enrich(map[string][]string{"bio": {"Filmmaker."}, model.SourceURLField: {"https://tmdb.example/person/608"}})
	if err := svc.Clear(ctx, model.EnrichEntityPerson, 7, "tmdb"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, found := sourceURLRow(t, r, model.EnrichEntityPerson, 7); found {
		t.Errorf("_source_url row survived Clear, want it removed with the provider's rows")
	}
}

// TestProviderLink_Precedence pins ADR-098 D3 per pill:
// template ?? stored[namespace] ?? degraded. The provider "tmdb" declares a person
// template AND has a stored page for the entity — the template wins; "acme" has only
// a stored page — it links; "imdb" has neither, and tmdb's stored page must never
// back it (F63 RD3).
func TestProviderLink_Precedence(t *testing.T) {
	fake := NewFake("tmdb")
	fake.LinkTemplates = map[string]map[string]string{"tmdb": {"person": "https://tmdb.example/person/{id}"}}
	fake.People["tmdb:608"] = FakePerson{Label: "Hayao Miyazaki", Fields: map[string][]string{"bio": {"Filmmaker."}}}
	svc, _ := newSourceURLService(t, fake)
	ctx := context.Background()
	// Any provider action persists the manifest's link_templates (verifiedClient).
	if _, err := svc.Enrich(ctx, model.EnrichEntityPerson, 7, "tmdb", "tmdb:608", false); err != nil {
		t.Fatalf("enrich (warm templates): %v", err)
	}
	stored := map[string]string{
		"tmdb": "https://tmdb.example/stored-page",
		"acme": "https://acme.example/items/miyazaki",
	}

	cases := []struct {
		namespace, id string
		want          string
		ok            bool
	}{
		{"tmdb", "608", "https://tmdb.example/person/608", true},    // template beats the stored page
		{"acme", "x1", "https://acme.example/items/miyazaki", true}, // no template → own stored page
		{"TMDB", "608", "https://tmdb.example/person/608", true},    // namespace case-normalized
		{"imdb", "nm1", "", false},                                  // foreign namespace: never tmdb's page
		{"acme", "", "", false},                                     // no id → no pill → no link
	}
	for _, tc := range cases {
		got, ok := svc.ProviderLink(ctx, tc.namespace, model.EnrichEntityPerson, tc.id, stored)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ProviderLink(%q, %q) = (%q, %v), want (%q, %v)", tc.namespace, tc.id, got, ok, tc.want, tc.ok)
		}
	}
	if got, ok := svc.ProviderLink(ctx, "acme", model.EnrichEntityPerson, "x1", nil); got != "" || ok {
		t.Errorf("ProviderLink with nil stored map = (%q, %v), want degraded", got, ok)
	}
}
