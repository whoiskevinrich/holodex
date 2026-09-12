package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/enrich"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// hintServer wires two video-capable providers with DIFFERENT search patterns over a
// mapping that resolves studio (Publisher tag) and actors (Artist tag), plus one
// video whose title is exactly its bracketed release stem. Each provider is its own
// Fake so a test can read the hint each one received (Fake.LastHint) after the
// Service's manifest gate — what a real sidecar would have seen on the wire.
func hintServer(t *testing.T, token string, fakes map[string]*enrich.Fake) (*httptest.Server, *repo.Repo, int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	ctx := context.Background()

	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath:  filepath.Join(dir, "lib", "Acme", "[Acme Pictures] Ada Lovelace (2023-08-01) 1080p.mp4"),
		FileSize:  1,
		Title:     "[Acme Pictures] Ada Lovelace (2023-08-01) 1080p",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, []model.ExtraMetadata{
		{SourceKey: "Publisher", Value: "Acme Pictures"},
		{SourceKey: "Artist", Value: "Ada Lovelace"},
		{SourceKey: "Year", Value: "2023-08-01"},
	})
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	if err := os.WriteFile(mpath, []byte(`fields:
  - canonical: title
    label: Title
    sources: [file:title]
  - canonical: studio
    label: Studio
    sources: [Publisher]
  - canonical: actors
    label: Actors
    sources: [Artist]
  - canonical: release_date
    label: Released
    sources: [Year]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	mstore, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}

	sp := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(sp, []byte(`sources:
  - name: studioed
    base_url: http://studioed:9100
    entity_types: [video]
    enabled: true
    search_pattern: "{studio} {title} {performers} {year}"
  - name: titled
    base_url: http://titled:9100
    entity_types: [video]
    enabled: true
    search_pattern: "{title} {year?}"
  - name: denied
    base_url: http://denied:9100
    entity_types: [video]
    enabled: true
    send_filename: false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	estore, err := enrich.NewStore(sp, log)
	if err != nil {
		t.Fatal(err)
	}
	svc := enrich.NewServiceWithClient(estore, r, log, func(src enrich.Source) enrich.ProviderClient {
		if f, ok := fakes[src.Name]; ok {
			return f
		}
		return enrich.NewFake(src.Name)
	})

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(mstore, cache.Noop{})
	h.SetEnrichment(svc)
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, vid
}

// optedIn builds a video-capable fake that opts into the given resolve_hints and
// advertises the given field keys.
func optedIn(name string, hints, fields []string) *enrich.Fake {
	f := enrich.NewFake(name)
	f.ResolveHints = hints
	f.ExtraFields = fields
	return f
}

// Interactive path (ADR-095 D4, F54 FR7/AC-14): query_source is derived by
// re-rendering the provider's own pattern — the seeded string untouched is
// "pattern", one edited character is "user", and a client-supplied query_source in
// the body is ignored (the handler never decodes it).
func TestEnrichVideoResolve_QuerySource(t *testing.T) {
	studioed := optedIn("studioed", []string{"fields"}, []string{"title"})
	titled := optedIn("titled", []string{"filename"}, nil)
	srv, _, vid := hintServer(t, "s3cret", map[string]*enrich.Fake{"studioed": studioed, "titled": titled})
	url := srv.URL + "/api/v1/media/" + itoa(vid) + "/enrich/resolve"

	// The residue rule (D5) fires on this stem: the title is exactly studio +
	// performer + date, so studioed's render is the other three tokens once.
	const studioedSeed = "Acme Pictures Ada Lovelace 2023"
	// titled's pattern has no {studio}/{performers} token, so the residue rule has
	// nothing in THIS tier to judge the title against: it renders in full (sanitized),
	// the date included, and {year?} follows.
	const titledSeed = "Acme Pictures Ada Lovelace 2023-08-01 2023"

	cases := []struct {
		name, provider, query string
		body                  map[string]any
		want                  string
		fake                  *enrich.Fake
	}{
		{"seed untouched ⇒ pattern", "studioed", studioedSeed, nil, enrich.QuerySourcePattern, studioed},
		{"one-character edit ⇒ user", "studioed", studioedSeed + "s", nil, enrich.QuerySourceUser, studioed},
		{"owner's own text ⇒ user", "studioed", "something else", nil, enrich.QuerySourceUser, studioed},
		{"client-supplied query_source ignored", "studioed", "typed by hand", map[string]any{"query_source": "pattern"}, enrich.QuerySourceUser, studioed},
		{"a different provider compares against ITS render", "titled", titledSeed, nil, enrich.QuerySourcePattern, titled},
		{"studioed's seed is 'user' text to titled", "titled", studioedSeed, nil, enrich.QuerySourceUser, titled},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := map[string]any{"provider": c.provider, "query": c.query}
			for k, v := range c.body {
				body[k] = v
			}
			if code, _ := postTok(t, url, "s3cret", body); code != http.StatusOK {
				t.Fatalf("resolve = %d, want 200", code)
			}
			if got := c.fake.LastHint.QuerySource; got != c.want {
				t.Errorf("query_source = %q, want %q (query %q)", got, c.want, c.query)
			}
			if c.fake.LastHint.Query != c.query {
				t.Errorf("hint.query = %q, want the submitted %q unchanged", c.fake.LastHint.Query, c.query)
			}
		})
	}
}

