package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"holodex/internal/repo"
)

// TestVideoTagEndpoints covers POST/DELETE /media/{id}/tags (F50, ADR-075 P0-7):
// gating, validation, the deny-list's 422, the item-11 length cap's 400, not-found,
// and the attach/detach round-trip.
func TestVideoTagEndpoints(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	seedTagVideo(t, r, "/m/a.mkv", "Existing")
	videos, _, err := r.ListVideos(context.Background(), repo.VideoFilter{})
	if err != nil || len(videos) != 1 {
		t.Fatalf("list videos: %v, %d", err, len(videos))
	}
	base := srv.URL + "/api/v1/media/" + itoa(videos[0].ID) + "/tags"

	// Gating.
	if code, _ := postTok(t, base, "", map[string]any{"name": "Action"}); code != http.StatusUnauthorized {
		t.Errorf("no-token attach = %d, want 401", code)
	}

	// Validation: empty name.
	if code, _ := postTok(t, base, "s3cret", map[string]any{"name": "  "}); code != http.StatusBadRequest {
		t.Errorf("empty name = %d, want 400", code)
	}

	// Length cap (item 11).
	if code, _ := postTok(t, base, "s3cret", map[string]any{"name": strings.Repeat("a", 201)}); code != http.StatusBadRequest {
		t.Errorf("over-long name = %d, want 400", code)
	}

	// Deny-list 422.
	if code, _ := postTok(t, srv.URL+"/api/v1/owner/tags/denylist", "s3cret", map[string]any{"term": "Gnome"}); code != http.StatusOK {
		t.Fatalf("deny gnome = %d, want 200", code)
	}
	if code, _ := postTok(t, base, "s3cret", map[string]any{"name": "Gnome"}); code != http.StatusUnprocessableEntity {
		t.Errorf("attach denied term = %d, want 422", code)
	}

	// Not found: unknown video.
	if code, _ := postTok(t, srv.URL+"/api/v1/media/999999/tags", "s3cret", map[string]any{"name": "Action"}); code != http.StatusNotFound {
		t.Errorf("attach to unknown video = %d, want 404", code)
	}

	// Success: attach a new tag.
	code, body := postTok(t, base, "s3cret", map[string]any{"name": "Action"})
	if code != http.StatusOK {
		t.Fatalf("attach = %d, want 200", code)
	}
	tag, _ := body["tag"].(map[string]any)
	tagID64, _ := tag["id"].(float64)
	if tag["name"] != "Action" || tagID64 == 0 {
		t.Fatalf("attach body = %v", body)
	}
	detachURL := base + "/" + itoa(int64(tagID64))

	// Detach gating.
	if code := sendTok(t, http.MethodDelete, detachURL, ""); code != http.StatusUnauthorized {
		t.Errorf("no-token detach = %d, want 401", code)
	}

	// Detach success.
	if code := sendTok(t, http.MethodDelete, detachURL, "s3cret"); code != http.StatusNoContent {
		t.Errorf("detach = %d, want 204", code)
	}

	// Detach again: not attached.
	if code := sendTok(t, http.MethodDelete, detachURL, "s3cret"); code != http.StatusNotFound {
		t.Errorf("detach again = %d, want 404", code)
	}
}
