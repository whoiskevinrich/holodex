package api_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// externalLinksEnv wires a fake HTTP provider whose /describe manifest declares
// link_templates (HOLODEX-266/ADR-083 D2), seeded into provider_link_templates via
// the real write path. persistLinkTemplates only fires as a side effect of
// Service.verifiedClient (any provider action, e.g. Resolve) — seeding requires
// actually invoking the fake provider once rather than writing the repo table
// directly, or the test would skip the D2 wiring it exists to cover.
type externalLinksEnv struct {
	db   *sql.DB
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
	// Video's pill comes off the resolver (HOLODEX-394, ADR-098 D4), so getMedia
	// needs a mapping that resolves external_provider_id — provider shadow first,
	// then the file tag — for TestExternalLinks_Video.
	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	mappingsYAML := "fields:\n" +
		"  - canonical: title\n    label: Title\n    sources: [file:title]\n" +
		"  - canonical: external_provider_id\n    label: External ID\n    sources: [fake:external_provider_id, file:ExternalId]\n"
	if err := os.WriteFile(mpath, []byte(mappingsYAML), 0o644); err != nil {
		t.Fatalf("write mappings: %v", err)
	}
	mappings, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatalf("load mappings: %v", err)
	}
	h.SetMetadataFields(mappings, cache.Noop{})
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	return &externalLinksEnv{db: database, repo: r, srv: srv, svc: svc}
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

