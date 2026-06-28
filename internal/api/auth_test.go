package api_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

// --- Owner session persistence (ADR-045) ---

const sessionCookieName = "holodex_session"

// exchange POSTs the token to /session, returning the response (so callers can
// inspect Set-Cookie). query is e.g. "?remember=1" or "".
func exchange(t *testing.T, base, token, query string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/session"+query, nil)
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /session: %v", err)
	}
	resp.Body.Close()
	return resp
}

// findCookie returns the named cookie from a response, or nil.
func findCookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// getCookie issues GET url carrying an optional session cookie, returning the
// full response and decoded body.
func getCookie(t *testing.T, url string, c *http.Cookie) (*http.Response, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if c != nil {
		req.AddCookie(c)
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
	return resp, body
}

// Exchange: correct token → 204 + an HttpOnly, SameSite=Strict cookie whose value
// is NOT the raw token; wrong token → 401, no cookie.
func TestSessionExchangeSetsCookie(t *testing.T) {
	srv, _ := authServer(t, "s3cret", false)

	ok := exchange(t, srv.URL, "s3cret", "")
	if ok.StatusCode != http.StatusNoContent {
		t.Fatalf("exchange code = %d, want 204", ok.StatusCode)
	}
	c := findCookie(ok, sessionCookieName)
	if c == nil {
		t.Fatal("expected a session cookie, got none")
	}
	if !c.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict", c.SameSite)
	}
	if c.Value == "" || strings.Contains(c.Value, "s3cret") {
		t.Errorf("cookie value must not be the raw token, got %q", c.Value)
	}

	bad := exchange(t, srv.URL, "wrong", "")
	if bad.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong-token exchange code = %d, want 401", bad.StatusCode)
	}
	if findCookie(bad, sessionCookieName) != nil {
		t.Error("no cookie should be set on a failed exchange")
	}
}

// A valid session cookie authorizes gated routes and is reflected in /capabilities
// — the reload-persistence property. The header path stays available too.
func TestCookieAuthorizesGatedRoute(t *testing.T) {
	srv, _ := authServer(t, "s3cret", false)
	c := findCookie(exchange(t, srv.URL, "s3cret", ""), sessionCookieName)
	if c == nil {
		t.Fatal("no session cookie")
	}

	resp, _ := getCookie(t, srv.URL+"/api/v1/admin/activity", c)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("cookie-authorized activity = %d, want 200", resp.StatusCode)
	}

	_, caps := getCookie(t, srv.URL+"/api/v1/capabilities", c)
	if caps["owner"] != true {
		t.Errorf("capabilities owner = %v, want true with valid cookie", caps["owner"])
	}

	// No credential at all is still 401 (header path regression guard lives in
	// TestGateRequiresTokenWhenSet).
	resp, _ = getCookie(t, srv.URL+"/api/v1/admin/activity", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no-credential activity = %d, want 401", resp.StatusCode)
	}
}

// A tampered cookie is no credential: 401, and the response expires the dead
// cookie so the browser stops resending it.
func TestTamperedCookieRejected(t *testing.T) {
	srv, _ := authServer(t, "s3cret", false)
	c := findCookie(exchange(t, srv.URL, "s3cret", ""), sessionCookieName)
	if c == nil {
		t.Fatal("no session cookie")
	}
	c.Value += "x" // break the HMAC

	resp, _ := getCookie(t, srv.URL+"/api/v1/admin/activity", c)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("tampered-cookie activity = %d, want 401", resp.StatusCode)
	}
	if cleared := findCookie(resp, sessionCookieName); cleared == nil || cleared.MaxAge >= 0 {
		t.Error("a rejected cookie should be expired in the 401 response")
	}
}

// Sign-out expires the cookie and is idempotent.
func TestSignOut(t *testing.T) {
	srv, _ := authServer(t, "s3cret", false)

	del := func() *http.Response {
		req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/session", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("DELETE /session: %v", err)
		}
		resp.Body.Close()
		return resp
	}

	resp := del()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("sign-out code = %d, want 204", resp.StatusCode)
	}
	if cleared := findCookie(resp, sessionCookieName); cleared == nil || cleared.MaxAge >= 0 {
		t.Error("sign-out must expire the cookie")
	}
	// Idempotent: a second sign-out (no session) still succeeds.
	if resp = del(); resp.StatusCode != http.StatusNoContent {
		t.Errorf("idempotent sign-out code = %d, want 204", resp.StatusCode)
	}
}

// "Trust this device" (?remember=1) issues a longer-lived cookie than the default,
// with the same security attributes; the lifetime is server-set, not client-chosen.
func TestRememberDeviceLongerLifetime(t *testing.T) {
	srv, _ := authServer(t, "s3cret", false)

	short := findCookie(exchange(t, srv.URL, "s3cret", ""), sessionCookieName)
	long := findCookie(exchange(t, srv.URL, "s3cret", "?remember=1"), sessionCookieName)
	if short == nil || long == nil {
		t.Fatal("missing session cookie(s)")
	}
	if long.MaxAge <= short.MaxAge {
		t.Errorf("remember Max-Age (%d) should exceed default (%d)", long.MaxAge, short.MaxAge)
	}
	if !long.HttpOnly || long.SameSite != http.SameSiteStrictMode {
		t.Error("trusted cookie must keep HttpOnly + SameSite=Strict")
	}
}

// Gate open (no ADMIN_TOKEN): the exchange is a no-op success and sets no cookie.
func TestSessionExchangeNoopWhenGateOpen(t *testing.T) {
	srv, _ := authServer(t, "", false)
	resp := exchange(t, srv.URL, "", "")
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("open-gate exchange code = %d, want 204", resp.StatusCode)
	}
	if findCookie(resp, sessionCookieName) != nil {
		t.Error("no cookie should be set when the gate is open")
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
