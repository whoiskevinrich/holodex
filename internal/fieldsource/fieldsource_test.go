package fieldsource_test

import (
	"strings"
	"testing"

	"holodex/internal/fieldsource"
)

func TestValid(t *testing.T) {
	ok := []string{"file", "manual", "provider:tmdb", "provider:imdb"}
	bad := []string{"", "provider:", "provider: ", "tmdb", "file:title", "PROVIDER:tmdb"}
	for _, s := range ok {
		if !fieldsource.Valid(s) {
			t.Errorf("want %q valid", s)
		}
	}
	for _, s := range bad {
		if fieldsource.Valid(s) {
			t.Errorf("want %q invalid", s)
		}
	}
}

func TestProviderRoundTrip(t *testing.T) {
	if got := fieldsource.Provider("provider:tmdb"); got != "tmdb" {
		t.Errorf("Provider(provider:tmdb) = %q, want tmdb", got)
	}
	if got := fieldsource.Provider("file"); got != "" {
		t.Errorf("Provider(file) = %q, want empty", got)
	}
	if got := fieldsource.ForProvider("imdb"); got != "provider:imdb" {
		t.Errorf("ForProvider(imdb) = %q", got)
	}
}

func TestForNamespace(t *testing.T) {
	cases := map[string]string{
		"file":   "file",
		"manual": "manual",
		"tmdb":   "provider:tmdb",
	}
	for ns, want := range cases {
		if got := fieldsource.ForNamespace(ns); got != want {
			t.Errorf("ForNamespace(%q) = %q, want %q", ns, got, want)
		}
	}
}

// G1 (F85): the local kinds are a closed set, and a token names one of them plus a
// sha256 — nothing path-shaped passes.
func TestLocalSources(t *testing.T) {
	for _, k := range []string{"frame", "upload", "automatic"} {
		if fieldsource.Local("local:"+k) != k || !fieldsource.Valid("local:"+k) {
			t.Errorf("want local:%s valid", k)
		}
	}
	for _, s := range []string{"local:", "local:file", "local:auto", "local:../x", "local:frame/..", "LOCAL:frame"} {
		if fieldsource.Local(s) != "" || fieldsource.Valid(s) {
			t.Errorf("want %q refused", s)
		}
	}
	hash := strings.Repeat("0123456789abcdef", 4)
	if k, h, ok := fieldsource.ParseLocalToken("local:upload#" + hash); !ok || k != "upload" || h != hash {
		t.Errorf("token parse = %q %q %v", k, h, ok)
	}
	for _, v := range []string{"local:upload", "local:upload#abc", "local:../x#" + hash, "local:upload#" + strings.ToUpper(hash), "https://x/y.jpg"} {
		if _, _, ok := fieldsource.ParseLocalToken(v); ok {
			t.Errorf("want token %q refused", v)
		}
	}
	if fieldsource.ForNamespace("local:frame") != "local:frame" {
		t.Error("a local namespace must map to itself")
	}
}
