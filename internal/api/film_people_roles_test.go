package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"holodex/internal/api"
	"holodex/internal/repo"
)

// filmPut issues a PUT with a JSON body, mirroring filmPost/filmDelete (films_test.go).
func filmPut(t *testing.T, srv *httptest.Server, token, path string, body any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/api/v1"+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do put %s: %v", path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// TestFilmPeopleRolesCRUD covers the owner-gated add/edit/remove endpoints
// (HOLODEX-281) and confirms getFilm's response separates the read-only inherited
// "cast" from the owner-entered "credited_roles".
func TestFilmPeopleRolesCRUD(t *testing.T) {
	srv, r, v1, _ := filmServer(t, "tok")
	ctx := context.Background()

	filmID, err := r.CreateFilm(ctx, "Roles API Test", 2024)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, filmID, v1, nil, false); err != nil {
		t.Fatalf("attach video: %v", err)
	}
	// Two people, so there's an uncredited-but-real person id distinct from any
	// video/film id -- video/people ids are independent autoincrement sequences and
	// can collide numerically, so an "uncredited" check must use a real person id,
	// not a stand-in like the video id.
	linkPeople(t, r, v1, "Cast Member", "Uncredited Person")
	personID, ok, err := r.PersonIDByName(ctx, "Cast Member")
	if !ok || err != nil {
		t.Fatalf("person id for Cast Member: ok=%v err=%v", ok, err)
	}
	uncreditedID, ok, err := r.PersonIDByName(ctx, "Uncredited Person")
	if !ok || err != nil {
		t.Fatalf("person id for Uncredited Person: ok=%v err=%v", ok, err)
	}

	type filmDetail struct {
		Cast          []struct {
			ID int64 `json:"id"`
		} `json:"cast"`
		CreditedRoles []repo.FilmPersonRole `json:"credited_roles"`
	}
	getDetail := func() filmDetail {
		t.Helper()
		resp, err := http.Get(srv.URL + "/api/v1/films/" + itoa(filmID))
		if err != nil {
			t.Fatalf("get film: %v", err)
		}
		defer resp.Body.Close()
		var d filmDetail
		if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
			t.Fatalf("decode film detail: %v", err)
		}
		return d
	}

	// Before crediting, cast shows both inherited people but credited_roles is empty.
	before := getDetail()
	if len(before.Cast) != 2 {
		t.Fatalf("cast before credit: got %+v, want 2", before.Cast)
	}
	if len(before.CreditedRoles) != 0 {
		t.Fatalf("credited_roles before credit: got %+v, want empty", before.CreditedRoles)
	}

	// Unauthenticated add is rejected.
	unauthResp := filmPost(t, srv, "", "/films/"+itoa(filmID)+"/roles", map[string]any{"person_id": personID, "role": "Actor"})
	if unauthResp.StatusCode != http.StatusUnauthorized && unauthResp.StatusCode != http.StatusForbidden {
		t.Fatalf("unauthenticated add role: got %d, want 401/403", unauthResp.StatusCode)
	}

	// Add.
	addResp := filmPost(t, srv, "tok", "/films/"+itoa(filmID)+"/roles", map[string]any{
		"person_id": personID, "role": "Actor", "billing_order": 1,
	})
	if addResp.StatusCode != http.StatusNoContent {
		t.Fatalf("add role: got %d, want 204", addResp.StatusCode)
	}

	after := getDetail()
	if len(after.CreditedRoles) != 1 || after.CreditedRoles[0].PersonID != personID ||
		after.CreditedRoles[0].Role != "Actor" || after.CreditedRoles[0].BillingOrder == nil || *after.CreditedRoles[0].BillingOrder != 1 {
		t.Fatalf("credited_roles after add: got %+v", after.CreditedRoles)
	}
	// The inherited cast field is untouched by crediting.
	if len(after.Cast) != 2 {
		t.Fatalf("cast after credit: got %+v, want unchanged (2)", after.Cast)
	}

	// Re-adding the same person is a 409.
	reAdd := filmPost(t, srv, "tok", "/films/"+itoa(filmID)+"/roles", map[string]any{"person_id": personID, "role": "Producer"})
	if reAdd.StatusCode != http.StatusConflict {
		t.Fatalf("re-add same person: got %d, want 409", reAdd.StatusCode)
	}

	// Adding an unknown person 404s.
	unknownResp := filmPost(t, srv, "tok", "/films/"+itoa(filmID)+"/roles", map[string]any{"person_id": int64(999999), "role": "Actor"})
	if unknownResp.StatusCode != http.StatusNotFound {
		t.Fatalf("add unknown person: got %d, want 404", unknownResp.StatusCode)
	}

	// Edit.
	editResp := filmPut(t, srv, "tok", "/films/"+itoa(filmID)+"/roles/"+itoa(personID), map[string]any{
		"role": "Lead Actor", "billing_order": 2,
	})
	if editResp.StatusCode != http.StatusNoContent {
		t.Fatalf("edit role: got %d, want 204", editResp.StatusCode)
	}
	edited := getDetail()
	if len(edited.CreditedRoles) != 1 || edited.CreditedRoles[0].Role != "Lead Actor" ||
		edited.CreditedRoles[0].BillingOrder == nil || *edited.CreditedRoles[0].BillingOrder != 2 {
		t.Fatalf("credited_roles after edit: got %+v", edited.CreditedRoles)
	}

	// Editing an uncredited person 404s.
	editMissing := filmPut(t, srv, "tok", "/films/"+itoa(filmID)+"/roles/"+itoa(uncreditedID), map[string]any{"role": "X"})
	if editMissing.StatusCode != http.StatusNotFound {
		t.Fatalf("edit uncredited person: got %d, want 404", editMissing.StatusCode)
	}

	// Remove.
	removeResp := filmDelete(t, srv, "tok", "/films/"+itoa(filmID)+"/roles/"+itoa(personID))
	if removeResp.StatusCode != http.StatusNoContent {
		t.Fatalf("remove role: got %d, want 204", removeResp.StatusCode)
	}
	removed := getDetail()
	if len(removed.CreditedRoles) != 0 {
		t.Fatalf("credited_roles after remove: got %+v, want empty", removed.CreditedRoles)
	}
	// Cast (inherited) is still unaffected.
	if len(removed.Cast) != 2 {
		t.Fatalf("cast after remove: got %+v, want unchanged (2)", removed.Cast)
	}

	// Re-removing is a 404 (not idempotent).
	reRemove := filmDelete(t, srv, "tok", "/films/"+itoa(filmID)+"/roles/"+itoa(personID))
	if reRemove.StatusCode != http.StatusNotFound {
		t.Fatalf("re-remove role: got %d, want 404", reRemove.StatusCode)
	}
}
