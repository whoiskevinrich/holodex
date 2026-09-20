package api_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// F65 (ADR-099 D3–D5): the materialized store behind the list surfaces. These
// tests drive the public HTTP surface against completenessBrowseServer's
// Bare / Half / Full fixture (required = 25 / 50 / 100 over the four mapped
// critical facets; extras null — no nice_to_have facet is mapped).

// completenessOf reads item["completeness"] as (required, extras) rendered
// like the spec's worked examples, or "absent" when the key is not present.
func completenessOf(t *testing.T, item map[string]any) string {
	t.Helper()
	raw, ok := item["completeness"]
	if !ok {
		return "absent"
	}
	c, _ := raw.(map[string]any)
	f := func(v any) string {
		if v == nil {
			return "null"
		}
		n, _ := v.(float64)
		return itoa(int64(n))
	}
	return f(c["required"]) + "/" + f(c["extras"])
}

func itemsByTitle(t *testing.T, body map[string]any, key string) map[string]map[string]any {
	t.Helper()
	items, _ := body["items"].([]any)
	out := make(map[string]map[string]any, len(items))
	for _, it := range items {
		m, _ := it.(map[string]any)
		name, _ := m[key].(string)
		out[name] = m
	}
	return out
}

// F65.5: every owner list item carries completeness on the default sort; a
// visitor's items never do — the same redaction posture as file metadata.
func TestListMedia_CompletenessOnItems_OwnerOnly(t *testing.T) {
	srv, _ := completenessBrowseServerWithRepo(t, "secret")

	_, body := getJSONTok(t, srv.URL+"/api/v1/media", "secret")
	got := itemsByTitle(t, body, "title")
	for title, want := range map[string]string{"Bare": "25/null", "Half": "50/null", "Full": "100/null"} {
		if c := completenessOf(t, got[title]); c != want {
			t.Errorf("owner %s completeness = %s, want %s", title, c, want)
		}
	}

	_, body = getJSONTok(t, srv.URL+"/api/v1/media", "")
	for title, item := range itemsByTitle(t, body, "title") {
		if c := completenessOf(t, item); c != "absent" {
			t.Errorf("visitor %s carries completeness %s, want absent", title, c)
		}
	}
}

// The owner gate's other half: a visitor read must never drain — never take
// writeMu and write the store — so the dirty set seeded by the fixture's
// inserts survives any number of visitor page loads and is consumed only by
// the first owner read.
func TestListMedia_VisitorReadNeverDrains(t *testing.T) {
	srv, r := completenessBrowseServerWithRepo(t, "secret")
	ctx := context.Background()

	before, err := r.DirtyCompleteness(ctx)
	if err != nil || len(before["video"]) != 3 {
		t.Fatalf("dirty before = %v, %v; want the 3 seeded videos", before, err)
	}
	for _, path := range []string{"/api/v1/media", "/api/v1/people", "/api/v1/studios"} {
		if code, _ := getJSONTok(t, srv.URL+path, ""); code != http.StatusOK {
			t.Fatalf("visitor %s: want 200, got %d", path, code)
		}
	}
	if after, _ := r.DirtyCompleteness(ctx); len(after["video"]) != 3 {
		t.Errorf("dirty after visitor reads = %v, want the 3 seeded videos untouched", after)
	}
	if _, err := r.StoredCompleteness(ctx, model.EnrichEntityVideo, before["video"][0]); err != repo.ErrNotFound {
		t.Errorf("store after visitor reads: err = %v, want ErrNotFound (nothing written)", err)
	}

	getJSONTok(t, srv.URL+"/api/v1/media", "secret")
	if after, _ := r.DirtyCompleteness(ctx); len(after["video"]) != 0 {
		t.Errorf("dirty after the first owner read = %v, want empty", after)
	}
}

