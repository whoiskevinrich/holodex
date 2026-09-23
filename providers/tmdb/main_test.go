package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Fixtures are assembled at run time rather than written as literals. A 32-character hex
// string or a three-segment JWT sitting in source is precisely what a secret scanner is
// built to flag, and CI runs gitleaks over full history (ADR-094 D4) — embedding them as
// literals reddens the gate on every run. Building them from short, obviously synthetic
// parts keeps the test readable without a path allowlist, which would be the other way to
// silence it and would also mask a real credential later committed to this file.
//
// Never paste a real credential here, live or rotated, to exercise a shape.

// hexKey returns a synthetic 32-character key, the shape of a TMDB v3 API key.
func hexKey() string { return strings.Repeat("0123456789abcdef", 2) }

// jwt joins segments into the three-part shape of a TMDB Read Access Token.
func jwt(segments ...string) string { return strings.Join(segments, ".") }

// The startup guard in main() turns a swapped TMDB credential into an immediate, named
// failure instead of opaque 401s during enrichment. Its correctness rests entirely on
// classifyCredential, so the shapes are locked down here.
func TestClassifyCredential(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want credentialKind
	}{
		{"read access token", jwt("header", "payload", "signature"), credReadAccessToken},
		{"token with url-safe base64 chars", jwt("aa-bb", "cc_dd", "ee-ff_gg"), credReadAccessToken},

		{"api key lowercase", hexKey(), credAPIKey},
		{"api key uppercase", strings.ToUpper(hexKey()), credAPIKey},

		{"empty", "", credUnknown},
		{"hex too short", strings.Repeat("ab", 8), credUnknown},
		{"hex too long", hexKey() + "00", credUnknown},
		{"non-hex 32 chars", strings.Repeat("z", 32), credUnknown},
		{"two-segment jwt", jwt("header", "payload"), credUnknown},
		{"four-segment jwt", jwt("a", "b", "c", "d"), credUnknown},
		{"empty jwt segment", jwt("a", "", "c"), credUnknown},
		{"jwt with trailing space", jwt("a", "b", "c") + " ", credUnknown},
		{"placeholder from the template", "<your local dev token>", credUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyCredential(tt.in); got != tt.want {
				t.Errorf("classifyCredential(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// The two kinds must be mutually exclusive. If an API key could satisfy the JWT shape, a
// swapped credential would be waved through as a valid bearer token and the guard in main()
// would be useless — which is the exact failure it exists to prevent.
func TestCredentialKindsAreMutuallyExclusive(t *testing.T) {
	if jwtShape.MatchString(hexKey()) {
		t.Error("an api key matched the JWT shape; a swapped credential would not be caught")
	}
	if apiKeyShape.MatchString(jwt("header", "payload", "signature")) {
		t.Error("a read access token matched the api key shape; a swapped credential would not be caught")
	}
}

// resolvePort is the one place the server's bind port and the container health probe agree
// (ADR-105 D1). If the two ever disagreed the probe would poll a port nothing listens on and
// report a healthy sidecar as unhealthy forever, so the precedence is pinned rather than
// re-derived at each call site.
func TestResolvePort(t *testing.T) {
	tests := []struct {
		name string
		flag string
		env  string
		want string
	}{
		{"flag wins over env", "9200", "9300", "9200"},
		{"flag wins with no env", "9200", "", "9200"},
		{"env when no flag", "", "9300", "9300"},
		{"default when neither", "", "", "9100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PORT", tt.env)
			if got := resolvePort(tt.flag); got != tt.want {
				t.Errorf("resolvePort(%q) with PORT=%q = %q, want %q", tt.flag, tt.env, got, tt.want)
			}
		})
	}
}

// The probe is the container's only health signal now that wget is gone with the Debian base
// (ADR-105 D2), so both directions matter: a false 0 keeps a dead sidecar in rotation, and a
// false 1 restart-loops a healthy one.
func TestRunHealthcheck(t *testing.T) {
	t.Run("200 is healthy", func(t *testing.T) {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
		}))
		defer srv.Close()
		if got := runHealthcheck(portOf(t, srv.URL)); got != 0 {
			t.Errorf("runHealthcheck on a 200 = %d, want 0", got)
		}
		// It must probe /healthz specifically — any 200-returning path would otherwise pass.
		if gotPath != "/healthz" {
			t.Errorf("probed %q, want /healthz", gotPath)
		}
	})

	// A sidecar that boots but cannot serve (bad credential, wedged handler) answers non-200.
	// Treating that as healthy is the failure this pins.
	t.Run("non-200 is unhealthy", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusServiceUnavailable)
		}))
		defer srv.Close()
		if got := runHealthcheck(portOf(t, srv.URL)); got != 1 {
			t.Errorf("runHealthcheck on a 503 = %d, want 1", got)
		}
	})

	t.Run("nothing listening is unhealthy", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		port := portOf(t, srv.URL)
		srv.Close() // frees the port; the dial must now fail rather than hang or pass
		if got := runHealthcheck(port); got != 1 {
			t.Errorf("runHealthcheck with no listener = %d, want 1", got)
		}
	})
}

// portOf extracts the port from an httptest server's URL. The probe always dials 127.0.0.1
// (it runs inside the container), which is the interface httptest binds, so passing the port
// alone is enough to point it at the test server.
func portOf(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parsing test server URL %q: %v", rawURL, err)
	}
	return u.Port()
}
