// Shape of the REST payloads (mirrors internal/model + handlers, ADR-006).

// PersonAlias is one owner-curated alternate name (F23, ADR-036). The id is the
// stable handle the delete control uses.
export interface PersonAlias {
	id: number;
	alias: string;
}

export interface Person {
	id: number;
	name: string;
	video_count?: number;
	// Headshot image id on the people-list read — the avatar's ?v= cache-buster so the
	// list refreshes when the headshot changes (e.g. after enrichment) instead of showing
	// the stale cached image (F25.29). Absent/0 = no headshot (placeholder).
	headshot_version?: number;
	aliases?: PersonAlias[]; // present on the person-detail read (F23)
}

// Person image roles (F25, ADR-038). Three single-slot core roles plus the
// free-form `extra` gallery. Mirrors model.ValidPersonImageRole on the server.
export type PersonImageRole = 'headshot' | 'banner' | 'poster' | 'extra';

// The three single-slot core roles (the croppable, promotable slots — not the `extra`
// gallery). Canonical list + derived type, so the role set lives in exactly one place;
// `satisfies` guarantees every entry is a real non-extra role.
export const CORE_ROLES = ['headshot', 'banner', 'poster'] as const satisfies readonly Exclude<
	PersonImageRole,
	'extra'
>[];
export type CoreRole = (typeof CORE_ROLES)[number];

// PersonImage is one stored gallery image in the person-detail read model (F25).
// Version mirrors the id and drives the `?v=` cache-buster on serving URLs.
export interface PersonImage {
	id: number;
	role: PersonImageRole;
	source: string; // 'upload' | 'enrichment' | 'promoted'
	version: number;
	width: number;
	height: number;
	sort_order: number;
	created_at: string;
}

// PersonImageSet is the person-detail image read model (F25): which core roles are
// filled (with the `?v=` version) and the ordered gallery. Empty core roles are
// simply absent from `roles`.
export interface PersonImageSet {
	roles: Record<string, { present: boolean; version: number }>;
	gallery: PersonImage[];
}

export interface Tag {
	id: number;
	name: string;
	video_count?: number;
}

// Studio is a first-class entity (F38, ADR-053). Same read shape as Tag; its name is
// derived identity (video_studios follows the resolved studio field, no rename/merge).
export interface Studio {
	id: number;
	name: string;
	video_count?: number;
}

export interface ExtraMetadata {
	source_key: string;
	value: string;
}

export interface Video {
	id: number;
	file_path: string;
	file_size: number;
	title: string;
	duration_sec: number;
	width: number;
	height: number;
	video_codec?: string; // ffprobe stream/format details (F12.4)
	audio_codec?: string;
	bitrate_kbps?: number;
	container?: string;
	recorded_at?: string | null;
	indexed_at: string;
	thumbnail_url?: string | null; // present once an image exists (ADR-009)
	people?: Person[];
	tags?: Tag[];
}

export interface MediaListResponse {
	items: Video[];
	total: number;
	limit: number;
	offset: number;
}

// MappedField is a configurable canonical field resolved for one video (F20.3).
export interface MappedField {
	canonical: string;
	label: string;
	values: string[];
}

// ResolvedValue is one surviving value of a field with its provenance + curation
// state (F30). A value present in multiple sources is reported once, with every
// contributing source namespace listed in `sources`.
export interface ResolvedValue {
	value: string;
	sources: string[]; // contributing namespaces, e.g. ["tmdb","file"], ["manual"]
	manual?: boolean; // owner-added value
	no_write?: boolean; // shown but excluded from the file write
}

// ResolvedField is one canonical field merged from all configured sources (F27/F30).
// `values` holds the surviving display values; `items` carries the per-value
// provenance + curation state the curation UI consumes. winning_source is the
// namespace:key that supplied the (first) value. display mirrors the registry hint.
export interface ResolvedField {
	canonical: string;
	label: string;
	display?: 'long_text' | 'image_url' | 'url';
	values: string[];
	items?: ResolvedValue[];
	multi?: boolean; // merge-mode (set) field: UI shows add + per-value remove
	winning_source?: string; // e.g. "tmdb:title" | "file:Title" | "manual:genres"
	// F36 (ADR-051) — per-field source-of-truth, present on replace (scalar) fields only.
	// `decision` is the standing source choice (absent ⇒ implicit file default); `in_sync`
	// is false when the decided value differs from the value embedded in the file; the
	// `candidates` feed the SourceSelect segments + the candidates line.
	decision?: FieldDecision;
	in_sync?: boolean;
	candidates?: FieldCandidate[];
}

// F36 — Per-field source-of-truth decisions (ADR-051). A standing, per-item, per-field
// choice naming which source is *true* for a replace (scalar) field: it overrides global
// mapping precedence and drives both display and writeback. Merge (set) fields are
// unchanged — they keep F30 per-value curation; the source decision is replace-only.

