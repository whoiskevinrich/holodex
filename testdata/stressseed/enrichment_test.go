package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// TestEnrichRungsSeedTheirNamespaces is the dimension's central claim: rung `05`
// really does leave five DIFFERENT namespaces holding a value, because that is
// the only shape the ADR-051 chip row renders as five chips. Four providers that
// agreed would fold to one chip, and the manifest would be claiming a conflict
// the page does not show.
func TestEnrichRungsSeedTheirNamespaces(t *testing.T) {
	dir := t.TempDir()
	entries, database := seedInto(t, dir)
	r := repo.New(database)

	seen := 0
	for _, e := range entries {
		if e.Dimension != "enrich" {
			continue
		}
		seen++
		want := e.Axes.Video.Namespaces

		rows, err := r.EnrichmentForEntity(context.Background(), model.EnrichEntityVideo, e.ID)
		if err != nil {
			t.Fatalf("read enrichment for %d: %v", e.ID, err)
		}

		providers := map[string]bool{}
		for _, row := range rows {
			providers[row.Provider] = true
		}
		if len(providers) != want {
			t.Errorf("%s/%s (id %d): %d namespaces stored, manifest says %d",
				e.Dimension, e.Variant, e.ID, len(providers), want)
		}

		// Per field, every stored value must be distinct across namespaces — the
		// fold rule. Checked per field rather than globally: two providers sharing a
		// *tagline* is the failure, two providers both having a tagline is the point.
		byField := map[string]map[string]string{}
		for _, row := range rows {
			v := strings.Join(row.Values, "\n")
			if byField[row.FieldKey] == nil {
				byField[row.FieldKey] = map[string]string{}
			}
			if other, dup := byField[row.FieldKey][v]; dup {
				t.Errorf("%s/%s: %s and %s both store %q for %q — the chip row folds by "+
					"value, so these render as one chip", e.Dimension, e.Variant,
					other, row.Provider, v, row.FieldKey)
			}
			byField[row.FieldKey][v] = row.Provider
		}
	}
	if seen == 0 {
		t.Fatal("no enrich entries in the manifest — the dimension did not run")
	}
}

// TestEnrichZeroRungStoresNothing is D2 for this dimension. The zero rung is not
// decoration: a single-source field and a five-source one take different branches
// in SourceBadge (isMultiSource), so "no providers" is a layout state of its own.
func TestEnrichZeroRungStoresNothing(t *testing.T) {
	dir := t.TempDir()
	entries, database := seedInto(t, dir)
	r := repo.New(database)

	for _, e := range entries {
		if e.Dimension != "enrich" || e.Variant != "00" {
			continue
		}
		rows, err := r.EnrichmentForEntity(context.Background(), model.EnrichEntityVideo, e.ID)
		if err != nil {
			t.Fatalf("read enrichment for %d: %v", e.ID, err)
		}
		if len(rows) != 0 {
			t.Fatalf("the zero rung (id %d) holds %d enrichment rows", e.ID, len(rows))
		}
		return
	}
	t.Fatal("no zero rung in the enrich dimension")
}

// TestNoOtherDimensionIsEnriched is the D3 guard. The enrichment axis has to be
// the ONLY thing that varies here — if the baseline ever gained a non-zero
// namespace count, every text and cardinality rung would grow a provider chip
// row too, and a layout failure on those pages could no longer be attributed.
func TestNoOtherDimensionIsEnriched(t *testing.T) {
	dir := t.TempDir()
	entries, database := seedInto(t, dir)
	r := repo.New(database)

	for _, e := range entries {
		if e.Dimension == "enrich" || e.Entity != kindVideo {
			continue
		}
		rows, err := r.EnrichmentForEntity(context.Background(), model.EnrichEntityVideo, e.ID)
		if err != nil {
			t.Fatalf("read enrichment for %d: %v", e.ID, err)
		}
		if len(rows) != 0 {
			t.Errorf("%s/%s (id %d) carries %d enrichment rows — only the enrich dimension "+
				"may, or a failure there cannot be attributed to its own axis",
				e.Dimension, e.Variant, e.ID, len(rows))
		}
	}
}

