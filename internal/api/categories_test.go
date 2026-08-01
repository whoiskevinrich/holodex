package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"holodex/internal/api"
)

// reqTokBody sends method with a JSON body and an optional owner token,
// returning the status and decoded body — the DELETE-with-body shape
// unassignCategoryTags needs, which postTok (fixed POST) and sendTok (no
// body) don't cover.
func reqTokBody(t *testing.T, method, url, token string, body any) (int, map[string]any) {
	t.Helper()
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// TestCategoryEndpoints covers the owner-gated category CRUD API (HOLODEX-240,
// ADR-078): gating, validation, cross-table collision (both directions, D3),
// same-table collision, rename, and delete's cascade (D2) — plus the public
// list/detail reads.
func TestCategoryEndpoints(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	seedTagVideo(t, r, "/m/a.mkv", "Sci Fi")
	categoriesURL := srv.URL + "/api/v1/categories"

	// Gating: create is owner-only; list/detail are public.
	if code, _ := postTok(t, categoriesURL, "", map[string]any{"name": "Holiday"}); code != http.StatusUnauthorized {
		t.Errorf("no-token create = %d, want 401", code)
	}
	if code := sendTok(t, http.MethodGet, categoriesURL, ""); code != http.StatusOK {
		t.Errorf("no-token list = %d, want 200", code)
	}

	// Validation: empty name.
	if code, _ := postTok(t, categoriesURL, "s3cret", map[string]any{"name": "  "}); code != http.StatusBadRequest {
		t.Errorf("empty name = %d, want 400", code)
	}

	// Cross-table collision (ADR-078 D3): a category can't take an existing
	// tag's name (fold-equivalent, "SciFi" == "Sci Fi").
	if code, body := postTok(t, categoriesURL, "s3cret", map[string]any{"name": "SciFi"}); code != http.StatusConflict {
		t.Errorf("category colliding with tag = %d %v, want 409", code, body)
	}

	// Create.
	code, body := postTok(t, categoriesURL, "s3cret", map[string]any{"name": "Holiday"})
	if code != http.StatusOK {
		t.Fatalf("create = %d, want 200", code)
	}
	cat, _ := body["category"].(map[string]any)
	catID := int64(cat["id"].(float64))
	if cat["name"] != "Holiday" {
		t.Errorf("created category = %v", cat)
	}

	// Same-table collision.
	if code, _ := postTok(t, categoriesURL, "s3cret", map[string]any{"name": "holiday"}); code != http.StatusConflict {
		t.Errorf("duplicate category (case-fold) = %d, want 409", code)
	}

	// The symmetric direction: attaching a tag named "Holiday" to a video
	// now collides with the category.
	attachURL := srv.URL + "/api/v1/media/" + itoa(seedVideo(t, r, "/m/collision.mkv", "V")) + "/tags"
	if code, _ := postTok(t, attachURL, "s3cret", map[string]any{"name": "Holiday"}); code != http.StatusConflict {
		t.Errorf("tag colliding with category = %d, want 409", code)
	}

	// Detail read.
	detailURL := categoriesURL + "/" + itoa(catID)
	code, body = getJSONTok(t, detailURL, "")
	if code != http.StatusOK {
		t.Fatalf("get = %d, want 200", code)
	}
	cat, _ = body["category"].(map[string]any)
	if tags, _ := cat["tags"].([]any); len(tags) != 0 {
		t.Errorf("new category tags = %v, want empty", tags)
	}
	if code := sendTok(t, http.MethodGet, categoriesURL+"/999999", ""); code != http.StatusNotFound {
		t.Errorf("get unknown = %d, want 404", code)
	}

	// Rename: gating, not-found, collision, success.
	renameURL := detailURL + "/rename"
	if code, _ := postTok(t, renameURL, "", map[string]any{"name": "Holidays"}); code != http.StatusUnauthorized {
		t.Errorf("no-token rename = %d, want 401", code)
	}
	if code, _ := postTok(t, categoriesURL+"/999999/rename", "s3cret", map[string]any{"name": "X"}); code != http.StatusNotFound {
		t.Errorf("rename unknown = %d, want 404", code)
	}
	code, body = postTok(t, renameURL, "s3cret", map[string]any{"name": "Holidays"})
	if code != http.StatusOK {
		t.Fatalf("rename = %d, want 200", code)
	}
	cat, _ = body["category"].(map[string]any)
	if cat["name"] != "Holidays" {
		t.Errorf("renamed category = %v", cat)
	}

	// Assign/unassign a tag: gating, then a full round trip.
	tagsURL := detailURL + "/tags"
	scifi := tagID(t, r, "Sci Fi")
	if code := sendTok(t, http.MethodDelete, tagsURL, ""); code != http.StatusUnauthorized {
		t.Errorf("no-token unassign = %d, want 401", code)
	}
	code, body = postTok(t, tagsURL, "s3cret", map[string]any{"tag_ids": []int64{scifi}})
	if code != http.StatusOK {
		t.Fatalf("assign = %d, want 200", code)
	}
	cat, _ = body["category"].(map[string]any)
	if tags, _ := cat["tags"].([]any); len(tags) != 1 {
		t.Fatalf("category after assign = %v, want 1 member tag", cat["tags"])
	}

	// The list endpoint's tag_count/tag_ids fields (S5 addition, HOLODEX-240) —
	// the /tags pill's count badge and the "Remove from category…" picker's
	// client-side membership filter both read these off GET /categories
	// directly, not the per-category detail read.
	code, listBody := getJSONTok(t, categoriesURL, "")
	if code != http.StatusOK {
		t.Fatalf("list = %d, want 200", code)
	}
	items, _ := listBody["items"].([]any)
	var listed map[string]any
	for _, it := range items {
		if m, _ := it.(map[string]any); int64(m["id"].(float64)) == catID {
			listed = m
		}
	}
	if listed == nil {
		t.Fatalf("list missing category %d: %v", catID, items)
	}
	if tc, _ := listed["tag_count"].(float64); tc != 1 {
		t.Errorf("list tag_count = %v, want 1", listed["tag_count"])
	}
	if ids, _ := listed["tag_ids"].([]any); len(ids) != 1 || int64(ids[0].(float64)) != scifi {
		t.Errorf("list tag_ids = %v, want [%d]", listed["tag_ids"], scifi)
	}

	// Idempotent re-assign.
	if code, _ := postTok(t, tagsURL, "s3cret", map[string]any{"tag_ids": []int64{scifi}}); code != http.StatusOK {
		t.Errorf("re-assign = %d, want 200", code)
	}
	code, body = reqTokBody(t, http.MethodDelete, tagsURL, "s3cret", map[string]any{"tag_ids": []int64{scifi}})
	if code != http.StatusOK {
		t.Fatalf("unassign = %d, want 200", code)
	}
	cat, _ = body["category"].(map[string]any)
	if tags, _ := cat["tags"].([]any); len(tags) != 0 {
		t.Errorf("category after unassign = %v, want empty", cat["tags"])
	}

	// Delete: gating, cascade (the tag survives), not-found on repeat.
	if code := sendTok(t, http.MethodDelete, detailURL, ""); code != http.StatusUnauthorized {
		t.Errorf("no-token delete = %d, want 401", code)
	}
	if code := sendTok(t, http.MethodDelete, detailURL, "s3cret"); code != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", code)
	}
	if code := sendTok(t, http.MethodDelete, detailURL, "s3cret"); code != http.StatusNotFound {
		t.Errorf("delete already-deleted = %d, want 404", code)
	}
	if code, body := getJSONTok(t, srv.URL+"/api/v1/tags/"+itoa(scifi), ""); code != http.StatusOK {
		t.Errorf("tag survives category delete = %d %v, want 200", code, body)
	}
}