// DecisionSource names the pinned source: the entity's baseline — `file` for videos,
// `record` for persons (F37, RD4: the baseline label is per-entity) — a specific matched
// provider (`provider:<name>`), or a frozen manual literal.
export type DecisionSource = 'file' | 'record' | `provider:${string}` | 'manual';

// FieldDecision is the per-field decision marker on a ResolvedField. `standing` is true for
// an explicit stored decision and false for the implicit file default (undecided).
// `manual_value` is present only when source === 'manual'.
export interface FieldDecision {
	source: DecisionSource;
	standing: boolean;
	manual_value?: string;
}

// FieldCandidate is one selectable source value for a replace field — the file value or a
// matched provider's value — feeding the SourceSelect `Adopt` segments and the candidates
// line. A provider candidate with an empty `value` yields no `Adopt` segment (you cannot
// adopt an empty value).
export interface FieldCandidate {
	source: DecisionSource; // 'file' | 'provider:<name>'
	provider?: string; // provider name when source is 'provider:<name>'
	value: string;
}

// DecisionRequest is the body of PUT …/decision (F36, ADR-051 §7). `manual_value` is
// required iff source === 'manual'; a `provider:<name>` must be a currently-matched provider.
export interface DecisionRequest {
	source: DecisionSource;
	manual_value?: string;
}

// CurationAction is a value-level owner decision (F30, ADR-048).
export type CurationAction = 'add' | 'suppress' | 'nowrite';

// CurationRequest records or clears one value-level decision for a field.
export interface CurationRequest {
	field: string;
	value: string;
	action: CurationAction;
}

export interface MediaDetailResponse {
	video: Video;
	metadata: ExtraMetadata[] | null;
	fields: MappedField[] | null;
	resolved?: ResolvedField[] | null; // unified merged view (F27); supersedes fields when present
	enriched?: EnrichedField[] | null;
	// Studio entities linked to this video (F38, ADR-053): the resolved studio value
	// links to its /studios/{id} page; the link target always matches the displayed
	// value because video_studios is derived from that same resolution (RD1).
	studios?: Studio[] | null;
}

// WritebackRequest asks the server to embed a batch of resolved field values
// into the media file's tags in a single exiftool pass (F28, ADR-041).
export interface WritebackRequest {
	fields: Array<{
		field: string;
		values: string[];
		source: string;
	}>;
}

// A soft-deleted item in the owner's Trash view (F24, ADR-037). purge_at is null
// when auto-purge is disabled (grace = 0) — it lingers until purged manually.
export interface TrashEntry {
	id: number;
	title: string;
	path: string;
	deleted_at: string;
	purge_at: string | null;
}

export interface FacetValue {
	value: string;
	count: number;
}

// Facet is a filterable mapped field plus its distinct values (F20.4).
export interface Facet {
	canonical: string;
	label: string;
	multi: boolean;
	values: FacetValue[];
}

// MetadataKey is one row of the library-wide key-discovery view (F20.9).
export interface MetadataKey {
	source_key: string;
	count: number;
	samples: string[];
	mapped: boolean;
}

export interface SearchResponse {
	videos: Video[] | null;
	people: Person[] | null;
	tags: Tag[] | null;
	studios: Studio[] | null;
}

// "More with …" related-media shelves (ADR-031, QW2/QW3). Either block is null when
// the item has no people / no tags; items is [] when the entity has no other siblings.
export interface RelatedShelf {
	id: number;
	name: string;
	items: Video[];
}

export interface RelatedResponse {
	person: RelatedShelf | null;
	tag: RelatedShelf | null;
}

// System Activity — "under the hood" read-model (F21, ADR-028).
export interface ScanSummary {
	trigger: string;
	finished_at: string;
	duration_ms: number;
	seen: number;
	added: number;
	updated: number;
	removed: number;
	skipped: number;
	errors: number;
}

export interface ScanStatus {
	state: 'idle' | 'running';
	trigger?: string;
	started_at?: string | null;
	last_run: ScanSummary | null;
	next_scheduled_at?: string | null;
}

export interface ThumbnailStats {
	depth: number;
	high: number;
	normal: number;
	in_flight: number;
	workers: number;
}

export interface LibraryCounts {
	videos_active: number;
	videos_inactive: number;
	people: number;
	tags: number;
}

export interface ActivitySystem {
	ready: boolean;
	version: string;
	media_path_present: boolean;
	controls_unauthenticated: boolean;
	uptime_seconds?: number;
}

export interface Activity {
	scan: ScanStatus;
	thumbnails: ThumbnailStats;
	library: LibraryCounts;
	system: ActivitySystem;
}

// JobRun is one row of the 30-day activity history (F21.3).
export interface JobRun {
	id: number;
	kind: string;
	trigger: string;
	status: string;
	started_at: string;
	finished_at: string;
	duration_ms: number;
	seen: number;
	added: number;
	updated: number;
	removed: number;
	skipped: number;
	errors: number;
	error_message?: string;
	detail?: string; // short description for non-scan jobs, e.g. enrich (F22.6b)
}