// F65.6: a mutation on an input table (here the enrichment shadow store)
// reaches the badge on the next owner read with no restart and no backfill —
// the trigger dirties the row, the list read drains it.
func TestListMedia_StoreRecomputesAfterMutation(t *testing.T) {
	srv, r := completenessBrowseServerWithRepo(t, "secret")
	ctx := context.Background()

	_, body := getJSONTok(t, srv.URL+"/api/v1/media", "secret") // fills the store
	half := itemsByTitle(t, body, "title")["Half"]
	if c := completenessOf(t, half); c != "50/null" {
		t.Fatalf("Half before = %s, want 50/null", c)
	}
	halfID := int64(half["id"].(float64))

	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, halfID, "tmdb", "ext-2", map[string][]string{
		"poster_url": {"https://cdn.example/half.jpg"},
	}); err != nil {
		t.Fatal(err)
	}
	_, body = getJSONTok(t, srv.URL+"/api/v1/media", "secret")
	if c := completenessOf(t, itemsByTitle(t, body, "title")["Half"]); c != "75/null" {
		t.Errorf("Half after poster enrichment = %s, want 75/null (3 of 4 critical present)", c)
	}
	if code, _ := getJSONTok(t, srv.URL+"/api/v1/media?sort=completeness_desc", "secret"); code != http.StatusOK {
		t.Errorf("sort after mutation: want 200, got %d", code)
	}
}

// F65.7: the completeness sort pages in SQL like every other sort — page 2 of
// the ascending order is a LIMIT/OFFSET slice with the full total, not a
// Go-side slice of a full-library resolve.
func TestListMedia_CompletenessSort_Pages(t *testing.T) {
	srv, _ := completenessBrowseServerWithRepo(t, "")

	_, body := getJSONTok(t, srv.URL+"/api/v1/media?sort=completeness_asc&limit=2&offset=1", "")
	if got, want := mediaTitles(t, body), []string{"Half", "Full"}; !slices.Equal(got, want) {
		t.Errorf("page 2 = %v, want %v", got, want)
	}
	if total, _ := body["total"].(float64); total != 3 {
		t.Errorf("total = %v, want 3", total)
	}
}

