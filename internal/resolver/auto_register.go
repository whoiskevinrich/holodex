package resolver

import (
	"slices"
	"strings"

	"holodex/internal/model"
	"holodex/internal/registry"
)

// AutoField is one stored non-canonical provider field considered for display-only
// auto-registration (F39, ADR-056): a provider, its field key, and the values it
// supplied for the entity.
type AutoField struct {
	Provider string
	Key      string
	Values   []string
}

// AutoHint is a provider-advertised presentation hint for a non-canonical key (F39):
// the label / render mode / ordering group / order the provider suggested. Render and
// Group are normalized to the registry vocabulary defensively when consumed.
type AutoHint struct {
	Label   string
	Display string
	Group   string
	Order   int
}

// autoAcc accumulates one auto-registered field across the (possibly multiple)
// providers that supply its key: the resolved presentation (from the tier-3 hint or
// the tier-4 floor) plus the deduped value union with per-value provenance.
type autoAcc struct {
	label    string
	display  string
	group    string
	order    int
	provider string // first provider to supply the key (label/render tier + winner)
	valOrder []string
	disp     map[string]string
	srcs     map[string][]string
}

// AutoRegisterFields builds the display-only resolved rows for an entity's stored
// non-canonical shadow fields (F39, ADR-056). It is entity-agnostic — video, person,
// and studio all feed their shadow fields through it after the canonical resolve —
// and pure (no I/O).
//
//   - fields: the entity's stored provider fields (provider + key + values).
//   - rendered: canonical keys already produced by the canonical resolve, which are
//     skipped here so a mapped/synthesized field is never double-rendered.
//   - hintFor: returns the provider hint for a (provider, key), ok=false when none —
//     the tier-3 lookup; absence falls through to the title-case floor (tier 4).
//
// A key is included iff it has a non-empty value, is not reserved (`_`-prefix), is
// non-canonical, and is not already rendered. Values for the same key from multiple
// providers merge (dedup union, combined provenance). The result is sorted after the
// canonical fields by (group rank, order, key) and every field carries
// AutoRegistered=true with no decision/curation state.
//
// Render modes are emitted as hinted; the image_url asset-host allowlist gate
// (ADR-039/056) is applied by the caller, next to the allowlist, so this pass stays
// free of enrichment/security concerns.
func AutoRegisterFields(
	fields []AutoField,
	rendered map[string]bool,
	hintFor func(provider, key string) (AutoHint, bool),
) []ResolvedField {
	byKey := map[string]*autoAcc{}
	var keyOrder []string

	for _, f := range fields {
		key := strings.ToLower(strings.TrimSpace(f.Key))
		if key == "" || strings.HasPrefix(key, model.InternalFieldPrefix) || registry.IsKnown(key) || rendered[key] {
			continue // reserved sidecar, canonical, or already rendered
		}
		vals := trimNonEmpty(f.Values)
		if len(vals) == 0 {
			continue // presence gate: no value → no field
		}
		a := byKey[key]
		if a == nil {
			a = newAutoAcc(f.Provider, key, hintFor)
			byKey[key] = a
			keyOrder = append(keyOrder, key)
		}
		for _, v := range vals {
			nk := normKey(v)
			if _, seen := a.disp[nk]; !seen {
				a.disp[nk] = v
				a.valOrder = append(a.valOrder, nk)
			}
			if !slices.Contains(a.srcs[nk], f.Provider) {
				a.srcs[nk] = append(a.srcs[nk], f.Provider)
			}
		}
	}

	out := make([]ResolvedField, 0, len(keyOrder))
	for _, key := range keyOrder {
		a := byKey[key]
		items := make([]ResolvedValue, 0, len(a.valOrder))
		values := make([]string, 0, len(a.valOrder))
		for _, nk := range a.valOrder {
			items = append(items, ResolvedValue{Value: a.disp[nk], Sources: a.srcs[nk]})
			values = append(values, a.disp[nk])
		}
		out = append(out, ResolvedField{
			Canonical:      key,
			Label:          a.label,
			Display:        a.display,
			Values:         values,
			Items:          items,
			WinningSource:  a.provider + ":" + key,
			AutoRegistered: true,
		})
	}

	slices.SortStableFunc(out, func(x, y ResolvedField) int {
		gx, gy := byKey[x.Canonical], byKey[y.Canonical]
		if d := registry.GroupRank(gx.group) - registry.GroupRank(gy.group); d != 0 {
			return d
		}
		if d := gx.order - gy.order; d != 0 {
			return d
		}
		return strings.Compare(x.Canonical, y.Canonical)
	})
	return out
}

// newAutoAcc seeds the per-key accumulator with the tier-3 provider hint or the
// tier-4 title-case floor (registry label, text render, extended group).
func newAutoAcc(provider, key string, hintFor func(provider, key string) (AutoHint, bool)) *autoAcc {
	a := &autoAcc{
		label:    registry.Lookup(key).Label, // title-case floor
		display:  registry.DisplayText,
		group:    registry.GroupExtended,
		provider: provider,
		disp:     map[string]string{},
		srcs:     map[string][]string{},
	}
	if hintFor != nil {
		if h, ok := hintFor(provider, key); ok {
			if h.Label != "" {
				a.label = h.Label
			}
			a.display = registry.NormalizeDisplay(h.Display)
			a.group = registry.NormalizeGroup(h.Group)
			a.order = h.Order
		}
	}
	return a
}

func trimNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if t := strings.TrimSpace(v); t != "" {
			out = append(out, t)
		}
	}
	return out
}
