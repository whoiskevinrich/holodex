package fieldsource_test

import (
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
