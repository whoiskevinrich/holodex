package api_test

import (
	"context"
	"encoding/json"
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

// F49 claimed provider keys (ADR-074), slice B: the DB claim store, the merge that
// materializes a claim as an appended candidate source, and the owner-gated API.
//
// The pure derivation + suppression half (slice A) is covered in
// internal/resolver/auto_register_test.go; these tests cover the seam it plugs into.

// claimServer wires the claim surface over a real repo: a person ("Alice") whose two
// matched providers spell one paragraph differently — `tmdb:biography` and
// `provb:life_story`. Both auto-register as their own display-only rows until claimed,
// which is GH #178 in miniature. Canonical `bio` is empty, which is exactly why the
// provider keys are the only biography on the page. token="" leaves the owner gate open.
func claimServer(t *testing.T, token string) (*httptest.Server, *repo.Repo, int64) {
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
	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/x.mkv", FileSize: 1, Title: "Clip", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
		People:    []model.Person{{Name: "Alice"}},
	}, nil)
	if err != nil {
		t.Fatalf("seed person: %v", err)
	}
	linkPeople(t, r, vid, "Alice")
	pid, _, err := r.PersonIDByName(ctx, "Alice")
	if err != nil {
		t.Fatalf("person id: %v", err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityPerson, pid, "tmdb", "ext-9", map[string][]string{
		"name":      {"Alice"},
		"biography": {"Alice grew up in Ohio."},
	}); err != nil {
		t.Fatalf("seed tmdb: %v", err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityPerson, pid, "provb", "ext-3", map[string][]string{
		"life_story": {"Alice grew up in Ohio."},
	}); err != nil {
		t.Fatalf("seed provb: %v", err)
	}

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, pid
}

func claimURL(srv *httptest.Server, entityType, provider, key string) string {
	return srv.URL + "/api/v1/admin/field-claims/" + entityType + "/" + provider + "/" + key
}

// getJSONList GETs an array endpoint (the claims and targets lists).
func getJSONList(t *testing.T, url string) (int, []map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	var out []map[string]any
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
	return resp.StatusCode, out
}

// TestClaim_SuppressesRowAndFeedsTarget is the cardinal flow and the GH #178 fix end to
// end: two providers' differently-named keys stop rendering as their own rows and become
// candidate sources of the canonical field that claimed them — which is what makes the
// value reachable at all, since `bio` is empty and empty undecided fields never resolve.
func TestClaim_SuppressesRowAndFeedsTarget(t *testing.T) {
	srv, _, pid := claimServer(t, "")

	// Before: two display-only rows for one paragraph, and no `bio` at all.
	if f, ok := personResolvedField(t, srv, pid, "biography"); !ok || f["auto_registered"] != true {
		t.Fatalf("biography should auto-register before the claim, got %v", f)
	}
	if _, ok := personResolvedField(t, srv, pid, "life_story"); !ok {
		t.Fatal("life_story should auto-register before the claim")
	}
	if _, ok := personResolvedField(t, srv, pid, "bio"); ok {
		t.Fatal("empty undecided bio should not resolve before the claim")
	}

	for _, c := range []struct{ provider, key string }{{"tmdb", "biography"}, {"provb", "life_story"}} {
		if code := sendDecision(t, http.MethodPut, claimURL(srv, "person", c.provider, c.key), "",
			map[string]any{"canonical": "bio"}); code != 204 {
			t.Fatalf("claim %s:%s: want 204, got %d", c.provider, c.key, code)
		}
	}

	// After: both rows are gone and their value resolves as Biography.
	if f, ok := personResolvedField(t, srv, pid, "biography"); ok {
		t.Errorf("claimed key must not auto-register, got %v", f)
	}
	if _, ok := personResolvedField(t, srv, pid, "life_story"); ok {
		t.Error("claimed key must not auto-register")
	}
	bio, ok := personResolvedField(t, srv, pid, "bio")
	if !ok {
		t.Fatal("the claimed value must resolve under the target field")
	}
	if vals, _ := bio["values"].([]any); len(vals) == 0 || vals[0] != "Alice grew up in Ohio." {
		t.Errorf("bio must carry the claimed value, got %v", bio["values"])
	}
	// Claims append at the END of the candidate list, below the field's own sources, and
	// among themselves in (provider, field_key) order rather than insertion order (D3):
	// `tmdb:biography` was claimed FIRST but `provb:life_story` resolves, because
	// lexicographic order is reproducible from the table's contents alone.
	if bio["winning_source"] != "provb:life_story" {
		t.Errorf("claims must append in lexicographic, not insertion, order, got %v", bio["winning_source"])
	}
	cands, _ := bio["candidates"].([]any)
	foundB := false
	for _, c := range cands {
		if m, _ := c.(map[string]any); m["source"] == "provider:provb" {
			foundB = true
		}
	}
	if !foundB {
		t.Errorf("both claimed providers must appear as candidates, got %v", cands)
	}
}

// TestClaim_ClearRestoresRow: removing a claim returns the key to F39 auto-registration.
// This is the "and undo it later" half of the acceptance criterion, served by the
// Attached keys list.
func TestClaim_ClearRestoresRow(t *testing.T) {
	srv, _, pid := claimServer(t, "")
	if code := sendDecision(t, http.MethodPut, claimURL(srv, "person", "tmdb", "biography"), "",
		map[string]any{"canonical": "bio"}); code != 204 {
		t.Fatalf("claim: got %d", code)
	}
	if _, ok := personResolvedField(t, srv, pid, "biography"); ok {
		t.Fatal("claimed key must not auto-register")
	}
	if code := sendDecision(t, http.MethodDelete, claimURL(srv, "person", "tmdb", "biography"), "", nil); code != 204 {
		t.Fatalf("clear claim: got %d", code)
	}
	f, ok := personResolvedField(t, srv, pid, "biography")
	if !ok || f["auto_registered"] != true {
		t.Errorf("removing the claim must bring the row back, got %v", f)
	}
}

// TestClaim_ProviderScoped pins the grain the PK exists for (D1): claiming one provider's
// spelling must leave another provider's identically-named key alone. Here both providers
// use `alias_note` for different things.
func TestClaim_ProviderScoped(t *testing.T) {
	srv, r, pid := claimServer(t, "")
	ctx := context.Background()
	for _, p := range []struct{ provider, ext, value string }{
		{"tmdb", "ext-9", "also known as Al"},
		{"provb", "ext-3", "note: unverified"},
	} {
		if err := r.UpsertEnrichment(ctx, model.EnrichEntityPerson, pid, p.provider, p.ext, map[string][]string{
			"alias_note": {p.value},
		}); err != nil {
			t.Fatalf("seed %s: %v", p.provider, err)
		}
	}
	// Claimed into `bio`: any known person canonical will do, and `aliases` is no longer
	// one (F58, ADR-088 D1). The grain under test is the provider scoping, not the target.
	if code := sendDecision(t, http.MethodPut, claimURL(srv, "person", "tmdb", "alias_note"), "",
		map[string]any{"canonical": "bio"}); code != 204 {
		t.Fatalf("claim: got %d", code)
	}

	// The row survives carrying the UNclaimed provider's value only.
	row, ok := personResolvedField(t, srv, pid, "alias_note")
	if !ok {
		t.Fatal("a partially-claimed key must still auto-register for the unclaimed provider")
	}
	vals, _ := row["values"].([]any)
	if len(vals) != 1 || vals[0] != "note: unverified" {
		t.Errorf("only the unclaimed provider's value should remain, got %v", vals)
	}
	if row["winning_source"] != "provb:alias_note" {
		t.Errorf("provenance must name the surviving provider, got %v", row["winning_source"])
	}
}

// TestClaim_ClearsPromotionAndDoesNotRestoreIt pins RD3/D5 from both ends: claiming a
// promoted key destroys the promotion in the same write, and un-claiming does NOT bring
// it back — the clear is a delete, not a suspension.
func TestClaim_ClearsPromotionAndDoesNotRestoreIt(t *testing.T) {
	srv, r, pid := claimServer(t, "")
	ctx := context.Background()

	if code := sendDecision(t, http.MethodPut, promoteURL(srv, "person", "biography"), "",
		map[string]any{"label": "Life"}); code != 204 {
		t.Fatalf("promote: got %d", code)
	}
	if f, _ := personResolvedField(t, srv, pid, "biography"); f["promoted"] != true {
		t.Fatalf("precondition: biography should be promoted, got %v", f)
	}

	if code := sendDecision(t, http.MethodPut, claimURL(srv, "person", "tmdb", "biography"), "",
		map[string]any{"canonical": "bio"}); code != 204 {
		t.Fatalf("claim: got %d", code)
	}
	if rows, err := r.PromotionsForEntityType(ctx, model.EnrichEntityPerson); err != nil || len(rows) != 0 {
		t.Fatalf("claiming must clear the promotion, got %v err=%v", rows, err)
	}
	if f, ok := personResolvedField(t, srv, pid, "biography"); ok {
		t.Errorf("a claimed key renders under its target, not as its own promoted field: %v", f)
	}

	// Un-claiming returns the key to the auto-registered floor — not to promoted.
	if code := sendDecision(t, http.MethodDelete, claimURL(srv, "person", "tmdb", "biography"), "", nil); code != 204 {
		t.Fatalf("clear claim: got %d", code)
	}
	f, ok := personResolvedField(t, srv, pid, "biography")
	if !ok || f["auto_registered"] != true || f["promoted"] == true {
		t.Errorf("removing a claim must not resurrect the promotion, got %v", f)
	}
}

// TestClaim_DanglingTargetIsInert pins D4. The row is written straight to the repo
// because the API's 422 makes it unreachable through the front door — the state arises
// from a mapping edit or a cleared promotion, not from a bad request. The claim then
// appends nothing AND suppresses nothing: the key auto-registers exactly as it did
// pre-F49, so the failure mode is visible rather than a value hidden with nowhere to go.
func TestClaim_DanglingTargetIsInert(t *testing.T) {
	srv, r, pid := claimServer(t, "")
	if err := r.SetClaim(context.Background(), model.EnrichEntityPerson, "tmdb", "biography", "tagline"); err != nil {
		t.Fatalf("seed dangling claim: %v", err)
	}
	f, ok := personResolvedField(t, srv, pid, "biography")
	if !ok || f["auto_registered"] != true {
		t.Errorf("a dangling claim must neither suppress nor append, got %v", f)
	}
	// And it survives: a transient absent target must not destroy owner intent.
	if rows, err := r.ClaimsForEntityType(context.Background(), model.EnrichEntityPerson); err != nil || len(rows) != 1 {
		t.Errorf("a dangling claim must never be pruned, got %v err=%v", rows, err)
	}
}

// TestClaim_Validation pins the FR4 matrix.
func TestClaim_Validation(t *testing.T) {
	srv, _, _ := claimServer(t, "")
	body := map[string]any{"canonical": "bio"}

	if code := sendDecision(t, http.MethodPut, claimURL(srv, "person", "tmdb", "bio"), "", body); code != 422 {
		t.Errorf("claim a canonical key: want 422, got %d", code)
	}
	if code := sendDecision(t, http.MethodPut, claimURL(srv, "person", "tmdb", "_secret"), "", body); code != 422 {
		t.Errorf("claim a reserved key: want 422, got %d", code)
	}
	if code := sendDecision(t, http.MethodPut, claimURL(srv, "widget", "tmdb", "biography"), "", body); code != 400 {
		t.Errorf("unknown entity type: want 400, got %d", code)
	}
	// The target must be a field the entity type declares — the constraint the ADR-074
	// security deferral rests on: a claim adds a candidate to a declared surface, it can
	// never invent one.
	if code := sendDecision(t, http.MethodPut, claimURL(srv, "person", "tmdb", "biography"), "",
		map[string]any{"canonical": "runtime"}); code != 422 {
		t.Errorf("undeclared target: want 422, got %d", code)
	}
	if code := sendDecision(t, http.MethodPut, claimURL(srv, "person", "tmdb", "biography"), "",
		map[string]any{}); code != 400 {
		t.Errorf("missing canonical: want 400, got %d", code)
	}
	if code := sendDecision(t, http.MethodPut, claimURL(srv, "person", "tmdb", "biography"), "", body); code != 204 {
		t.Errorf("valid claim: want 204, got %d", code)
	}
	if code := sendDecision(t, http.MethodDelete, claimURL(srv, "person", "tmdb", "nothing_here"), "", nil); code != 204 {
		t.Errorf("idempotent clear: want 204, got %d", code)
	}
}

// TestClaim_ListRoundTrips covers the read the Attached keys list (FR8) is built on.
func TestClaim_ListRoundTrips(t *testing.T) {
	srv, _, _ := claimServer(t, "")
	if code := sendDecision(t, http.MethodPut, claimURL(srv, "person", "tmdb", "biography"), "",
		map[string]any{"canonical": "bio"}); code != 204 {
		t.Fatalf("claim: got %d", code)
	}
	code, list := getJSONList(t, srv.URL+"/api/v1/admin/field-claims/person")
	if code != 200 || len(list) != 1 {
		t.Fatalf("list claims = %d %v", code, list)
	}
	if list[0]["provider"] != "tmdb" || list[0]["field_key"] != "biography" || list[0]["canonical"] != "bio" {
		t.Errorf("claim view = %v", list[0])
	}
	// Entity types are independent stores.
	if _, other := getJSONList(t, srv.URL+"/api/v1/admin/field-claims/studio"); len(other) != 0 {
		t.Errorf("a person claim must not appear under studio, got %v", other)
	}
}

// TestFieldTargets_ListsWholeEffectiveSet pins DD2's finding: the picker's targets come
// from the entity type's field set, not from what the page renders — `bio` is empty for
// this person and so is absent from resolved[], which is precisely when the owner needs
// to pick it. It also covers the `merge` flag the outcome preview depends on, and that a
// promoted field is a legal target.
func TestFieldTargets_ListsWholeEffectiveSet(t *testing.T) {
	srv, _, pid := claimServer(t, "")
	if _, ok := personResolvedField(t, srv, pid, "bio"); ok {
		t.Fatal("precondition: empty bio should not be on the page")
	}

	code, targets := getJSONList(t, srv.URL+"/api/v1/admin/field-targets/person")
	if code != 200 {
		t.Fatalf("field targets = %d", code)
	}
	byCanonical := map[string]map[string]any{}
	for _, tgt := range targets {
		byCanonical[tgt["canonical"].(string)] = tgt
	}
	bio, ok := byCanonical["bio"]
	if !ok {
		t.Fatalf("the target list must include the empty field the page omits, got %v", targets)
	}
	if bio["label"] == "" || bio["merge"] != false {
		t.Errorf("bio is a labelled replace field, got %v", bio)
	}
	// A promoted key becomes a legal claim target (D5: claims merge after promotions),
	// and promoting with the chips renderer is now the only way a person gets a merge
	// field at all — F58 retired `aliases`, which used to be the one this asserted on
	// (ADR-088 D1). The `merge` flag still has to reach the target list, since the
	// outcome preview depends on it.
	if code := sendDecision(t, http.MethodPut, promoteURL(srv, "person", "life_story"), "",
		map[string]any{"label": "Life story", "render": "chips"}); code != 204 {
		t.Fatalf("promote: got %d", code)
	}
	_, targets = getJSONList(t, srv.URL+"/api/v1/admin/field-targets/person")
	found := false
	for _, tgt := range targets {
		if tgt["canonical"] == "life_story" && tgt["label"] == "Life story" {
			found = true
		}
	}
	if !found {
		t.Errorf("the effective set must include promoted fields, got %v", targets)
	}
	byCanonical = map[string]map[string]any{}
	for _, tgt := range targets {
		byCanonical[tgt["canonical"].(string)] = tgt
	}
	if life := byCanonical["life_story"]; life == nil || life["merge"] != true {
		t.Errorf("a chips-rendered promotion is a merge target — the outcome preview depends on it, got %v", life)
	}
	if code := sendDecision(t, http.MethodPut, claimURL(srv, "person", "tmdb", "biography"), "",
		map[string]any{"canonical": "life_story"}); code != 204 {
		t.Errorf("claiming into a promoted field: want 204, got %d", code)
	}
}

// TestClaim_OwnerGated confirms the whole surface sits behind requireOwner.
func TestClaim_OwnerGated(t *testing.T) {
	srv, _, _ := claimServer(t, "s3cret")
	body := map[string]any{"canonical": "bio"}

	if code := sendDecision(t, http.MethodPut, claimURL(srv, "person", "tmdb", "biography"), "", body); code != 401 {
		t.Errorf("PUT claim without token: want 401, got %d", code)
	}
	if code := sendDecision(t, http.MethodDelete, claimURL(srv, "person", "tmdb", "biography"), "", nil); code != 401 {
		t.Errorf("DELETE claim without token: want 401, got %d", code)
	}
	if code, _ := getJSONList(t, srv.URL+"/api/v1/admin/field-claims/person"); code != 401 {
		t.Errorf("GET claims without token: want 401, got %d", code)
	}
	if code, _ := getJSONList(t, srv.URL+"/api/v1/admin/field-targets/person"); code != 401 {
		t.Errorf("GET targets without token: want 401, got %d", code)
	}
	if code := sendDecision(t, http.MethodPut, claimURL(srv, "person", "tmdb", "biography"), "s3cret", body); code != 204 {
		t.Errorf("PUT claim with token: want 204, got %d", code)
	}
}
