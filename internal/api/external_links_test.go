package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/enrich"
	"holodex/internal/repo"
)

// externalLinksEnv wires a fake HTTP provider whose /describe manifest declares
// link_templates (HOLODEX-266/ADR-083 D2), seeded into provider_link_templates via
// the real write path. persistLinkTemplates only fires as a side effect of
// Service.verifiedClient (any provider action, e.g. Resolve) — seeding requires
// actually invoking the fake provider once rather than writing the repo table
// directly, or the test would skip the D2 wiring it exists to cover.
type externalLinksEnv struct {
	repo *repo.Repo
	srv  *httptest.Server
	svc  *enrich.Service
}

func newExternalLinksEnv(t *testing.T, linkTemplates map[string]map[string]string) *externalLinksEnv {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)

	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/describe":
			m := map[string]any{
				"provider":         "fake",
				"version":          "1.0.0",
				"protocol_version": 1,
				"entity_types":     []string{"person", "studio"},
				"id_namespaces":    []string{"fake"},
				"fields":           []string{"bio"},
			}
			if linkTemplates != nil {
				m["link_templates"] = linkTemplates
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(m)
		case "/resolve":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{}})
		case "/enrich":
			// Always hands core the provider's own page for the entity (contract
			// §4.12, ADR-098 D1) so the _source_url fallback tests below can exercise
			// the real ingest path rather than writing the row directly.
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"fields": map[string]any{
				"bio":         []string{"Enriched."},
				"_source_url": []string{"https://fake.example/pages/miyazaki"},
			}})
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(fake.Close)

	sp := filepath.Join(dir, "sources.yaml")
	yaml := "sources:\n  - name: fake\n    base_url: " + fake.URL + "\n    entity_types: [person, studio]\n    enabled: true\n"
	if err := os.WriteFile(sp, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write sources: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := enrich.NewStore(sp, log)
	if err != nil {
		t.Fatalf("sources store: %v", err)
	}
	svc := enrich.NewService(store, r, log)
	// The manifest's link_templates aren't entity-type-scoped, so any provider
	// action seeds all of them regardless of which entity type this call names.
	if _, err := svc.Resolve(context.Background(), "fake", "person", enrich.Hint{Query: "x"}); err != nil {
		t.Fatalf("resolve (seed link templates): %v", err)
	}

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetEnrichment(svc)
	// Films ride the same projection (HOLODEX-393); the flag only mounts the routes.
	h.SetFilmsEnabled(true)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	return &externalLinksEnv{repo: r, srv: srv, svc: svc}
}

// seedSourceURLPerson creates a person carrying a foreign "other:1" id (attached
// through the video-credit reconcile, like any provider-emitted foreign id) and then
// enriches it through the fake provider, which attaches "fake:1" as the identity row
// and stores the provider's _source_url — the two-pill shape ADR-098 D3 rules on.
func seedSourceURLPerson(t *testing.T, env *externalLinksEnv) int64 {
	t.Helper()
	ctx := context.Background()
	vid := seedVideo(t, env.repo, "/m/source-url.mkv", "Source URL Clip")
	if err := env.repo.ReconcileVideoPeople(ctx, vid,
		[]repo.PersonRoleName{{Name: "Source Url", Role: "actor"}},
		map[string]string{"Source Url": "other:1"}); err != nil {
		t.Fatalf("attach other id: %v", err)
	}
	pid, _, err := env.repo.PersonIDByName(ctx, "Source Url")
	if err != nil {
		t.Fatalf("lookup person: %v", err)
	}
	if _, err := env.svc.Enrich(ctx, "person", pid, "fake", "fake:1", false); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	return pid
}

// TestExternalLinks_SourceURLFallback is ADR-098 D3's fallback branch end to end:
// the provider declared no template for its own namespace, so its pill links to the
// stored _source_url — while the foreign "other" pill on the same person stays
// degraded (F63 RD3: a stored page backs only its own provider's pill).
func TestExternalLinks_SourceURLFallback(t *testing.T) {
	env := newExternalLinksEnv(t, nil)
	pid := seedSourceURLPerson(t, env)

	_, body := getJSON(t, env.srv.URL+"/api/v1/people/"+itoa(pid))
	links, _ := body["external_links"].([]any)
	if len(links) != 2 {
		t.Fatalf("external_links = %v, want 2 entries", body["external_links"])
	}
	byProvider := linksByProvider(t, links)
	if lm := byProvider["fake"]; lm == nil || lm["url"] != "https://fake.example/pages/miyazaki" {
		t.Errorf("fake badge = %v, want the stored _source_url", lm)
	}
	if lm := byProvider["other"]; lm == nil || lm["url"] != nil {
		t.Errorf("other badge = %v, want degraded (no url) — a foreign namespace never takes the provider's stored page", lm)
	}
}