// Capabilities tells the SPA whether it may act as owner (F21.7).
export interface Capabilities {
	owner: boolean;
	auth_required: boolean;
	// Soft-delete grace window in seconds (F24); 0 = auto-purge disabled.
	delete_grace_period_seconds: number;
	// card_layout is the operator's preferred browse-grid aspect ratio:
	// "wide" (16:9, default) for personal/AMV libraries, "poster" (2:3) for film libraries.
	card_layout: 'wide' | 'poster';
	// person_gallery_max is the per-person 'extra' gallery cap (F25), so the gallery
	// can warn at the limit and offer the owner an explicit over-cap "add anyway".
	person_gallery_max: number;
}

// Metadata source plugins — People enrichment (F22, ADR-033).

// EnrichSource is one configured provider the owner can enrich from (no base_url
// or secrets — F22.9d).
export interface EnrichSource {
	name: string;
	entity_types: string[];
}

// EnrichCandidate is one provider match the owner confirms (F22.5b). Confidence
// is advisory — the owner always confirms (no silent auto-apply).
export interface EnrichCandidate {
	external_id: string;
	namespace: string;
	label: string;
	confidence: number;
	disambiguation?: string;
}

// EnrichedField is a resolved field with provenance (F22.7). Provider is the
// source name ("from <provider>"); an empty Provider would denote a file value.
// display hints the render mode: "image_url" → <img>, "long_text" → block paragraph,
// "url" → link (opens in a new tab), absent/other → inline text.
export interface EnrichedField {
	canonical: string;
	label: string;
	display?: 'text' | 'long_text' | 'image_url' | 'url';
	values: string[];
	provider: string;
	external_id?: string;
	fetched_at?: string;
}

// Per-item metadata refresh outcome (F31, ADR-047). One entry per attempted
// source (file first, then each linked provider). sources_disagree is reserved
// (populated by the future batch op, F31.11); single-item it is false.
export interface RefreshSourceResult {
	source: string; // "file" or the provider name
	ok: boolean;
	changed: boolean;
	error?: string;
}

export interface RefreshReport {
	video_id: number;
	sources: RefreshSourceResult[];
	changed: boolean;
	sources_disagree: boolean;
}

export interface PersonDetailResponse {
	person: Person;
	items: Video[];
	total: number;
	// F37: unified resolved view (same shape as media detail's resolved[], baseline `record`;
	// `in_sync` is always absent — persons have no file). Supersedes the retired enriched[].
	resolved?: ResolvedField[] | null;
	images?: PersonImageSet; // F25: per-role presence + version + ordered gallery
}

// StudioDetailResponse is GET /studios/{id} (F38, ADR-053): the studio, its videos,
// and resolved[] in the record vocabulary (in_sync always absent — studios have no
// file). Details render only when a field beyond `name` has a value or a decision.
export interface StudioDetailResponse {
	studio: Studio;
	items: Video[];
	total: number;
	resolved?: ResolvedField[] | null;
}

// PersonRenameConflict is the 409 body of POST /people/{id}/rename (F37, RD1): the
// existing person the new name collides with, so the UI can offer the F23 merge flow
// (with both video counts) instead — never an auto-merge.
export interface PersonRenameConflict {
	id: number;
	name: string;
	video_count: number;
}

export type Resolution = 'All' | 'SD' | 'HD' | 'FHD' | '4K';

// Sort keys accepted by GET /media?sort= (F12.1). Mirrors repo.VideoFilter.orderBy.
// "random" is a seeded shuffle paired with a ?seed= param (ADR-045).
export type SortOrder =
	| 'added_desc'
	| 'added_asc'
	| 'title_asc'
	| 'title_desc'
	| 'duration_desc'
	| 'duration_asc'
	| 'resolution_desc'
	| 'resolution_asc'
	| 'random';

// Sort options for the unpaged People/Tags indexes — the single source of truth for
// both pages. 'name'/'count' map to the server toggle; 'random' is a client-side
// seeded shuffle of the name-ordered list (ADR-045 §3).
export const PEOPLE_TAG_SORTS = ['name', 'count', 'random'] as const;
export type PeopleTagSort = (typeof PEOPLE_TAG_SORTS)[number];

export interface MediaFilters {
	q?: string;
	person?: number[];
	tag?: number[];
	// Studio-entity filter (F38, HOLODEX-120): repeatable ?studio_id, filtering by the
	// derived video_studios link. Distinct from the legacy mapped `studio` string filter
	// (still under `mapped`, kept for REST/MCP back-compat but no longer surfaced in the UI).
	studio_id?: number[];
	duration_min?: number; // minutes
	duration_max?: number; // minutes
	resolution?: Resolution;
	year_min?: number;
	year_max?: number;
	sort?: SortOrder;
	// seed parameterizes the "random" sort's deterministic shuffle (ADR-045); sent
	// with the API request (paging param) but not the shareable URL. Ignored unless
	// sort==='random'.
	seed?: number;
	mapped?: Record<string, string>; // configurable mapped-field filters (F20.5)
	limit?: number;
	offset?: number;
}