// TestEnrichPlanRefusesAMultiField covers the silent-failure case. A merge field
// never reaches replaceMarkers, so it has no candidate list and no chip row at
// all — the namespaces would be stored and simply invisible, which is exactly the
// "manifest claims what the page does not render" failure the ladder exists to
// avoid.
func TestEnrichPlanRefusesAMultiField(t *testing.T) {
	body := testMappingsYAML + `  - canonical: genres
    multi: true
    sources:
      - file:Genre
      - alpha:genres
`
	_, err := loadFields(writeMappings(t, "multi-provider", body), testPersonasPath(t), demands(ladder))
	if err == nil {
		t.Fatal("a multi field with provider sources was accepted")
	}
	for _, want := range []string{"genres", "multi: true", "chip row"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal should mention %q, got: %v", want, err)
		}
	}
}

// TestEnrichPlanRefusesAMappingWithNoProviderSources is the video surface's
// particular trap: a person's chip row grows from stored rows alone, so it is
// tempting to assume a video's does too. It does not — a video's field set comes
// from the mapping — and the failure mode is a fixture that seeds rows nothing
// ever renders.
func TestEnrichPlanRefusesAMappingWithNoProviderSources(t *testing.T) {
	body := `fields:
  - canonical: actors
    multi: true
    sources:
      - Cast
  - canonical: studio
    multi: true
    sources:
      - Publisher
  - canonical: overview
    sources:
      - Comment
`
	_, err := loadFields(writeMappings(t, "no-providers", body), testPersonasPath(t), demands(ladder))
	if err == nil {
		t.Fatal("a mapping naming no provider namespace was accepted")
	}
	if !strings.Contains(err.Error(), "sources:") {
		t.Errorf("the refusal should show the mapping it wants, got: %v", err)
	}
}

// TestEnrichPlanRefusesTooFewPersonas keeps the ladder and the persona table in
// step from the other side: raising the top rung past what personas.json declares
// has to fail loudly rather than quietly seeding fewer namespaces than the
// manifest goes on to claim.
func TestEnrichPlanRefusesTooFewPersonas(t *testing.T) {
	plan, err := loadEnrichPlan(loadTestMapping(t), testPersonasPath(t), "mappings.yaml", 0)
	if err != nil {
		t.Fatalf("loadEnrichPlan: %v", err)
	}
	tooMany := plan.namespaces() + 1

	_, err = loadEnrichPlan(loadTestMapping(t), testPersonasPath(t), "mappings.yaml", tooMany)
	if err == nil {
		t.Fatalf("a rung of %d was accepted with only %d personas", tooMany, plan.namespaces())
	}
	if !strings.Contains(err.Error(), "precedence") {
		t.Errorf("the refusal should name the group it counted, got: %v", err)
	}
}

// TestShippedSourcesRegisterEveryPersona is the drift guard between the two
// committed files. personas.json is what the stub serves; sources.yaml is what
// the server dials. A persona missing from the registry is a provider that exists
// and is never asked anything — it would look exactly like a persona that
// answered nothing, which is the kind of failure that costs an afternoon.
func TestShippedSourcesRegisterEveryPersona(t *testing.T) {
	personas := loadShippedPersonas(t)

	raw, err := os.ReadFile(filepath.Join("..", "..", stressSourcesPath))
	if err != nil {
		t.Fatalf("read %s: %v", stressSourcesPath, err)
	}
	var file struct {
		Sources []struct {
			Name        string   `yaml:"name"`
			BaseURL     string   `yaml:"base_url"`
			Enabled     bool     `yaml:"enabled"`
			EntityTypes []string `yaml:"entity_types"`
		} `yaml:"sources"`
	}
	if err := yaml.Unmarshal(raw, &file); err != nil {
		t.Fatalf("parse %s: %v", stressSourcesPath, err)
	}

	registered := map[string]string{}
	for _, src := range file.Sources {
		if !src.Enabled {
			t.Errorf("source %q is registered but disabled — the fixture's providers are "+
				"meant to be reachable on boot", src.Name)
		}
		registered[src.Name] = src.BaseURL
	}

	for _, p := range personas {
		base, ok := registered[p.Name]
		if !ok {
			t.Errorf("persona %q is served by the stub but absent from %s", p.Name, stressSourcesPath)
			continue
		}
		// The path suffix is what addresses the persona: core concatenates base_url
		// with "/resolve", so a base_url pointing at the wrong prefix silently serves
		// a different provider's values.
		if want := "/p/" + p.Slug; !strings.HasSuffix(base, want) {
			t.Errorf("persona %q is registered at %q, which does not address it (want suffix %q)",
				p.Name, base, want)
		}
	}
	if len(registered) != len(personas) {
		t.Errorf("%s registers %d sources for %d personas — an entry names a provider the "+
			"stub does not serve", stressSourcesPath, len(registered), len(personas))
	}
}