// TestExternalLinks_TemplateBeatsSourceURL is ADR-098 D3's first branch: with a
// template declared for the provider's own namespace, the template renders and the
// stored _source_url (also present) is dead weight.
func TestExternalLinks_TemplateBeatsSourceURL(t *testing.T) {
	env := newExternalLinksEnv(t, map[string]map[string]string{
		"fake": {"person": "https://fake.example/person/{id}"},
	})
	pid := seedSourceURLPerson(t, env)

	_, body := getJSON(t, env.srv.URL+"/api/v1/people/"+itoa(pid))
	links, _ := body["external_links"].([]any)
	byProvider := linksByProvider(t, links)
	if lm := byProvider["fake"]; lm == nil || lm["url"] != "https://fake.example/person/1" {
		t.Errorf("fake badge = %v, want the template URL over the stored _source_url", lm)
	}
}

// linksByProvider indexes an external_links JSON array by its "provider" key, so
// tests can assert on each badge without depending on response order.
func linksByProvider(t *testing.T, links []any) map[string]map[string]any {
	t.Helper()
	byProvider := make(map[string]map[string]any, len(links))
	for _, l := range links {
		lm := l.(map[string]any)
		byProvider[lm["provider"].(string)] = lm
	}
	return byProvider
}

// TestExternalLinks_MultiBadge proves ADR-083 D3 ("one badge per stored external-id
// row, 0..N"): two distinct namespaced ids attached to the same person each
// round-trip as their own ExternalLink, correctly namespace-split, labeled, and
// resolved through the provider's declared link_templates.
func TestExternalLinks_MultiBadge(t *testing.T) {
	env := newExternalLinksEnv(t, map[string]map[string]string{
		"tmdb": {"person": "https://tmdb.example/person/{id}"},
		"imdb": {"person": "https://imdb.example/name/{id}"},
	})
	ctx := context.Background()
	vid := seedVideo(t, env.repo, "/m/multi.mkv", "Multi Badge Clip")

	// Two Reconcile calls, same name+role, different namespaced id each time:
	// attachExternalID's INSERT OR IGNORE is additive across namespaces, so both
	// rows land on the same person — the mechanism a real entity enriched by two
	// providers, one action at a time, would go through.
	if err := env.repo.ReconcileVideoPeople(ctx, vid,
		[]repo.PersonRoleName{{Name: "Multi Badge", Role: "actor"}},
		map[string]string{"Multi Badge": "tmdb:1"}); err != nil {
		t.Fatalf("attach tmdb id: %v", err)
	}
	if err := env.repo.ReconcileVideoPeople(ctx, vid,
		[]repo.PersonRoleName{{Name: "Multi Badge", Role: "actor"}},
		map[string]string{"Multi Badge": "imdb:nm1"}); err != nil {
		t.Fatalf("attach imdb id: %v", err)
	}
	pid, _, err := env.repo.PersonIDByName(ctx, "Multi Badge")
	if err != nil {
		t.Fatalf("lookup person: %v", err)
	}

	_, body := getJSON(t, env.srv.URL+"/api/v1/people/"+itoa(pid))
	links, _ := body["external_links"].([]any)
	if len(links) != 2 {
		t.Fatalf("external_links = %v, want 2 entries", body["external_links"])
	}
	byProvider := linksByProvider(t, links)
	if lm := byProvider["tmdb"]; lm == nil || lm["label"] != "TMDB" || lm["url"] != "https://tmdb.example/person/1" {
		t.Errorf("tmdb badge = %v", lm)
	}
	if lm := byProvider["imdb"]; lm == nil || lm["label"] != "IMDb" || lm["url"] != "https://imdb.example/name/nm1" {
		t.Errorf("imdb badge = %v", lm)
	}
}

