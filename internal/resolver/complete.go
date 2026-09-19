package resolver

import (
	"math"
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

// Completeness is the F55 completeness score plus the separate actionability
// signal for one entity, computed as a pure post-pass over its resolved fields
// (ADR-081 D3) — mirrors Derive's shape, but needs no clock since nothing here is
// time-based. v2 (F65, ADR-099 D1): the score is the required band alone and
// extras is a separate number; the two are never combined.
type Completeness struct {
	// Required is round(100 × present / applicable) over the entity's critical
	// facets — THE score: the panel headline, the card ring, the primary sort
	// key. nil when the entity has no applicable critical facet (studios; or a
	// video with every critical facet marked not-applicable) — never a vacuous
	// 100. Serialized as `score` so the detail payload's key is unchanged from
	// v1 (ADR-099 D5).
	Required *int `json:"score"`
	// Extras is the same ratio over the nice_to_have band: the sort tiebreaker
	// and the ring's overfill, never blended into Required. nil when there is
	// no applicable nice_to_have facet.
	Extras *int `json:"extras"`
	// Actionability is the fraction of missing scored facets that have a cached,
	// unapplied provider candidate — nil (not zero) when there are no missing
	// scored facets, since the ratio is undefined rather than zero.
	Actionability *float64     `json:"actionability,omitempty"`
	Facets        []FacetScore `json:"facets"`
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
// muted status, but it is excluded from both bands and from Actionability.
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

// Complete computes the completeness bands and actionability signal for one
// entity from its configured fields, already-resolved values, and not-applicable
// exclusions (F55, ADR-081 D3; formula per ADR-099 D1).
//
// fields is the same field list passed to ResolveFields — Complete needs it, not
// just resolved, because ResolveFields drops an empty, undecided field entirely
// (spec RD-adjacent behavior predating F55): a genuinely missing scored facet may
// have no row in resolved at all, and dropping it from the band's denominator
// would silently inflate every entity's completeness. notApplicable is keyed by
// canonical, same casing FacetsNotApplicableForEntity returns.
//
// Presence is binary and read entirely off ResolvedField.WinningSource — a field
// absent from resolved (WinningSource "") is missing, anything else is present.
// The provider/curated distinction survives only as FacetScore.Tier for the
// panel's ProvenanceBadge. A field with no registry.FieldDef.Criticality tag
// (including every Computed field, D1's invariant) is skipped entirely — never
// scored, never listed.
func Complete(fields []mapping.Field, resolved []ResolvedField, notApplicable map[string]bool) Completeness {
	byCanonical := make(map[string]ResolvedField, len(resolved))
	for _, rf := range resolved {
		byCanonical[rf.Canonical] = rf
	}

	var required, extras band
	var missing, actionable int
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

		b := &extras
		if def.Criticality == registry.CriticalityCritical {
			b = &required
		}
		b.applicable++
		switch t {
		case missingTier:
			missing++
			if provider, ok := actionableCandidate(rf); ok {
				fs.Actionable = true
				fs.Provider = provider
				actionable++
			}
		case providerTier:
			b.present++
			fs.Provider = winningNamespace(rf.WinningSource)
		default:
			b.present++
		}
		facets = append(facets, fs)
	}

	var actionability *float64
	if missing > 0 {
		a := float64(actionable) / float64(missing)
		actionability = &a
	}
	return Completeness{Required: required.score(), Extras: extras.score(), Actionability: actionability, Facets: facets}
}

// band tallies one criticality band's applicable and present facet counts
// (ADR-099 D1): required over critical facets, extras over nice_to_have.
type band struct {
	applicable, present int
}

// score is round(100 × present / applicable), or nil when the band has no
// applicable facet — the spec's null rule, never a vacuous 100.
func (b band) score() *int {
	if b.applicable == 0 {
		return nil
	}
	s := int(math.Round(100 * float64(b.present) / float64(b.applicable)))
	return &s
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