// ADR-099 D3: the detail page computes live and rewrites a stored row that
// disagrees with it — a stale badge cannot outlive a look at the entity.
func TestGetMedia_SelfHealsStoredCompleteness(t *testing.T) {
	srv, r := completenessBrowseServerWithRepo(t, "secret")
	ctx := context.Background()

	_, body := getJSONTok(t, srv.URL+"/api/v1/media", "secret")
	fullID := int64(itemsByTitle(t, body, "title")["Full"]["id"].(float64))

	// Corrupt the cached row behind the triggers' back (no input table changes,
	// so nothing is dirty and a list read would serve it as-is).
	wrong := 1
	if err := r.StoreCompleteness(ctx, repo.CompletenessRow{
		EntityType: model.EnrichEntityVideo, EntityID: fullID, Required: &wrong,
		Missing: []repo.MissingFacet{{Canonical: "title", Band: "critical"}},
	}); err != nil {
		t.Fatal(err)
	}
	if code, _ := getJSONTok(t, srv.URL+"/api/v1/media/"+itoa(fullID), "secret"); code != http.StatusOK {
		t.Fatalf("detail: want 200, got %d", code)
	}
	stored, err := r.StoredCompleteness(ctx, model.EnrichEntityVideo, fullID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Required == nil || *stored.Required != 100 || len(stored.Missing) != 0 {
		t.Errorf("stored after detail view = required %v, missing %v; want 100 and none (self-healed)", stored.Required, stored.Missing)
	}
}

// F65.5 on the row lists: people and studios items carry completeness for the
// owner (person ring = photo only → 0/…; studio required is null, extras is
// its ring), never for a visitor; and the studio completeness sort honors the
// D1 null rule by ordering on extras.
func TestListPeopleAndStudios_CompletenessOnItems(t *testing.T) {
	srv, r := completenessBrowseServerWithRepo(t, "secret")
	ctx := context.Background()

	_, body := getJSONTok(t, srv.URL+"/api/v1/media", "secret")
	fullID := int64(itemsByTitle(t, body, "title")["Full"]["id"].(float64))
	if err := r.ReconcileVideoPeople(ctx, fullID, []repo.PersonRoleName{{Name: "Ada", Role: "actor"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := r.ReconcileVideoStudios(ctx, fullID, []string{"Acme"}, nil); err != nil {
		t.Fatal(err)
	}

	_, body = getJSONTok(t, srv.URL+"/api/v1/people", "secret")
	if c := completenessOf(t, itemsByTitle(t, body, "name")["Ada"]); c != "0/0" {
		t.Errorf("owner person completeness = %s, want 0/0 (no photo; bio/birthdate missing)", c)
	}
	_, body = getJSONTok(t, srv.URL+"/api/v1/people", "")
	if c := completenessOf(t, itemsByTitle(t, body, "name")["Ada"]); c != "absent" {
		t.Errorf("visitor person completeness = %s, want absent", c)
	}

	_, body = getJSONTok(t, srv.URL+"/api/v1/studios?sort=completeness_asc", "secret")
	if c := completenessOf(t, itemsByTitle(t, body, "name")["Acme"]); c != "null/0" {
		t.Errorf("owner studio completeness = %s, want null/0 (no required band; no branding)", c)
	}
	_, body = getJSONTok(t, srv.URL+"/api/v1/studios", "")
	if c := completenessOf(t, itemsByTitle(t, body, "name")["Acme"]); c != "absent" {
		t.Errorf("visitor studio completeness = %s, want absent", c)
	}
}

// F65.7: the facets endpoint reads entity_completeness_missing — person
// counts come from the store and carry the band as criticality.
func TestCompletenessFacets_FromStore(t *testing.T) {
	srv, r := completenessBrowseServerWithRepo(t, "secret")
	ctx := context.Background()

	_, body := getJSONTok(t, srv.URL+"/api/v1/media", "secret")
	fullID := int64(itemsByTitle(t, body, "title")["Full"]["id"].(float64))
	if err := r.ReconcileVideoPeople(ctx, fullID, []repo.PersonRoleName{{Name: "Ada", Role: "actor"}}, nil); err != nil {
		t.Fatal(err)
	}

	_, body = getJSONTok(t, srv.URL+"/api/v1/completeness/facets?entity_type=person", "secret")
	facets, _ := body["facets"].([]any)
	got := map[string]string{}
	for _, f := range facets {
		m, _ := f.(map[string]any)
		got[m["canonical"].(string)] = m["criticality"].(string) + ":" + itoa(int64(m["missing_count"].(float64)))
	}
	want := map[string]string{"photo": "critical:1", "bio": "nice_to_have:1", "birthdate": "nice_to_have:1"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("facet %s = %q, want %q (got %v)", k, got[k], v, got)
		}
	}
	if _, ok := got["nationality"]; ok {
		t.Errorf("optional facet nationality offered by the chip: %v", got)
	}
}

// F65.8 (HOLODEX-435): the ring button re-reads its own bands after firing
// refresh-all. The endpoint drains like a list read (the first owner touch
// here lands the seeded dirty rows), is owner-only, and 404s for an id the
// store does not know.
func TestEntityCompletenessSummary(t *testing.T) {
	srv, r := completenessBrowseServerWithRepo(t, "secret")
	ctx := context.Background()

	// A visitor list read never drains, so it is a safe way to learn the ids.
	_, body := getJSONTok(t, srv.URL+"/api/v1/media", "")
	items := itemsByTitle(t, body, "title")
	idOf := func(title string) string { return itoa(int64(items[title]["id"].(float64))) }
	if dirty, _ := r.DirtyCompleteness(ctx); len(dirty["video"]) != 3 {
		t.Fatalf("dirty before = %v, want the 3 seeded videos", dirty)
	}

	code, got := getJSONTok(t, srv.URL+"/api/v1/media/"+idOf("Bare")+"/completeness", "secret")
	if code != http.StatusOK || completenessOf(t, map[string]any{"completeness": got}) != "25/null" {
		t.Errorf("owner Bare: code %d body %v, want 200 25/null", code, got)
	}
	if dirty, _ := r.DirtyCompleteness(ctx); len(dirty["video"]) != 0 {
		t.Errorf("dirty after the summary read = %v, want empty (the read drains)", dirty)
	}
	if code, _ := getJSONTok(t, srv.URL+"/api/v1/media/"+idOf("Full")+"/completeness", ""); code == http.StatusOK {
		t.Errorf("visitor summary read: want an owner-gate refusal, got 200")
	}
	if code, _ := getJSONTok(t, srv.URL+"/api/v1/media/999999/completeness", "secret"); code != http.StatusNotFound {
		t.Errorf("unknown id: want 404, got %d", code)
	}
}
