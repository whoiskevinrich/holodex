package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// The enrichment half of the fixture (HOLODEX-348) seeds the ADR-090 *precedence*
// layer: several provider namespaces holding DIFFERENT values for one field, so
// the ADR-051 SourceBadge chip row has something to choose between.
//
// Why seed it at all, when the stub can be enriched from by hand? Because the
// epic's regression mechanism is a measurable invariant at a stable address (spec
// D6), and "five competing chips on media 900" is only an address if it is there
// after `go run ./testdata/stressseed` and not after somebody remembered to click
// Enrich five times. The adoption layer is the opposite case and is deliberately
// NOT seeded: a candidate list only exists during a resolve, so it is reachable
// only by opening the picker against a running stub.
//
// The seeder writes only entity_enrichment. That is the whole dependency —
// verified against the read path: personProviders/studioProviders/filmProviders
// union the providers named by stored rows, and for a video the mapping's
// `sources:` list names them. No provider registration table exists, a missing
// brand icon degrades to a monogram, and field hints only affect auto-registered
// (non-canonical) keys. So a bare UpsertEnrichment is enough to make a namespace
// render, with or without the stub running.

// personasPath is the table the stub serves and the seeder seeds, relative to the
// repo root the seeder is run from. One file rather than two copies: a seeded
// value that disagreed with the stub's would be overwritten the moment anyone hit
// Refresh, and the fixture would have been claiming something the page then
// stopped showing.
const personasPath = "testdata/enrich-stub/personas.json"

// precedenceGroup is the only group the seeder cares about. The adoption and
// fault personas exist to be *called*, not stored — a 5xx has no value to seed,
// and a candidate list is not a field.
const precedenceGroup = "precedence"

// persona is one fake provider as personas.json declares it. Only the fields the
// seeder needs are decoded; the stub owns the rest (icons, faults, candidates).
type persona struct {
	Slug   string              `json:"slug"`
	Name   string              `json:"name"`
	Group  string              `json:"group"`
	Values map[string][]string `json:"values"`
}

// enrichPlan is what the ladder's namespace rungs are built from: an ordered list
// of namespaces, and the canonical fields they are allowed to disagree about.
//
// Order is the file's order and is load-bearing — rung `01` must always be the
// same namespace, or an assertion written against one run addresses a different
// provider on the next.
type enrichPlan struct {
	personas []persona
	fields   []string
}

// namespaces is how many competing namespaces this plan can express, which is
// what the ladder's top rung is checked against.
func (p enrichPlan) namespaces() int { return len(p.personas) }

