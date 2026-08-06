// Package model holds the core domain types shared across layers.
package model

import (
	"strings"
	"time"
)

// MaxNameLen bounds a stored entity name/alias/tag (F23.1, generalized by ADR-075
// item 11) — generous for any real name while keeping the row and the FTS term
// sane. One constant shared by every layer that validates a name (the rename/alias
// HTTP handlers, resolveOrCreateByName's tag-creation choke point) so the cap can't
// drift between them.
const MaxNameLen = 200

// Thumbnail pipeline states stored in Video.ThumbnailState (ADR-009). The empty
// string is the zero value, meaning "never attempted". Centralized here so the
// repo (SQL), the thumbnail pipeline, and the API agree on one vocabulary.
const (
	ThumbnailNone      = ""
	ThumbnailEmbedded  = "embedded"  // Tier 1: extracted from container cover art
	ThumbnailGenerated = "generated" // Tier 2: extracted frame via ffmpeg
	ThumbnailUploaded  = "uploaded"  // Tier 0: owner-uploaded poster (F52) — highest
	// precedence; excluded from the startup sweep (repo.go's NULL/failed-only
	// query) same as embedded/generated, so it is never auto-replaced.
	ThumbnailFailed = "failed" // last attempt errored; retried by startup sweep
)

// HasThumbnailImage reports whether a thumbnail state implies an image exists on
// disk (and thus a serving URL can be offered).
func HasThumbnailImage(state string) bool {
	return state == ThumbnailEmbedded || state == ThumbnailGenerated || state == ThumbnailUploaded
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
	// "embedded", "generated", "uploaded" (F52), or "failed". Internal bookkeeping —
	// the API exposes ThumbnailURL and PosterUploaded instead.
	ThumbnailState string `json:"-"`
	// ThumbnailURL is the serving URL, set by the API layer when an image exists.
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	// PosterURL is the larger detail-page poster tier's serving URL (F53,
	// HOLODEX-253), computed by the API layer alongside ThumbnailURL. It always
	// resolves to a valid image once ThumbnailURL does — the route falls back to
	// the thumbnail-tier bytes until a poster-tier derivative has been generated.
	PosterURL string `json:"poster_url,omitempty"`
	// PosterUploaded reports whether the current poster is an owner upload (F52) —
	// the one bit of ThumbnailState the SPA needs, to show a "Remove" action.
	PosterUploaded bool `json:"poster_uploaded,omitempty"`

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
	// PosterVersion mirrors HeadshotVersion for the poster role (F55 P0-6) — independent
	// of HeadshotVersion, since a person can have one role without the other. 0 = no
	// poster (the placeholder is served).
	PosterVersion int64 `json:"poster_version,omitempty"`
	// Aliases are owner-curated alternate names (F23, ADR-036), each searchable.
	// Populated only on the person-detail read; omitted (nil) elsewhere.
	Aliases []PersonAlias `json:"aliases,omitempty"`
}

// EntityAlias is one owner-curated alternate name for a named entity — person,
// studio, or tag — stored in the shared entity_aliases spine (F43, ADR-061). The id
// gives the UI and the delete endpoint a stable handle.
type EntityAlias struct {
	ID    int64  `json:"id"`
	Alias string `json:"alias"`
}

// PersonAlias is the person-entity alias (F23, ADR-036): the shared EntityAlias
// shape under its original name, so the F23 person model and endpoints read unchanged.
type PersonAlias = EntityAlias

// EntityRef is the minimal identity of a named entity — id, name, and active-video
// count — that Person/Studio/Tag all satisfy (F43 S5, ADR-061). Used by the near-miss
// review queue to carry both sides of a possible-duplicate pair and the editor
// soft-warning's look-alike, without pulling the full per-entity payload.
type EntityRef struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	VideoCount int    `json:"video_count,omitempty"`
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
	// Aliases are owner-curated alternate names (F43, ADR-061), each searchable.
	// Populated on the tags-list read (tags have no detail page, RD7); omitted elsewhere.
	Aliases []EntityAlias `json:"aliases,omitempty"`
	// ParentTagID is the broader tag this tag sits under, or nil at the root
	// (F50, ADR-075 D1) — a strict one-parent tree, no DAG.
	ParentTagID *int64 `json:"parent_tag_id,omitempty"`
	// Ancestors is this tag's ancestor chain, root-first (F50, ADR-075 D1
	// P1-3) — the tag-detail breadcrumb. Populated only on the tag-detail
	// read (GetTag); omitted on the /tags list, which has no per-row use
	// for it.
	Ancestors []string `json:"ancestors,omitempty"`
	// Source is this tag's provenance on the one video it was read alongside —
	// "file" / "manual" / "provider:<name>" (F50, ADR-075 D3). Populated only on
	// Video.Tags (attachAssociations); empty on the /tags list and tag-detail reads,
	// which have no single video context for a per-link column to describe.
	Source string `json:"source,omitempty"`
	// WritebackEnabled is whether this tag's name contributes to a video's
	// Genre writeback value (HOLODEX-239, ADR-077 D1) — defaults true, so an
	// existing tag behaves identically until an owner explicitly excludes it.
	// Affects only that one output, never creation/search/attachment.
	// Populated on the tag-detail read (GetTag).
	WritebackEnabled bool `json:"writeback_enabled"`
}

