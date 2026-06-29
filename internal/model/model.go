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
	ID       int64  `json:"id"`
	FilePath string `json:"file_path"` // canonical absolute path (ADR-011)
	FileSize int64  `json:"file_size"`
	Title    string `json:"title"`
	Duration int    `json:"duration_sec"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	// Stream/container details from ffprobe (F12.4). Empty/zero until a file has
	// been (re)indexed after migration 0003.
	VideoCodec  string     `json:"video_codec,omitempty"`
	AudioCodec  string     `json:"audio_codec,omitempty"`
	BitrateKbps int        `json:"bitrate_kbps,omitempty"`
	Container   string     `json:"container,omitempty"`
	RecordedAt  *time.Time `json:"recorded_at,omitempty"`
	IndexedAt   time.Time  `json:"indexed_at"`
	FileMtime   time.Time  `json:"file_mtime"`
	Active      bool       `json:"-"`

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
	// HeadshotVersion is the headshot image id (== its ?v= cache-buster) on the
	// people-list read, so the list avatar URL changes when the headshot does (e.g.
	// after enrichment) instead of serving the stale cached image (F25.29). 0 = no
	// headshot (the placeholder is served). Omitted on the detail read (which carries
	// the full image set).
	HeadshotVersion int64 `json:"headshot_version,omitempty"`
	// Aliases are owner-curated alternate names (F23, ADR-036), each searchable.
	// Populated only on the person-detail read; omitted (nil) elsewhere.
	Aliases []PersonAlias `json:"aliases,omitempty"`
}

// PersonAlias is one owner-curated alternate name for a person (F23, ADR-036).
// The id gives the UI and the delete endpoint a stable handle.
type PersonAlias struct {
	ID    int64  `json:"id"`
	Alias string `json:"alias"`
}

// Person image roles (F25, ADR-038). The three "core" roles are single-slot per
// person (one headshot, one banner, one poster); "extra" is the unbounded-but-
// capped gallery. Centralized here so the migration's partial unique index, the
// repo, the personimage placeholder resolver, and the API agree on one vocabulary.
const (
	PersonImageHeadshot = "headshot" // 1:1 portrait — the default avatar
	PersonImageBanner   = "banner"   // 16:9 wide header
	PersonImagePoster   = "poster"   // 2:3 tall poster
	PersonImageExtra    = "extra"    // gallery item (no single-slot constraint)
)

// Person image sources (F25, ADR-038): how a stored image arrived, for provenance.
const (
	PersonImageSourceUpload     = "upload"     // owner-uploaded file
	PersonImageSourceEnrichment = "enrichment" // fetched from a metadata provider asset
	PersonImageSourcePromoted   = "promoted"   // copied from a gallery item into a core slot
)

// CorePersonImageRole reports whether a role is a single-slot core role (not the
// gallery). The empty string and unknown roles are not core.
func CorePersonImageRole(role string) bool {
	switch role {
	case PersonImageHeadshot, PersonImageBanner, PersonImagePoster:
		return true
	default:
		return false
	}
}

// ValidPersonImageRole reports whether role is one of the four known roles —
// the enum every request value is validated against (never a filesystem path).
func ValidPersonImageRole(role string) bool {
	return CorePersonImageRole(role) || role == PersonImageExtra
}

// PersonImage is one stored image for a person (F25, ADR-038). The bytes live on
// disk; this is the metadata index. Version mirrors the id and drives the
// cache-busting `?v=` on serving URLs (a replaced core slot gets a new id → new
// version → the browser re-fetches past the immutable cache).
type PersonImage struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"`
	Source    string    `json:"source"`
	Version   int64     `json:"version"` // == ID; the ?v= cache-buster
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

// PersonImageSet is the person-detail read model for images (F25, ADR-038): which
// core roles are filled (with their version for the `?v=` URL) and the ordered
// gallery image ids. The frontend builds /people/{id}/image/{role}?v={version} for
// filled roles and falls back to the themed placeholder for empty ones. Roles is
// always non-nil so it serializes as {} not null.
type PersonImageSet struct {
	Roles   map[string]PersonImageSlot `json:"roles"`
	Gallery []PersonImage              `json:"gallery"`
}

// PersonImageSlot is one filled core role: present=true with the version that
// busts the cache. An absent role is simply missing from PersonImageSet.Roles.
type PersonImageSlot struct {
	Present bool  `json:"present"`
	Version int64 `json:"version"`
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
	JobKindScan      = "scan"
	JobKindEnrich    = "enrich"
	JobKindPurge     = "purge"     // grace-period hard-delete sweep (F24, ADR-037)
	JobKindWriteback = "writeback" // queued batch metadata write (F30, ADR-048)
	JobStatusOK      = "success"
	JobStatusErr     = "error"
)

// Enrichment entity types stored in entity_enrichment (F22, ADR-033).
const (
	EnrichEntityPerson = "person"
	EnrichEntityVideo  = "video"
)

// EnrichedField is a canonical field resolved for one entity from a metadata
// source plugin (F22, ADR-033). It is shadow data kept distinct from the
// file-extracted fields; Provider carries the provenance the UI labels
// ("from <provider>"). An empty Provider would denote a file-sourced value when
// the resolver later interleaves the two (designed-in; provider-only in v1).
// Display hints how the SPA should render this field's value: "text" (default
// inline), "long_text" (block paragraph), "image_url" (render as <img>), or
// "url" (render as a link that opens in a new tab).
// Populated by the service layer from the field-metadata registry.
type EnrichedField struct {
	Canonical  string    `json:"canonical"`
	Label      string    `json:"label"`
	Display    string    `json:"display,omitempty"` // "text" | "long_text" | "image_url" | "url"
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