// TestResolveOrCreateTagEndpoint covers POST /tags (HOLODEX-240, ADR-078):
// owner-gating, success with no video attach, idempotent resolve, and the
// three error mappings resolveOrCreateByName's tag-creation choke point
// shares with attachVideoTag (422 denied, 400 too long, 409 category
// collision) — createOrResolveTag routes through writeTagResolveError, the
// same translator attachVideoTag uses.
func TestResolveOrCreateTagEndpoint(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	tagsURL := srv.URL + "/api/v1/tags"

	if code, _ := postTok(t, tagsURL, "", map[string]any{"name": "Documentary"}); code != http.StatusUnauthorized {
		t.Errorf("no-token create = %d, want 401", code)
	}

	code, body := postTok(t, tagsURL, "s3cret", map[string]any{"name": "Documentary"})
	if code != http.StatusOK {
		t.Fatalf("create = %d, want 200", code)
	}
	tag, _ := body["tag"].(map[string]any)
	if tag["name"] != "documentary" {
		t.Errorf("created tag = %v", tag)
	}
	tagID1 := int64(tag["id"].(float64))

	// No video attach: the tag exists (GET /tags/{id}, a direct-by-id read)
	// and GET /tags (the plain list) now includes a zero-video tag too
	// (HOLODEX-243: ListTags left-joins video_tags instead of inner-joining),
	// so a bare tag created via the /tags "+ New" affordance shows up
	// immediately rather than being invisible until some video is tagged.
	code, detail := getJSONTok(t, tagsURL+"/"+itoa(tagID1), "")
	if code != http.StatusOK {
		t.Fatalf("get created tag = %d, want 200", code)
	}
	if got, _ := detail["tag"].(map[string]any); got["name"] != "documentary" {
		t.Errorf("created tag detail = %v", got)
	}
	code, listBody := getJSONTok(t, tagsURL, "")
	if code != http.StatusOK {
		t.Fatalf("list tags = %d", code)
	}
	items, _ := listBody["items"].([]any)
	if len(items) != 1 {
		t.Errorf("list tags with only a zero-video tag = %v, want one item", items)
	} else if first, _ := items[0].(map[string]any); first["name"] != "documentary" || first["video_count"] != nil {
		// video_count is omitempty on the JSON model, so a zero count is absent, not 0.
		t.Errorf("list tags item = %v, want name=documentary with no video_count", first)
	}

	// Idempotent: resolving the same (case/whitespace-variant) name again
	// returns the same tag, not a duplicate.
	code, body = postTok(t, tagsURL, "s3cret", map[string]any{"name": "  documentary "})
	if code != http.StatusOK {
		t.Fatalf("re-resolve = %d, want 200", code)
	}
	tag, _ = body["tag"].(map[string]any)
	if int64(tag["id"].(float64)) != tagID1 {
		t.Errorf("re-resolve id = %v, want %d (same tag)", tag["id"], tagID1)
	}

	// Too long.
	if code, _ := postTok(t, tagsURL, "s3cret", map[string]any{"name": strings.Repeat("a", 201)}); code != http.StatusBadRequest {
		t.Errorf("over-long name = %d, want 400", code)
	}

	// Denied.
	if _, err := r.DenyTag(context.Background(), "Gnome"); err != nil {
		t.Fatalf("deny: %v", err)
	}
	if code, _ := postTok(t, tagsURL, "s3cret", map[string]any{"name": "Gnome"}); code != http.StatusUnprocessableEntity {
		t.Errorf("denied term = %d, want 422", code)
	}

	// Category collision (ADR-078 D3).
	if code, _ := postTok(t, srv.URL+"/api/v1/categories", "s3cret", map[string]any{"name": "Holiday"}); code != http.StatusOK {
		t.Fatalf("seed category")
	}
	if code, _ := postTok(t, tagsURL, "s3cret", map[string]any{"name": "Holiday"}); code != http.StatusConflict {
		t.Errorf("category-colliding name = %d, want 409", code)
	}
}
