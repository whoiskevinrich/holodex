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
// video wrapper over ResolveFields, supplying the file layer as the baseline.
func Resolve(
	v *model.Video,
	extra []model.ExtraMetadata,
	enrichment Enrichment,
	curation Curation,
	fields []mapping.Field,
) []ResolvedField {
	return ResolveFields(NewVideoBaseline(v, extra), enrichment, curation, fields)
}

// ResolveFields is the entity-agnostic resolution core: it merges the supplied
// baseline with enrichment + curation and returns the result in field declaration
// order. Resolve wraps it with the video file layer; a person/studio entity wraps
// it with its own BaselineSource — so they inherit the source model without
// reopening this function (ADR-052). curation may be nil.
func ResolveFields(
	baseline BaselineSource,
	enrichment Enrichment,
	curation Curation,
	fields []mapping.Field,
) []ResolvedField {
	out := make([]ResolvedField, 0, len(fields))
	for _, f := range fields {
		items, winner := resolveField(baseline, enrichment, curation[f.Canonical], f)
		if len(items) == 0 {
			continue
		}
		values := make([]string, len(items))
		for i, it := range items {
			values[i] = it.Value
		}
		def := registry.Lookup(f.Canonical)
		label := f.Label
		if label == "" {
			label = def.Label
		}
		out = append(out, ResolvedField{
			Canonical:     f.Canonical,
			Label:         label,
			Display:       def.Display,
			Values:        values,
			Items:         items,
			Multi:         f.Multi || f.Merge,
			WinningSource: winner,
		})
	}
	return out
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
) (title, source string) {
	baseline := NewVideoBaseline(v, extra)
	for _, f := range fields {
		if !f.Browse {
			continue
		}
		items, winner := resolveField(baseline, enrichment, curation[f.Canonical], f)
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
		return resolveMerge(gather, fc, f)
	}
	return resolvePrecedence(gather, fc, f)
}

// resolvePrecedence resolves a scalar field: a manual value overrides; otherwise
// the first non-empty, non-suppressed source wins a single value.
func resolvePrecedence(gather func(mapping.Source) []string, fc FieldCuration, f mapping.Field) ([]ResolvedValue, string) {
	for _, mv := range fc.Add {
		mv = strings.TrimSpace(mv)
		if mv == "" || fc.Suppress[normKey(mv)] {
			continue
		}
		val := applyCasing(mv, f.Casing)
		return []ResolvedValue{{Value: val, Sources: []string{"manual"}, Manual: true, NoWrite: fc.NoWrite[normKey(mv)]}},
			"manual:" + f.Canonical
	}
	for _, src := range f.ParsedSources {
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
