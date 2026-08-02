// Package resolver implements unified field resolution (F27) and granular
// curation/merge (F30): it merges file-metadata, enrichment, and manual-curation
// sources into a per-canonical-field value, as configured in metadata-mappings.yaml.
//
// Two resolution modes (F30.1):
//
//   - precedence (scalar fields): the first non-empty source wins a single value;
//     an owner manual value overrides it.
//   - merge (multi/merge fields): the value is the deduplicated UNION of every
//     configured source plus the manual source, with per-value provenance.
//
// Value-level curation (F30.2) layers on top: manual additions join the union,
// suppression removes a value (by normalized key) everywhere, and "no-write"
// flags a value as display-only (excluded from file writeback).
//
// The resolver does no I/O — callers supply pre-loaded data, so changing config or
// curation takes effect without re-fetching providers or re-scanning files.
//
// The baseline (file) layer is reached through a BaselineSource, which keeps the
// merge core entity-agnostic: a video supplies the file layer, while a future
// person/studio entity supplies its own scan-derived baseline (ADR-052).
package resolver

import (
	"slices"
	"strings"
	"unicode"

	"holodex/internal/fieldsource"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
)

// ResolvedValue is one surviving value of a field with its provenance and curation
// flags (F30). A value present in multiple sources is reported once, with every
// contributing source namespace listed in Sources.
type ResolvedValue struct {
	Value   string   `json:"value"`
	Sources []string `json:"sources"`            // contributing namespaces, e.g. ["tmdb","file"]
	Manual  bool     `json:"manual,omitempty"`   // owner-added value
	NoWrite bool     `json:"no_write,omitempty"` // shown but excluded from file write
}

// ResolvedField is one canonical field resolved for one video. Values holds the
// surviving display values (suppressed ones removed) for back-compat; Items carries
// the richer per-value provenance + curation state the curation UI consumes (F30).
type ResolvedField struct {
	Canonical     string          `json:"canonical"`
	Label         string          `json:"label"`
	Display       string          `json:"display,omitempty"` // "" | "long_text" | "image_url" | "url"
	Values        []string        `json:"values"`
	Items         []ResolvedValue `json:"items,omitempty"`
	Multi         bool            `json:"multi,omitempty"`          // merge-mode (set) field: UI shows add + per-value remove
	WinningSource string          `json:"winning_source,omitempty"` // e.g. "tmdb:title", "file:Title", "manual:genres"

	// AutoRegistered marks a display-only non-canonical field surfaced by F39
	// auto-registration (ADR-056): rendered read-only (label + values + provenance)
	// with no source-decision or curation controls. Canonical/mapped fields leave it
	// false. The SPA renders these in the "Additional details" group.
	AutoRegistered bool `json:"auto_registered,omitempty"`

	// Promoted marks a non-canonical field that became first-class curatable via an
	// in-app promotion (F44, ADR-062) rather than a native mapping. It is otherwise a
	// normal mapped field (AutoRegistered=false, full Decision/Candidates/curation); the
	// flag only tells the SPA to offer the owner-only Edit / Remove-promotion affordance.
	// The resolver never sets it — the API layer stamps it after resolve for the keys it
	// materialized from a promotion row.
	Promoted bool `json:"promoted,omitempty"`

	// Computed marks a derived, source-less, read-only row appended by the Derive
	// post-pass (F45, ADR-063): computed-on-read from other resolved fields, never a
	// stored value. Like an auto-registered row it carries no Decision/Candidates
	// (structurally non-adoptable) and its WinningSource is "computed:<canonical>".
	// DerivedFrom holds the human labels of the input fields (e.g. ["Born"]) for the
	// transitive "calculated from …" provenance badge.
	Computed    bool     `json:"computed,omitempty"`
	DerivedFrom []string `json:"derived_from,omitempty"`

	// F36 (ADR-051) — per-field source-of-truth, populated on replace (scalar)
	// fields only. Decision is the standing source choice (Standing=false for the
	// implicit file-first default); InSync is false when the decided value differs
	// from the file-embedded value; Candidates feed the SourceSelect segments + the
	// candidates line. Merge fields leave all three nil (replace-only, RD1).
	Decision   *FieldDecision   `json:"decision,omitempty"`
	InSync     *bool            `json:"in_sync,omitempty"`
	Candidates []FieldCandidate `json:"candidates,omitempty"`
}

