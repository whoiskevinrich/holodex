package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// promotionServer wires the F44 in-app promotion surface over a real repo: a person
// ("Alice") whose matched `tmdb` provider supplies a non-canonical shadow key
// (`measurements`) — the F39 auto-registered kind a promotion targets. token="" leaves
// the owner gate open.
func promotionServer(t *testing.T, token string) (*httptest.Server, *repo.Repo, int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx := context.Background()
	if _, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/x.mkv", FileSize: 1, Title: "Clip", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
		People:    []model.Person{{Name: "Alice"}},
	}, nil); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	pid, _, err := r.PersonIDByName(ctx, "Alice")
	if err != nil {
		t.Fatalf("person id: %v", err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityPerson, pid, "tmdb", "ext-9", map[string][]string{
		"name":         {"Alice"},
		"measurements": {"34-24-36"}, // non-canonical → auto-registered until promoted
	}); err != nil {
		t.Fatalf("seed enrichment: %v", err)
	}

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, pid
}

// countResolved counts how many resolved fields carry the given canonical (a promoted
// key must render exactly once — never doubled as auto row + mapped row).
func countResolved(t *testing.T, srv *httptest.Server, id int64, canonical string) int {
	t.Helper()
	body := getPersonBody(t, srv, id)
	raw, _ := body["resolved"].([]any)
	n := 0
	for _, f := range raw {
		if m, _ := f.(map[string]any); m["canonical"] == canonical {
			n++
		}
	}
	return n
}

func promoteURL(srv *httptest.Server, entityType, key string) string {
	return srv.URL + "/api/v1/admin/field-promotions/" + entityType + "/" + key
}

// TestPromotion_PromoteMakesCuratable is the cardinal flow (AC1/2/5/6): an
// auto-registered non-canonical field promotes into a first-class curatable field with
// the F36 source picker + candidates, renders once, and reverts on de-promote.
func TestPromotion_PromoteMakesCuratable(t *testing.T) {
	srv, _, pid := promotionServer(t, "")

	// Before promotion: display-only auto-registered row, no decision/candidates.
	before, ok := personResolvedField(t, srv, pid, "measurements")
	if !ok {
		t.Fatal("measurements should surface as an auto-registered field")
	}
	if before["auto_registered"] != true {
		t.Errorf("pre-promotion measurements must be auto_registered, got %v", before)
	}
	if _, has := before["decision"]; has {
		t.Errorf("auto-registered field must not carry a decision: %v", before)
	}

	// Promote with a curated label + render mode.
	if code := sendDecision(t, http.MethodPut, promoteURL(srv, "person", "measurements"), "",
		map[string]any{"label": "Vitals", "render": "text", "group": "attributes", "order": 2}); code != 204 {
		t.Fatalf("promote: want 204, got %d", code)
	}

	// After promotion: curatable — curated label, no longer auto-registered, promoted
	// flag set, full F36 markers (decision + candidates), renders once.
	after, ok := personResolvedField(t, srv, pid, "measurements")
	if !ok {
		t.Fatal("promoted measurements must still resolve")
	}
	if after["auto_registered"] == true {
		t.Errorf("promoted field must not be auto_registered: %v", after)
	}
	if after["promoted"] != true {
		t.Errorf("promoted field must carry promoted=true: %v", after)
	}
	if after["label"] != "Vitals" {
		t.Errorf("promoted label should win (tier-0), got %v", after["label"])
	}
	if after["values"].([]any)[0] != "34-24-36" || after["winning_source"] != "tmdb:measurements" {
		t.Errorf("promoted field must resolve the provider value: %v", after)
	}
	dec, hasDec := after["decision"].(map[string]any)
	if !hasDec || dec["source"] != "provider:tmdb" {
		t.Errorf("promoted field must gain an F36 decision marker: %v", after["decision"])
	}
	// Candidates: the record baseline (empty) + the tmdb provider value.
	cands, _ := after["candidates"].([]any)
	foundTmdb := false
	for _, c := range cands {
		if m, _ := c.(map[string]any); m["source"] == "provider:tmdb" && m["value"] == "34-24-36" {
			foundTmdb = true
		}
	}
	if !foundTmdb {
		t.Errorf("promoted field must derive the tmdb candidate from shadow provenance: %v", cands)
	}
	if n := countResolved(t, srv, pid, "measurements"); n != 1 {
		t.Errorf("promoted key must render exactly once, got %d", n)
	}

	// De-promote reverts to the auto-registered display-only row (AC5); the shadow
	// value is untouched.
	if code := sendDecision(t, http.MethodDelete, promoteURL(srv, "person", "measurements"), "", nil); code != 204 {
		t.Fatalf("de-promote: want 204, got %d", code)
	}
	reverted, ok := personResolvedField(t, srv, pid, "measurements")
	if !ok || reverted["auto_registered"] != true {
		t.Errorf("de-promoted field must return to auto-registered: %v", reverted)
	}
	if reverted["promoted"] == true {
		t.Errorf("de-promoted field must not stay promoted: %v", reverted)
	}
}

