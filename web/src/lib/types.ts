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

export interface MediaDetailResponse {
	video: Video;
	metadata: ExtraMetadata[] | null;
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
	limit?: number;
	offset?: number;
}
