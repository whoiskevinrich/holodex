package api_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/repo"
)

// authServer builds a server whose admin surface is gated by token (empty = open)
// and bound as exposed/loopback, exercising the F21.7 gate (ADR-030).
func authServer(t *testing.T, token string, exposed bool) (*httptest.Server, *repo.Repo) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetActivity(nil, api.NewHealth(), "test", time.Time{}, true)
	h.SetAuth(api.NewAuth(token), exposed)

	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r
}

// getTok issues GET url with an optional owner token, returning status + decoded body.
func getTok(t *testing.T, url, token string) (int, map[string]any) {
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
	if resp.Header.Get("Content-Type") != "" {
		_ = json.NewDecoder(resp.Body).Decode(&body)
	}
	return resp.StatusCode, body
}

// Open by default: with no token, the admin surface is reachable (single-user).
func TestGateOpenWhenNoToken(t *testing.T) {
	srv, _ := authServer(t, "", false)
	if code, _ := getTok(t, srv.URL+"/api/v1/admin/activity", ""); code != http.StatusOK {
		t.Errorf("no-token activity code = %d, want 200 (open)", code)
	}
}

// Gated: with a token, owner-only routes require the matching header (F21.7 cond. 2).
func TestGateRequiresTokenWhenSet(t *testing.T) {
	srv, _ := authServer(t, "s3cret", false)

	if code, _ := getTok(t, srv.URL+"/api/v1/admin/activity", ""); code != http.StatusUnauthorized {
		t.Errorf("missing-token code = %d, want 401", code)
	}
	if code, _ := getTok(t, srv.URL+"/api/v1/admin/activity", "wrong"); code != http.StatusUnauthorized {
		t.Errorf("wrong-token code = %d, want 401", code)
	}
	if code, _ := getTok(t, srv.URL+"/api/v1/admin/activity", "s3cret"); code != http.StatusOK {
		t.Errorf("correct-token code = %d, want 200", code)
	}
	// A non-admin read endpoint stays public regardless.
	if code, _ := getTok(t, srv.URL+"/api/v1/media", ""); code != http.StatusOK {
		t.Errorf("public media code = %d, want 200", code)
	}
}

// CSRF posture (F21.7 cond. 3): the state-changing control requires the header,
// which a cross-site form cannot set, so a header-less POST is rejected.
func TestGateRejectsHeaderlessControl(t *testing.T) {
	srv, _ := authServer(t, "s3cret", false)
	resp, err := http.Post(srv.URL+"/api/v1/admin/rescan", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("headerless rescan code = %d, want 401", resp.StatusCode)
	}
}

// Capabilities tells the SPA whether it is an owner and whether a token is needed.
func TestCapabilities(t *testing.T) {
	open, _ := authServer(t, "", false)
	_, body := getJSON(t, open.URL+"/api/v1/capabilities")
	if body["owner"] != true || body["auth_required"] != false {
		t.Errorf("open caps = %v, want owner=true auth_required=false", body)
	}

	gated, _ := authServer(t, "s3cret", false)
	_, body = getJSON(t, gated.URL+"/api/v1/capabilities")
	if body["owner"] != false || body["auth_required"] != true {
		t.Errorf("gated (no header) caps = %v, want owner=false auth_required=true", body)
	}
}

// Fail-loud (F21.7 cond. 1): exposed bind + no token surfaces
// controls_unauthenticated=true; loopback or a set token clears it.
func TestControlsUnauthenticatedFlag(t *testing.T) {
	cases := []struct {
		name    string
		token   string
		exposed bool
		want    bool
	}{
		{"exposed, no token", "", true, true},
		{"exposed, token set", "s3cret", true, false},
		{"loopback, no token", "", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := authServer(t, c.token, c.exposed)
			code, body := getTok(t, srv.URL+"/api/v1/admin/activity", c.token)
			if code != http.StatusOK {
				t.Fatalf("activity code = %d, want 200", code)
			}
			sys, _ := body["system"].(map[string]any)
			if sys["controls_unauthenticated"] != c.want {
				t.Errorf("controls_unauthenticated = %v, want %v", sys["controls_unauthenticated"], c.want)
			}
		})
	}
}
