package main

import (
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