// loadEnrichPlan reads the persona table and works out which canonical fields the
// configured mapping actually lets those namespaces compete over.
//
// The fields are DERIVED from the mapping rather than named here, so adding a
// provider source to mappings.yaml is all it takes to widen the fixture — the
// same property the ladder table has, and for the same reason: a list written in
// two places is a list that goes stale in one of them.
func loadEnrichPlan(m *mapping.Mappings, personasFile, mappingsPath string, want int) (enrichPlan, error) {
	raw, err := os.ReadFile(personasFile)
	if err != nil {
		return enrichPlan{}, fmt.Errorf(
			"cannot read the fake-provider table at %s: %w.\n"+
				"It is committed beside the stub that serves it; run the seeder from the\n"+
				"repository root", personasFile, err)
	}
	var file struct {
		Personas []persona `json:"personas"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return enrichPlan{}, fmt.Errorf("parse %s: %w", personasFile, err)
	}

	plan := enrichPlan{}
	for _, p := range file.Personas {
		if p.Group == precedenceGroup {
			plan.personas = append(plan.personas, p)
		}
	}
	if plan.namespaces() < want {
		return enrichPlan{}, fmt.Errorf(
			"the ladder has a namespaces rung at %d, but %s declares only %d personas in\n"+
				"group %q. Add another persona there (and a matching entry in\n"+
				"testdata/stressseed/sources.yaml), or lower the rung",
			want, personasFile, plan.namespaces(), precedenceGroup)
	}

	plan.fields, err = enrichableFields(m, plan.personas, mappingsPath)
	if err != nil {
		return enrichPlan{}, err
	}
	if want > 0 && len(plan.fields) == 0 {
		return enrichPlan{}, fmt.Errorf(
			"the ladder has a namespaces rung at %d, but no field in %s names any of the\n"+
				"fixture's provider namespaces (%s) in its `sources:` list.\n"+
				"A video's field set comes from the mapping, so a provider that is not named\n"+
				"there is never a candidate and the chip row stays single-source. Add e.g.:\n\n"+
				"  - canonical: tagline\n    sources:\n      - file:Tagline\n      - %s:tagline\n",
			want, mappingsPath, strings.Join(namesOf(plan.personas), ", "), plan.personas[0].Name)
	}
	return plan, nil
}

// enrichableFields returns every canonical field the mapping lets these
// namespaces compete over, in mapping order.
//
// Two things disqualify a field, and both are silent failures rather than errors
// if left unchecked — the field simply renders without a chip row, and the
// manifest would claim a conflict the page does not show:
//
//   - a merge field. resolveField only builds a candidate list for a REPLACE
//     field (replaceMarkers is called under `!rf.Multi`), so a `multi: true`
//     field has no chips at all, however many namespaces hold values.
//   - a namespace that has no value for it. The chip row folds by value, so a
//     namespace contributing nothing is not a competitor.
//
// A field named by *some* of the namespaces is kept, and the seeder writes only
// the ones that have a value — that is a legitimate partial conflict, not a
// mistake, so it is not worth refusing over.
func enrichableFields(m *mapping.Mappings, personas []persona, mappingsPath string) ([]string, error) {
	byName := map[string]bool{}
	for _, p := range personas {
		byName[p.Name] = true
	}

	var out []string
	for _, f := range m.Fields() {
		var claimed []string
		for _, src := range f.ParsedSources {
			if byName[src.Namespace] {
				claimed = append(claimed, src.Namespace)
			}
		}
		if len(claimed) == 0 {
			continue
		}
		if f.Multi {
			return nil, fmt.Errorf(
				"field %q in %s names provider namespaces (%s) but is `multi: true`.\n"+
					"The resolver only builds a candidate list for a replace field, so a merge\n"+
					"field renders no chip row at all and the namespaces would be invisible.\n"+
					"Drop `multi: true` from that field, or drop its provider sources",
				f.Canonical, mappingsPath, strings.Join(dedupe(claimed), ", "))
		}
		out = append(out, f.Canonical)
	}
	return out, nil
}

// seedEnrichment writes one video's competing namespaces: the first n personas,
// each storing its own value for every field the plan found.
//
// The rows go in through repo.UpsertEnrichment rather than raw INSERTs, for the
// same reason every other entity here goes through the repo API — it is the write
// path the app itself uses, so the fixture cannot drift from what an actual enrich
// would have produced.
func seedEnrichment(ctx context.Context, r *repo.Repo, plan enrichPlan, videoID int64, n int) error {
	if n > plan.namespaces() {
		return fmt.Errorf("namespaces rung %d exceeds the %d personas in %s",
			n, plan.namespaces(), personasPath)
	}
	for _, p := range plan.personas[:n] {
		fields := map[string][]string{}
		for _, canonical := range plan.fields {
			if v, ok := p.Values[canonical]; ok && len(v) > 0 {
				fields[canonical] = v
			}
		}
		// Refuse rather than skip. A persona that contributes nothing still counts
		// toward the rung, so continuing here would store four namespaces on an
		// entity whose manifest entry says five — the fixture stating a number the
		// page does not render, which is the one failure mode the addressing scheme
		// exists to prevent.
		if len(fields) == 0 {
			return fmt.Errorf(
				"persona %q has no value for any of the enrichable fields (%s), so rung %d\n"+
					"would store fewer namespaces than the manifest claims. Give it one in %s,\n"+
					"or move it out of group %q",
				p.Name, strings.Join(plan.fields, ", "), n, personasPath, precedenceGroup)
		}
		// A non-empty external_id is what the "linked" provider chip reads, and it
		// is the id the stub answers to, so a Refresh against the running stub
		// re-fetches the same record rather than reopening the picker.
		externalID := fmt.Sprintf("%s:608", p.Name)
		if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, videoID, p.Name, externalID, fields); err != nil {
			return fmt.Errorf("seed enrichment %s for video %d: %w", p.Name, videoID, err)
		}
	}
	return nil
}

func namesOf(personas []persona) []string {
	out := make([]string, 0, len(personas))
	for _, p := range personas {
		out = append(out, p.Name)
	}
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
