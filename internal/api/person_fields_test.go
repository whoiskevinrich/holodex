package api

import (
	"slices"
	"testing"

	"holodex/internal/resolver"
)

// --- Synthesized person schema (F37 P0-1) ------------------------------------------

func TestPersonFields_Synthesis(t *testing.T) {
	fields := personFields([]string{"tmdb", "fake"})

	var canonicals []string
	for _, f := range fields {
		canonicals = append(canonicals, f.Canonical)
	}
	// No `aliases`: retired in F58 (ADR-088 D1). A person has no merge field in the
	// default configuration now — an operator can create one by promoting a chips-rendered
	// provider key, which is what the merge-field tests elsewhere in this package do.
	want := []string{"name", "bio", "birthdate", "deathdate", "nationality", "website"}
	if !slices.Equal(canonicals, want) {
		t.Fatalf("canonicals = %v, want %v", canonicals, want)
	}

	// name: baseline source first, then one name source per provider.
	if !slices.Equal(fields[0].Sources, []string{"file:name", "tmdb:name", "fake:name"}) {
		t.Errorf("name sources = %v", fields[0].Sources)
	}
	// Scalars carry provider sources only (empty baseline is the personBaseline's
	// claim, not a configured source) and are replace fields.
	if !slices.Equal(fields[1].Sources, []string{"tmdb:bio", "fake:bio"}) || fields[1].Multi {
		t.Errorf("bio field wrong: %+v", fields[1])
	}
	// Nothing synthesized is a merge field any more. Asserted rather than dropped: a
	// stray Multi here would put an empty second list of names back on the person page.
	for _, f := range fields {
		if f.Multi {
			t.Errorf("person has no synthesized merge field after F58, got %+v", f)
		}
	}
	// Labels come from the registry ("photo" must not be synthesized — asset only).
	if fields[2].Label != "Born" || fields[1].Label != "Bio" {
		t.Errorf("registry labels not applied: %q %q", fields[1].Label, fields[2].Label)
	}
	if _, ok := personFieldByCanonical("photo"); ok {
		t.Error("photo must not be a synthesized field")
	}
}

// --- The record⇄file edge mapping (RD4 / spec Open Q1) -----------------------------

func TestPersonDecisionSource_EdgeMapping(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"record", "file", true},   // the person baseline token maps to the internal grammar
		{"manual", "manual", true}, // passes through
		{"provider:tmdb", "provider:tmdb", true},
		{"file", "", false},      // internal token is not person vocabulary
		{"provider:", "", false}, // empty provider name
		{"bogus", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := personDecisionSource(c.in)
		if got != c.want || ok != c.wantOK {
			t.Errorf("personDecisionSource(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.wantOK)
		}
	}
}

func TestPersonizeResolved(t *testing.T) {
	inSync := true
	fields := []resolver.ResolvedField{{
		Canonical:     "name",
		Values:        []string{"Alice"},
		Items:         []resolver.ResolvedValue{{Value: "Alice", Sources: []string{"file"}}},
		WinningSource: "file:name",
		Decision:      &resolver.FieldDecision{Source: "file"},
		InSync:        &inSync,
		Candidates: []resolver.FieldCandidate{
			{Source: "file", Value: "Alice"},
			{Source: "provider:tmdb", Provider: "tmdb", Value: "Alicia"},
		},
	}, {
		Canonical:     "bio",
		Values:        []string{"text"},
		Items:         []resolver.ResolvedValue{{Value: "text", Sources: []string{"tmdb"}}},
		WinningSource: "tmdb:bio",
		Decision:      &resolver.FieldDecision{Source: "provider:tmdb", Standing: true},
		InSync:        &inSync,
	}}

	got := personizeResolved(fields)

	name := got[0]
	if name.InSync != nil {
		t.Error("in_sync must be stripped from person fields")
	}
	if name.Decision.Source != "record" || name.WinningSource != "record:name" {
		t.Errorf("file tokens must read record: %+v", name)
	}
	if name.Candidates[0].Source != "record" || name.Candidates[1].Source != "provider:tmdb" {
		t.Errorf("candidate sources wrong: %+v", name.Candidates)
	}
	if name.Items[0].Sources[0] != "record" {
		t.Errorf("item provenance must read record: %+v", name.Items)
	}

	bio := got[1]
	if bio.InSync != nil || bio.Decision.Source != "provider:tmdb" || bio.WinningSource != "tmdb:bio" {
		t.Errorf("provider tokens must pass through untouched: %+v", bio)
	}
}