// TestExternalLinks_Studio mirrors the multi-badge case for the studio wiring
// (studios.go), mixed with a degraded entry: only a tmdb/studio template is
// declared, so the studio's tmdb id resolves to a URL and its imdb id renders
// label-only — proving both externalLinksForEntity call sites (person, studio)
// share the same projection.
func TestExternalLinks_Studio(t *testing.T) {
	env := newExternalLinksEnv(t, map[string]map[string]string{
		"tmdb": {"studio": "https://tmdb.example/company/{id}"},
	})
	ctx := context.Background()
	vid := seedVideo(t, env.repo, "/m/studio.mkv", "Studio Clip")

	if err := env.repo.ReconcileVideoStudios(ctx, vid, []string{"Multi Studio"},
		map[string]string{"Multi Studio": "tmdb:7"}); err != nil {
		t.Fatalf("attach tmdb id: %v", err)
	}
	if err := env.repo.ReconcileVideoStudios(ctx, vid, []string{"Multi Studio"},
		map[string]string{"Multi Studio": "imdb:co1"}); err != nil {
		t.Fatalf("attach imdb id: %v", err)
	}
	studios, err := env.repo.ListStudios(ctx, false)
	if err != nil || len(studios) != 1 {
		t.Fatalf("list studios: %v (%d)", err, len(studios))
	}
	sid := studios[0].ID

	_, body := getJSON(t, env.srv.URL+"/api/v1/studios/"+itoa(sid))
	links, _ := body["external_links"].([]any)
	if len(links) != 2 {
		t.Fatalf("external_links = %v, want 2 entries", body["external_links"])
	}
	byProvider := linksByProvider(t, links)
	if lm := byProvider["tmdb"]; lm == nil || lm["url"] != "https://tmdb.example/company/7" {
		t.Errorf("tmdb badge = %v", lm)
	}
	if lm, ok := byProvider["imdb"]; !ok {
		t.Fatal("imdb badge missing")
	} else if _, present := lm["url"]; present {
		t.Errorf("imdb badge url = %v, want omitted (no imdb/studio template declared)", lm["url"])
	}
}

// TestExternalLinks_Film is HOLODEX-393 (F63 P0-6): getFilm projects the film's
// stored ids through the same externalLinksForEntity path as person and studio —
// one pill per id, linked when a film template exists for the namespace, label-only
// otherwise — and a film with no ids projects no entries (null, the person/studio
// contract the page already tolerates), so the header meta line stays byte-identical.
func TestExternalLinks_Film(t *testing.T) {
	env := newExternalLinksEnv(t, map[string]map[string]string{
		"tmdb": {"film": "https://tmdb.example/movie/{id}"},
	})
	ctx := context.Background()
	fid, err := env.repo.CreateFilm(ctx, "Badge Film", 1999)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	for _, ext := range []string{"tmdb:603", "imdb:tt0133093"} {
		if err := env.repo.AttachExternalID(ctx, "film", fid, ext); err != nil {
			t.Fatalf("attach %s: %v", ext, err)
		}
	}

	_, body := getJSON(t, env.srv.URL+"/api/v1/films/"+itoa(fid))
	links, _ := body["external_links"].([]any)
	if len(links) != 2 {
		t.Fatalf("external_links = %v, want 2 entries", body["external_links"])
	}
	byProvider := linksByProvider(t, links)
	if lm := byProvider["tmdb"]; lm == nil || lm["url"] != "https://tmdb.example/movie/603" {
		t.Errorf("tmdb badge = %v", lm)
	}
	if lm, ok := byProvider["imdb"]; !ok {
		t.Fatal("imdb badge missing")
	} else if _, present := lm["url"]; present {
		t.Errorf("imdb badge url = %v, want omitted (no imdb/film template declared)", lm["url"])
	}

	bare, err := env.repo.CreateFilm(ctx, "Bare Film", 2001)
	if err != nil {
		t.Fatalf("create bare film: %v", err)
	}
	_, body = getJSON(t, env.srv.URL+"/api/v1/films/"+itoa(bare))
	if got, _ := body["external_links"].([]any); len(got) != 0 {
		t.Errorf("bare film external_links = %v, want none", body["external_links"])
	}
}

