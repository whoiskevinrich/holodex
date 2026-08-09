package api_test

import (
	"context"
	"net/http"
	"testing"
)

func TestFacetNotApplicableAPI_SetThenClear(t *testing.T) {
	srv, r, id := decisionServer(t, "")
	base := srv.URL + "/api/v1/media/" + itoa(id) + "/fields/external_provider_id/not-applicable"

	if code := sendDecision(t, http.MethodPut, base, "", nil); code != 204 {
		t.Fatalf("mark not-applicable: want 204, got %d", code)
	}
	facets, err := r.FacetsNotApplicableForEntity(context.Background(), "video", id)
	if err != nil || !facets["external_provider_id"] {
		t.Fatalf("want external_provider_id excluded, got %v err=%v", facets, err)
	}

	if code := sendDecision(t, http.MethodDelete, base, "", nil); code != 204 {
		t.Fatalf("clear not-applicable: want 204, got %d", code)
	}
	facets, err = r.FacetsNotApplicableForEntity(context.Background(), "video", id)
	if err != nil || len(facets) != 0 {
		t.Fatalf("want no exclusions after clear, got %v err=%v", facets, err)
	}
}

func TestFacetNotApplicableAPI_Validation(t *testing.T) {
	srv, r, id := decisionServer(t, "")

	// Unknown field → 404.
	unknown := srv.URL + "/api/v1/media/" + itoa(id) + "/fields/nope/not-applicable"
	if code := sendDecision(t, http.MethodPut, unknown, "", nil); code != 404 {
		t.Errorf("unknown field: want 404, got %d", code)
	}

	// Unknown video id → 404.
	badID := srv.URL + "/api/v1/media/99999/fields/external_provider_id/not-applicable"
	if code := sendDecision(t, http.MethodPut, badID, "", nil); code != 404 {
		t.Errorf("unknown id: want 404, got %d", code)
	}

	// Soft-deleted id → 409.
	if err := r.SoftDelete(context.Background(), id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	live := srv.URL + "/api/v1/media/" + itoa(id) + "/fields/external_provider_id/not-applicable"
	if code := sendDecision(t, http.MethodPut, live, "", nil); code != 409 {
		t.Errorf("soft-deleted: want 409, got %d", code)
	}
}

func TestFacetNotApplicableAPI_OwnerGated(t *testing.T) {
	srv, _, id := decisionServer(t, "secret")
	base := srv.URL + "/api/v1/media/" + itoa(id) + "/fields/external_provider_id/not-applicable"

	if code := sendDecision(t, http.MethodPut, base, "", nil); code != 401 {
		t.Errorf("PUT without token: want 401, got %d", code)
	}
	if code := sendDecision(t, http.MethodDelete, base, "", nil); code != 401 {
		t.Errorf("DELETE without token: want 401, got %d", code)
	}
	if code := sendDecision(t, http.MethodPut, base, "secret", nil); code != 204 {
		t.Errorf("PUT with token: want 204, got %d", code)
	}
}