// Category groups tags for browsing without merging or altering them
// (HOLODEX-240, ADR-078) -- deliberately reduced compared to Tag/Person/Studio:
// no provenance, no alias/merge machinery, create/rename/delete only (D1). No
// video count (categories don't attach to videos directly, only tags do --
// spec Non-Goals). Tags holds the category's member tags, populated only on
// the category-detail read (GetCategory); omitted on the /categories list,
// which has no per-row use for it.
type Category struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// TagCount is the member-tag count, populated on both the /categories list
	// (the pill's count badge) and the detail read (len(Tags), for free). Not
	// omitempty -- an empty category legitimately reports 0, not an absent field.
	TagCount int `json:"tag_count"`
	// TagIDs is the member tag id set, populated only on the /categories list
	// (HOLODEX-240) -- lets the "Remove from category…" picker filter to
	// categories that actually contain one of the selected tags, entirely
	// client-side against this already-loaded, unpaged list (no per-click
	// round trip). Omitted on the detail read, which has Tags (full objects)
	// instead.
	TagIDs []int64 `json:"tag_ids,omitempty"`
	Tags   []Tag   `json:"tags,omitempty"`
}

// Studio image roles (F51, ADR-079). All three are "core" single-slot roles — unlike
// Person (ADR-038) a studio has no gallery/extra role, so every role is single-slot by
// construction (studio_images' unique index is a plain composite, not partial).
const (
	StudioImageIcon   = "icon"   // studios list well
	StudioImageLogo   = "logo"   // studio detail header (formerly the studio_logos cache)
	StudioImagePoster = "poster" // reserved for future use; no consumer yet
)

// Studio image sources (F51, ADR-079): how a stored image arrived, for provenance and the
// ADR-049-style provenance lock. No "promoted" — there is no gallery to promote from.
const (
	StudioImageSourceUpload     = "upload"     // owner-uploaded file
	StudioImageSourceEnrichment = "enrichment" // fetched from a metadata provider asset
)

// ValidStudioImageRole reports whether role is one of the three known roles — the enum
// every request value is validated against (never a filesystem path).
func ValidStudioImageRole(role string) bool {
	switch role {
	case StudioImageIcon, StudioImageLogo, StudioImagePoster:
		return true
	default:
		return false
	}
}

// Studio is a first-class production-company/publisher entity (F38, ADR-053). Its
// name is a derived identity — video_studios links follow the resolved `studio`
// field, not raw file extraction. F43 (ADR-061) adds owner alias/merge/rename over
// the shared name-identity spine: a merge registers the loser's name as an alias so
// it survives RelinkVideoStudios re-derivation (superseding ADR-053 RD4).
type Studio struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	VideoCount int    `json:"video_count,omitempty"`
	// Aliases are owner-curated alternate names (F43, ADR-061), each searchable.
	// Populated on the studio-detail read; omitted (nil) elsewhere.
	Aliases []EntityAlias `json:"aliases,omitempty"`
	// IconURL/LogoURL/PosterURL are serving URLs for the studio's self-hosted image
	// roles (F51, ADR-079): /api/v1/studios/{id}/images/{role}?v={id} when that role's
	// slot is filled — pointing at Holodex's own origin, never a hotlinked provider
	// CDN. Empty when a role has no image (the SPA renders its per-role fallback).
	// Always populated on both list and detail reads, so a future consumer of
	// PosterURL needs no backend change. LogoURL replaces the pre-F51
	// studio_logos-backed field of the same name (ADR-057, superseded).
	IconURL   string `json:"icon_url,omitempty"`
	LogoURL   string `json:"logo_url,omitempty"`
	PosterURL string `json:"poster_url,omitempty"`
	// ImageVersions holds the studio_images row id per filled role (the ?v= cache
	// buster) — internal; the API layer turns it into the three URLs above via
	// setStudioImageURLs. Absent role = no image. Mirrors the old LogoVersion field,
	// generalized to a map across three roles instead of one int.
	ImageVersions map[string]int64 `json:"-"`
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
	JobKindScan              = "scan"
	JobKindEnrich            = "enrich"
	JobKindPurge             = "purge"               // grace-period hard-delete sweep (F24, ADR-037)
	JobKindRefresh           = "refresh"             // per-item forced re-extract + re-enrich (F31, ADR-047)
	JobKindWriteback         = "writeback"           // queued batch metadata write (F30, ADR-048)
	JobKindStudioBackfill    = "studio-backfill"     // one-time video→studio link derivation (F38, ADR-053)
	JobKindIdentityBackfill  = "identity-backfill"   // one-time near-miss review-queue seed (F43, ADR-061)
	JobKindExtraction        = "extraction"          // library-wide filename extraction pass (F48.5b, ADR-067)
	JobKindPersonBackfill    = "person-backfill"     // one-time video→person link derivation (F40, ADR-072)
	JobKindPersonOrphanSweep = "person-orphan-sweep" // periodic unauthored-orphan prune (F40, ADR-072)
	JobStatusOK              = "success"
	JobStatusErr             = "error"
)

