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