// FieldDecision is the per-field source-of-truth marker on a replace field (F36,
// ADR-051). Source is "file" | "provider:<name>" | "manual"; Standing is true for an
// explicit stored decision and false for the implicit file-first default (where
// Source names whichever source currently wins). ManualValue is set only when
// Source == "manual".
type FieldDecision struct {
	Source      string `json:"source"`
	Standing    bool   `json:"standing"`
	ManualValue string `json:"manual_value,omitempty"`
}

// FieldCandidate is one selectable source value for a replace field — the file
// baseline value or a matched provider's value (F36, ADR-051). It feeds the
// SourceSelect `Adopt` segments and the candidates line. The file candidate is
// always present (its Value may be ""); a provider candidate is included only when
// it has a non-empty value (you cannot adopt an empty value).
type FieldCandidate struct {
	Source   string `json:"source"`             // "file" | "provider:<name>"
	Provider string `json:"provider,omitempty"` // provider name when Source is "provider:<name>"
	Value    string `json:"value"`
}

// Default-source modes (F36, ADR-051). A decision pins a replace field to one
// source (grammar in internal/fieldsource); the global DefaultSource governs the
// *undecided* winner — file-first (the bug fix, RD4) or legacy mapping order.
const (
	DefaultSourceFile    = "file"    // undecided fields resolve file-first (default)
	DefaultSourceMapping = "mapping" // undecided fields keep first-non-empty mapping order
)

// Decision is one pre-loaded standing decision (the resolver's input form): a
// pinned Source plus the frozen ManualValue (only when Source == "manual").
type Decision struct {
	Source      string // "file" | "provider:<name>" | "manual"
	ManualValue string
}

// Decisions is the pre-loaded standing decision map for one entity, keyed by
// canonical field (lower-cased). Built from repo.DecisionsForEntity. The resolver
// consults it before mapping order — no per-field query, resolution stays pure.
type Decisions map[string]Decision

// Options carries the F36 resolution inputs. The zero value means "no standing
// decisions, file-first default, mapping-order among providers" — the F36 default
// behavior (RD4).
type Options struct {
	Decisions     Decisions
	DefaultSource string // DefaultSourceFile ("" default) | DefaultSourceMapping

	// ProviderTrustOrder ranks providers for the *undecided* winner among them on a
	// replace field (F36 P1-2, ADR-051 §8): when several matched providers supply a
	// value and no per-field decision exists, the first-listed provider wins. The
	// file/baseline layer still wins overall under the file-first default, and a
	// per-field decision always overrides. Unlisted providers keep mapping order
	// behind the listed ones; empty means today's mapping-order fallback.
	ProviderTrustOrder []string
}

// fileFirst reports whether undecided fields resolve file-first (the default) vs.
// legacy mapping order.
func (o Options) fileFirst() bool { return o.DefaultSource != DefaultSourceMapping }

// trustRank returns a provider namespace's position in the configured inter-provider
// trust order, or a sentinel past the end for an unranked provider — so ranked
// providers sort ahead of unranked ones, which keep their mapping order.
func (o Options) trustRank(ns string) int {
	if i := slices.Index(o.ProviderTrustOrder, ns); i >= 0 {
		return i
	}
	return len(o.ProviderTrustOrder)
}

// sortByTrust stably orders provider sources by the configured trust order (a no-op
// when none is configured: every source ranks equal, so the stable sort preserves
// mapping order).
func (o Options) sortByTrust(srcs []mapping.Source) {
	if len(o.ProviderTrustOrder) == 0 {
		return
	}
	slices.SortStableFunc(srcs, func(a, b mapping.Source) int {
		return o.trustRank(a.Namespace) - o.trustRank(b.Namespace)
	})
}

