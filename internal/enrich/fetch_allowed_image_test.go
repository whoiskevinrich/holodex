package enrich

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestStore builds a Store backed by the given sources directly (no config
// file on disk) — same-package test helper, mirrors how Registry is built by
// parse() but skips the YAML round trip (HOLODEX-212).
func newTestStore(sources ...Source) *Store {
	st := &Store{}
	st.cur.Store(&Registry{sources: sources})
	return st
}

// TestFetchAllowedImage_AllowedViaBaseHost confirms a URL on a provider's own
// base_url host is fetched (ADR-039) even with no explicit asset_hosts entry —
// http is permitted on the base host, so this exercises the whole path,
// including the real network fetch, with a plain httptest server.
func TestFetchAllowedImage_AllowedViaBaseHost(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("bytes"))
	}))
	defer origin.Close()

	store := newTestStore(Source{
		Name:    "tmdb",
		BaseURL: origin.URL,
		Enabled: true,
	})
	svc := NewService(store, nil, nil)

	data, err := svc.FetchAllowedImage(context.Background(), origin.URL+"/poster.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "bytes" {
		t.Errorf("data = %q, want %q", data, "bytes")
	}
}

// TestFetchAllowedImage_AssetHostsExtendTheAllowlist confirms a host listed in
// asset_hosts is selected (not refused as "not allowlisted") — the remaining
// https enforcement for a non-base host is AssetClient's own concern (already
// covered by TestAssetClientNonBaseHostRequiresHTTPS) and isn't re-tested here.
// A connection-refused loopback port keeps the dial itself fast and deterministic.
func TestFetchAllowedImage_AssetHostsExtendTheAllowlist(t *testing.T) {
	store := newTestStore(Source{
		Name:       "tmdb",
		BaseURL:    "http://provider.example:9100",
		Enabled:    true,
		AssetHosts: []string{"127.0.0.1:1"},
	})
	svc := NewService(store, nil, nil)

	_, err := svc.FetchAllowedImage(context.Background(), "https://127.0.0.1:1/poster.jpg")
	if err == nil {
		t.Fatal("want error dialing a closed port, got nil")
	}
	if strings.Contains(err.Error(), "not allowlisted") {
		t.Errorf("host listed in asset_hosts should be selected, not refused as unlisted; got: %v", err)
	}
}

// TestFetchAllowedImage_RefusesUnlistedHost confirms a host covered by no
// enabled provider's allowlist is refused outright — no dial, no partial match.
func TestFetchAllowedImage_RefusesUnlistedHost(t *testing.T) {
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("must not dial a host outside every enabled provider's allowlist")
	}))
	defer evil.Close()

	store := newTestStore(Source{
		Name:       "tmdb",
		BaseURL:    "http://provider.example:9100",
		Enabled:    true,
		AssetHosts: []string{"image.tmdb.org"},
	})
	svc := NewService(store, nil, nil)

	_, err := svc.FetchAllowedImage(context.Background(), evil.URL+"/poster.jpg")
	if err == nil {
		t.Fatal("want error for a host not on any enabled provider's allowlist")
	}
	if !strings.Contains(err.Error(), "not allowlisted") {
		t.Errorf("error = %v, want it to mention the host is not allowlisted", err)
	}
}

// TestFetchAllowedImage_IgnoresDisabledProvider confirms a disabled provider's
// base host does not count — Enabled() already excludes it, this just pins the
// behavior at the FetchAllowedImage seam.
func TestFetchAllowedImage_IgnoresDisabledProvider(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("must not dial a host belonging only to a disabled provider")
	}))
	defer origin.Close()

	store := newTestStore(Source{
		Name:    "tmdb",
		BaseURL: origin.URL,
		Enabled: false,
	})
	svc := NewService(store, nil, nil)

	if _, err := svc.FetchAllowedImage(context.Background(), origin.URL+"/poster.jpg"); err == nil {
		t.Fatal("want error when the only matching provider is disabled")
	}
}

// TestFetchAllowedImage_UnionAcrossMultipleProviders confirms every enabled
// provider is checked, not just the first — a host matching the second
// provider's base host is still fetched.
func TestFetchAllowedImage_UnionAcrossMultipleProviders(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("bytes"))
	}))
	defer origin.Close()

	store := newTestStore(
		Source{Name: "other", BaseURL: "http://other.example:9100", Enabled: true},
		Source{Name: "tmdb", BaseURL: origin.URL, Enabled: true},
	)
	svc := NewService(store, nil, nil)

	if _, err := svc.FetchAllowedImage(context.Background(), origin.URL+"/poster.jpg"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
