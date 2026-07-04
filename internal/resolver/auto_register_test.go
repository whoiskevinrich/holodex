package resolver

import (
	"testing"
)

// hint builds a hintFor closure from a static map keyed "provider\x00key".
func hintLookup(m map[string]AutoHint) func(provider, key string) (AutoHint, bool) {
	return func(provider, key string) (AutoHint, bool) {
		h, ok := m[provider+"\x00"+key]
		return h, ok
	}
}

func fieldByKey(fields []ResolvedField, key string) (ResolvedField, bool) {
	for _, f := range fields {
		if f.Canonical == key {
			return f, true
		}
	}
	return ResolvedField{}, false
}

func TestAutoRegisterFields_PredicateExclusions(t *testing.T) {
	fields := []AutoField{
		{Provider: "tmdb", Key: "gender", Values: []string{"Female"}}, // non-canonical → in
		{Provider: "tmdb", Key: "bio", Values: []string{"…"}},         // canonical → skipped
		{Provider: "tmdb", Key: "_studio_external_ids", Values: []string{"tmdb:1 X"}}, // reserved → skipped
		{Provider: "tmdb", Key: "trivia", Values: []string{"  "}},     // present but blank → presence gate drops
		{Provider: "tmdb", Key: "already", Values: []string{"v"}},     // already rendered → skipped
	}
	rendered := map[string]bool{"already": true}

	out := AutoRegisterFields(fields, rendered, nil, nil)

	if len(out) != 1 {
		t.Fatalf("want 1 auto-registered field, got %d: %+v", len(out), out)
	}
	if out[0].Canonical != "gender" {
		t.Fatalf("want gender, got %q", out[0].Canonical)
	}
	if !out[0].AutoRegistered {
		t.Fatal("field must be marked AutoRegistered")
	}
	if out[0].Decision != nil || out[0].Candidates != nil || out[0].InSync != nil {
		t.Fatal("auto-registered fields must carry no decision/candidate/in-sync state")
	}
	if out[0].Label != "Gender" { // title-case floor (no hint)
		t.Fatalf("want title-case floor label 'Gender', got %q", out[0].Label)
	}
}

func TestAutoRegisterFields_HintLabelRenderAndOrder(t *testing.T) {
	fields := []AutoField{
		{Provider: "tmdb", Key: "zeta", Values: []string{"z"}},
		{Provider: "tmdb", Key: "alpha", Values: []string{"a"}},
		{Provider: "tmdb", Key: "trivia", Values: []string{"t"}},
	}
	hints := hintLookup(map[string]AutoHint{
		"tmdb\x00zeta":   {Label: "Zeta", Display: "text", Group: "attributes", Order: 5},
		"tmdb\x00alpha":  {Label: "Alpha", Display: "chips", Group: "attributes", Order: 10},
		"tmdb\x00trivia": {Label: "Trivia", Display: "long_text", Group: "extended"},
	})

	out := AutoRegisterFields(fields, nil, hints, nil)

	// attributes(zeta order5, alpha order10) before extended(trivia).
	got := []string{out[0].Canonical, out[1].Canonical, out[2].Canonical}
	want := []string{"zeta", "alpha", "trivia"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordering: want %v, got %v", want, got)
		}
	}
	if z, _ := fieldByKey(out, "zeta"); z.Label != "Zeta" || z.Display != "" {
		t.Fatalf("zeta hint label/display wrong: %+v", z)
	}
	if a, _ := fieldByKey(out, "alpha"); a.Display != "chips" {
		t.Fatalf("alpha should render chips, got %q", a.Display)
	}
	if tr, _ := fieldByKey(out, "trivia"); tr.Display != "long_text" {
		t.Fatalf("trivia should render long_text, got %q", tr.Display)
	}
}

func TestAutoRegisterFields_ImageURLAllowlistGate(t *testing.T) {
	fields := []AutoField{
		{Provider: "tmdb", Key: "badge_ok", Values: []string{"https://cdn.ok/x.png"}},
		{Provider: "tmdb", Key: "badge_bad", Values: []string{"https://evil.example/x.png"}},
	}
	hints := hintLookup(map[string]AutoHint{
		"tmdb\x00badge_ok":  {Label: "OK", Display: "image_url", Group: "extended"},
		"tmdb\x00badge_bad": {Label: "Bad", Display: "image_url", Group: "extended"},
	})
	allowed := func(provider, url string) bool { return url == "https://cdn.ok/x.png" }

	out := AutoRegisterFields(fields, nil, hints, allowed)

	if ok, _ := fieldByKey(out, "badge_ok"); ok.Display != "image_url" {
		t.Fatalf("allowlisted host should keep image_url, got %q", ok.Display)
	}
	// Non-allowlisted image degrades to text (no <img> beacon).
	if bad, _ := fieldByKey(out, "badge_bad"); bad.Display != "" {
		t.Fatalf("non-allowlisted image_url should fall back to text, got %q", bad.Display)
	}
}

func TestAutoRegisterFields_MultiProviderMerge(t *testing.T) {
	fields := []AutoField{
		{Provider: "tmdb", Key: "credited_as", Values: []string{"A. King", "Ada"}},
		{Provider: "acme", Key: "credited_as", Values: []string{"ada", "Countess"}}, // "ada" dupes "Ada"
	}
	out := AutoRegisterFields(fields, nil, nil, nil)
	if len(out) != 1 {
		t.Fatalf("want 1 merged field, got %d", len(out))
	}
	f := out[0]
	if len(f.Values) != 3 { // A. King, Ada, Countess (case-insensitive dedup)
		t.Fatalf("want 3 deduped values, got %v", f.Values)
	}
	// The value present in both providers carries both provenance sources.
	var adaSources int
	for _, it := range f.Items {
		if it.Value == "Ada" {
			adaSources = len(it.Sources)
		}
	}
	if adaSources != 2 {
		t.Fatalf("shared value should carry both provider sources, got %d", adaSources)
	}
}

func TestAutoRegisterFields_NoNonCanonical_Empty(t *testing.T) {
	fields := []AutoField{{Provider: "tmdb", Key: "bio", Values: []string{"x"}}}
	if out := AutoRegisterFields(fields, nil, nil, nil); len(out) != 0 {
		t.Fatalf("only-canonical input should yield no auto-registered fields, got %+v", out)
	}
}
