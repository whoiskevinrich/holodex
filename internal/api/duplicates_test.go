package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"holodex/internal/api"
)

// getJSONTok is getJSON with an owner token header, for the gated review-queue reads.
func getJSONTok(t *testing.T, url, token string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

// TestDuplicatesEndpoints covers the review-queue API (F43 S5): owner-gated list +
// dismiss, dismiss removing the pair, and entity_type validation.
func TestDuplicatesEndpoints(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	// Two near-miss tags (same loose key, different nameKey) → one flagged pair.
	seedTagVideo(t, r, "/m/a.mkv", "sci-fi")
	seedTagVideo(t, r, "/m/b.mkv", "scifi")

	list := srv.URL + "/api/v1/owner/duplicates"

	// Gating.
	if code := sendTok(t, http.MethodGet, list, ""); code != http.StatusUnauthorized {
		t.Errorf("no-token list = %d, want 401", code)
	}

	// List returns the pair with both names + counts + variation.
	code, body := getJSONTok(t, list, "s3cret")
	if code != http.StatusOK {
		t.Fatalf("list = %d, want 200", code)
	}
	pairs, _ := body["pairs"].([]any)
	if len(pairs) != 1 {
		t.Fatalf("pairs = %v, want one", body["pairs"])
	}
	pair, _ := pairs[0].(map[string]any)
	a, _ := pair["a"].(map[string]any)
	b, _ := pair["b"].(map[string]any)
	if pair["entity_type"] != "tag" || pair["variation"] != "punctuation" || a["name"] == nil || b["name"] == nil {
		t.Fatalf("pair shape = %v", pair)
	}
	idA := int64(a["id"].(float64))
	idB := int64(b["id"].(float64))

	dismiss := srv.URL + "/api/v1/owner/duplicates/dismiss"

	// Dismiss gating + validation.
	if code, _ := postTok(t, dismiss, "", map[string]any{"entity_type": "tag", "id_a": idA, "id_b": idB}); code != http.StatusUnauthorized {
		t.Errorf("no-token dismiss = %d, want 401", code)
	}
	if code, _ := postTok(t, dismiss, "s3cret", map[string]any{"entity_type": "bogus", "id_a": idA, "id_b": idB}); code != http.StatusBadRequest {
		t.Errorf("bad entity_type = %d, want 400", code)
	}
	if code, _ := postTok(t, dismiss, "s3cret", map[string]any{"entity_type": "tag", "id_a": idA, "id_b": idA}); code != http.StatusBadRequest {
		t.Errorf("same ids = %d, want 400", code)
	}

	// Dismiss removes the pair.
	if code, _ := postTok(t, dismiss, "s3cret", map[string]any{"entity_type": "tag", "id_a": idA, "id_b": idB}); code != http.StatusNoContent {
		t.Fatalf("dismiss = %d, want 204", code)
	}
	_, body = getJSONTok(t, list, "s3cret")
	if pairs, _ := body["pairs"].([]any); len(pairs) != 0 {
		t.Fatalf("after dismiss: %v, want empty", body["pairs"])
	}
}

// TestNearMissEndpoint covers the editor soft-warning lookup (P1-5): owner-gated, and
// a candidate name surfaces the fuzzy look-alike.
func TestNearMissEndpoint(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	seedTagVideo(t, r, "/m/a.mkv", "scifi")
	seedTagVideo(t, r, "/m/b.mkv", "drama")
	drama, _, _ := r.TagIDByName(context.Background(), "drama")
	url := srv.URL + "/api/v1/tags/" + itoa(drama) + "/near-miss?name=sci-fi"

	if code := sendTok(t, http.MethodGet, url, ""); code != http.StatusUnauthorized {
		t.Errorf("no-token near-miss = %d, want 401", code)
	}
	code, body := getJSONTok(t, url, "s3cret")
	if code != http.StatusOK {
		t.Fatalf("near-miss = %d, want 200", code)
	}
	nm, _ := body["near_miss"].(map[string]any)
	if nm == nil || nm["name"] != "scifi" {
		t.Fatalf("near_miss = %v, want scifi", body["near_miss"])
	}
}
