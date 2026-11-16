package resolver

import (
	"strings"

	"holodex/internal/fieldsource"
	"holodex/internal/mapping"
	"holodex/internal/registry"
)

// Tier names (FacetScore.Tier / JSON).
const (
	TierMissing  = "missing"
	TierProvider = "provider"
	TierCurated  = "curated"
)

// tier names a facet's resolved provenance for the breakdown panel (F55). v2
// (F65, ADR-099 D1) dropped the per-tier scoring weight — presence is binary, so
// the tier is display-only — but the three values are still compared by == below.
type tier string

const (
	missingTier  tier = TierMissing
	providerTier tier = TierProvider
	curatedTier  tier = TierCurated
)

// Completeness is one entity's scored facets — each one's tier and status, for the
// detail page's field list and the owner queue — computed as a pure post-pass over its
// resolved fields (ADR-081 D3). The score, extras number and actionability ratio it once
// carried were retired with the ring and the Completeness sorts (F90 P0-6).
type Completeness struct {
	Facets []FacetScore `json:"facets"`
}

// Missing returns the scored (critical / nice_to_have), applicable facets the
// entity lacks — the rows entity_completeness_missing stores for the "Missing
// facet" chip and its counts (ADR-099 D3). Optional and not-applicable facets
// are never missing.
func (c Completeness) Missing() []FacetScore {
	var out []FacetScore
	for _, f := range c.Facets {
		if f.Tier == TierMissing && !f.NotApplicable && f.Criticality != registry.CriticalityOptional {
			out = append(out, f)
		}
	}
	return out
}

// FacetScore is one scored facet's tier/status for the completeness breakdown
// panel (F55). A not-applicable facet is still listed, so the UI can render its
// muted status, but it is never missing or actionable.
type FacetScore struct {
	Canonical     string `json:"canonical"`
	Label         string `json:"label"`
	Criticality   string `json:"criticality"` // registry.CriticalityCritical | CriticalityNiceToHave
	Tier          string `json:"tier"`        // TierMissing | TierProvider | TierCurated
	NotApplicable bool   `json:"not_applicable,omitempty"`
	// Actionable is true only for a missing (non-excluded, non-not-applicable)
	// facet that has a cached unapplied provider candidate.
	Actionable bool `json:"actionable,omitempty"`
	// Provider is a provider namespace (e.g. "tmdb"), set in either of two
	// cases: when Actionable, the candidate it refers to, so the remediation
	// queue (F55.7) can show which provider it would come from and apply it
	// via setFieldDecision without a second lookup into resolved fields the API
	// layer doesn't retain; or when Tier is TierProvider, the namespace that
	// actually won the field, so the completeness breakdown panel (F55.13,
	// design handoff DD7) can render a ProvenanceBadge naming the resolved
	// source instead of a bare "Provider" label.
	Provider string `json:"provider,omitempty"`
	// Curatable marks a plain-text replace field — single-value, default display —
	// the owner can set from nothing. The media page's `#field-<canonical>` deep
	// link renders a missing curatable facet as an empty SourceBadge row (F60
	// RD11); image, url, long-text and merge fields have their own editors and
	// are never synthesised that way.
	Curatable bool `json:"curatable,omitempty"`
}

// Complete lists one entity's scored facets from its configured fields, already-resolved
// values, and not-applicable exclusions (F55, ADR-081 D3).
//
// fields is the same field list passed to ResolveFields — Complete needs it, not
// just resolved, because ResolveFields drops an empty, undecided field entirely
// (spec RD-adjacent behavior predating F55): a genuinely missing scored facet may
// have no row in resolved at all, and leaving it out would hide it from the queue. notApplicable is keyed by
// canonical, same casing FacetsNotApplicableForEntity returns.
//
// Presence is binary and read entirely off ResolvedField.WinningSource — a field
// absent from resolved (WinningSource "") is missing, anything else is present.
// The provider/curated distinction survives only as FacetScore.Tier for the
// panel's ProvenanceBadge. A field with no registry.FieldDef.Criticality tag
// (including every Computed field, D1's invariant) is skipped entirely — never
// listed.
func Complete(fields []mapping.Field, resolved []ResolvedField, notApplicable map[string]bool) Completeness {
	byCanonical := make(map[string]ResolvedField, len(resolved))
	for _, rf := range resolved {
		byCanonical[rf.Canonical] = rf
	}

	facets := make([]FacetScore, 0, len(fields))

	for _, f := range fields {
		def := registry.Lookup(f.Canonical)
		if def.Criticality == "" {
			continue
		}
		rf := byCanonical[f.Canonical] // zero value (WinningSource=="") when never resolved
		t := classifyTier(rf.WinningSource)
		_, display := LabelAndDisplay(f)
		fs := FacetScore{
			Canonical:   f.Canonical,
			Label:       def.Label,
			Criticality: def.Criticality,
			Tier:        string(t),
			Curatable:   !f.Multi && !f.Merge && display == "",
		}
		if notApplicable[f.Canonical] {
			fs.NotApplicable = true
			facets = append(facets, fs)
			continue
		}
		if def.Criticality == registry.CriticalityOptional {
			// Listed for the SPA (label, tier, Curatable — the deep-linked empty
			// row needs them) but never scored, counted as missing or actionable.
			facets = append(facets, fs)
			continue
		}

		switch t {
		case missingTier:
			if provider, ok := actionableCandidate(rf); ok {
				fs.Actionable = true
				fs.Provider = provider
			}
		case providerTier:
			fs.Provider = winningNamespace(rf.WinningSource)
		}
		facets = append(facets, fs)
	}

	return Completeness{Facets: facets}
}

// classifyTier maps a resolved field's winning source to its completeness tier
// (ADR-081 D3 tier table). It delegates the file/manual-vs-provider distinction
// to fieldsource.ForNamespace — the package that already owns this grammar
// (fieldsource.go) — rather than re-deriving it here.
func classifyTier(winningSource string) tier {
	if winningSource == "" {
		return missingTier
	}
	switch fieldsource.ForNamespace(winningNamespace(winningSource)) {
	case fieldsource.File, fieldsource.Manual:
		return curatedTier
	default:
		return providerTier
	}
}

// winningNamespace extracts the namespace portion of a "namespace:key"
// WinningSource string (the provider name for a provider-tier facet).
func winningNamespace(winningSource string) string {
	ns, _, _ := strings.Cut(winningSource, ":")
	return ns
}

// actionableCandidate reports the provider namespace of a missing replace
// field's cached, non-empty candidate sitting unapplied (F55 actionability),
// if any — reading the same Candidates list F36's SourceSelect renders
// (ADR-051) rather than re-deriving availability from raw enrichment data, so
// the two can't diverge on what counts as "available." Merge fields carry no
// Candidates (F36 is replace-only, RD1): every value any matched provider
// supplies is already merged into the field, so a missing merge field already
// means no candidate exists anywhere, and correctly reports not actionable.
func actionableCandidate(rf ResolvedField) (string, bool) {
	for _, c := range rf.Candidates {
		if c.Source != fieldsource.File && c.Value != "" {
			return c.Provider, true
		}
	}
	return "", false
}
