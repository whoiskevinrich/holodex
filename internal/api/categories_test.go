package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
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
// ADR-077): gating, validation, cross-table collision (both directions, D3),
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

	// Cross-table collision (ADR-077 D3): a category can't take an existing
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
