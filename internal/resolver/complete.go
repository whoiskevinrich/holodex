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

// tier pairs a facet's source-trust weight with its display name (F55, ADR-081 D3
// tier table) so the two can never drift apart under a future edit — classifyTier
// returns exactly one of the three package vars below, comparable by ==.
type tier struct {
	weight float64
	name   string
}

var (
	missingTier  = tier{0.0, TierMissing}
	providerTier = tier{0.7, TierProvider}
	curatedTier  = tier{1.0, TierCurated}
)

// Criticality weights (spec § Scoring model, facet weight/tier table). Distinct
// from registry.CriticalityCritical/CriticalityNiceToHave, which name the weight
// class; these are the numeric weights the formula applies for each class.
const (
	weightCritical   = 3
	weightNiceToHave = 1
)

// Completeness is the F55 completeness score plus the separate actionability
// signal for one entity, computed as a pure post-pass over its resolved fields
// (ADR-081 D3) — mirrors Derive's shape, but needs no clock since nothing here is
// time-based.
type Completeness struct {
	// Score is round(100 * Σ(weight*tier) / Σ(weight)) over the entity's scored,
	// non-not-applicable facets. 0 when there are none to score.
	Score int `json:"score"`
	// Actionability is the fraction of missing scored facets that have a cached,
	// unapplied provider candidate — nil (not zero) when there are no missing
	// scored facets, since the ratio is undefined rather than zero.
	Actionability *float64     `json:"actionability,omitempty"`
	Facets        []FacetScore `json:"facets"`
}

// FacetScore is one scored facet's tier/status for the completeness breakdown
// panel (F55). A not-applicable facet is still listed, so the UI can render its
// muted status, but it is excluded from Completeness.Score and Actionability.
type FacetScore struct {
	Canonical     string `json:"canonical"`
	Label         string `json:"label"`
	Criticality   string `json:"criticality"` // registry.CriticalityCritical | CriticalityNiceToHave
	Tier          string `json:"tier"`        // TierMissing | TierProvider | TierCurated
	NotApplicable bool   `json:"not_applicable,omitempty"`
	// Actionable is true only for a missing (non-excluded, non-not-applicable)
	// facet that has a cached unapplied provider candidate.
	Actionable bool `json:"actionable,omitempty"`
	// Provider is the namespace of the candidate Actionable refers to (e.g.
	// "tmdb") — set only when Actionable, so the remediation queue (F55.7) can
	// show which provider it would come from and apply it via setFieldDecision
	// without a second lookup into resolved fields the API layer doesn't retain.
	Provider string `json:"provider,omitempty"`
}

// Complete computes the completeness score and actionability signal for one
// entity from its configured fields, already-resolved values, and not-applicable
// exclusions (F55, ADR-081 D3).
//
// fields is the same field list passed to ResolveFields — Complete needs it, not
// just resolved, because ResolveFields drops an empty, undecided field entirely
// (spec RD-adjacent behavior predating F55): a genuinely missing scored facet may
// have no row in resolved at all, and dropping it from the score's denominator
// would silently inflate every entity's completeness. notApplicable is keyed by
// canonical, same casing FacetsNotApplicableForEntity returns.
//
// Per-facet tier is derived entirely from ResolvedField.WinningSource — no new
// per-field resolution logic (D3): a field absent from resolved (WinningSource
// "") is missing; a "file:"/"manual:" namespace is curated; any other namespace
// is a matched provider. A field with no registry.FieldDef.Criticality tag
// (including every Computed field, D1's invariant) is skipped entirely — never
// scored, never listed.
func Complete(fields []mapping.Field, resolved []ResolvedField, notApplicable map[string]bool) Completeness {
	byCanonical := make(map[string]ResolvedField, len(resolved))
	for _, rf := range resolved {
		byCanonical[rf.Canonical] = rf
	}

	var weightSum, weightedSum float64
	var missing, actionable int
	facets := make([]FacetScore, 0, len(fields))

	for _, f := range fields {
		def := registry.Lookup(f.Canonical)
		if def.Criticality == "" {
			continue
		}
		rf := byCanonical[f.Canonical] // zero value (WinningSource=="") when never resolved
		t := classifyTier(rf.WinningSource)
		fs := FacetScore{
			Canonical:   f.Canonical,
			Label:       def.Label,
			Criticality: def.Criticality,
			Tier:        t.name,
		}
		if notApplicable[f.Canonical] {
			fs.NotApplicable = true
			facets = append(facets, fs)
			continue
		}

		weight := criticalityWeight(def.Criticality)
		weightSum += weight
		weightedSum += weight * t.weight
		if t == missingTier {
			missing++
			if provider, ok := actionableCandidate(rf); ok {
				fs.Actionable = true
				fs.Provider = provider
				actionable++
			}
		}
		facets = append(facets, fs)
	}

	score := 0
	if weightSum > 0 {
		score = int(math.Round(100 * weightedSum / weightSum))
	}
	var actionability *float64
	if missing > 0 {
		a := float64(actionable) / float64(missing)
		actionability = &a
	}
	return Completeness{Score: score, Actionability: actionability, Facets: facets}
}

// classifyTier maps a resolved field's winning source to its completeness tier
// (ADR-081 D3 tier table). It delegates the file/manual-vs-provider distinction
// to fieldsource.ForNamespace — the package that already owns this grammar
// (fieldsource.go) — rather than re-deriving it here.
func classifyTier(winningSource string) tier {
	if winningSource == "" {
		return missingTier
	}
	ns, _, _ := strings.Cut(winningSource, ":")
	switch fieldsource.ForNamespace(ns) {
	case fieldsource.File, fieldsource.Manual:
		return curatedTier
	default:
		return providerTier
	}
}

// criticalityWeight maps a facet's criticality tag to its numeric scoring weight
// (spec § Scoring model). Any tag other than critical is nice_to_have by
// construction — Complete's caller already filtered out the untagged ("") case.
func criticalityWeight(c string) float64 {
	if c == registry.CriticalityCritical {
		return weightCritical
	}
	return weightNiceToHave
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
