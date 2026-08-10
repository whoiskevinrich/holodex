package api

import "testing"

// TestNamespaceLabel covers the HOLODEX-266/ADR-083 display-label lookup: known
// namespaces use their well-known override (not a naive title-case, which would
// mangle "imdb" -> "Imdb" instead of "IMDb"); an unrecognized namespace falls back
// to a title-cased rendering of the raw string.
func TestNamespaceLabel(t *testing.T) {
	cases := []struct {
		namespace string
		want      string
	}{
		{"imdb", "IMDb"},
		{"tmdb", "TMDB"},
		{"anidb", "Anidb"}, // unrecognized -> title-case fallback
		{"", ""},
	}
	for _, c := range cases {
		if got := namespaceLabel(c.namespace); got != c.want {
			t.Errorf("namespaceLabel(%q) = %q, want %q", c.namespace, got, c.want)
		}
	}
}
