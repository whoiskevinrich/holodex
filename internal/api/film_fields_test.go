package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFilmFieldDecision covers the film per-field decision endpoint (F56, mirrors
// studio_fields_test.go): set a manual decision on description, confirm it
// resolves, clear it, pin that a name decision is display-only, and confirm the
// unknown-field/auth guards.
func TestFilmFieldDecision(t *testing.T) {
	srv, r, _, _ := filmServer(t, "tok")
	id, err := r.CreateFilm(t.Context(), "Decision Test", 2019)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	base := srv.URL + "/api/v1/films/" + itoa(id) + "/fields/"

	if code := sendDecision(t, http.MethodPut, base+"description/decision", "tok",
		map[string]string{"source": "manual", "manual_value": "Owner's own summary."}); code != 204 {
		t.Fatalf("set manual decision: got %d, want 204", code)
	}

	resp, err := http.Get(srv.URL + "/api/v1/films/" + itoa(id))
	if err != nil {
		t.Fatalf("get film: %v", err)
	}
	defer resp.Body.Close()
	var detail struct {
		Resolved []map[string]any `json:"resolved"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode film detail: %v", err)
	}
	var found bool
	for _, f := range detail.Resolved {
		if f["canonical"] == "description" {
			found = true
			values, _ := f["values"].([]any)
			if len(values) != 1 || values[0] != "Owner's own summary." {
				t.Fatalf("description resolved values: got %v, want [manual value]", f["values"])
			}
		}
	}
	if !found {
		t.Fatalf("description field missing from resolved[]: %+v", detail.Resolved)
	}

	if code := sendDecision(t, http.MethodDelete, base+"description/decision", "tok", nil); code != 204 {
		t.Fatalf("clear decision: got %d, want 204", code)
	}

	// name pins too (F60 RD9, HOLODEX-378): the decision is the display spelling.
	// The canonical column — half the (name, year) identity key — is untouched.
	if code := sendDecision(t, http.MethodPut, base+"name/decision", "tok",
		map[string]string{"source": "manual", "manual_value": "New Name"}); code != 204 {
		t.Fatalf("decide name: got %d, want 204", code)
	}
	if got, err := r.GetFilm(t.Context(), id); err != nil || got.Name != "Decision Test" {
		t.Fatalf("film.name after display decision = %q (%v), want canonical \"Decision Test\"", got.Name, err)
	}
	if v := filmResolvedValue(t, srv, id, "name"); v != "New Name" {
		t.Errorf("resolved name = %q, want the display spelling", v)
	}
	if code := sendDecision(t, http.MethodDelete, base+"name/decision", "tok", nil); code != 204 {
		t.Fatalf("clear name decision: got %d, want 204", code)
	}
	if v := filmResolvedValue(t, srv, id, "name"); v != "Decision Test" {
		t.Errorf("resolved name after clear = %q, want canonical", v)
	}
	// Unknown field.
	if code := sendDecision(t, http.MethodPut, base+"nope/decision", "tok",
		map[string]string{"source": "manual", "manual_value": "x"}); code != 404 {
		t.Fatalf("decide unknown field: got %d, want 404", code)
	}
	// Unauthenticated.
	if code := sendDecision(t, http.MethodPut, base+"description/decision", "",
		map[string]string{"source": "manual", "manual_value": "x"}); code != 401 && code != 403 {
		t.Fatalf("unauthenticated decide: got %d, want 401/403", code)
	}
	// Unknown film id.
	if code := sendDecision(t, http.MethodPut, srv.URL+"/api/v1/films/99999/fields/description/decision", "tok",
		map[string]string{"source": "manual", "manual_value": "x"}); code != 404 {
		t.Fatalf("decide on unknown film: got %d, want 404", code)
	}
}

// filmResolvedValue is GET /films/{id}'s resolved[] first value for one canonical
// ("" when the field is absent).
func filmResolvedValue(t *testing.T, srv *httptest.Server, id int64, canonical string) string {
	t.Helper()
	resp, err := http.Get(srv.URL + "/api/v1/films/" + itoa(id))
	if err != nil {
		t.Fatalf("get film: %v", err)
	}
	defer resp.Body.Close()
	var detail struct {
		Resolved []struct {
			Canonical string   `json:"canonical"`
			Values    []string `json:"values"`
		} `json:"resolved"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode film: %v", err)
	}
	for _, f := range detail.Resolved {
		if f.Canonical == canonical && len(f.Values) > 0 {
			return f.Values[0]
		}
	}
	return ""
}
