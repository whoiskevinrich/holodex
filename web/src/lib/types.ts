// Shape of the REST payloads (mirrors internal/model + handlers, ADR-006).

export interface Person {
	id: number;
	name: string;
	video_count?: number;
}

export interface Tag {
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

export interface MediaDetailResponse {
	video: Video;
	metadata: ExtraMetadata[] | null;
	fields: MappedField[] | null;
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
export interface EnrichedField {
	canonical: string;
	label: string;
	values: string[];
	provider: string;
	external_id?: string;
	fetched_at?: string;
}

export interface PersonDetailResponse {
	person: Person;
	items: Video[];
	total: number;
	enriched?: EnrichedField[] | null;
}

export type Resolution = 'All' | 'SD' | 'HD' | 'FHD' | '4K';

// Sort keys accepted by GET /media?sort= (F12.1). Mirrors repo.VideoFilter.orderBy.
export type SortOrder =
	| 'added_desc'
	| 'added_asc'
	| 'title_asc'
	| 'title_desc'
	| 'duration_desc'
	| 'duration_asc'
	| 'resolution_desc'
	| 'resolution_asc';

export interface MediaFilters {
	q?: string;
	person?: number[];
	tag?: number[];
	duration_min?: number; // minutes
	duration_max?: number; // minutes
	resolution?: Resolution;
	year_min?: number;
	year_max?: number;
	sort?: SortOrder;
	mapped?: Record<string, string>; // configurable mapped-field filters (F20.5)
	limit?: number;
	offset?: number;
}
