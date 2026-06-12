// Package model holds the core domain types shared across layers.
package model

import "time"

// Thumbnail pipeline states stored in Video.ThumbnailState (ADR-009). The empty
// string is the zero value, meaning "never attempted". Centralized here so the
// repo (SQL), the thumbnail pipeline, and the API agree on one vocabulary.
const (
	ThumbnailNone      = ""
	ThumbnailEmbedded  = "embedded"  // Tier 1: extracted from container cover art
	ThumbnailGenerated = "generated" // Tier 2: extracted frame via ffmpeg
	ThumbnailFailed    = "failed"    // last attempt errored; retried by startup sweep
)

// HasThumbnailImage reports whether a thumbnail state implies an image exists on
// disk (and thus a serving URL can be offered).
func HasThumbnailImage(state string) bool {
	return state == ThumbnailEmbedded || state == ThumbnailGenerated
}

// Video is one indexed media file. file metadata is the source of truth; this
// record is a rebuildable cache of it (ADR-003/004).
type Video struct {
	ID         int64      `json:"id"`
	FilePath   string     `json:"file_path"` // canonical absolute path (ADR-011)
	FileSize   int64      `json:"file_size"`
	Title      string     `json:"title"`
	Duration   int        `json:"duration_sec"`
	Width      int        `json:"width"`
	Height     int        `json:"height"`
	RecordedAt *time.Time `json:"recorded_at,omitempty"`
	IndexedAt  time.Time  `json:"indexed_at"`
	FileMtime  time.Time  `json:"file_mtime"`
	Active     bool       `json:"-"`

	// ThumbnailState is the cover-image pipeline state (ADR-009): "" (none yet),
	// "embedded", "generated", or "failed". Internal bookkeeping — the API exposes
	// ThumbnailURL instead.
	ThumbnailState string `json:"-"`
	// ThumbnailURL is the serving URL, set by the API layer when an image exists.
	ThumbnailURL string `json:"thumbnail_url,omitempty"`

	People []Person `json:"people,omitempty"`
	Tags   []Tag    `json:"tags,omitempty"`
}

type Person struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	VideoCount int    `json:"video_count,omitempty"`
}

type Tag struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	VideoCount int    `json:"video_count,omitempty"`
}

// ExtraMetadata is a captured raw container tag not mapped to a first-class
// field (ADR-013, F2.9).
type ExtraMetadata struct {
	SourceKey string `json:"source_key"`
	Value     string `json:"value"`
}
