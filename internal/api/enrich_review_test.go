package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/enrich"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// resolveCounter wraps the fake provider to count /resolve calls specifically — the
// "Refresh never re-searches" invariant (ADR-065 RD7/RD8) needs a call-count assertion
// finer than Fake's single combined Calls counter.
type resolveCounter struct {
	*enrich.Fake
	n *int
}

func (c *resolveCounter) Resolve(ctx context.Context, entityType string, hint enrich.Hint) ([]enrich.Candidate, error) {
	*c.n++
	return c.Fake.Resolve(ctx, entityType, hint)
}

// reviewServer wires the owner-gated F47 review-workflow surface over one fake
// provider supporting person+studio+video, seeding one entity of each type: a person
// ("Hayao Miyazaki", matching the fake's canned record), a studio ("Studio Ghibli",
// via the link-reconcile path, also fake-matchable), and that same video. Returns the
// resolve-call counter so tests can assert "Refresh never re-searches".
func reviewServer(t *testing.T, token string) (srv *httptest.Server, r *repo.Repo, pid, sid, vid int64, resolveCalls *int) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r = repo.New(database)
	ctx := context.Background()

	sp := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(sp, []byte("sources:\n  - name: fake\n    base_url: http://fake:9100\n    entity_types: [person, studio, video]\n    enabled: true\n"), 0o644); err != nil {
		t.Fatalf("write sources: %v", err)
	}
	store, err := enrich.NewStore(sp)
	if err != nil {
		t.Fatalf("sources store: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	resolveCalls = new(int)
	svc := enrich.NewServiceWithClient(store, r, log, func(enrich.Source) enrich.ProviderClient {
		return &resolveCounter{Fake: enrich.NewFake("fake"), n: resolveCalls}
	})

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetEnrichment(svc)
	h.SetAuth(api.NewAuth(token), false)
	srv = httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	vid, err = r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/x.mkv", Title: "Clip", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
		People:    []model.Person{{Name: "Hayao Miyazaki"}},
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, vid, []string{"Studio Ghibli"}, nil); err != nil {
		t.Fatalf("seed studio: %v", err)
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil || len(studios) != 1 {
		t.Fatalf("list studios: %v (%d)", err, len(studios))
	}
	sid = studios[0].ID
	pid, _, err = r.PersonIDByName(ctx, "Hayao Miyazaki")
	if err != nil {
		t.Fatalf("person id: %v", err)
	}
	return srv, r, pid, sid, vid, resolveCalls
}

// The queue is a zero-cost DB read (RD2/P0-1): every seeded entity appears unreviewed,
// with no provider call made to build the list.
func TestEnrichQueue_Listing(t *testing.T) {
	srv, _, pid, sid, vid, resolveCalls := reviewServer(t, "s3cret")
	url := srv.URL + "/api/v1/owner/enrich-queue"

	if code := sendTok(t, http.MethodGet, url, ""); code != http.StatusUnauthorized {
		t.Errorf("no-token queue = %d, want 401", code)
	}

	code, body := getJSONTok(t, url, "s3cret")
	if code != http.StatusOK {
		t.Fatalf("queue = %d, want 200", code)
	}
	if *resolveCalls != 0 {
		t.Errorf("resolveCalls = %d, want 0 (queue must never call a provider)", *resolveCalls)
	}
	rows, _ := body["rows"].([]any)
	if len(rows) != 3 {
		t.Fatalf("rows = %v, want 3 (person+studio+video)", body["rows"])
	}
	// entity_id alone is not unique across types (each table auto-increments from 1),
	// so key on (entity_type, entity_id) together.
	type key struct {
		et string
		id int64
	}
	byKey := map[key]map[string]any{}
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		byKey[key{row["entity_type"].(string), int64(row["entity_id"].(float64))}] = row
	}
	for _, want := range []key{
		{model.EnrichEntityPerson, pid},
		{model.EnrichEntityStudio, sid},
		{model.EnrichEntityVideo, vid},
	} {
		row, ok := byKey[want]
		if !ok {
			t.Fatalf("entity %+v missing from queue: %v", want, rows)
		}
		providers, _ := row["providers"].([]any)
		if len(providers) != 1 {
			t.Fatalf("entity %+v providers = %v, want one", want, providers)
		}
		p, _ := providers[0].(map[string]any)
		if p["provider"] != "fake" || p["state"] != "unreviewed" {
			t.Errorf("entity %+v provider state = %v, want fake/unreviewed", want, p)
		}
	}
}