// lookup returns the standing decision for a canonical field, if any.
func (o Options) lookup(canonical string) (Decision, bool) {
	if o.Decisions == nil {
		return Decision{}, false
	}
	d, ok := o.Decisions[normKey(canonical)]
	return d, ok
}

// Enrichment holds the pre-loaded enrichment shadow data for one video, keyed by
// provider then field key.  Built from repo.EnrichmentForVideos.
//
//	enrichment["tmdb"]["title"] = []string{"Fight Club"}
//	enrichment["tmdb"]["genres"] = []string{"Drama", "Thriller"}
type Enrichment map[string]map[string][]string

// FieldCuration is the pre-loaded value-level curation for one canonical field
// (F30.2). Suppress/NoWrite are keyed by the normalized value so they apply
// regardless of which source re-supplies the value on a later scan/enrich.
type FieldCuration struct {
	Add      []string        // owner-added manual values (display form)
	Suppress map[string]bool // normalized value → suppressed (tombstone)
	NoWrite  map[string]bool // normalized value → excluded from file write
}

// Curation is the pre-loaded curation for one video, keyed by canonical field.
type Curation map[string]FieldCuration

// normKey is the dedup/match key: trim + case-fold. Kept consistent with
// mapping.Dedupe so behavior matches the file-only path.
func normKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// NormKey exports normKey's fold rule for other packages that need the identical
// dedup key without duplicating it (e.g. genre writeback's tag/raw-genres union,
// internal/api/genre_writeback.go) — the one place this convention is computed.
func NormKey(s string) string { return normKey(s) }

// BaselineSource supplies an entity's baseline ("intrinsic") field values — the
// layer that is the default source of truth before enrichment and curation are
// layered on. For a video the baseline is the file layer: the file-tag columns
// (video_metadata) plus the filename-derived videos.title. A future person or
// studio entity supplies its own scan-derived baseline; only the baseline's
// identity differs, so the merge core stays entity-agnostic (ADR-052, realizing
// ADR-051 §9 fast-follow ①).
//
// Baseline reports (vals, true) when src targets this entity's baseline layer —
// even when it carries no value, so resolution does not fall through to a provider
// for a baseline source — and (nil, false) when src names a provider/enrichment
// namespace. Deciding "which namespace is intrinsic" belongs to the baseline, not
// the resolver, which is what keeps the merge core free of any "file" special-casing.
type BaselineSource interface {
	Baseline(src mapping.Source) (vals []string, ok bool)
}

// videoBaseline is the video implementation of BaselineSource: the file layer.
type videoBaseline struct {
	title     string              // filename-derived videos.title
	byFileTag map[string][]string // normalized file-tag key → values (video_metadata)
}

// NewVideoBaseline builds the file-layer BaselineSource for a video from its row
// and pre-loaded file-tag rows. v may be nil (an empty baseline).
func NewVideoBaseline(v *model.Video, extra []model.ExtraMetadata) BaselineSource {
	b := videoBaseline{byFileTag: indexExtra(extra)}
	if v != nil {
		b.title = v.Title
	}
	return b
}

// Baseline resolves a file-namespace source against the video's file layer:
// file:title → videos.title, file:<tag> → the matching video_metadata values.
func (b videoBaseline) Baseline(src mapping.Source) ([]string, bool) {
	switch {
	case src.IsFileTitle():
		if b.title != "" {
			return []string{b.title}, true
		}
		return nil, true
	case src.Namespace == "file":
		return b.byFileTag[normKey(src.Key)], true
	}
	return nil, false
}

