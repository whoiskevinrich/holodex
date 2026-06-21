// Package resolver implements unified field resolution (F27): it merges
// file-metadata and enrichment sources into a single precedence-ordered value per
// canonical field, as configured in metadata-mappings.yaml.
//
// The resolver is the single place where "first non-empty source wins" is applied
// across namespaces. Callers supply pre-loaded data; the resolver does no I/O.
package resolver

import (
	"strings"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
)

// ResolvedField is one canonical field resolved for one video, with the winning
// source recorded for provenance display.
type ResolvedField struct {
	Canonical     string   `json:"canonical"`
	Label         string   `json:"label"`
	Display       string   `json:"display,omitempty"` // "" | "long_text" | "image_url"
	Values        []string `json:"values"`
	WinningSource string   `json:"winning_source,omitempty"` // e.g. "tmdb:title", "file:Title"
}

// Enrichment holds the pre-loaded enrichment shadow data for one video, keyed by
// provider then field key.  Built from repo.EnrichmentForVideos.
//
//	enrichment["tmdb"]["title"] = []string{"Fight Club"}
//	enrichment["tmdb"]["genres"] = []string{"Drama", "Thriller"}
type Enrichment map[string]map[string][]string

// Resolve applies the configured fields to the supplied video data and returns the
// merged result in field declaration order.
//
// For each field, sources are walked in precedence order; the first namespace+key
// that yields a non-empty value wins. The WinningSource records which entry won
// (e.g. "tmdb:title") so the SPA can show a provenance badge.
func Resolve(
	v *model.Video,
	extra []model.ExtraMetadata,
	enrichment Enrichment,
	fields []mapping.Field,
) []ResolvedField {
	byFileTag := indexExtra(extra)

	out := make([]ResolvedField, 0, len(fields))
	for _, f := range fields {
		values, winner := resolveField(v, byFileTag, enrichment, f)
		if len(values) == 0 {
			continue
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
			WinningSource: winner,
		})
	}
	return out
}

// BrowseTitle returns the highest-precedence title for a video given the configured
// fields. It is a targeted helper for the list-media handler: rather than resolving
// all fields for every list item, it only resolves fields marked browse:true.
//
// Returns ("", "") when no browse field resolves, meaning the caller should keep
// the existing video.Title unchanged.
func BrowseTitle(
	v *model.Video,
	extra []model.ExtraMetadata,
	enrichment Enrichment,
	fields []mapping.Field,
) (title, source string) {
	byFileTag := indexExtra(extra)
	for _, f := range fields {
		if !f.Browse {
			continue
		}
		values, winner := resolveField(v, byFileTag, enrichment, f)
		if len(values) > 0 {
			return values[0], winner
		}
	}
	return "", ""
}

func resolveField(
	v *model.Video,
	byFileTag map[string][]string,
	enrichment Enrichment,
	f mapping.Field,
) (values []string, winner string) {
	for _, src := range f.ParsedSources {
		var vals []string
		switch {
		case src.IsFileTitle():
			// Special: videos.title column.
			if v != nil && v.Title != "" {
				vals = []string{v.Title}
			}
		case src.Namespace == "file":
			vals = byFileTag[strings.ToLower(strings.TrimSpace(src.Key))]
		default:
			// Provider enrichment namespace.
			if pFields, ok := enrichment[src.Namespace]; ok {
				vals = pFields[src.Key]
			}
		}

		if len(vals) == 0 {
			continue
		}

		if f.Multi {
			for _, v := range vals {
				values = append(values, mapping.SplitMulti(v)...)
			}
		} else {
			values = []string{strings.TrimSpace(vals[0])}
		}
		winner = src.Namespace + ":" + src.Key
		break // first non-empty source wins
	}
	return mapping.Dedupe(values), winner
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
		k := strings.ToLower(strings.TrimSpace(e.SourceKey))
		m[k] = append(m[k], e.Value)
	}
	return m
}
