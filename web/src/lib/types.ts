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

export interface MediaFilters {
	q?: string;
	person?: number[];
	tag?: number[];
	duration_min?: number; // minutes
	duration_max?: number; // minutes
	resolution?: Resolution;
	year_min?: number;
	year_max?: number;
	limit?: number;
	offset?: number;
}
