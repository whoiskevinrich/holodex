package api_test

import (
	"net/http"
	"testing"
)

// TestTagDenylistEndpoints covers the owner-gated deny-list management API
// (F50, ADR-075 D2): gating, add/list/remove, and validation.
func TestTagDenylistEndpoints(t *testing.T) {
	srv, _ := identityServer(t, "s3cret")
	list := srv.URL + "/api/v1/owner/tags/denylist"

	// Gating.
	if code := sendTok(t, http.MethodGet, list, ""); code != http.StatusUnauthorized {
		t.Errorf("no-token list = %d, want 401", code)
	}
	if code, _ := postTok(t, list, "", map[string]any{"term": "gnome"}); code != http.StatusUnauthorized {
		t.Errorf("no-token deny = %d, want 401", code)
	}

	// Validation: empty term.
	if code, _ := postTok(t, list, "s3cret", map[string]any{"term": "  "}); code != http.StatusBadRequest {
		t.Errorf("empty term = %d, want 400", code)
	}

	// Add, then list reflects it. The response reports whether the term also
	// names a live tag (F50 S8) — none seeded here, so false.
	code, denyBody := postTok(t, list, "s3cret", map[string]any{"term": "gnome"})
	if code != http.StatusOK {
		t.Fatalf("deny = %d, want 200", code)
	}
	if existing, _ := denyBody["existing_tag"].(bool); existing {
		t.Errorf("deny existing_tag = %v, want false (no live tag named gnome)", existing)
	}
	code, body := getJSONTok(t, list, "s3cret")
	if code != http.StatusOK {
		t.Fatalf("list = %d, want 200", code)
	}
	terms, _ := body["terms"].([]any)
	if len(terms) != 1 {
		t.Fatalf("terms = %v, want one", body["terms"])
	}
	term, _ := terms[0].(map[string]any)
	if term["term"] != "gnome" {
		t.Fatalf("term shape = %v", term)
	}

	// Remove: gating, then not-found for a never-denied term, then success
	// (case-insensitive, matching the deny-list's fold). term is a query param
	// (not a path segment), so it round-trips terms containing "/" cleanly.
	if code := sendTok(t, http.MethodDelete, list+"?term=gnome", ""); code != http.StatusUnauthorized {
		t.Errorf("no-token remove = %d, want 401", code)
	}
	if code := sendTok(t, http.MethodDelete, list+"?term=never-denied", "s3cret"); code != http.StatusNotFound {
		t.Errorf("remove never-denied = %d, want 404", code)
	}
	if code := sendTok(t, http.MethodDelete, list+"?term=GNOME", "s3cret"); code != http.StatusNoContent {
		t.Errorf("remove (case-insensitive) = %d, want 204", code)
	}
	_, body = getJSONTok(t, list, "s3cret")
	if terms, _ := body["terms"].([]any); len(terms) != 0 {
		t.Fatalf("after remove: %v, want empty", body["terms"])
	}
}
