package enrich

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateLinkTemplate(t *testing.T) {
	cases := []struct {
		tmpl string
		want bool
	}{
		{"https://www.imdb.com/title/{id}/", true},
		{"http://example.com/{id}", true},
		{"https://example.com/no-placeholder", false},                       // missing {id}
		{"https://example.com/{id}/{extra}", false},                         // second brace token
		{"https://example.com/{id}/{id}", false},                            // {id} twice
		{"javascript:alert(1)//{id}", false},                                // non-http(s) scheme
		{"{id}", false},                                                     // no host/scheme at all
		{"", false},                                                         // empty
		{"https://example.com/" + strings.Repeat("x", 600) + "{id}", false}, // over length cap
	}
	for _, c := range cases {
		if got := ValidateLinkTemplate(c.tmpl); got != c.want {
			t.Errorf("ValidateLinkTemplate(%q) = %v, want %v", c.tmpl, got, c.want)
		}
	}
}

func TestBuildLink(t *testing.T) {
	got := BuildLink("https://www.imdb.com/title/{id}/", "tt1234567")
	if want := "https://www.imdb.com/title/tt1234567/"; got != want {
		t.Fatalf("BuildLink() = %q, want %q", got, want)
	}
	// The id is path-escaped, not substituted raw (ADR-083 D2 security posture).
	got = BuildLink("https://example.com/{id}", "a/b")
	if want := "https://example.com/a%2Fb"; got != want {
		t.Fatalf("BuildLink() with slash in id = %q, want %q", got, want)
	}
}

func TestSanitizeLinkTemplates_DropsInvalidNormalizesKeys(t *testing.T) {
	in := map[string]map[string]string{
		"IMDb": {
			"Person": "https://www.imdb.com/name/{id}/",
			"video":  "not-a-url", // invalid → dropped
		},
		" tmdb ": {
			"video": "https://www.themoviedb.org/movie/{id}",
		},
		"": { // empty namespace → dropped entirely
			"video": "https://example.com/{id}",
		},
	}
	out := SanitizeLinkTemplates(in)

	if _, ok := out[""]; ok {
		t.Fatal("empty namespace must be dropped")
	}
	imdb, ok := out["imdb"]
	if !ok {
		t.Fatalf("namespace key must be lower-cased, got %+v", out)
	}
	if imdb["person"] != "https://www.imdb.com/name/{id}/" {
		t.Fatalf("person template not preserved: %+v", imdb)
	}
	if _, ok := imdb["video"]; ok {
		t.Fatal("invalid template must be dropped")
	}
	if out["tmdb"]["video"] == "" {
		t.Fatal("trimmed namespace key should still resolve")
	}
}

func TestSanitizeLinkTemplates_EmptyAndNil(t *testing.T) {
	if SanitizeLinkTemplates(nil) != nil {
		t.Fatal("nil in -> nil out")
	}
	if got := SanitizeLinkTemplates(map[string]map[string]string{"imdb": {"video": "bad"}}); got != nil {
		t.Fatalf("all-dropped map should be nil, got %+v", got)
	}
}

func TestManifest_LinkTemplatesDecodeBackwardCompat(t *testing.T) {
	var m Manifest
	if err := json.Unmarshal([]byte(`{
		"provider":"acme","version":"1.0.0","protocol_version":1,
		"entity_types":["person"],"fields":["bio"]
	}`), &m); err != nil {
		t.Fatal(err)
	}
	if m.LinkTemplates != nil {
		t.Fatalf("absent link_templates should decode nil, got %+v", m.LinkTemplates)
	}

	var m2 Manifest
	if err := json.Unmarshal([]byte(`{
		"provider":"acme","protocol_version":1,"entity_types":["person"],
		"fields":["bio"],
		"link_templates":{"imdb":{"person":"https://www.imdb.com/name/{id}/"}}
	}`), &m2); err != nil {
		t.Fatal(err)
	}
	if tmpl := m2.LinkTemplates["imdb"]["person"]; tmpl != "https://www.imdb.com/name/{id}/" {
		t.Fatalf("link_templates not decoded: %+v", m2.LinkTemplates)
	}
}