// Resolve applies the configured fields to the supplied video data and returns the
// merged, curated result in field declaration order. curation may be nil. It is the
// video wrapper over ResolveFields, supplying the file layer as the baseline. opts
// carries the F36 standing decisions + default-source mode (the zero value is the
// file-first default with no decisions).
func Resolve(
	v *model.Video,
	extra []model.ExtraMetadata,
	enrichment Enrichment,
	curation Curation,
	fields []mapping.Field,
	opts Options,
) []ResolvedField {
	return ResolveFields(NewVideoBaseline(v, extra), enrichment, curation, fields, opts)
}

// ResolveFields is the entity-agnostic resolution core: it merges the supplied
// baseline with enrichment + curation and returns the result in field declaration
// order. Resolve wraps it with the video file layer; a person/studio entity wraps
// it with its own BaselineSource — so they inherit the source model without
// reopening this function (ADR-052). curation may be nil. opts carries the F36
// standing decisions + default-source mode.
func ResolveFields(
	baseline BaselineSource,
	enrichment Enrichment,
	curation Curation,
	fields []mapping.Field,
	opts Options,
) []ResolvedField {
	out := make([]ResolvedField, 0, len(fields))
	for _, f := range fields {
		items, winner := resolveField(baseline, enrichment, curation[f.Canonical], opts, f)
		if len(items) == 0 {
			// A replace field with a *standing* decision stays in the output even
			// when the decided value is empty (e.g. a blank-pin to an empty person
			// baseline, F37 RD3) — dropping it would hide the pin and leave no
			// control to change or clear it. Undecided empty fields drop as before.
			if _, decided := opts.lookup(f.Canonical); !decided || f.Multi || f.Merge {
				continue
			}
		}
		values := make([]string, len(items))
		for i, it := range items {
			values[i] = it.Value
		}
		label, display := LabelAndDisplay(f)
		rf := ResolvedField{
			Canonical:     f.Canonical,
			Label:         label,
			Display:       display,
			Values:        values,
			Items:         items,
			Multi:         f.Multi || f.Merge,
			WinningSource: winner,
		}
		// F36 markers are replace-only (RD1): merge fields keep F30 per-value
		// curation and carry no source decision.
		if !rf.Multi {
			rf.Decision, rf.Candidates, rf.InSync = replaceMarkers(baseline, enrichment, optDecision(opts, f), f, items)
		}
		out = append(out, rf)
	}
	return out
}

// LabelAndDisplay resolves a mapping.Field's display label and mode: an explicit
// mapping override wins over the registry.Lookup default for each independently (a
// field can set one without the other). F39: this also covers a synthesized
// auto-registered field's provider-hinted Display. Exported so a caller that
// synthesizes a ResolvedField outside ResolveFields (e.g. the API layer's
// genre-writeback display override, internal/api/genre_writeback.go) derives the
// same label/display a normal resolve pass would, rather than re-deriving the
// fallback rule and risking the two diverging.
func LabelAndDisplay(f mapping.Field) (label, display string) {
	def := registry.Lookup(f.Canonical)
	label = f.Label
	if label == "" {
		label = def.Label
	}
	display = def.Display
	if f.Display != "" {
		display = f.Display
	}
	return label, display
}

// optDecision returns the standing decision for a field as a pointer (nil when
// undecided) so the marker helper can distinguish a standing choice from the
// implicit default.
func optDecision(opts Options, f mapping.Field) *Decision {
	if d, ok := opts.lookup(f.Canonical); ok {
		return &d
	}
	return nil
}

// BrowseTitle returns the highest-precedence title for a video given the configured
// fields, honoring curation. It is a targeted helper for the list-media handler:
// rather than resolving all fields for every list item, it only resolves fields
// marked browse:true.
//
// Returns ("", "") when no browse field resolves, meaning the caller should keep
// the existing video.Title unchanged.
func BrowseTitle(
	v *model.Video,
	extra []model.ExtraMetadata,
	enrichment Enrichment,
	curation Curation,
	fields []mapping.Field,
	opts Options,
) (title, source string) {
	baseline := NewVideoBaseline(v, extra)
	for _, f := range fields {
		if !f.Browse {
			continue
		}
		items, winner := resolveField(baseline, enrichment, curation[f.Canonical], opts, f)
		if len(items) > 0 {
			return items[0].Value, winner
		}
	}
	return "", ""
}