// Interactive path (F54 FR8/AC-15): the structured keys a provider receives follow
// its manifest opt-in, the operator deny, and the five-field vocabulary — and carry
// the RESOLVED values (mapping-derived studio/actors, the raw bracketed title, the
// basename with no directory component).
func TestEnrichVideoResolve_StructuredHints(t *testing.T) {
	studioed := optedIn("studioed", []string{"fields", "filename"}, []string{"title", "studio", "actors", "overview"})
	titled := optedIn("titled", nil, []string{"title", "studio"}) // no opt-in at all
	denied := optedIn("denied", []string{"filename"}, nil)        // opted in, operator says no
	srv, _, vid := hintServer(t, "s3cret", map[string]*enrich.Fake{"studioed": studioed, "titled": titled, "denied": denied})
	url := srv.URL + "/api/v1/media/" + itoa(vid) + "/enrich/resolve"

	for _, p := range []string{"studioed", "titled", "denied"} {
		if code, _ := postTok(t, url, "s3cret", map[string]any{"provider": p, "query": "x"}); code != http.StatusOK {
			t.Fatalf("%s resolve = %d, want 200", p, code)
		}
	}

	got := studioed.LastHint
	if got.Filename != "[Acme Pictures] Ada Lovelace (2023-08-01) 1080p.mp4" {
		t.Errorf("filename = %q, want the verbatim basename", got.Filename)
	}
	if strings.ContainsAny(got.Filename, `/\`) {
		t.Errorf("filename carries a directory component: %q", got.Filename)
	}
	wantFields := map[string]string{
		"title":  "[Acme Pictures] Ada Lovelace (2023-08-01) 1080p", // as-is: brackets survive
		"studio": "Acme Pictures",
		"actors": "Ada Lovelace",
	}
	for k, v := range wantFields {
		if vals := got.Fields[k]; len(vals) != 1 || vals[0] != v {
			t.Errorf("fields[%q] = %v, want [%q]", k, vals, v)
		}
	}
	for _, k := range []string{"overview", "release_date", "director"} {
		// overview: outside the vocabulary however the provider advertises it;
		// release_date: resolved but NOT advertised by this provider; director: no value.
		if _, ok := got.Fields[k]; ok {
			t.Errorf("fields must not carry %q: %v", k, got.Fields)
		}
	}

	if h := titled.LastHint; h.Fields != nil || h.Filename != "" || h.QuerySource != "" {
		t.Errorf("a provider with no resolve_hints must receive nothing structured: %+v", h)
	}
	if h := denied.LastHint; h.Filename != "" || h.Fields != nil {
		t.Errorf("operator-denied filename must be absent: %+v", h)
	} else if h.QuerySource == "" {
		t.Errorf("query_source still rides the opt-in when only the deny withheld filename: %+v", h)
	}
}

// The interactive response surfaces the provider's searched[] (for the HOLODEX-369
// caption) and omits the key when the provider sent none.
func TestEnrichVideoResolve_SearchedInResponse(t *testing.T) {
	studioed := optedIn("studioed", nil, nil)
	studioed.Searched = []string{"[Acme Pictures] Ada Lovelace (2023-08-01) 1080p.mp4", "Acme Pictures Ada Lovelace"}
	titled := optedIn("titled", nil, nil)
	srv, _, vid := hintServer(t, "s3cret", map[string]*enrich.Fake{"studioed": studioed, "titled": titled})
	url := srv.URL + "/api/v1/media/" + itoa(vid) + "/enrich/resolve"

	_, body := postTok(t, url, "s3cret", map[string]any{"provider": "studioed", "query": "x"})
	searched, _ := body["searched"].([]any)
	if len(searched) != 2 || searched[1] != "Acme Pictures Ada Lovelace" {
		t.Errorf("searched = %v, want the provider's two entries in order", body["searched"])
	}
	if _, ok := body["candidates"]; !ok {
		t.Errorf("candidates key missing: %v", body)
	}

	_, body = postTok(t, url, "s3cret", map[string]any{"provider": "titled", "query": "x"})
	if _, ok := body["searched"]; ok {
		t.Errorf("searched must be omitted when the provider sent none: %v", body)
	}
}

// Batch path (ADR-095 D8, F54 AC-16): refresh-all builds a distinct hint per
// provider INSIDE the fan-out — two providers with different patterns receive two
// different hint.query renders, each with its own opted-in keys, all "pattern" —
// and a provider's searched[] lands in the enrich-run activity detail (AC-18) with
// no path separator, while a provider that sent none leaves no such entry.
func TestEnrichRefreshAll_PerProviderHintsAndSearched(t *testing.T) {
	studioed := optedIn("studioed", []string{"fields", "filename"}, []string{"title", "studio", "actors"})
	studioed.Searched = []string{"[Acme Pictures] Ada Lovelace (2023-08-01) 1080p.mp4", "Acme Pictures Ada Lovelace"}
	titled := optedIn("titled", []string{"filename"}, nil)
	denied := optedIn("denied", nil, nil)
	srv, r, vid := hintServer(t, "s3cret", map[string]*enrich.Fake{"studioed": studioed, "titled": titled, "denied": denied})

	code, _ := postTok(t, srv.URL+"/api/v1/media/"+itoa(vid)+"/enrich/refresh-all", "s3cret", nil)
	if code != http.StatusOK {
		t.Fatalf("refresh-all = %d, want 200", code)
	}

	// Each provider got ITS OWN render — the ADR-080 AI5 shared-hint regression guard.
	if got, want := studioed.LastHint.Query, "Acme Pictures Ada Lovelace 2023"; got != want {
		t.Errorf("studioed hint.query = %q, want %q", got, want)
	}
	if got, want := titled.LastHint.Query, "Acme Pictures Ada Lovelace 2023-08-01 2023"; got != want {
		t.Errorf("titled hint.query = %q, want %q", got, want)
	}
	// denied has no pattern anywhere ⇒ the D4 sanitized-title floor (brackets, parens
	// and 1080p stripped), untouched by the residue rule.
	if got, want := denied.LastHint.Query, "Acme Pictures Ada Lovelace 2023-08-01"; got != want {
		t.Errorf("denied hint.query = %q, want the sanitized-title floor %q", got, want)
	}
	for name, f := range map[string]*enrich.Fake{"studioed": studioed, "titled": titled} {
		if f.LastHint.QuerySource != enrich.QuerySourcePattern {
			t.Errorf("%s query_source = %q, want pattern on the batch path", name, f.LastHint.QuerySource)
		}
		if f.LastHint.Filename == "" {
			t.Errorf("%s opted into filename and should have received it", name)
		}
	}
	if studioed.LastHint.Fields["studio"] == nil || titled.LastHint.Fields != nil {
		t.Errorf("fields must follow each provider's own opt-in: studioed=%v titled=%v", studioed.LastHint.Fields, titled.LastHint.Fields)
	}
	if h := denied.LastHint; h.Fields != nil || h.Filename != "" || h.QuerySource != "" {
		t.Errorf("no opt-in ⇒ nothing structured: %+v", h)
	}

	// searched[] → job_runs.detail, only for the provider that reported it.
	runs, err := r.ListJobRuns(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	var searchedRuns []model.JobRun
	for _, run := range runs {
		if strings.Contains(run.Detail, "searched:") {
			searchedRuns = append(searchedRuns, run)
		}
	}
	if len(searchedRuns) != 1 {
		t.Fatalf("searched entries = %d, want exactly one (studioed only): %+v", len(searchedRuns), runs)
	}
	want := "studioed → video #" + itoa(vid) + " (0 candidates) · searched: [Acme Pictures] Ada Lovelace (2023-08-01) 1080p.mp4 · Acme Pictures Ada Lovelace"
	if searchedRuns[0].Detail != want {
		t.Errorf("detail\n got %q\nwant %q", searchedRuns[0].Detail, want)
	}
	if strings.ContainsAny(searchedRuns[0].Detail, `/\`) {
		t.Errorf("detail carries a path separator: %q", searchedRuns[0].Detail)
	}
	if searchedRuns[0].EntityType != model.EnrichEntityVideo || searchedRuns[0].EntityID != vid {
		t.Errorf("run attribution = %+v", searchedRuns[0])
	}
}
