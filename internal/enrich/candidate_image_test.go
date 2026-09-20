package enrich

import (
	"context"
	"strings"
	"testing"

	"holodex/internal/model"
)

// F64 (HOLODEX-406): candidates[].image_url is rendered by the owner's browser as an
// <img src>, so it passes exactly the gate a render:image_url field value passes —
// assetHostAllowed over the provider's {base_url host} ∪ asset_hosts allowlist. This
// table is the riskiest-assumption test for the feature: that the allowlist is the
// whole perimeter. Anything that fails is cleared, never truncated, never an error.
func TestSanitizeImageURL(t *testing.T) {
	src := Source{
		Name:       "acme",
		BaseURL:    "http://acme.example:9100",
		AssetHosts: []string{"cdn.acme.example"},
	}
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"base host, http", "http://acme.example:9100/t/1.jpg", "http://acme.example:9100/t/1.jpg"},
		{"asset_hosts entry, https", "https://cdn.acme.example/t/w185/1.jpg", "https://cdn.acme.example/t/w185/1.jpg"},
		{"asset_hosts entry over http", "http://cdn.acme.example/t/1.jpg", ""},
		{"foreign host", "https://img.other.example/1.jpg", ""},
		{"suffix spoof of the base host", "https://acme.example.evil.example/1.jpg", ""},
		{"prefix spoof of the asset host", "https://cdn.acme.example.evil.example/1.jpg", ""},
		{"ftp", "ftp://acme.example:9100/1.jpg", ""},
		{"javascript", "javascript:alert(1)", ""},
		{"data", "data:image/png;base64,AAAA", ""},
		{"protocol-relative", "//cdn.acme.example/1.jpg", ""},
		{"relative", "/t/1.jpg", ""},
		{"malformed", "http://[::1", ""},
		{"empty", "", ""},
		{"whitespace", "   ", ""},
		{"control chars stripped then allowed", "https://cdn.acme.example/t/1\x00.jpg", "https://cdn.acme.example/t/1.jpg"},
		{"over the value cap is cleared, not truncated", "https://cdn.acme.example/" + strings.Repeat("a", maxFieldLen), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitizeImageURL(src, c.in); got != c.want {
				t.Errorf("sanitizeImageURL(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// The gate runs inside Service.Resolve — upstream of every resolve handler (person,
// video, film, studio), which all pass res.Candidates through untouched. A candidate
// whose image is on the provider's own host round-trips; one on a foreign host
// arrives with the field cleared and is otherwise intact (label, profile_url).
func TestServiceResolveGatesImageURL(t *testing.T) {
	fake := NewFake("fake")
	fake.People["tmdb:1"] = FakePerson{Label: "Pictured Match", ImageURL: "http://fake:9100/t/w185/1.jpg", ProfileURL: "https://www.themoviedb.org/person/1"}
	fake.People["tmdb:2"] = FakePerson{Label: "Foreign Match", ImageURL: "https://img.other.example/2.jpg", ProfileURL: "https://www.themoviedb.org/person/2"}
	fake.People["tmdb:3"] = FakePerson{Label: "Unpictured Match"}
	svc, _ := newSvc(t, fake)

	res, err := svc.Resolve(context.Background(), "fake", model.EnrichEntityPerson, Hint{Query: "match"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	got := map[string]Candidate{}
	for _, c := range res.Candidates {
		got[c.ExternalID] = c
	}
	if got["tmdb:1"].ImageURL != "http://fake:9100/t/w185/1.jpg" {
		t.Errorf("own-host image_url = %q, want round-tripped unchanged", got["tmdb:1"].ImageURL)
	}
	if got["tmdb:2"].ImageURL != "" {
		t.Errorf("foreign-host image_url survived the gate: %q", got["tmdb:2"].ImageURL)
	}
	if got["tmdb:2"].Label != "Foreign Match" || got["tmdb:2"].ProfileURL != "https://www.themoviedb.org/person/2" {
		t.Errorf("clearing image_url must leave the candidate intact: %+v", got["tmdb:2"])
	}
	if got["tmdb:3"].ImageURL != "" {
		t.Errorf("absent image_url must stay absent: %q", got["tmdb:3"].ImageURL)
	}
}