// TestExternalLinks_Video covers ADR-098 D4 + HOLODEX-424 (spec P0-7b) end to end:
// the media page's pills come from the resolver's winning external_provider_id and
// from the provider match stamped on the video's enrichment rows, deduped by
// namespace with the resolved value first, each linked through the same per-pill
// precedence as the entity kinds — template, else that provider's own stored
// _source_url, else degraded — and absent entirely when neither yields an id
// (F63 P0-7: the meta line stays byte-identical).
func TestExternalLinks_Video(t *testing.T) {
	env := newExternalLinksEnv(t, map[string]map[string]string{
		"tmdb": {"video": "https://tmdb.example/movie/{id}"},
	})
	ctx := context.Background()

	cases := []struct {
		name     string
		path     string
		extra    []model.ExtraMetadata
		enriched map[string][]string // fake provider's shadow rows, nil for none
		match    string              // external id stamped on those rows; "" = extraction-style
		want     map[string]string   // namespace -> url; "" = degraded (url omitted)
	}{
		{
			name:     "provider value with a template",
			path:     "/m/templated.mkv",
			enriched: map[string][]string{"external_provider_id": {"tmdb:603"}},
			want:     map[string]string{"tmdb": "https://tmdb.example/movie/603"},
		},
		{
			name: "provider value falls back to its own stored _source_url",
			path: "/m/stored.mkv",
			enriched: map[string][]string{
				"external_provider_id": {"fake:99"},
				model.SourceURLField:   {"https://fake.example/videos/99"},
			},
			want: map[string]string{"fake": "https://fake.example/videos/99"},
		},
		{
			name: "a stored page never backs a foreign namespace (RD3)",
			path: "/m/foreign.mkv",
			enriched: map[string][]string{
				"external_provider_id": {"other:7"},
				model.SourceURLField:   {"https://fake.example/videos/7"},
			},
			want: map[string]string{"other": ""},
		},
		{
			name:  "file-layer winner with no template renders degraded",
			path:  "/m/file.mkv",
			extra: []model.ExtraMetadata{{SourceKey: "ExternalId", Value: "imdb:tt0133093"}},
			want:  map[string]string{"imdb": ""},
		},
		// HOLODEX-424 — the match itself is a pill input.
		{
			name:     "match only, no field value, links through the template",
			path:     "/m/match.mkv",
			enriched: map[string][]string{"description": {"matched"}},
			match:    "tmdb:812",
			want:     map[string]string{"tmdb": "https://tmdb.example/movie/812"},
		},
		{
			name:     "match plus a foreign-namespace file tag is two pills",
			path:     "/m/match-tag.mkv",
			extra:    []model.ExtraMetadata{{SourceKey: "ExternalId", Value: "imdb:tt0103639"}},
			enriched: map[string][]string{"description": {"matched"}},
			match:    "tmdb:812",
			want:     map[string]string{"imdb": "", "tmdb": "https://tmdb.example/movie/812"},
		},
		{
			name:     "match in the file tag's namespace dedups to the resolved value",
			path:     "/m/match-same.mkv",
			extra:    []model.ExtraMetadata{{SourceKey: "ExternalId", Value: "tmdb:1"}},
			enriched: map[string][]string{"description": {"matched"}},
			match:    "tmdb:812",
			want:     map[string]string{"tmdb": "https://tmdb.example/movie/1"},
		},
		{
			name:     "match with no template and no _source_url renders degraded",
			path:     "/m/match-degraded.mkv",
			enriched: map[string][]string{"description": {"matched"}},
			match:    "fake:5",
			want:     map[string]string{"fake": ""},
		},
		{
			name:     "extraction-style rows with no match id yield no pill",
			path:     "/m/extracted.mkv",
			enriched: map[string][]string{"description": {"extracted"}},
			want:     map[string]string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vid, err := env.repo.UpsertVideo(ctx, &model.Video{
				FilePath: tc.path, FileSize: 1, Title: "Clip",
				FileMtime: time.Now().UTC().Truncate(time.Second),
			}, tc.extra)
			if err != nil {
				t.Fatalf("seed video: %v", err)
			}
			if tc.enriched != nil {
				if err := env.repo.UpsertEnrichment(ctx, "video", vid, "fake", tc.match, tc.enriched); err != nil {
					t.Fatalf("upsert enrichment: %v", err)
				}
			}
			_, body := getJSON(t, env.srv.URL+"/api/v1/media/"+itoa(vid))
			links, _ := body["external_links"].([]any)
			if len(links) != len(tc.want) {
				t.Fatalf("external_links = %v, want %d pill(s) %v", body["external_links"], len(tc.want), tc.want)
			}
			got := linksByProvider(t, links)
			for ns, wantURL := range tc.want {
				lm := got[ns]
				if lm == nil {
					t.Fatalf("pill namespace %q missing from %v", ns, links)
				}
				if u, present := lm["url"]; wantURL == "" && present {
					t.Errorf("%s url = %v, want omitted (degraded)", ns, u)
				} else if wantURL != "" && u != wantURL {
					t.Errorf("%s url = %v, want %q", ns, u, wantURL)
				}
			}
		})
	}

	// A re-match to a different id upserts without clearing (Service.Enrich), so a
	// key the new payload omits keeps the old id on its row. backdrop_url sorts
	// before description in EnrichmentForEntity's field_key order, so without the
	// newest-row rule the stale tmdb:1 would win the namespace dedup.
	t.Run("re-match wins over rows the new payload left behind", func(t *testing.T) {
		vid := seedVideo(t, env.repo, "/m/rematch.mkv", "Rematched Clip")
		if err := env.repo.UpsertEnrichment(ctx, "video", vid, "fake", "tmdb:1",
			map[string][]string{"backdrop_url": {"https://fake.example/a.jpg"}, "description": {"first"}}); err != nil {
			t.Fatalf("first match: %v", err)
		}
		// fetched_at is second-resolution (RFC3339); backdate the first match so the
		// re-match below is strictly newer instead of racing the wall clock.
		if _, err := env.db.ExecContext(ctx,
			`UPDATE entity_enrichment SET fetched_at = ? WHERE entity_type = 'video' AND entity_id = ?`,
			time.Now().UTC().Add(-time.Minute).Format(time.RFC3339), vid); err != nil {
			t.Fatalf("backdate: %v", err)
		}
		if err := env.repo.UpsertEnrichment(ctx, "video", vid, "fake", "tmdb:812",
			map[string][]string{"description": {"second"}}); err != nil {
			t.Fatalf("re-match: %v", err)
		}
		_, body := getJSON(t, env.srv.URL+"/api/v1/media/"+itoa(vid))
		links, _ := body["external_links"].([]any)
		if len(links) != 1 {
			t.Fatalf("external_links = %v, want one pill", body["external_links"])
		}
		if got := linksByProvider(t, links)["tmdb"]["url"]; got != "https://tmdb.example/movie/812" {
			t.Errorf("url = %v, want the re-matched movie 812", got)
		}
	})

	bare := seedVideo(t, env.repo, "/m/bare.mkv", "Bare Clip")
	_, body := getJSON(t, env.srv.URL+"/api/v1/media/"+itoa(bare))
	if got, _ := body["external_links"].([]any); len(got) != 0 {
		t.Errorf("bare video external_links = %v, want none", body["external_links"])
	}
}
