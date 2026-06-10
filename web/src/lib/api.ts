// Typed client for the Holodex REST API. In dev, Vite proxies /api -> :7800;
// in production the Go binary serves both from the same origin (ADR-007).
import { filtersToParams } from './filters';
import type {
	MediaDetailResponse,
	MediaFilters,
	MediaListResponse,
	Person,
	SearchResponse,
	Tag,
	Video
} from './types';

const BASE = '/api/v1';

async function get<T>(path: string, fetchFn: typeof fetch = fetch): Promise<T> {
	const res = await fetchFn(`${BASE}${path}`);
	if (!res.ok) {
		throw new Error(`API ${path} failed: ${res.status}`);
	}
	return res.json() as Promise<T>;
}

function buildQuery(f: MediaFilters): string {
	const s = filtersToParams(f).toString();
	return s ? `?${s}` : '';
}

export const api = {
	listMedia: (f: MediaFilters = {}, fetchFn?: typeof fetch) =>
		get<MediaListResponse>(`/media${buildQuery(f)}`, fetchFn),

	getMedia: (id: number, fetchFn?: typeof fetch) =>
		get<MediaDetailResponse>(`/media/${id}`, fetchFn),

	streamURL: (id: number) => `${BASE}/media/${id}/stream`,

	listPeople: (sort: 'name' | 'count' = 'name', fetchFn?: typeof fetch) =>
		get<{ items: Person[] }>(`/people${sort === 'count' ? '?sort=count' : ''}`, fetchFn),

	getPerson: (id: number, fetchFn?: typeof fetch) =>
		get<{ person: Person; items: Video[]; total: number }>(`/people/${id}`, fetchFn),

	listTags: (sort: 'name' | 'count' = 'name', fetchFn?: typeof fetch) =>
		get<{ items: Tag[] }>(`/tags${sort === 'count' ? '?sort=count' : ''}`, fetchFn),

	getTag: (id: number, fetchFn?: typeof fetch) =>
		get<{ tag: Tag; items: Video[]; total: number }>(`/tags/${id}`, fetchFn),

	search: (q: string, fetchFn?: typeof fetch) =>
		get<SearchResponse>(`/search?q=${encodeURIComponent(q)}`, fetchFn)
};
