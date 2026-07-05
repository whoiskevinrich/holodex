package enrich

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAssetHostAllowed(t *testing.T) {
	src := Source{
		Name:       "tmdb",
		BaseURL:    "http://tmdb-sidecar:9100",
		AssetHosts: []string{"image.tmdb.org"},
	}
	cases := []struct {
		url  string
		want bool
	}{
		{"http://tmdb-sidecar:9100/x.jpg", true},  // base host, http allowed
		{"https://image.tmdb.org/x.jpg", true},    // operator asset_host, https
		{"http://image.tmdb.org/x.jpg", false},    // non-base host must be https
		{"https://evil.example/x.jpg", false},     // host not allowlisted
		{"ftp://tmdb-sidecar:9100/x.jpg", false},  // non-http scheme
		{"https://image.tmdb.org.evil/x", false},  // lookalike host, not exact
		{"", false},                               // unparseable / no host
	}
	for _, c := range cases {
		if got := assetHostAllowed(src, c.url); got != c.want {
			t.Errorf("assetHostAllowed(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestSanitizeFieldHints_DropsCanonicalReservedAndUnknownVocab(t *testing.T) {
	in := map[string]FieldHint{
		"bio":        {Label: "Biography", Render: "long_text"},        // canonical → dropped (registry owns it)
		"_secret":    {Label: "Secret", Render: "text"},                // reserved sidecar → dropped
		"gender":     {Label: "Gender", Render: "text", Group: "attributes", Order: 3},
		"credited_as": {Label: "Also credited as", Render: "chips", Group: "bogus"}, // unknown group → extended
		"badge":      {Label: "Badge", Render: "hologram"},             // unknown render → text
	}
	out := SanitizeFieldHints(in)

	if _, ok := out["bio"]; ok {
		t.Fatal("canonical key must be dropped")
	}
	if _, ok := out["_secret"]; ok {
		t.Fatal("reserved _-key must be dropped")
	}
	// "text" is the inline-text render mode, which normalizes to "" (the Display
	// vocabulary's default), consistent with registry.FieldDef.Display.
	if g := out["gender"]; g.Label != "Gender" || g.Render != "" || g.Group != "attributes" || g.Order != 3 {
		t.Fatalf("gender hint not preserved: %+v", g)
	}
	if c := out["credited_as"]; c.Render != "chips" || c.Group != "extended" {
		t.Fatalf("unknown group should normalize to extended; render kept: %+v", c)
	}
	if b := out["badge"]; b.Render != "" {
		t.Fatalf("unknown render should normalize to text (empty), got %q", b.Render)
	}
}

func TestSanitizeFieldHints_LabelSanitized(t *testing.T) {
	long := strings.Repeat("x", 200)
	out := SanitizeFieldHints(map[string]FieldHint{
		"note": {Label: "line1\nline2\ttab\x07bell " + long},
	})
	got := out["note"].Label
	if strings.ContainsAny(got, "\n\t\x07") {
		t.Fatalf("control chars must be stripped: %q", got)
	}
	if len(got) > maxHintLabelLen {
		t.Fatalf("label must be capped at %d, got %d", maxHintLabelLen, len(got))
	}
}

func TestSanitizeFieldHints_EmptyAndNil(t *testing.T) {
	if SanitizeFieldHints(nil) != nil {
		t.Fatal("nil in → nil out")
	}
	// A map that fully drops (only canonical/reserved) → nil.
	if got := SanitizeFieldHints(map[string]FieldHint{"bio": {Label: "B"}}); got != nil {
		t.Fatalf("all-dropped map should be nil, got %+v", got)
	}
}

func TestManifest_DecodeBackwardCompat(t *testing.T) {
	// A pre-F39 manifest without field_hints decodes with a nil map (no change).
	var m Manifest
	if err := json.Unmarshal([]byte(`{
		"provider":"acme","version":"1.0.0","protocol_version":1,
		"entity_types":["person"],"fields":["bio","gender"]
	}`), &m); err != nil {
		t.Fatal(err)
	}
	if m.FieldHints != nil {
		t.Fatalf("absent field_hints should decode nil, got %+v", m.FieldHints)
	}

	// A manifest with field_hints decodes the map.
	var m2 Manifest
	if err := json.Unmarshal([]byte(`{
		"provider":"acme","protocol_version":1,"entity_types":["person"],
		"fields":["gender"],
		"field_hints":{"gender":{"label":"Gender","render":"text","group":"attributes","order":2}}
	}`), &m2); err != nil {
		t.Fatal(err)
	}
	if h, ok := m2.FieldHints["gender"]; !ok || h.Label != "Gender" || h.Order != 2 {
		t.Fatalf("field_hints not decoded: %+v", m2.FieldHints)
	}
}
