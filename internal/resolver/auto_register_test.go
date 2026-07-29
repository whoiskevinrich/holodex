package resolver

import (
	"testing"

	"holodex/internal/mapping"
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

	out := AutoRegisterFields(fields, nil, nil, hints)

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

// The image_url allowlist gate is applied by the caller (api.appendAutoRegistered),
// not the pure resolver — so this pass emits image_url exactly as hinted. The gate
// itself is covered by the enrich asset-host allowlist tests.
func TestAutoRegisterFields_EmitsImageURLAsHinted(t *testing.T) {
	fields := []AutoField{{Provider: "tmdb", Key: "badge", Values: []string{"https://cdn/x.png"}}}
	hints := hintLookup(map[string]AutoHint{
		"tmdb\x00badge": {Label: "Badge", Display: "image_url", Group: "extended"},
	})
	out := AutoRegisterFields(fields, nil, nil, hints)
	if b, _ := fieldByKey(out, "badge"); b.Display != "image_url" {
		t.Fatalf("resolver should emit image_url as hinted (caller gates), got %q", b.Display)
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

// ---- F49 claimed provider keys (ADR-074) ----

// src builds a mapping.Field carrying only what ClaimedKeys reads.
func claimField(canonical string, sources ...mapping.Source) mapping.Field {
	return mapping.Field{Canonical: canonical, ParsedSources: sources}
}

func TestClaimedKeys_OnlyNamespacedProviderSources(t *testing.T) {
	// As parsed from: sources: [tmdb:overview, provA:synopsis, file:Title, Comment]
	got := ClaimedKeys([]mapping.Field{claimField("overview",
		mapping.Source{Namespace: "tmdb", Key: "overview"},
		mapping.Source{Namespace: "provA", Key: "synopsis"},
		mapping.Source{Namespace: "file", Key: "Title"},
		mapping.Source{Namespace: "file", Key: "Comment"}, // bare `Comment` parses to file:
	)})

	want := map[string]bool{"tmdb:overview": true, "prova:synopsis": true}
	if len(got) != len(want) {
		t.Fatalf("want exactly %v, got %v", want, got)
	}
	for k := range want {
		if !got[k] {
			t.Fatalf("want %q claimed, got %v", k, got)
		}
	}
	// A file tag must never claim: one mapping's `Comment` source would otherwise
	// swallow every provider's `comment` key.
	if got["file:title"] || got["file:comment"] {
		t.Fatalf("file tags must claim nothing, got %v", got)
	}
}

func TestClaimedKeys_LowercasedAndTrimmed(t *testing.T) {
	// Synthesized fields (person/studio, F44 promotions) are built in code, so the
	// case/whitespace normalization cannot be assumed to have happened at parse.
	got := ClaimedKeys([]mapping.Field{claimField("overview",
		mapping.Source{Namespace: " ProvA ", Key: " Synopsis "},
		mapping.Source{Namespace: "provB", Key: ""}, // empty key claims nothing
	)})
	if !got["prova:synopsis"] || len(got) != 1 {
		t.Fatalf("want only prova:synopsis, got %v", got)
	}
}

// GH #178: three providers naming one paragraph differently produced three identical
// rows, because suppression compared canonical names against raw provider keys.
func TestAutoRegisterFields_ClaimedKeysSuppressed(t *testing.T) {
	effective := []mapping.Field{claimField("overview",
		mapping.Source{Namespace: "tmdb", Key: "overview"},
		mapping.Source{Namespace: "provA", Key: "synopsis"},
		mapping.Source{Namespace: "provB", Key: "comments"},
	)}
	fields := []AutoField{
		{Provider: "tmdb", Key: "overview", Values: []string{"A plot."}},
		{Provider: "provA", Key: "synopsis", Values: []string{"A plot."}},
		{Provider: "provB", Key: "comments", Values: []string{"A plot."}},
		{Provider: "provA", Key: "filming_locations", Values: []string{"Osaka"}}, // unclaimed
	}
	// `overview` renders canonically, so `rendered` alone would still have caught
	// tmdb:overview — the point is that synopsis and comments are caught too.
	out := AutoRegisterFields(fields, map[string]bool{"overview": true}, ClaimedKeys(effective), nil)

	if len(out) != 1 || out[0].Canonical != "filming_locations" {
		t.Fatalf("only the unclaimed key should auto-register, got %+v", out)
	}
}

func TestAutoRegisterFields_ClaimSuppressionIsProviderScoped(t *testing.T) {
	// provA:rating is an age certificate and is claimed; provB:rating is a 1–10 score
	// and is not. Claiming one must leave the other alone (spec §6.5 S2).
	claimed := ClaimedKeys([]mapping.Field{
		claimField("content_rating", mapping.Source{Namespace: "provA", Key: "rating"}),
	})
	fields := []AutoField{
		{Provider: "provA", Key: "rating", Values: []string{"PG-13"}},
		{Provider: "provB", Key: "rating", Values: []string{"7.4"}},
	}
	out := AutoRegisterFields(fields, nil, claimed, nil)

	if len(out) != 1 || out[0].Canonical != "rating" {
		t.Fatalf("partially-claimed key should still auto-register once, got %+v", out)
	}
	// It carries the *unclaimed* provider's value and provenance only — the claimed
	// provider's value now lives behind the target field's source chip.
	if len(out[0].Values) != 1 || out[0].Values[0] != "7.4" {
		t.Fatalf("want only provB's value, got %v", out[0].Values)
	}
	if len(out[0].Items) != 1 || len(out[0].Items[0].Sources) != 1 || out[0].Items[0].Sources[0] != "provB" {
		t.Fatalf("want provB provenance only, got %+v", out[0].Items)
	}
	if out[0].WinningSource != "provB:rating" {
		t.Fatalf("want provB:rating as winning source, got %q", out[0].WinningSource)
	}
}

// Suppression is a config-level statement about identity, so it cannot vary with
// which source won resolution for the entity being viewed. AutoRegisterFields is
// given no resolution outcome at all, which is what makes that structural rather
// than a rule to remember; this pins the observable half — precedence order within
// the claiming field changes nothing.
func TestAutoRegisterFields_ClaimSuppressionIsUnconditional(t *testing.T) {
	fields := []AutoField{{Provider: "provA", Key: "synopsis", Values: []string{"A plot."}}}

	for _, tc := range []struct {
		name    string
		sources []mapping.Source
	}{
		{"claimed source wins (listed first)", []mapping.Source{
			{Namespace: "provA", Key: "synopsis"},
			{Namespace: "tmdb", Key: "overview"},
		}},
		{"claimed source loses (listed last)", []mapping.Source{
			{Namespace: "tmdb", Key: "overview"},
			{Namespace: "provA", Key: "synopsis"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claimed := ClaimedKeys([]mapping.Field{claimField("overview", tc.sources...)})
			if out := AutoRegisterFields(fields, nil, claimed, nil); len(out) != 0 {
				t.Fatalf("claimed key must not auto-register, got %+v", out)
			}
		})
	}
}

func TestAutoRegisterFields_NoClaims_Unchanged(t *testing.T) {
	// The whole existing F39 surface with an empty/derived-empty claimed set: a
	// mapping made only of file tags claims nothing, so nothing is suppressed.
	effective := []mapping.Field{claimField("title",
		mapping.Source{Namespace: "file", Key: "Title"},
	)}
	fields := []AutoField{
		{Provider: "tmdb", Key: "gender", Values: []string{"Female"}},
		{Provider: "tmdb", Key: "trivia", Values: []string{"t"}},
	}
	claimedOut := AutoRegisterFields(fields, nil, ClaimedKeys(effective), nil)
	nilOut := AutoRegisterFields(fields, nil, nil, nil)

	if len(claimedOut) != 2 {
		t.Fatalf("no provider source claimed → nothing suppressed, got %+v", claimedOut)
	}
	if len(claimedOut) != len(nilOut) || claimedOut[0].Canonical != nilOut[0].Canonical {
		t.Fatalf("empty claimed set must be a no-op: %+v vs %+v", claimedOut, nilOut)
	}
}
