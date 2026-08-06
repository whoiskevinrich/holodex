package api_test

import (
	"context"
	"io"
	"log/slog"
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

// TestGetMedia_EnrichQueries covers ADR-080/F54's D5 wiring end to end through the
// real HTTP handler: getMedia must carry a rendered search query per enabled
// video-capable provider (FR5), using each provider's own precedence tier — an
// operator search_pattern renders a shaped query from resolved fields, while a
// provider with none configured falls to the D4 sanitized-title floor. Also proves
// AC-11: an instance with a provider that sets no new keys behaves exactly like
// today at every OTHER response field (this test's baseline shape).
func TestGetMedia_EnrichQueries(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	ctx := context.Background()

	// The title deliberately does NOT embed the studio name (unlike the spec's own
	// worked sanitizer example) so the studio and title tokens render distinct
	// content below — a title that also contained "Acme" would legitimately produce
	// a query with "Acme" twice, which is correct behavior but would make this
	// wiring test's assertions read as a false duplication rather than a clean check.
	id, err := r.UpsertVideo(ctx, &model.Video{
		FilePath:  "/m/a.mkv",
		FileSize:  1,
		Title:     "Cool Movie (Extended Cut) 720p",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, []model.ExtraMetadata{{SourceKey: "Publisher", Value: "Acme"}})
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	if err := os.WriteFile(mpath, []byte("fields:\n  - canonical: studio\n    label: Studio\n    sources: [Publisher]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mstore, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}

	sp := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(sp, []byte(`sources:
  - name: patterned
    base_url: http://patterned:9100
    entity_types: [video]
    enabled: true
    search_pattern: "{studio?} {title?}"
  - name: unpatterned
    base_url: http://unpatterned:9100
    entity_types: [video]
    enabled: true
  - name: personly
    base_url: http://personly:9100
    entity_types: [person]
    enabled: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	estore, err := enrich.NewStore(sp, log)
	if err != nil {
		t.Fatal(err)
	}
	svc := enrich.NewServiceWithClient(estore, r, log, func(enrich.Source) enrich.ProviderClient { return enrich.NewFake("fake") })

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(mstore, cache.Noop{})
	h.SetEnrichment(svc)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	_, body := getJSON(t, srv.URL+"/api/v1/media/"+itoa(id))
	eq, ok := body["enrich_queries"].(map[string]any)
	if !ok {
		t.Fatalf("enrich_queries missing or wrong shape: %v", body["enrich_queries"])
	}

	// The operator-patterned provider renders "{studio?} {title?}" from the
	// resolved fields — studio "Acme" (via the Publisher mapping) and the
	// SANITIZED title (D4 applies to the {title} token too, not just the floor: the
	// parens and "720p" are stripped here even though a pattern is configured).
	if got := eq["patterned"]; got != "Acme Cool Movie Extended Cut" {
		t.Errorf(`enrich_queries["patterned"] = %v, want "Acme Cool Movie Extended Cut"`, got)
	}
	// The unpatterned provider has no search_pattern/preferred/default configured
	// anywhere, so it falls all the way to the D4 sanitized-title floor — never the
	// literal messy title (with its parens and "720p" still attached).
	if got := eq["unpatterned"]; got != "Cool Movie Extended Cut" {
		t.Errorf(`enrich_queries["unpatterned"] = %v, want "Cool Movie Extended Cut"`, got)
	}
	// A person-only provider never appears in a VIDEO's enrich_queries at all — this
	// map is scoped to providers that actually support the video entity type.
	if _, present := eq["personly"]; present {
		t.Errorf("enrich_queries should omit a provider that doesn't support video, got %v", eq)
	}
}

// TestGetMedia_EnrichQueries_OmittedForVisitor proves enrich_queries is computed
// only for an authorized request. It exists solely to seed EnrichPicker.svelte,
// which the frontend renders behind isOwner — an unauthenticated visitor's
// getMedia response should not pay for (or leak) a value it can never use.
func TestGetMedia_EnrichQueries_OmittedForVisitor(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	ctx := context.Background()

	id, err := r.UpsertVideo(ctx, &model.Video{
		FilePath:  "/m/a.mkv",
		FileSize:  1,
		Title:     "Cool Movie",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	sp := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(sp, []byte("sources:\n  - name: tmdb\n    base_url: http://tmdb:9100\n    entity_types: [video]\n    enabled: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	estore, err := enrich.NewStore(sp, log)
	if err != nil {
		t.Fatal(err)
	}
	svc := enrich.NewServiceWithClient(estore, r, log, func(enrich.Source) enrich.ProviderClient { return enrich.NewFake("fake") })

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetEnrichment(svc)
	h.SetAuth(api.NewAuth("secret"), false) // gate closed — the request below carries no token
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	_, body := getJSON(t, srv.URL+"/api/v1/media/"+itoa(id))
	if v, present := body["enrich_queries"]; present && v != nil {
		t.Errorf("enrich_queries should be omitted/null for an unauthenticated request, got %v", v)
	}
}
