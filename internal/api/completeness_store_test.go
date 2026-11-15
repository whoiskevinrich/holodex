package api_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// F65 (ADR-099 D3–D4): the store behind the queue and missing_facet, after the score was
// retired (F90 P0-6). These tests drive the public HTTP surface against
// completenessBrowseServer's Bare / Half / Full fixture, which has 3 / 2 / 0 of the four
// mapped critical facets missing.

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

func storedMissing(t *testing.T, r *repo.Repo, id int64) []string {
	t.Helper()
	got, err := r.StoredMissing(context.Background(), model.EnrichEntityVideo, id)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, m := range got {
		out = append(out, m.Canonical)
	}
	return out
}

// F90 P0-6 (§42.3 Q3, §42.5 A8): no list item carries a completeness summary for anyone, and
// a retired Completeness sort in a link or saved preference reads as the list's default sort
// with no error, for the owner and a visitor alike.
func TestCompletenessScoreIsRetired(t *testing.T) {
	srv, _ := completenessBrowseServerWithRepo(t, "secret")

	for _, tok := range []string{"secret", ""} {
		for _, path := range []string{"media", "people", "studios"} {
			_, body := getJSONTok(t, srv.URL+"/api/v1/"+path, tok)
			for _, it := range body["items"].([]any) {
				if _, ok := it.(map[string]any)["completeness"]; ok {
					t.Errorf("%s item carries completeness (owner=%v)", path, tok != "")
				}
			}
		}
		_, def := getJSONTok(t, srv.URL+"/api/v1/media", tok)
		for _, sort := range []string{"completeness_asc", "completeness_desc"} {
			code, body := getJSONTok(t, srv.URL+"/api/v1/media?sort="+sort, tok)
			if code != http.StatusOK || !slices.Equal(mediaTitles(t, body), mediaTitles(t, def)) {
				t.Errorf("media?sort=%s (owner=%v) = %d %v, want 200 in the default order %v", sort, tok != "", code, mediaTitles(t, body), mediaTitles(t, def))
			}
			for _, path := range []string{"people", "studios", "media/ids"} {
				if code, _ := getJSONTok(t, srv.URL+"/api/v1/"+path+"?sort="+sort, tok); code != http.StatusOK {
					t.Errorf("%s?sort=%s (owner=%v) = %d, want 200", path, sort, tok != "", code)
				}
			}
		}
	}
	// The ring's one-entity read went with the ring.
	_, body := getJSONTok(t, srv.URL+"/api/v1/media", "secret")
	id := itoa(int64(itemsByTitle(t, body, "title")["Bare"]["id"].(float64)))
	if code, _ := getJSONTok(t, srv.URL+"/api/v1/media/"+id+"/completeness", "secret"); code == http.StatusOK {
		t.Error("GET /media/{id}/completeness still answers")
	}
	// A8: the detail page keeps its field list and carries no score.
	_, detail := getJSONTok(t, srv.URL+"/api/v1/media/"+id, "secret")
	c, _ := detail["completeness"].(map[string]any)
	if c == nil || c["facets"] == nil {
		t.Fatalf("detail completeness = %v, want the facet list", detail["completeness"])
	}
	for _, k := range []string{"score", "extras", "actionability"} {
		if _, ok := c[k]; ok {
			t.Errorf("detail completeness still carries %q", k)
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
	if got := storedMissing(t, r, before["video"][0]); len(got) != 0 {
		t.Errorf("store after visitor reads = %v, want nothing written", got)
	}

	getJSONTok(t, srv.URL+"/api/v1/media", "secret")
	if after, _ := r.DirtyCompleteness(ctx); len(after["video"]) != 0 {
		t.Errorf("dirty after the first owner read = %v, want empty", after)
	}
}

// F65.6: a mutation on an input table (here the enrichment shadow store)
// reaches the store on the next owner read with no restart and no backfill —
// the trigger dirties the row, the list read drains it.
func TestListMedia_StoreRecomputesAfterMutation(t *testing.T) {
	srv, r := completenessBrowseServerWithRepo(t, "secret")
	ctx := context.Background()

	_, body := getJSONTok(t, srv.URL+"/api/v1/media", "secret") // fills the store
	halfID := int64(itemsByTitle(t, body, "title")["Half"]["id"].(float64))
	if got := storedMissing(t, r, halfID); !slices.Contains(got, "poster_url") {
		t.Fatalf("Half missing before = %v, want poster_url among them", got)
	}

	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, halfID, "tmdb", "ext-2", map[string][]string{
		"poster_url": {"https://cdn.example/half.jpg"},
	}); err != nil {
		t.Fatal(err)
	}
	getJSONTok(t, srv.URL+"/api/v1/media", "secret")
	if got := storedMissing(t, r, halfID); slices.Contains(got, "poster_url") {
		t.Errorf("Half missing after poster enrichment = %v, want poster_url gone", got)
	}
}

// ADR-099 D3: the detail page computes live and rewrites stored missing facets that
// disagree with it — a stale queue row cannot outlive a look at the entity.
func TestGetMedia_SelfHealsStoredCompleteness(t *testing.T) {
	srv, r := completenessBrowseServerWithRepo(t, "secret")
	ctx := context.Background()

	_, body := getJSONTok(t, srv.URL+"/api/v1/media", "secret")
	fullID := int64(itemsByTitle(t, body, "title")["Full"]["id"].(float64))

	// Corrupt the cached row behind the triggers' back (no input table changes,
	// so nothing is dirty and a list read would serve it as-is).
	if err := r.StoreCompleteness(ctx, repo.CompletenessRow{
		EntityType: model.EnrichEntityVideo, EntityID: fullID,
		Missing: []repo.MissingFacet{{Canonical: "title", Band: "critical"}},
	}); err != nil {
		t.Fatal(err)
	}
	if code, _ := getJSONTok(t, srv.URL+"/api/v1/media/"+itoa(fullID), "secret"); code != http.StatusOK {
		t.Fatalf("detail: want 200, got %d", code)
	}
	if got := storedMissing(t, r, fullID); len(got) != 0 {
		t.Errorf("stored after detail view = %v, want none (self-healed)", got)
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
