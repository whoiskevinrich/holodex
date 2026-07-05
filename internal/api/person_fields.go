package api

import (
	"net/http"
	"slices"
	"strings"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// Person fields are synthesized in code, not configured (F37 P0-1): unlike
// video fields (operator-shaped via metadata-mappings.yaml), the canonical
// person schema is a registry contract shared with the provider protocol, so
// there is nothing for an operator to remap.

// The person baseline vocabulary ("record", F37 RD4) is the shared record vocab in
// record_vocab.go, reused by studio (F38). personDecisionSource / personizeResolved
// remain as the person-named entry points the F37 tests pin.

// personScalarFields are the provider-backed replace person fields, in
// registry documentation order. name is synthesized separately (the only
// baseline-backed field); aliases is the merge field (RD2); photo is excluded
// — it is delivered as an asset, not a field value.
var personScalarFields = []string{"bio", "birthdate", "deathdate", "nationality", "website"}

// personFields synthesizes the []mapping.Field for person resolution: name
// (record baseline + one candidate per provider), the scalar registry fields,
// then aliases as a merge field. providers is the person-capable provider
// name list; labels come from the registry.
func personFields(providers []string) []mapping.Field {
	fields := make([]mapping.Field, 0, len(personScalarFields)+2)
	fields = append(fields, personField("name", false,
		append([]mapping.Source{{Namespace: "file", Key: "name"}}, providerSources(providers, "name")...)))
	for _, canonical := range personScalarFields {
		fields = append(fields, personField(canonical, false, providerSources(providers, canonical)))
	}
	fields = append(fields, personField("aliases", true, providerSources(providers, "aliases")))
	return fields
}

// personField builds one synthesized field; the raw Sources strings mirror
// ParsedSources so the field round-trips like a parsed YAML one.
func personField(canonical string, multi bool, sources []mapping.Source) mapping.Field {
	raw := make([]string, len(sources))
	for i, s := range sources {
		raw[i] = s.Namespace + ":" + s.Key
	}
	return mapping.Field{
		Canonical:     canonical,
		Label:         registry.Lookup(canonical).Label,
		Sources:       raw,
		ParsedSources: sources,
		Multi:         multi,
	}
}

// providerSources maps each provider to a namespaced source for one field key.
func providerSources(providers []string, key string) []mapping.Source {
	out := make([]mapping.Source, len(providers))
	for i, p := range providers {
		out[i] = mapping.Source{Namespace: p, Key: key}
	}
	return out
}

// personSchema is the provider-independent synthesized schema used for field
// validation. Field identity is static — the provider list only widens
// sources — so it is built once, not per request.
var personSchema = personFields(nil)

// personFieldByCanonical resolves a canonical name against the synthesized
// person schema.
func personFieldByCanonical(canonical string) (mapping.Field, bool) {
	canonical = strings.ToLower(strings.TrimSpace(canonical))
	for _, f := range personSchema {
		if f.Canonical == canonical {
			return f, true
		}
	}
	return mapping.Field{}, false
}

// personDecisionSource maps a person-payload decision source to the internal
// fieldsource grammar (the shared record vocab). Kept as the person-named entry
// point the F37 tests pin.
func personDecisionSource(s string) (string, bool) { return recordDecisionSource(s) }

// personizeResolved converts resolver output to the person payload vocabulary via
// the shared record vocab (record labels, no in_sync). Kept as the person-named
// entry point the F37 tests pin.
func personizeResolved(fields []resolver.ResolvedField) []resolver.ResolvedField {
	return recordizeResolved(fields)
}

// personResolved resolves a person's fields through the unified resolver (F37
// P0-2): the record baseline + shadow enrichment + curation + standing
// decisions, personized to the record vocabulary. Mirrors the getMedia
// preload; degraded reads log and resolve without that layer, as there.
func (h *Handlers) personResolved(r *http.Request, id int64, p *model.Person) []resolver.ResolvedField {
	rows, err := h.repo.EnrichmentForEntity(r.Context(), model.EnrichEntityPerson, id)
	if err != nil {
		h.log.Warn("enrichment for person detail", "id", id, "err", err)
		rows = nil
	}
	var cur resolver.Curation
	if curRows, curErr := h.repo.CurationForEntity(r.Context(), model.EnrichEntityPerson, id); curErr != nil {
		h.log.Warn("curation for person detail", "id", id, "err", curErr)
	} else {
		cur = curationFromRows(curRows)
	}
	var dec resolver.Decisions
	if decRows, decErr := h.repo.DecisionsForEntity(r.Context(), model.EnrichEntityPerson, id); decErr != nil {
		h.log.Warn("decisions for person detail", "id", id, "err", decErr)
	} else {
		dec = decisionsFromRows(decRows)
	}
	fields := personFields(h.personProviders(rows))
	resolved := resolver.ResolveFields(resolver.NewPersonBaseline(p), enrichmentFromRows(rows), cur, fields, h.resolveOptions(dec))
	return h.appendAutoRegistered(r.Context(), rows, personizeResolved(resolved))
}

// personProviders lists the provider namespaces the synthesized person fields
// consult: the registry's person-capable providers plus any provider already
// matched to this person — so stored shadow values keep rendering even if
// their registry entry is later disabled, consistent with the matched-provider
// decision precondition, which also reads the rows.
func (h *Handlers) personProviders(rows []repo.EnrichmentRow) []string {
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	if h.enrich != nil {
		for _, s := range h.enrich.Sources() {
			if slices.Contains(s.EntityTypes, model.EnrichEntityPerson) {
				add(s.Name)
			}
		}
	}
	for _, row := range rows {
		add(row.Provider)
	}
	return out
}