func resolveField(
	baseline BaselineSource,
	enrichment Enrichment,
	fc FieldCuration,
	opts Options,
	f mapping.Field,
) (items []ResolvedValue, winner string) {
	gather := func(src mapping.Source) []string {
		if vals, ok := baseline.Baseline(src); ok {
			return vals
		}
		if pFields, ok := enrichment[src.Namespace]; ok {
			return pFields[src.Key]
		}
		return nil
	}

	if f.Multi || f.Merge {
		// Merge fields keep F30 per-value curation, untouched by F36 (RD1).
		return resolveMerge(gather, fc, f)
	}
	// Replace field: a standing decision short-circuits mapping order (F36); else
	// the configured default-source order decides the undecided winner.
	if dec, ok := opts.lookup(f.Canonical); ok {
		return resolveDecided(baseline, enrichment, fc, dec, f)
	}
	return resolvePrecedence(gather, fc, f, orderedSources(baseline, f.ParsedSources, opts))
}

// resolveDecided returns the value of the decided source for a replace field (F36):
// the file/baseline value, a matched provider's current value, or the frozen manual
// literal. It pins the *source*, not the value, so a later refresh re-extract or
// re-enrich flows straight through. A decided source with no current value yields no
// item (the field drops), exactly as an undecided empty field would.
func resolveDecided(baseline BaselineSource, enrichment Enrichment, fc FieldCuration, dec Decision, f mapping.Field) ([]ResolvedValue, string) {
	if name := fieldsource.Provider(dec.Source); name != "" {
		pFields := enrichment[name]
		for _, src := range f.ParsedSources {
			if src.Namespace != name {
				continue
			}
			if cand := firstNonEmpty(pFields[src.Key]); cand != "" {
				return decidedItem(cand, name, fc, f, false), name + ":" + src.Key
			}
		}
		return nil, ""
	}
	switch dec.Source {
	case fieldsource.File:
		if cand, src, ok := baselineValue(baseline, f); ok {
			return decidedItem(cand, src.Namespace, fc, f, false), src.Namespace + ":" + src.Key
		}
		return nil, ""
	default: // manual
		if cand := strings.TrimSpace(dec.ManualValue); cand != "" {
			return decidedItem(cand, fieldsource.Manual, fc, f, true), fieldsource.Manual + ":" + f.Canonical
		}
		return nil, ""
	}
}

// baselineValue returns the first baseline (file) source of the field that carries
// a non-empty value, plus that value. It is the single "which file value backs this
// field" scan shared by the decided-file path and the candidate/in-sync markers, so
// the two never diverge on which source wins.
func baselineValue(baseline BaselineSource, f mapping.Field) (val string, src mapping.Source, ok bool) {
	for _, s := range f.ParsedSources {
		vals, isBaseline := baseline.Baseline(s)
		if !isBaseline {
			continue
		}
		if v := firstNonEmpty(vals); v != "" {
			return v, s, true
		}
	}
	return "", mapping.Source{}, false
}

// decidedItem builds the single ResolvedValue for a decided replace field, applying
// the field's output casing and carrying any F30 no-write flag for the value.
func decidedItem(raw, ns string, fc FieldCuration, f mapping.Field, manual bool) []ResolvedValue {
	val := applyCasing(strings.TrimSpace(raw), f.Casing)
	return []ResolvedValue{{
		Value:   val,
		Sources: []string{ns},
		Manual:  manual,
		NoWrite: fc.NoWrite[normKey(val)],
	}}
}