// Enrichment entity types stored in entity_enrichment (F22, ADR-033). Tag is not
// enrichable, but shares the name-identity entity_type namespace (entity_aliases /
// keep-separate / review-queue) with person and studio (F43, ADR-061).
const (
	EnrichEntityPerson = "person"
	EnrichEntityVideo  = "video"
	EnrichEntityStudio = "studio"
	EntityTag          = "tag"
)

// EntityTypeForField maps an entity-field's canonical field key to the
// entity_type F43's identity spine uses (people values are Person entities;
// studio values are Studio entities) — shared by internal/extract (F48.3d's
// entity resolution) and internal/repo (F48.6's suggested-entity-name
// lookup) so both packages read the same mapping without an import cycle
// (extract already depends on writequeue, which depends on repo).
var EntityTypeForField = map[string]string{
	"people": EnrichEntityPerson,
	"studio": EnrichEntityStudio,
}

// MultiValueDelimiter joins/splits a multi-value field's values in the
// extraction review row (e.g. a `{people}` cast rendered as one comparable
// string). Centralized here — like EntityTypeForField, without an import cycle
// — so the write side (internal/extract's join on resolve) and the read side
// (internal/repo's per-value candidate split for the queue) can never drift.
const MultiValueDelimiter = ", "

// SplitJoined reverses a MultiValueDelimiter join back into individual values,
// dropping empties. The shared inverse of the review row's join, used by both
// internal/extract's resolve path and internal/repo's queue read.
func SplitJoined(joined string) []string {
	parts := strings.Split(joined, MultiValueDelimiter)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// InternalFieldPrefix marks a provider→core "sidecar" enrichment field-key: it is
// persisted in entity_enrichment like any other field but is never displayed
// (FieldsFromRows skips it) and never resolved (it is not a mapped canonical field).
// It carries plumbing the core consumes directly. See StudioExternalIDsField and
// ADR-054.
const InternalFieldPrefix = "_"

// StudioExternalIDsField is the wire field-key a video provider uses to hand per
// production-company external ids to studio-link derivation (HOLODEX-122, ADR-054).
// Each value is self-describing "<namespace>:<id> <name>" (the id token has no space,
// so the name is the remainder), e.g. "tmdb:174 Warner Bros. Pictures".
// RelinkVideoStudios parses these into a name→external_id side-map. It starts with
// InternalFieldPrefix so it never displays or resolves.
const StudioExternalIDsField = InternalFieldPrefix + "studio_external_ids"

// PersonExternalIDsField is the wire field-key core enrich synthesizes from a video
// provider's structured `people[]` credits (F32, contract §4.5, ADR-055) — the
// person analogue of StudioExternalIDsField. Unlike the studio sidecar (which a
// provider emits directly alongside its flat `studio` field), `people[]` arrives as
// a separate top-level array; core builds this sidecar from it so
// RelinkVideoPeople's caller can recover a name→external_id map the same way studio
// already does. Same self-describing "<namespace>:<id> <name>" value shape.
const PersonExternalIDsField = InternalFieldPrefix + "person_external_ids"

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
	// It is free text for the operator: nothing parses it (ADR-071).
	Detail string `json:"detail,omitempty"`
	// EntityType/EntityID name the single entity this run acted on (ADR-071),
	// using the EnrichEntity* vocabulary. Zero-valued for library-wide kinds
	// (scan, the backfills), which read as "not attributed". Intentionally not a
	// foreign key — job_runs is an audit table, so a run outlives the entity it
	// describes and EntityID may dangle.
	EntityType string `json:"entity_type,omitempty"`
	EntityID   int64  `json:"entity_id,omitempty"`
	// BatchID is the writeback snapshot batch (migration 0027) this run belongs
	// to, carried as a field so Revert reads it structurally instead of parsing
	// it back out of Detail (ADR-071).
	BatchID string `json:"batch_id,omitempty"`
}
