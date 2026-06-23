// Package writeback embeds enrichment values into media files via exiftool
// (F28, ADR-041). The format-mapping table (this file) and the atomic write
// function (writeback.go) are the only two public surfaces.
package writeback

// ImageTagForField returns (attachmentName, true) when the canonical field maps
// to a binary image embedded in the file — either a cover art attachment (MKV/WebM)
// or an exiftool binary tag (MP4/mp3/flac). Returns ("", false) when the field
// has no image mapping for the given container. The tag name is the attachment
// filename for MKV/WebM paths and the exiftool tag name for everything else.
func ImageTagForField(canonical, container string) (string, bool) {
	tags, ok := imageFormatMap[container]
	if !ok {
		return "", false
	}
	tag, ok := tags[canonical]
	return tag, ok
}

// imageFormatMap maps canonical image fields to their per-container embedding
// target. Matroska/WebM use the attachment filename; MP4/mp3/flac use the
// exiftool binary-write tag name.
var imageFormatMap = map[string]map[string]string{
	"Matroska": {"poster_url": "cover.jpg"},
	"WebM":     {"poster_url": "cover.jpg"},
	"MP4":      {"poster_url": "QuickTime:CoverArt"},
	"mp3":      {"poster_url": "Picture"},
	"flac":     {"poster_url": "Picture"},
}

// TagForField returns the exiftool tag name and true for the given canonical
// field in the given normalised container string (as produced by
// internal/metadata.normalizeContainer — "Matroska", "MP4", "WebM", "mp3", …).
// Returns ("", false) when no mapping is defined for the combination so the
// caller can return 422 rather than writing an unknown tag.
func TagForField(canonical, container string) (string, bool) {
	tags, ok := formatMap[container]
	if !ok {
		return "", false
	}
	tag, ok := tags[canonical]
	return tag, ok
}

// formatMap is the canonical-field → exiftool-tag-name table, keyed by the
// normalised container string. Only containers that exiftool can write are
// included; unsupported containers return false from TagForField.
//
// Tag names follow exiftool conventions:
//   - Bare names (e.g. "Title") use the format's default group.
//   - Prefixed names (e.g. "QuickTime:Title") force the exact group so
//     exiftool does not write to the wrong tag in a multi-group container.
//   - MKV/Matroska tags map to the GENERAL block (target=50, level 0),
//     consistent with ADR-010.
var formatMap = map[string]map[string]string{
	"Matroska": {
		"title":             "Title",
		"original_title":    "OriginalMediaType",
		"overview":          "Comment",   // plot summary → COMMENT tag
		"tagline":           "Subtitle",  // short tagline → SUBTITLE (avoids Comment collision)
		"release_date":      "Year",      // year/date → YEAR tag
		"genres":            "Genre",
		"original_language": "Language",
		"actors":            "Artist",    // cast → ARTIST tag, comma-delimited
		"studio":            "Publisher", // production company → PUBLISHER tag
	},
	"WebM": {
		"title":             "Title",
		"overview":          "Comment",
		"tagline":           "Subtitle",
		"release_date":      "Year",
		"genres":            "Genre",
		"original_language": "Language",
		"actors":            "Artist",
		"studio":            "Publisher",
	},
	"MP4": {
		"title":             "QuickTime:Title",
		"overview":          "QuickTime:Comment",
		"tagline":           "QuickTime:Keywords",
		"release_date":      "QuickTime:Year",
		"genres":            "QuickTime:Genre",
		"original_language": "QuickTime:MediaLanguage",
		"actors":            "QuickTime:Artist",
		"studio":            "QuickTime:Publisher",
	},
	"mp3": {
		"title":        "Title",
		"overview":     "Comment",
		"genres":       "Genre",
		"release_date": "Year",
		"actors":       "Artist",
		"studio":       "Publisher",
	},
	"flac": {
		"title":             "Title",
		"overview":          "Comment",
		"genres":            "Genre",
		"release_date":      "Year",
		"original_language": "Language",
		"actors":            "Artist",
		"studio":            "Publisher",
	},
}
