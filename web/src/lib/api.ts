// Typed client for the Holodex REST API. In dev, Vite proxies /api -> :7800;
// in production the Go binary serves both from the same origin (ADR-007).
import { filtersToParams } from './filters';
import type {
	Activity,
	Capabilities,
	EnrichCandidate,
	EnrichedField,
	EnrichSource,
	Facet,
	JobRun,
	MediaDetailResponse,
	MediaFilters,
	MediaListResponse,
	MetadataKey,
	Person,
	PersonAlias,
	PersonDetailResponse,
	PersonImageRole,
	PersonImageSet,
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

// sendAuthed issues a write (POST/DELETE) on the owner surface with the token
// header and an optional JSON body, returning the decoded response (or {}).
async function sendAuthed<T>(method: 'POST' | 'DELETE', path: string, body?: unknown): Promise<T> {
	const res = await fetch(`${BASE}${path}`, {
		method,
		headers: { ...adminHeaders(), ...(body ? { 'Content-Type': 'application/json' } : {}) },
		body: body ? JSON.stringify(body) : undefined
	});
	if (!res.ok && res.status !== 204) {
		throw new Error(`API ${path} failed: ${res.status}`);
	}
	return (res.status === 204 ? {} : await res.json().catch(() => ({}))) as T;
}

// uploadAuthed POSTs multipart FormData on the owner surface with the token header
// (no Content-Type — the browser sets the multipart boundary). Returns the decoded
// JSON body. A 409 is surfaced verbatim so callers can show "gallery is full".
async function uploadAuthed<T>(path: string, form: FormData): Promise<T> {
	const res = await fetch(`${BASE}${path}`, {
		method: 'POST',
		headers: adminHeaders(),
		body: form
	});
	if (!res.ok) {
		const body = (await res.json().catch(() => ({}))) as { error?: string };
		throw new Error(body.error || `API ${path} failed: ${res.status}`);
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

	// Person images (F24, ADR-037). Reads are public; a filled role serves the real
	// JPEG, an empty one a themed placeholder (the server reads the active skin from
	// ?skin= and the person's gender). Always pass the active skin; pass version for
	// the immutable cache-bust after a replace.
	personImageURL: (
		id: number,
		role: PersonImageRole,
		opts?: { version?: number; skin?: string }
	): string => {
		const q = new URLSearchParams();
		if (opts?.skin) q.set('skin', opts.skin);
		if (opts?.version) q.set('v', String(opts.version));
		const s = q.toString();
		return `${BASE}/people/${id}/image/${role}${s ? `?${s}` : ''}`;
	},

	// A specific gallery image by id, version-stamped for the immutable cache. Mirrors
	// personImageURL's opts so callers don't hand-append the skin/version query.
	personGalleryImageURL: (
		id: number,
		imageId: number,
		opts?: { version?: number; skin?: string }
	): string => {
		const q = new URLSearchParams();
		if (opts?.skin) q.set('skin', opts.skin);
		if (opts?.version) q.set('v', String(opts.version));
		const s = q.toString();
		return `${BASE}/people/${id}/images/${imageId}${s ? `?${s}` : ''}`;
	},

	// Owner-gated mutations (ADR-030). Upload posts a multipart {image, role};
	// 409 surfaces "gallery is full" via uploadAuthed's error.
	uploadPersonImage: (id: number, file: File, role: PersonImageRole) => {
		const form = new FormData();
		form.append('image', file);
		form.append('role', role);
		return uploadAuthed<{ id: number; version: number }>(`/people/${id}/image`, form);
	},

	deletePersonImage: (id: number, imageId: number) =>
		sendAuthed<Record<string, never>>('DELETE', `/people/${id}/images/${imageId}`),

	// Promote a gallery extra into a core slot (a copy; the original is untouched).
	promotePersonImage: (id: number, imageId: number, role: PersonImageRole) =>
		sendAuthed<{ id: number; version: number }>(
			'POST',
			`/people/${id}/images/${imageId}/promote`,
			{ role }
		),

	// Persist gallery order; returns the refreshed image set.
	reorderPersonImages: (id: number, order: number[]) =>
		sendAuthed<{ images: PersonImageSet }>('POST', `/people/${id}/images/reorder`, { order }),

	listPeople: (sort: 'name' | 'count' = 'name', fetchFn?: typeof fetch) =>
		get<{ items: Person[] }>(`/people${sort === 'count' ? '?sort=count' : ''}`, fetchFn),

	getPerson: (id: number, fetchFn?: typeof fetch) =>
		get<PersonDetailResponse>(`/people/${id}`, fetchFn),

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
		const body = await sendAuthed<{ started?: boolean }>('POST', `/admin/rescan`);
		return { started: Boolean(body.started) };
	},

	// Reload metadata-mappings.yaml + metadata-sources.yaml without a restart (F20.10/F22.2d).
	reloadConfig: async (): Promise<{ fields: number }> => {
		const body = await sendAuthed<{ fields?: number }>('POST', `/admin/reload-config`);
		return { fields: Number(body.fields ?? 0) };
	},

	// Metadata source plugins — People enrichment (F22). All owner-gated.
	enrichSources: () => getAuthed<{ sources: EnrichSource[] }>(`/enrich/sources`),

	enrichResolve: (personId: number, provider: string, query: string) =>
		sendAuthed<{ candidates: EnrichCandidate[] }>('POST', `/people/${personId}/enrich/resolve`, {
			provider,
			query
		}),

	enrichApply: (personId: number, provider: string, externalId: string) =>
		sendAuthed<{ enriched: EnrichedField[] }>('POST', `/people/${personId}/enrich`, {
			provider,
			external_id: externalId
		}),

	enrichClear: (personId: number, provider: string) =>
		sendAuthed<Record<string, never>>('DELETE', `/people/${personId}/enrich/${encodeURIComponent(provider)}`),

	// Person aliases & merge (F23, ADR-036). All owner-gated.
	//
	// addAlias returns either the updated alias list, or — when the name already
	// belongs to another person — that person as a `conflict` (HTTP 409), so the UI
	// can offer a merge instead of silently collapsing possibly-distinct people.
	addAlias: async (
		personId: number,
		alias: string
	): Promise<{ aliases?: PersonAlias[]; conflict?: Person }> => {
		const res = await fetch(`${BASE}/people/${personId}/aliases`, {
			method: 'POST',
			headers: { ...adminHeaders(), 'Content-Type': 'application/json' },
			body: JSON.stringify({ alias })
		});
		if (res.status === 409) {
			const body = await res.json().catch(() => ({}));
			return { conflict: body.conflict as Person };
		}
		if (!res.ok) {
			throw new Error(`API /people/${personId}/aliases failed: ${res.status}`);
		}
		return { aliases: ((await res.json()) as { aliases: PersonAlias[] }).aliases };
	},

	deleteAlias: (personId: number, aliasId: number) =>
		sendAuthed<Record<string, never>>('DELETE', `/people/${personId}/aliases/${aliasId}`),

	// Merge `fromId` into `canonicalId`: the from-person's videos move to canonical,
	// its name becomes an alias, and it is deleted. Returns the updated canonical.
	mergePersons: (canonicalId: number, fromId: number) =>
		sendAuthed<{ person: Person }>('POST', `/people/${canonicalId}/merge`, { from_id: fromId })
};