// TestExternalLinks_TemplateMismatch covers ADR-083 D2's degraded state: a stored
// external id whose namespace/entity-kind has no matching link_templates entry
// still surfaces as a badge (the identity signal), just without a URL — never an
// error and never a broken link. Table-driven over the ways a template can fail to
// match.
func TestExternalLinks_TemplateMismatch(t *testing.T) {
	cases := []struct {
		name          string
		linkTemplates map[string]map[string]string
	}{
		{name: "no templates declared", linkTemplates: nil},
		{name: "template declared for a different namespace", linkTemplates: map[string]map[string]string{
			"imdb": {"person": "https://imdb.example/name/{id}"},
		}},
		{name: "template declared for a different entity kind", linkTemplates: map[string]map[string]string{
			"tmdb": {"studio": "https://tmdb.example/company/{id}"},
		}},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newExternalLinksEnv(t, tc.linkTemplates)
			ctx := context.Background()
			vid := seedVideo(t, env.repo, fmt.Sprintf("/m/mismatch-%d.mkv", i), "Mismatch Clip")
			if err := env.repo.ReconcileVideoPeople(ctx, vid,
				[]repo.PersonRoleName{{Name: "Mismatch Person", Role: "actor"}},
				map[string]string{"Mismatch Person": "tmdb:99"}); err != nil {
				t.Fatalf("attach id: %v", err)
			}
			pid, _, err := env.repo.PersonIDByName(ctx, "Mismatch Person")
			if err != nil {
				t.Fatalf("lookup person: %v", err)
			}

			_, body := getJSON(t, env.srv.URL+"/api/v1/people/"+itoa(pid))
			links, _ := body["external_links"].([]any)
			if len(links) != 1 {
				t.Fatalf("external_links = %v, want 1 label-only entry", body["external_links"])
			}
			lm := links[0].(map[string]any)
			if lm["provider"] != "tmdb" || lm["label"] != "TMDB" {
				t.Errorf("badge = %v, want tmdb/TMDB", lm)
			}
			if _, present := lm["url"]; present {
				t.Errorf("badge url = %v, want omitted (degraded state)", lm["url"])
			}
		})
	}
}

// TestExternalLinks_EnrichmentDisabled covers the other ADR-083 D2 degraded path:
// no enrichment service wired at all (h.enrich == nil), a real deployment state
// (enrichment is optional), not just a template gap. Unlike the table above, this
// needs no fake HTTP provider — a bare repo/handlers pair without SetEnrichment is
// enough, and cheaper than bootstrapping newExternalLinksEnv just to discard it.
func TestExternalLinks_EnrichmentDisabled(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	vid := seedVideo(t, r, "/m/no-enrich.mkv", "No Enrich Clip")
	if err := r.ReconcileVideoPeople(ctx, vid,
		[]repo.PersonRoleName{{Name: "No Enrich Person", Role: "actor"}},
		map[string]string{"No Enrich Person": "tmdb:99"}); err != nil {
		t.Fatalf("attach id: %v", err)
	}
	pid, _, err := r.PersonIDByName(ctx, "No Enrich Person")
	if err != nil {
		t.Fatalf("lookup person: %v", err)
	}

	_, body := getJSON(t, srv.URL+"/api/v1/people/"+itoa(pid))
	links, _ := body["external_links"].([]any)
	if len(links) != 1 {
		t.Fatalf("external_links = %v, want 1 label-only entry", body["external_links"])
	}
	lm := links[0].(map[string]any)
	if lm["provider"] != "tmdb" || lm["label"] != "TMDB" {
		t.Errorf("badge = %v, want tmdb/TMDB", lm)
	}
	if _, present := lm["url"]; present {
		t.Errorf("badge url = %v, want omitted (no enrichment service wired)", lm["url"])
	}
}

// TestExternalLinks_MalformedIDSkipped proves a stored external id that isn't
// "namespace:id" (can't happen through the enrich-write path, but the column has
// no format constraint) is silently skipped — never a broken/errored badge.
func TestExternalLinks_MalformedIDSkipped(t *testing.T) {
	env := newExternalLinksEnv(t, map[string]map[string]string{
		"tmdb": {"person": "https://tmdb.example/person/{id}"},
	})
	ctx := context.Background()
	vid := seedVideo(t, env.repo, "/m/malformed.mkv", "Malformed Clip")
	if err := env.repo.ReconcileVideoPeople(ctx, vid,
		[]repo.PersonRoleName{{Name: "Malformed Id", Role: "actor"}},
		map[string]string{"Malformed Id": "not-namespaced"}); err != nil {
		t.Fatalf("attach id: %v", err)
	}
	pid, _, err := env.repo.PersonIDByName(ctx, "Malformed Id")
	if err != nil {
		t.Fatalf("lookup person: %v", err)
	}

	_, body := getJSON(t, env.srv.URL+"/api/v1/people/"+itoa(pid))
	links, _ := body["external_links"].([]any)
	if len(links) != 0 {
		t.Fatalf("external_links = %v, want empty (malformed id skipped)", body["external_links"])
	}
}
