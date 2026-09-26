package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"holodex/internal/api"
	"holodex/internal/model"
)

// putStudioHalo PUTs a halo choice for one role and returns the response code.
func putStudioHalo(t *testing.T, srv *httptest.Server, token string, sid int64, role, body string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut,
		srv.URL+"/api/v1/studios/"+itoa(sid)+"/images/"+role+"/halo", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do put halo: %v", err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// studioDetailHalo reads image_halo off GET /studios/{id}.
func studioDetailHalo(t *testing.T, srv *httptest.Server, sid int64) map[string][]string {
	t.Helper()
	resp, err := http.Get(srv.URL + "/api/v1/studios/" + itoa(sid))
	if err != nil {
		t.Fatalf("get studio: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Studio struct {
			ImageHalo map[string][]string `json:"image_halo"`
		} `json:"studio"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.Studio.ImageHalo
}

// TestStudioImageHalo_DefaultOffThenPerModeToggle (HOLODEX-463, ADR-109): no halo is
// on by default; each (role, mode) toggles independently; turning one off leaves the
// other palette's choice alone; the choice survives an image replace.
func TestStudioImageHalo_DefaultOffThenPerModeToggle(t *testing.T) {
	srv, r, sid := studioImageServer(t, "tok")

	if got := studioDetailHalo(t, srv, sid); len(got) != 0 {
		t.Fatalf("default image_halo = %v, want none (off)", got)
	}

	for _, c := range []struct{ role, body string }{
		{model.StudioImageLogo, `{"mode":"dark","on":true}`},
		{model.StudioImageLogo, `{"mode":"light","on":true}`},
		{model.StudioImageIcon, `{"mode":"light","on":true}`},
		{model.StudioImageIcon, `{"mode":"light","on":true}`}, // idempotent
	} {
		if code := putStudioHalo(t, srv, "tok", sid, c.role, c.body); code != http.StatusNoContent {
			t.Fatalf("put %s %s = %d, want 204", c.role, c.body, code)
		}
	}
	want := map[string][]string{"logo": {"dark", "light"}, "icon": {"light"}}
	if got := studioDetailHalo(t, srv, sid); !reflect.DeepEqual(got, want) {
		t.Fatalf("image_halo = %v, want %v", got, want)
	}

	// Off in dark leaves light on.
	if code := putStudioHalo(t, srv, "tok", sid, model.StudioImageLogo, `{"mode":"dark","on":false}`); code != http.StatusNoContent {
		t.Fatalf("put off = %d", code)
	}
	// A replace (delete + insert on studio_images) keeps the choice.
	if code := uploadStudioImage(t, srv, "tok", sid, model.StudioImageLogo, tinyJPEG(t)); code != http.StatusCreated {
		t.Fatalf("upload = %d", code)
	}
	want = map[string][]string{"logo": {"light"}, "icon": {"light"}}
	if got := studioDetailHalo(t, srv, sid); !reflect.DeepEqual(got, want) {
		t.Fatalf("after off + replace image_halo = %v, want %v", got, want)
	}

	// The list read carries it too (StudioListRow / completeness queue source).
	studios, err := r.ListStudios(context.Background(), false)
	if err != nil || len(studios) != 1 {
		t.Fatalf("list: %v", err)
	}
	if !reflect.DeepEqual(studios[0].ImageHalo, want) {
		t.Fatalf("list ImageHalo = %v, want %v", studios[0].ImageHalo, want)
	}
}

// TestStudioImageHalo_Validation — owner-gated, and role/mode/studio are validated.
func TestStudioImageHalo_Validation(t *testing.T) {
	srv, _, sid := studioImageServer(t, "tok")
	ok := `{"mode":"dark","on":true}`
	for _, c := range []struct {
		name, token, role, body string
		sid                     int64
		want                    int
	}{
		{"no token", "", model.StudioImageLogo, ok, sid, http.StatusUnauthorized},
		{"bad role", "tok", "banner", ok, sid, http.StatusBadRequest},
		{"bad mode", "tok", model.StudioImageLogo, `{"mode":"sepia","on":true}`, sid, http.StatusBadRequest},
		{"missing mode", "tok", model.StudioImageLogo, `{"on":true}`, sid, http.StatusBadRequest},
		{"unknown studio", "tok", model.StudioImageLogo, ok, sid + 999, http.StatusNotFound},
	} {
		if code := putStudioHalo(t, srv, c.token, c.sid, c.role, c.body); code != c.want {
			t.Errorf("%s = %d, want %d", c.name, code, c.want)
		}
	}
}
