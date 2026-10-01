package api

import (
	"net/url"
	"strings"
	"testing"
)

// canon runs canonicalPlaylistQuery on a raw query string, failing the test on a refusal.
func canon(t *testing.T, h *Handlers, raw string) string {
	t.Helper()
	q, err := parseQueryString(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	out, err := h.canonicalPlaylistQuery(q)
	if err != nil {
		t.Fatalf("canonicalPlaylistQuery(%q): %v", raw, err)
	}
	return out
}

// permutations returns every ordering of pairs (the rows below stay small).
func permutations(pairs []string) [][]string {
	if len(pairs) <= 1 {
		return [][]string{append([]string(nil), pairs...)}
	}
	var out [][]string
	for i := range pairs {
		rest := append(append([]string(nil), pairs[:i]...), pairs[i+1:]...)
		for _, p := range permutations(rest) {
			out = append(out, append([]string{pairs[i]}, p...))
		}
	}
	return out
}

// TestCanonicalPlaylistQueryTable covers testing-strategy §23.2's canonicalPlaylistQuery
// table (ADR-121 D2): fetch keys strip to nothing, id facets sort numerically (not
// lexically), the output is idempotent and order-insensitive, and a bare query stores "".
func TestCanonicalPlaylistQueryTable(t *testing.T) {
	h := &Handlers{} // no mappings: only the fixed browse keys are filterable

	// Each fetch key alone, and the bare query, store "everything".
	for _, raw := range []string{"", "?", "limit=20", "offset=40", "seed=9", "sort=title_asc", "limit=1&offset=2&seed=3&sort=random"} {
		if got := canon(t, h, raw); got != "" {
			t.Errorf("canon(%q) = %q, want \"\"", raw, got)
		}
	}

	rows := []struct {
		pairs []string
		want  string
	}{
		// Numeric, not lexical: 9 sorts before 10 (lexically "10" < "9").
		{[]string{"person=10", "person=9"}, "person=9&person=10"},
		{[]string{"person=40", "person=12", "person=40"}, "person=12&person=40"},
		{[]string{"tag=100", "tag=20", "tag=3", "q=noir", "limit=5"}, "q=noir&tag=3&tag=20&tag=100"},
		{[]string{"studio_id=11", "studio_id=2", "category_id=7", "resolution=FHD", "seed=4"}, "category_id=7&resolution=FHD&studio_id=2&studio_id=11"},
		{[]string{"missing_facet=studio", "missing_facet=poster_url", "year_min=2000", "sort=title_desc"}, "missing_facet=poster_url&missing_facet=studio&year_min=2000"},
		// A zero-padded id is the same id.
		{[]string{"person=012", "person=12"}, "person=12"},
		// Empty values drop out.
		{[]string{"q=", "year_max=", "tag=5"}, "tag=5"},
	}
	for _, row := range rows {
		got := canon(t, h, strings.Join(row.pairs, "&"))
		if got != row.want {
			t.Errorf("canon(%v) = %q, want %q", row.pairs, got, row.want)
		}
		if again := canon(t, h, got); again != got {
			t.Errorf("not idempotent: canon(%q) = %q", got, again)
		}
		for _, perm := range permutations(row.pairs) {
			if p := canon(t, h, strings.Join(perm, "&")); p != row.want {
				t.Errorf("permutation %v = %q, want %q", perm, p, row.want)
			}
		}
	}

	// The comma form browse also accepts is the same set.
	if got := canon(t, h, "person=10,9"); got != "person=9&person=10" {
		t.Errorf("comma form = %q, want person=9&person=10", got)
	}
	// Output parses back to itself through url.Values.Encode.
	q, _ := url.ParseQuery("tag=3&tag=20")
	if q.Encode() != canon(t, h, "tag=20&tag=3") {
		t.Errorf("canonical form is not url.Values.Encode'd")
	}
}