// orderedSources returns the field's sources in resolution order. Under the
// file-first default (RD4) baseline sources are tried before provider sources (so a
// provider no longer masks the file — the F31 bug fix), and the provider partition
// is ranked by the configured inter-provider trust order (F36 P1-2); under
// DefaultSourceMapping the configured order is preserved unchanged. Source identity
// ("which namespace is the baseline") is asked of the BaselineSource, keeping this
// entity-agnostic.
func orderedSources(baseline BaselineSource, srcs []mapping.Source, opts Options) []mapping.Source {
	if !opts.fileFirst() {
		return srcs
	}
	// Count first so a single-namespace field returns unchanged with no allocation:
	// all-baseline never reorders, and all-provider only reorders when a trust order
	// is configured (otherwise it is already in mapping order).
	nBase := 0
	for _, s := range srcs {
		if _, ok := baseline.Baseline(s); ok {
			nBase++
		}
	}
	if nBase == len(srcs) || (nBase == 0 && len(opts.ProviderTrustOrder) == 0) {
		return srcs
	}
	base := make([]mapping.Source, 0, nBase)
	other := make([]mapping.Source, 0, len(srcs)-nBase)
	for _, s := range srcs {
		if _, ok := baseline.Baseline(s); ok {
			base = append(base, s)
		} else {
			other = append(other, s)
		}
	}
	opts.sortByTrust(other) // rank matched providers among themselves (P1-2)
	return append(base, other...)
}

// resolvePrecedence resolves an undecided scalar field: a manual value overrides;
// otherwise the first non-empty, non-suppressed source in the supplied order wins a
// single value. The order is file-first or mapping order per the default-source mode.
func resolvePrecedence(gather func(mapping.Source) []string, fc FieldCuration, f mapping.Field, sources []mapping.Source) ([]ResolvedValue, string) {
	for _, mv := range fc.Add {
		mv = strings.TrimSpace(mv)
		if mv == "" || fc.Suppress[normKey(mv)] {
			continue
		}
		val := applyCasing(mv, f.Casing)
		return []ResolvedValue{{Value: val, Sources: []string{"manual"}, Manual: true, NoWrite: fc.NoWrite[normKey(mv)]}},
			"manual:" + f.Canonical
	}
	for _, src := range sources {
		vals := gather(src)
		if len(vals) == 0 {
			continue
		}
		cand := strings.TrimSpace(vals[0])
		if cand == "" || fc.Suppress[normKey(cand)] {
			continue
		}
		val := applyCasing(cand, f.Casing)
		return []ResolvedValue{{Value: val, Sources: []string{src.Namespace}, NoWrite: fc.NoWrite[normKey(cand)]}},
			src.Namespace + ":" + src.Key
	}
	return nil, ""
}

// replaceMarkers computes the F36 source-of-truth markers for a replace field: the
// candidate list (file value + each matched provider's value), the decision marker
// (standing or the implicit file-first default winner), and the in-sync flag. A
// field is out of sync only when a *standing* decision's value differs from the
// file-embedded value — an undecided (file-default) field is in sync by construction.
func replaceMarkers(baseline BaselineSource, enrichment Enrichment, dec *Decision, f mapping.Field, items []ResolvedValue) (*FieldDecision, []FieldCandidate, *bool) {
	// File baseline candidate (always present; Value may be "").
	fileRaw, _, _ := baselineValue(baseline, f)
	fileVal := applyCasing(fileRaw, f.Casing)
	candidates := []FieldCandidate{{Source: fieldsource.File, Value: fileVal}}

	// One provider candidate per matched provider that supplies a non-empty value.
	seen := map[string]bool{}
	for _, src := range f.ParsedSources {
		if _, ok := baseline.Baseline(src); ok {
			continue // baseline source, already handled
		}
		name := src.Namespace
		if seen[name] {
			continue
		}
		pv := applyCasing(firstNonEmpty(enrichment[name][src.Key]), f.Casing)
		if pv == "" {
			continue // can't adopt an empty value
		}
		seen[name] = true
		candidates = append(candidates, FieldCandidate{Source: fieldsource.ForProvider(name), Provider: name, Value: pv})
	}

	// Decision marker: a standing decision, else the implicit default winner so the
	// SourceSelect highlights whichever source currently supplies the value.
	marker := &FieldDecision{}
	inSync := true
	if dec != nil {
		marker.Source = dec.Source
		marker.Standing = true
		if dec.Source == fieldsource.Manual {
			marker.ManualValue = strings.TrimSpace(dec.ManualValue)
		}
		decided := ""
		if len(items) > 0 {
			decided = items[0].Value
		}
		inSync = decided == fileVal
	} else {
		marker.Source = winnerToDecisionSource(items)
	}
	return marker, candidates, &inSync
}