// Dismissing removes an entity from the queue and blocks /resolve (409) until an
// explicit undismiss clears it (RD4).
func TestEnrichDismissUndismiss(t *testing.T) {
	srv, _, pid, _, _, _ := reviewServer(t, "s3cret")
	base := srv.URL + "/api/v1/people/" + itoa(pid)
	dismiss := base + "/enrich/fake/dismiss"
	resolve := base + "/enrich/resolve"
	queue := srv.URL + "/api/v1/owner/enrich-queue"

	if code := sendTok(t, http.MethodPost, dismiss, ""); code != http.StatusUnauthorized {
		t.Errorf("no-token dismiss = %d, want 401", code)
	}
	if code := sendTok(t, http.MethodPost, dismiss, "s3cret"); code != http.StatusNoContent {
		t.Fatalf("dismiss = %d, want 204", code)
	}

	// The queue no longer lists the person (its only provider is now dismissed).
	_, body := getJSONTok(t, queue, "s3cret")
	rows, _ := body["rows"].([]any)
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		if row["entity_type"] == "person" && int64(row["entity_id"].(float64)) == pid {
			t.Fatalf("dismissed person must drop out of the queue: %v", rows)
		}
	}

	// /resolve is blocked while dismissed.
	if code, _ := postTok(t, resolve, "s3cret", map[string]string{"provider": "fake", "query": "miyazaki"}); code != http.StatusConflict {
		t.Errorf("resolve while dismissed = %d, want 409", code)
	}

	// Undismiss ("Try again") clears the block.
	if code := sendTok(t, http.MethodDelete, dismiss, "s3cret"); code != http.StatusNoContent {
		t.Fatalf("undismiss = %d, want 204", code)
	}
	if code, _ := postTok(t, resolve, "s3cret", map[string]string{"provider": "fake", "query": "miyazaki"}); code != http.StatusOK {
		t.Errorf("resolve after undismiss = %d, want 200", code)
	}
}

// Refresh calls apply() directly with the stored external_id — no /resolve — and 400s
// when the provider isn't linked yet (RD7/P0-5).
func TestEnrichRefresh(t *testing.T) {
	srv, _, pid, _, _, resolveCalls := reviewServer(t, "")
	base := srv.URL + "/api/v1/people/" + itoa(pid)
	refresh := base + "/enrich/fake/refresh"

	if code, _ := postTok(t, refresh, "", nil); code != http.StatusBadRequest {
		t.Fatalf("refresh before linking = %d, want 400", code)
	}

	if code, _ := postTok(t, base+"/enrich", "", map[string]string{"provider": "fake", "external_id": "tmdb:608"}); code != http.StatusOK {
		t.Fatalf("apply = %d, want 200", code)
	}
	before := *resolveCalls

	code, body := postTok(t, refresh, "", nil)
	if code != http.StatusOK {
		t.Fatalf("refresh = %d, want 200", code)
	}
	if enriched, _ := body["enriched"].([]any); len(enriched) == 0 {
		t.Fatalf("refresh enriched empty: %v", body["enriched"])
	}
	if *resolveCalls != before {
		t.Errorf("resolveCalls = %d, want unchanged from %d (refresh must never call /resolve)", *resolveCalls, before)
	}
}

// Refresh-all fans out over the entity's providers: a single strong match auto-applies,
// and a dismissed unlinked provider is left out of the response (RD4 still blocks it
// inside a bulk fan-out).
func TestEnrichRefreshAll(t *testing.T) {
	srv, _, pid, _, _, _ := reviewServer(t, "")
	base := srv.URL + "/api/v1/people/" + itoa(pid)
	refreshAll := base + "/enrich/refresh-all"

	code, body := postTok(t, refreshAll, "", nil)
	if code != http.StatusOK {
		t.Fatalf("refresh-all = %d, want 200", code)
	}
	results, _ := body["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results = %v, want one (fake)", results)
	}
	res, _ := results[0].(map[string]any)
	if res["provider"] != "fake" || res["status"] != "auto_applied" {
		t.Fatalf("result = %v, want fake/auto_applied (single strong match)", res)
	}
	if enriched, _ := res["enriched"].([]any); len(enriched) == 0 {
		t.Errorf("auto_applied result carries no enriched fields: %v", res)
	}

	// A second refresh-all now refreshes the already-linked provider directly.
	code, body = postTok(t, refreshAll, "", nil)
	if code != http.StatusOK {
		t.Fatalf("refresh-all (2nd) = %d, want 200", code)
	}
	results, _ = body["results"].([]any)
	res, _ = results[0].(map[string]any)
	if res["status"] != "refreshed" {
		t.Fatalf("2nd refresh-all status = %v, want refreshed", res)
	}

	// Clear the link and dismiss the provider — refresh-all must leave it out of the
	// response entirely (never silently re-resolved).
	if code := sendTok(t, http.MethodDelete, base+"/enrich/fake", ""); code != http.StatusNoContent {
		t.Fatalf("clear = %d, want 204", code)
	}
	if code := sendTok(t, http.MethodPost, base+"/enrich/fake/dismiss", ""); code != http.StatusNoContent {
		t.Fatalf("dismiss = %d, want 204", code)
	}
	code, body = postTok(t, refreshAll, "", nil)
	if code != http.StatusOK {
		t.Fatalf("refresh-all (dismissed) = %d, want 200", code)
	}
	if results, _ := body["results"].([]any); len(results) != 0 {
		t.Fatalf("dismissed provider must be left out of refresh-all: %v", results)
	}
}
