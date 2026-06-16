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
	// Stream/container details from ffprobe (F12.4). Empty/zero until a file has
	// been (re)indexed after migration 0003.
	VideoCodec  string `json:"video_codec,omitempty"`
	AudioCodec  string `json:"audio_codec,omitempty"`
	BitrateKbps int    `json:"bitrate_kbps,omitempty"`
	Container   string `json:"container,omitempty"`
	RecordedAt  *time.Time `json:"recorded_at,omitempty"`
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

// Scan trigger kinds (F21.2). A scan pass is driven by exactly one of these, and
// the value is surfaced in the activity status and recorded on each JobRun.
const (
	TriggerInitial  = "initial"  // the bootstrap scan at startup
	TriggerPeriodic = "periodic" // the interval ticker / explicit ScanOnce
	TriggerWatch    = "watch"    // a debounced filesystem-watch event
	TriggerManual   = "manual"   // an admin-triggered rescan (F13.3)
)

// Job kinds and statuses recorded in job_runs (F21.3, ADR-028). Kind is
// extensible; scan is the only producer today, with enrichment (F22) the next.
const (
	JobKindScan   = "scan"
	JobKindEnrich = "enrich"
	JobStatusOK   = "success"
	JobStatusErr  = "error"
)

// Enrichment entity types stored in entity_enrichment (F22, ADR-033). People is
// the v1 slice; series/video reuse the same shadow table when the design
// generalizes.
const EnrichEntityPerson = "person"

// EnrichedField is a canonical field resolved for one entity from a metadata
// source plugin (F22, ADR-033). It is shadow data kept distinct from the
// file-extracted fields; Provider carries the provenance the UI labels
// ("from <provider>"). An empty Provider would denote a file-sourced value when
// the resolver later interleaves the two (designed-in; provider-only in v1).
type EnrichedField struct {
	Canonical  string    `json:"canonical"`
	Label      string    `json:"label"`
	Values     []string  `json:"values"`
	Provider   string    `json:"provider"`
	ExternalID string    `json:"external_id,omitempty"`
	FetchedAt  time.Time `json:"fetched_at,omitempty"`
}

// ScanSummary is the outcome of one completed scan pass (F21.2). It is both the
// scanner's "last run" status and the source of a JobRun history record.
type ScanSummary struct {
	Trigger    string    `json:"trigger"`
	FinishedAt time.Time `json:"finished_at"`
	DurationMs int64     `json:"duration_ms"`
	Seen       int       `json:"seen"`
	Added      int       `json:"added"`
	Updated    int       `json:"updated"`
	Removed    int       `json:"removed"`
	Skipped    int       `json:"skipped"`
	Errors     int       `json:"errors"`
}

// ScanStatus is the scanner's live state for the activity surface (F21.1/F21.2).
// StartedAt is set only while running; LastRun is nil until the first pass
// completes; NextScheduledAt is a best-effort estimate (nil when scanning is
// disabled). It carries no filesystem paths — the no-secrets invariant (ADR-028).
type ScanStatus struct {
	State           string       `json:"state"` // "idle" | "running"
	Trigger         string       `json:"trigger,omitempty"`
	StartedAt       *time.Time   `json:"started_at,omitempty"`
	LastRun         *ScanSummary `json:"last_run"`
	NextScheduledAt *time.Time   `json:"next_scheduled_at,omitempty"`
}

// JobRun is one completed background job pass recorded for the 30-day activity
// history (F21.3, ADR-028). Counts mirror a scan summary; ErrorMessage is empty
// unless the whole pass errored.
type JobRun struct {
	ID           int64     `json:"id"`
	Kind         string    `json:"kind"`
	Trigger      string    `json:"trigger"`
	Status       string    `json:"status"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at"`
	DurationMs   int64     `json:"duration_ms"`
	Seen         int       `json:"seen"`
	Added        int       `json:"added"`
	Updated      int       `json:"updated"`
	Removed      int       `json:"removed"`
	Skipped      int       `json:"skipped"`
	Errors       int       `json:"errors"`
	ErrorMessage string    `json:"error_message,omitempty"`
	// Detail is a short human description for non-scan jobs (F22.6b) — e.g.
	// "tmdb → person #18 (5 fields)". Empty for scans (their counts say it all).
	Detail string `json:"detail,omitempty"`
}
