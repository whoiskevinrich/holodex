package api_test

import (
	"bytes"
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
	"holodex/internal/theme"
)

// themedServer is authServer with the Handlers exposed, so a test can wire a palette
// the way main.go does (SetCustomTheme) before hitting the routes.
func themedServer(t *testing.T, token string) (*httptest.Server, *api.Handlers) {
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
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, h
}

// putTok issues PUT url with a JSON body and an optional owner token.
func putTok(t *testing.T, url, token string, body any) (int, map[string]any) {
	t.Helper()
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func themeOf(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	th, ok := body["theme"].(map[string]any)
	if !ok {
		t.Fatalf("no theme object in %v", body)
	}
	return th
}

// With no row stored the instance skin is the default, for owner and visitor alike,
// and custom is null while no palette is configured (spec R3, RD10).
func TestCapabilitiesThemeDefault(t *testing.T) {
	srv, _ := authServer(t, "tok", false)
	for _, tok := range []string{"", "tok"} {
		_, body := getTok(t, srv.URL+"/api/v1/capabilities", tok)
		th := themeOf(t, body)
		if th["active"] != api.ThemeDefault || th["custom"] != nil {
			t.Errorf("token %q: theme = %v, want active=%s custom=null", tok, th, api.ThemeDefault)
		}
	}
}

// PUT /admin/theme is owner-only and validates the id (spec R2); the stored value
// is what every viewer then receives (R3).
func TestSetTheme(t *testing.T) {
	srv, _ := authServer(t, "tok", false)
	url := srv.URL + "/api/v1/admin/theme"

	if code, _ := putTok(t, url, "", map[string]string{"theme": "broadcast"}); code != http.StatusUnauthorized && code != http.StatusForbidden {
		t.Fatalf("visitor PUT = %d, want 401/403", code)
	}
	if code, _ := putTok(t, url, "tok", map[string]string{"theme": "neon"}); code != http.StatusBadRequest {
		t.Fatalf("unknown theme = %d, want 400", code)
	}
	if code, body := putTok(t, url, "tok", map[string]string{"theme": "custom"}); code != http.StatusBadRequest {
		t.Fatalf("custom without config = %d %v, want 400", code, body)
	}
	if code, _ := putTok(t, url, "tok", map[string]string{"theme": ""}); code != http.StatusBadRequest {
		t.Fatalf("empty theme = %d, want 400", code)
	}

	// A visitor's view must not have moved after the rejected writes.
	_, body := getTok(t, srv.URL+"/api/v1/capabilities", "")
	if th := themeOf(t, body); th["active"] != api.ThemeDefault {
		t.Fatalf("after rejected writes active = %v, want default", th["active"])
	}

	code, body := putTok(t, url, "tok", map[string]string{"theme": "broadcast"})
	if code != http.StatusOK {
		t.Fatalf("set broadcast = %d %v", code, body)
	}
	if th := themeOf(t, body); th["active"] != "broadcast" {
		t.Fatalf("response theme = %v, want broadcast", th)
	}
	_, body = getTok(t, srv.URL+"/api/v1/capabilities", "")
	if th := themeOf(t, body); th["active"] != "broadcast" {
		t.Fatalf("visitor capabilities active = %v, want broadcast", th["active"])
	}
}

// A stored "custom" whose palette is no longer configured reports the default and
// leaves the row intact (spec R3, story 8).
func TestThemeStoredCustomWithoutConfigFallsBack(t *testing.T) {
	srv, r := authServer(t, "tok", false)
	if err := r.PutSetting(t.Context(), "theme.active", "custom"); err != nil {
		t.Fatal(err)
	}
	_, body := getTok(t, srv.URL+"/api/v1/capabilities", "tok")
	if th := themeOf(t, body); th["active"] != api.ThemeDefault {
		t.Fatalf("active = %v, want default fallback", th["active"])
	}
	if v, _, _ := r.GetSetting(t.Context(), "theme.active"); v != "custom" {
		t.Fatalf("stored row = %q, want untouched \"custom\"", v)
	}
}

// With a palette configured, "custom" is selectable and every viewer receives the
// palette (name, base, normalised tokens, contrast) in capabilities (spec R3, R9, R15).
func TestSetThemeCustomWhenConfigured(t *testing.T) {
	srv, h := themedServer(t, "tok")
	custom, err := theme.Parse(theme.Input{Name: "Rich Archive", Base: "cinematheque",
		BG: "#0b0a0c", Ink: "#efe9e0", Accent: "#C0483F", Muted: "#9a9188", Warn: "#e2603f"})
	if err != nil {
		t.Fatal(err)
	}
	h.SetCustomTheme(custom)

	code, body := putTok(t, srv.URL+"/api/v1/admin/theme", "tok", map[string]string{"theme": "custom"})
	if code != http.StatusOK {
		t.Fatalf("set custom = %d %v", code, body)
	}
	_, body = getTok(t, srv.URL+"/api/v1/capabilities", "")
	th := themeOf(t, body)
	if th["active"] != "custom" {
		t.Fatalf("active = %v, want custom", th["active"])
	}
	c, ok := th["custom"].(map[string]any)
	if !ok || c["name"] != "Rich Archive" || c["base"] != "cinematheque" {
		t.Fatalf("custom = %v", th["custom"])
	}
	if tokens := c["tokens"].(map[string]any); tokens["accent"] != "#c0483f" || len(tokens) != 5 {
		t.Fatalf("tokens = %v, want five normalised hexes", tokens)
	}
	if pairs := c["contrast"].([]any); len(pairs) != 4 {
		t.Fatalf("contrast pairs = %d, want 4", len(pairs))
	}
}