// TestShippedPersonasDisagree asserts the property the whole precedence layer
// rests on, at the table itself rather than only after a seed: within a field,
// no two precedence personas may share a value.
func TestShippedPersonasDisagree(t *testing.T) {
	var precedence []persona
	for _, p := range loadShippedPersonas(t) {
		if p.Group == precedenceGroup {
			precedence = append(precedence, p)
		}
	}
	if len(precedence) < 5 {
		t.Fatalf("only %d precedence personas; the ticket asks for at least 5", len(precedence))
	}

	byField := map[string]map[string]string{}
	for _, p := range precedence {
		for field, values := range p.Values {
			v := strings.Join(values, "\n")
			if byField[field] == nil {
				byField[field] = map[string]string{}
			}
			if other, dup := byField[field][v]; dup {
				t.Errorf("%s and %s both give %q for %q — the chip row folds by value, so "+
					"they would render as one chip", other, p.Name, v, field)
			}
			byField[field][v] = p.Name
		}
	}
}

// loadTestMapping parses the minimal test mapping, for the plan-level checks that
// have no need to build a fixture.
func loadTestMapping(t *testing.T) *mapping.Mappings {
	t.Helper()
	m, err := mapping.Load(testMappingsPath(t))
	if err != nil {
		t.Fatalf("load test mapping: %v", err)
	}
	return m
}

func loadShippedPersonas(t *testing.T) []persona {
	t.Helper()
	raw, err := os.ReadFile(testPersonasPath(t))
	if err != nil {
		t.Fatalf("read %s: %v", personasPath, err)
	}
	var file struct {
		Personas []persona `json:"personas"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("parse %s: %v", personasPath, err)
	}
	return file.Personas
}

// TestEncodedNamesDistinguishRungsWithinADimension is the guard that was missing
// when the enrich dimension was added: `namespaces` went into spec but not into
// encodeName, so all three rungs rendered the SAME h1 — "STRESS people=02
// tags=03 studios=01 text=plain image=none" — and the fixture silently stopped
// keeping the promise its names exist to keep, that an entity's coordinate is
// readable off the page without opening the manifest.
//
// Nothing caught it. A video is identified by file path, so colliding names cost
// no rows and broke no test; the damage was only to legibility, which is exactly
// the kind of failure that survives a review.
//
// The check is WITHIN a dimension, not across the table, and the difference is
// D3 rather than laxity: every rung mutates the same neutral baseline in one
// axis, so a rung whose value happens to BE the baseline's encodes to the
// baseline coordinate — `studios/01`, `videoimage/none` and `enrich/00` all
// legitimately do. Those are three entities that genuinely share a coordinate,
// told apart by their id. Two rungs of ONE dimension sharing one is the real
// failure: it means that dimension's own axis is missing from encodeName, so
// nothing on the page says which rung it is.
//
// Dimensions that own the title are exempt — their entity IS the palette string,
// and encodeName is only what the manifest and the carrier video carry.
func TestEncodedNamesDistinguishRungsWithinADimension(t *testing.T) {
	for _, dim := range ladder {
		if dim.ownsTitle {
			continue
		}
		seen := map[string]string{}
		for _, rg := range dim.rungs {
			sp := baseline()
			rg.apply(&sp)
			name := encodeName(dim.entity, sp)
			if prev, dup := seen[name]; dup {
				t.Errorf("%s rungs %q and %q both encode to %q — this dimension's axis is "+
					"missing from encodeName, so nothing on the page says which rung it is",
					dim.key, prev, rg.variant, name)
			}
			seen[name] = rg.variant
		}
	}
}