// winnerToDecisionSource maps the winning value's namespace to a decision source so
// an undecided field reports its implicit selection ("file" / "provider:<name>" /
// "manual"). Defaults to "file" when nothing resolved.
func winnerToDecisionSource(items []ResolvedValue) string {
	if len(items) == 0 || len(items[0].Sources) == 0 {
		return fieldsource.File
	}
	return fieldsource.ForNamespace(items[0].Sources[0])
}

// firstNonEmpty returns the first trimmed non-empty value, or "".
func firstNonEmpty(vals []string) string {
	for _, v := range vals {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

// resolveMerge resolves a set field: the deduplicated union of all sources plus the
// manual source, with per-value provenance. Suppressed values are dropped.
func resolveMerge(gather func(mapping.Source) []string, fc FieldCuration, f mapping.Field) ([]ResolvedValue, string) {
	var order []string            // normkeys, first-seen order
	disp := map[string]string{}   // normkey → display value (first occurrence)
	srcs := map[string][]string{} // normkey → contributing namespaces (ordered, unique)
	manual := map[string]bool{}   // normkey → manual-contributed
	winner := ""

	add := func(raw, ns string) {
		for _, part := range mapping.SplitMulti(raw) {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			nk := normKey(part)
			if fc.Suppress[nk] {
				continue
			}
			if _, ok := disp[nk]; !ok {
				disp[nk] = part
				order = append(order, nk)
			}
			if !slices.Contains(srcs[nk], ns) {
				srcs[nk] = append(srcs[nk], ns)
			}
			if ns == "manual" {
				manual[nk] = true
			}
		}
	}

	for _, src := range f.ParsedSources {
		vals := gather(src)
		if len(vals) == 0 {
			continue
		}
		if winner == "" {
			winner = src.Namespace + ":" + src.Key
		}
		for _, raw := range vals {
			add(raw, src.Namespace)
		}
	}
	for _, mv := range fc.Add {
		add(mv, "manual")
	}

	if len(order) == 0 {
		return nil, ""
	}
	if winner == "" {
		winner = "manual:" + f.Canonical // manual-only field
	}
	items := make([]ResolvedValue, 0, len(order))
	for _, nk := range order {
		items = append(items, ResolvedValue{
			Value:   applyCasing(disp[nk], f.Casing),
			Sources: srcs[nk],
			Manual:  manual[nk],
			NoWrite: fc.NoWrite[nk],
		})
	}
	return items, winner
}

// applyCasing applies a field's configured output casing (F30, decision #4). Dedup
// is always case-insensitive; this only sets the displayed/written form.
func applyCasing(s, mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "lower":
		return strings.ToLower(s)
	case "upper":
		return strings.ToUpper(s)
	case "title":
		return titleCase(s)
	default: // "" / "preserve"
		return s
	}
}

// titleCase upper-cases the first rune of each whitespace-separated word, leaving
// the remainder untouched so acronyms/mixed-case (e.g. "iMac") survive.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// indexExtra builds a case-insensitive lookup map from ExtraMetadata.
// Returns nil (not an empty map) when extra is empty, avoiding a heap allocation
// in the list-media path where BrowseTitle is always called with nil extra.
func indexExtra(extra []model.ExtraMetadata) map[string][]string {
	if len(extra) == 0 {
		return nil
	}
	m := make(map[string][]string, len(extra))
	for _, e := range extra {
		k := normKey(e.SourceKey)
		m[k] = append(m[k], e.Value)
	}
	return m
}