// TestPromotion_LabelInherit covers the empty-column inherit: a promotion whose only
// purpose is "make this curatable" keeps the title-case label (tier-4 floor).
func TestPromotion_LabelInherit(t *testing.T) {
	srv, _, pid := promotionServer(t, "")
	if code := sendDecision(t, http.MethodPut, promoteURL(srv, "person", "measurements"), "",
		map[string]any{}); code != 204 {
		t.Fatalf("promote (inherit): want 204, got %d", code)
	}
	f, _ := personResolvedField(t, srv, pid, "measurements")
	if f["promoted"] != true || f["label"] != "Measurements" {
		t.Errorf("empty-label promotion must inherit the title-case label, got %v", f["label"])
	}
}

// TestPromotion_Validation pins the promotion predicate: canonical/`_` keys and unknown
// entity types are rejected; a valid non-canonical promote succeeds.
func TestPromotion_Validation(t *testing.T) {
	srv, _, _ := promotionServer(t, "")

	// Canonical key — the registry owns it, you cannot promote `bio`.
	if code := sendDecision(t, http.MethodPut, promoteURL(srv, "person", "bio"), "", map[string]any{}); code != 422 {
		t.Errorf("promote canonical: want 422, got %d", code)
	}
	// Reserved sidecar key.
	if code := sendDecision(t, http.MethodPut, promoteURL(srv, "person", "_secret"), "", map[string]any{}); code != 422 {
		t.Errorf("promote reserved key: want 422, got %d", code)
	}
	// Unknown entity type.
	if code := sendDecision(t, http.MethodPut, promoteURL(srv, "widget", "measurements"), "", map[string]any{}); code != 400 {
		t.Errorf("promote unknown entity: want 400, got %d", code)
	}
	// Valid non-canonical key.
	if code := sendDecision(t, http.MethodPut, promoteURL(srv, "person", "measurements"), "", map[string]any{}); code != 204 {
		t.Errorf("promote non-canonical: want 204, got %d", code)
	}
	// De-promote of a missing row is idempotent.
	if code := sendDecision(t, http.MethodDelete, promoteURL(srv, "person", "handedness"), "", nil); code != 204 {
		t.Errorf("idempotent de-promote: want 204, got %d", code)
	}
}

// TestPromotion_UnknownRenderCoerces confirms an unknown render mode coerces to text
// (defense in depth — the SPA <select> prevents it, the server is the authority).
func TestPromotion_UnknownRenderCoerces(t *testing.T) {
	srv, r, pid := promotionServer(t, "")
	if code := sendDecision(t, http.MethodPut, promoteURL(srv, "person", "measurements"), "",
		map[string]any{"render": "marquee"}); code != 204 {
		t.Fatalf("promote: want 204, got %d", code)
	}
	rows, _ := r.PromotionsForEntityType(context.Background(), model.EnrichEntityPerson)
	if len(rows) != 1 || rows[0].Render != "" {
		t.Errorf("unknown render must coerce to text (stored empty), got %+v", rows)
	}
	f, _ := personResolvedField(t, srv, pid, "measurements")
	if _, hasDisplay := f["display"]; hasDisplay {
		t.Errorf("coerced-to-text field must render as inline text (no display), got %v", f["display"])
	}
}

// TestPromotion_OwnerGated confirms every mutation is behind requireOwner.
func TestPromotion_OwnerGated(t *testing.T) {
	srv, _, _ := promotionServer(t, "s3cret")

	if code := sendDecision(t, http.MethodPut, promoteURL(srv, "person", "measurements"), "", map[string]any{"label": "X"}); code != 401 {
		t.Errorf("PUT promotion without token: want 401, got %d", code)
	}
	if code := sendDecision(t, http.MethodDelete, promoteURL(srv, "person", "measurements"), "", nil); code != 401 {
		t.Errorf("DELETE promotion without token: want 401, got %d", code)
	}
	if code, _ := getJSONTok(t, srv.URL+"/api/v1/admin/field-promotions/person", ""); code != 401 {
		t.Errorf("GET promotions without token: want 401, got %d", code)
	}
	// With the token they succeed.
	if code := sendDecision(t, http.MethodPut, promoteURL(srv, "person", "measurements"), "s3cret", map[string]any{"label": "Vitals"}); code != 204 {
		t.Errorf("PUT promotion with token: want 204, got %d", code)
	}
}
