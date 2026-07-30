package api_test

import (
	"net/http"
	"testing"
)

// TestTagParentEndpoint covers POST /tags/{id}/parent (F50, ADR-075 D1):
// gating, validation, the cycle guard's 400, not-found, and the success/clear
// round-trip.
func TestTagParentEndpoint(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	seedTagVideo(t, r, "/m/a.mkv", "Animal")
	seedTagVideo(t, r, "/m/d.mkv", "Dog")
	animal := tagID(t, r, "Animal")
	dog := tagID(t, r, "Dog")
	parentURL := func(id int64) string { return srv.URL + "/api/v1/tags/" + itoa(id) + "/parent" }

	// Gating.
	if code, _ := postTok(t, parentURL(dog), "", map[string]any{"parent_id": animal}); code != http.StatusUnauthorized {
		t.Errorf("no-token = %d, want 401", code)
	}

	// Validation: parent_id <= 0.
	if code, _ := postTok(t, parentURL(dog), "s3cret", map[string]any{"parent_id": 0}); code != http.StatusBadRequest {
		t.Errorf("parent_id=0 = %d, want 400", code)
	}

	// Not found: unknown tag id, unknown parent id.
	if code, _ := postTok(t, parentURL(999999), "s3cret", map[string]any{"parent_id": animal}); code != http.StatusNotFound {
		t.Errorf("unknown tag = %d, want 404", code)
	}
	if code, _ := postTok(t, parentURL(dog), "s3cret", map[string]any{"parent_id": 999999}); code != http.StatusNotFound {
		t.Errorf("unknown parent = %d, want 404", code)
	}

	// Success: Dog's parent becomes Animal.
	code, body := postTok(t, parentURL(dog), "s3cret", map[string]any{"parent_id": animal})
	if code != http.StatusOK {
		t.Fatalf("set parent = %d, want 200", code)
	}
	tag, _ := body["tag"].(map[string]any)
	if pid, _ := tag["parent_tag_id"].(float64); int64(pid) != animal {
		t.Errorf("tag.parent_tag_id = %v, want %d", tag["parent_tag_id"], animal)
	}

	// Cycle: Animal's parent cannot become Dog now that Dog is its child.
	code, body = postTok(t, parentURL(animal), "s3cret", map[string]any{"parent_id": dog})
	if code != http.StatusBadRequest {
		t.Fatalf("cycle = %d, want 400", code)
	}
	if cycle, _ := body["cycle"].(bool); !cycle {
		t.Errorf("cycle body = %v, want {cycle: true}", body)
	}

	// Clear: parent_id: null drops Dog back to root.
	code, body = postTok(t, parentURL(dog), "s3cret", map[string]any{"parent_id": nil})
	if code != http.StatusOK {
		t.Fatalf("clear parent = %d, want 200", code)
	}
	tag, _ = body["tag"].(map[string]any)
	if _, present := tag["parent_tag_id"]; present {
		t.Errorf("tag.parent_tag_id after clear = %v, want absent (omitempty)", tag["parent_tag_id"])
	}
}
