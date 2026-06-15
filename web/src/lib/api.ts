// Typed client for the Holodex REST API. In dev, Vite proxies /api -> :7800;
// in production the Go binary serves both from the same origin (ADR-007).
import { filtersToParams } from './filters';
import type {
	Activity,
	Capabilities,
	Facet,
	JobRun,
	MediaDetailResponse,
	MediaFilters,
	MediaListResponse,
	MetadataKey,
	Person,
	RelatedResponse,
	SearchResponse,
	Tag,
	Video
} from './types';

const BASE = '/api/v1';

async function get<T>(path: string, fetchFn: typeof fetch = fetch, init?: RequestInit): Promise<T> {
	const res = await fetchFn(`${BASE}${path}`, init);
	if (!res.ok) {
		throw new Error(`API ${path} failed: ${res.status}`);
	}
	return res.json() as Promise<T>;
}

// Owner token for the gated /admin surface (F21.7). Kept in memory only — never
// localStorage (ADR-030 condition 2: avoid XSS exfiltration); re-entered after a
// reload. Empty when no token is configured (the single-user open default).
let adminToken = '';
export function setAdminToken(t: string) {
	adminToken = t.trim();
}
function adminHeaders(): HeadersInit {
	return adminToken ? { 'X-Admin-Token': adminToken } : {};
}

// getAuthed is get() carrying the X-Admin-Token header for the owner surface.
const getAuthed = <T>(path: string): Promise<T> => get<T>(path, fetch, { headers: adminHeaders() });

function buildQuery(f: MediaFilters): string {
	const s = filtersToParams(f).toString();
	return s ? `?${s}` : '';
}

export const api = {
	listMedia: (f: MediaFilters = {}, fetchFn?: typeof fetch) =>
		get<MediaListResponse>(`/media${buildQuery(f)}`, fetchFn),

	getMedia: (id: number, fetchFn?: typeof fetch) =>
		get<MediaDetailResponse>(`/media/${id}`, fetchFn),

	// "More with …" related shelves for a media item (ADR-031).
	related: (id: number, fetchFn?: typeof fetch) =>
		get<RelatedResponse>(`/media/${id}/related`, fetchFn),

	streamURL: (id: number) => `${BASE}/media/${id}/stream`,

	thumbnailURL: (id: number) => `${BASE}/media/${id}/thumbnail`,

	// Request background re-extraction (202 Accepted; image appears once ready).
	regenerateThumbnail: async (id: number, fetchFn: typeof fetch = fetch) => {
		const res = await fetchFn(`${BASE}/media/${id}/thumbnail`, { method: 'POST' });
		if (!res.ok && res.status !== 202) {
			throw new Error(`regenerate thumbnail failed: ${res.status}`);
		}
	},

	listPeople: (sort: 'name' | 'count' = 'name', fetchFn?: typeof fetch) =>
		get<{ items: Person[] }>(`/people${sort === 'count' ? '?sort=count' : ''}`, fetchFn),

	getPerson: (id: number, fetchFn?: typeof fetch) =>
		get<{ person: Person; items: Video[]; total: number }>(`/people/${id}`, fetchFn),

	listTags: (sort: 'name' | 'count' = 'name', fetchFn?: typeof fetch) =>
		get<{ items: Tag[] }>(`/tags${sort === 'count' ? '?sort=count' : ''}`, fetchFn),

	getTag: (id: number, fetchFn?: typeof fetch) =>
		get<{ tag: Tag; items: Video[]; total: number }>(`/tags/${id}`, fetchFn),

	search: (q: string, fetchFn?: typeof fetch) =>
		get<SearchResponse>(`/search?q=${encodeURIComponent(q)}`, fetchFn),

	// Configurable metadata fields (F20): filterable facets + key-discovery view.
	facets: (fetchFn?: typeof fetch) => get<{ facets: Facet[] }>(`/facets`, fetchFn),

	metadataKeys: (fetchFn?: typeof fetch) =>
		get<{ keys: MetadataKey[] }>(`/metadata-keys`, fetchFn),

	// System Activity (F21). capabilities is ungated, but it still carries the
	// X-Admin-Token header so the server can report owner:true once a token is set
	// (otherwise controls would stay locked even with a valid token).
	capabilities: () => getAuthed<Capabilities>(`/capabilities`),

	activity: () => getAuthed<Activity>(`/admin/activity`),

	activityHistory: (days = 30) =>
		getAuthed<{ runs: JobRun[] }>(`/admin/activity/history?days=${days}`),

	// Trigger a full re-index (F13.3). 202 + {started:false} means a scan was
	// already running — not an error.
	rescan: async (): Promise<{ started: boolean }> => {
		const res = await fetch(`${BASE}/admin/rescan`, { method: 'POST', headers: adminHeaders() });
		if (!res.ok && res.status !== 202) {
			throw new Error(`rescan failed: ${res.status}`);
		}
		const body = await res.json().catch(() => ({}));
		return { started: Boolean(body.started) };
	},

	// Reload metadata-mappings.yaml without a restart (F20.10).
	reloadConfig: async (): Promise<{ fields: number }> => {
		const res = await fetch(`${BASE}/admin/reload-config`, {
			method: 'POST',
			headers: adminHeaders()
		});
		if (!res.ok) {
			throw new Error(`reload failed: ${res.status}`);
		}
		const body = await res.json().catch(() => ({}));
		return { fields: Number(body.fields ?? 0) };
	}
};
