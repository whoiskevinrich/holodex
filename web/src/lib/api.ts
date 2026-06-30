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
	PeopleTagSort,
	PersonImageRole,
	PersonImageSet,
	RefreshReport,
	RelatedResponse,
	SearchResponse,
	Tag,
	TrashEntry,
	Video,
	WritebackRequest,
	CurationRequest,
	DecisionRequest
} from './types';

const BASE = '/api/v1';

// ApiError carries the HTTP status so callers can branch on it (e.g. a 401 owner
// expiry) without parsing the message string. Extends Error, so existing
// toMessage()/catch sites keep working unchanged.
export class ApiError extends Error {
	constructor(
		readonly status: number,
		path: string,
		message?: string
	) {
		super(message ?? `API ${path} failed: ${status}`);
		this.name = 'ApiError';
	}
}

async function get<T>(path: string, fetchFn: typeof fetch = fetch, init?: RequestInit): Promise<T> {
	const res = await fetchFn(`${BASE}${path}`, init);
	if (!res.ok) {
		throw new ApiError(res.status, path);
	}
	return res.json() as Promise<T>;
}

// Owner auth (ADR-046). The SPA authenticates once via POST /session, which sets
// an HttpOnly session cookie — the token itself is never held in JS (no
// localStorage/sessionStorage; ADR-030 condition 2: avoid XSS exfiltration). The
// cookie carries auth across reloads and rides every authed request via
// credentials: 'same-origin'.
const CREDS: RequestCredentials = 'same-origin';

// startSession exchanges the admin token for the session cookie. remember=true
// requests the longer "trust this device" lifetime. The token travels only in the
// X-Admin-Token header of this single request, never persisted client-side.
export async function startSession(token: string, remember = false): Promise<void> {
	const res = await fetch(`${BASE}/session${remember ? '?remember=1' : ''}`, {
		method: 'POST',
		credentials: CREDS,
		headers: { 'X-Admin-Token': token.trim() }
	});
	if (!res.ok) {
		throw new ApiError(res.status, '/session');
	}
}

// endSession signs the owner out (clears the cookie). Best-effort.
export async function endSession(): Promise<void> {
	await fetch(`${BASE}/session`, { method: 'DELETE', credentials: CREDS }).catch(() => {});
}

// getAuthed is get() sending the session cookie for the owner surface.
const getAuthed = <T>(path: string): Promise<T> => get<T>(path, fetch, { credentials: CREDS });

// sendAuthed issues a write (POST/PUT/DELETE) on the owner surface carrying the
// session cookie and an optional JSON body, returning the decoded response (or {}).
async function sendAuthed<T>(method: 'POST' | 'PUT' | 'DELETE', path: string, body?: unknown): Promise<T> {
	const res = await fetch(`${BASE}${path}`, {
		method,
		credentials: CREDS,
		headers: { ...(body ? { 'Content-Type': 'application/json' } : {}) },
		body: body ? JSON.stringify(body) : undefined
	});
	if (!res.ok && res.status !== 204) {
		throw new ApiError(res.status, path);
	}
	return (res.status === 204 ? {} : await res.json().catch(() => ({}))) as T;
}

// uploadAuthed POSTs multipart FormData on the owner surface with the session
// cookie (no Content-Type — the browser sets the multipart boundary). Returns the
// decoded JSON body. A 409 is surfaced verbatim so callers can show "gallery is full".
async function uploadAuthed<T>(path: string, form: FormData): Promise<T> {
	const res = await fetch(`${BASE}${path}`, {
		method: 'POST',
		credentials: CREDS,
		body: form
	});
	if (!res.ok) {
		const body = (await res.json().catch(() => ({}))) as { error?: string };
		throw new ApiError(res.status, path, body.error);
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

	// Append a client-side reload counter to a thumbnail URL after a regenerate or
	// writeback, forcing the <img>/<video poster> to re-fetch within the session.
	// The base may already carry the server's ?v={mtime} token, so join with & when
	// a query string is present. n=0 leaves the URL untouched (no cache-bust yet).
	thumbnailReload: (url: string, n: number) =>
		n > 0 ? `${url}${url.includes('?') ? '&' : '?'}r=${n}` : url,

	// Request re-extraction. Returns the HTTP status: 200 = embedded art extracted
	// synchronously, 202 = queued for frame generation.
	regenerateThumbnail: async (id: number, fetchFn: typeof fetch = fetch): Promise<number> => {
		const res = await fetchFn(`${BASE}/media/${id}/thumbnail`, { method: 'POST' });
		if (!res.ok && res.status !== 202) {
			throw new Error(`regenerate thumbnail failed: ${res.status}`);
		}
		return res.status;
	},

	// Person images (F25, ADR-038). Reads are public; a filled role serves the real
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
	// 409 surfaces "gallery is full" via uploadAuthed's error. allowOverCap lets the
	// owner deliberately exceed the gallery cap (F25); ignored for core roles.
	uploadPersonImage: (id: number, file: File, role: PersonImageRole, allowOverCap = false) => {
		const form = new FormData();
		form.append('image', file);
		form.append('role', role);
		if (allowOverCap) form.append('allow_over_cap', 'true');
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

	// People/Tags accept name|count|random (ADR-045 §3). The server returns the full
	// list in canonical name order for both 'name' and 'random' — the random shuffle
	// is applied client-side (these lists are unpaged) — so only 'count' reorders
	// server-side. 'random' is still sent so the contract is exercised/observable.
	listPeople: (sort: PeopleTagSort = 'name', fetchFn?: typeof fetch) =>
		get<{ items: Person[] }>(`/people${sort === 'name' ? '' : `?sort=${sort}`}`, fetchFn),

	getPerson: (id: number, fetchFn?: typeof fetch) =>
		get<PersonDetailResponse>(`/people/${id}`, fetchFn),

	listTags: (sort: PeopleTagSort = 'name', fetchFn?: typeof fetch) =>
		get<{ items: Tag[] }>(`/tags${sort === 'name' ? '' : `?sort=${sort}`}`, fetchFn),

	getTag: (id: number, fetchFn?: typeof fetch) =>
		get<{ tag: Tag; items: Video[]; total: number }>(`/tags/${id}`, fetchFn),

	search: (q: string, fetchFn?: typeof fetch) =>
		get<SearchResponse>(`/search?q=${encodeURIComponent(q)}`, fetchFn),

	// Configurable metadata fields (F20): filterable facets + key-discovery view.
	facets: (fetchFn?: typeof fetch) => get<{ facets: Facet[] }>(`/facets`, fetchFn),

	metadataKeys: (fetchFn?: typeof fetch) =>
		get<{ keys: MetadataKey[] }>(`/metadata-keys`, fetchFn),

	// System Activity (F21). capabilities is ungated, but it carries the session
	// cookie (credentials) so the server reports owner:true once authenticated —
	// including across reloads, since the cookie persists (ADR-046).
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

	// Video/film enrichment (F26). All owner-gated.
	enrichVideoResolve: (videoId: number, provider: string, query: string) =>
		sendAuthed<{ candidates: EnrichCandidate[] }>('POST', `/media/${videoId}/enrich/resolve`, {
			provider,
			query
		}),

	enrichVideoApply: (videoId: number, provider: string, externalId: string) =>
		sendAuthed<{ enriched: EnrichedField[] }>('POST', `/media/${videoId}/enrich`, {
			provider,
			external_id: externalId
		}),

	enrichVideoClear: (videoId: number, provider: string) =>
		sendAuthed<Record<string, never>>('DELETE', `/media/${videoId}/enrich/${encodeURIComponent(provider)}`),

	// Per-item metadata refresh (F31, ADR-047) — forced file re-extract + re-enrich
	// of the item's linked providers. Owner-gated; returns the combined report.
	refreshMedia: (videoId: number) =>
		sendAuthed<RefreshReport>('POST', `/media/${videoId}/refresh`),

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
			credentials: CREDS,
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ alias })
		});
		if (res.status === 409) {
			const body = await res.json().catch(() => ({}));
			return { conflict: body.conflict as Person };
		}
		if (!res.ok) {
			throw new ApiError(res.status, `/people/${personId}/aliases`);
		}
		return { aliases: ((await res.json()) as { aliases: PersonAlias[] }).aliases };
	},

	deleteAlias: (personId: number, aliasId: number) =>
		sendAuthed<Record<string, never>>('DELETE', `/people/${personId}/aliases/${aliasId}`),

	// Merge `fromId` into `canonicalId`: the from-person's videos move to canonical,
	// its name becomes an alias, and it is deleted. Returns the updated canonical.
	mergePersons: (canonicalId: number, fromId: number) =>
		sendAuthed<{ person: Person }>('POST', `/people/${canonicalId}/merge`, { from_id: fromId }),

	// Media soft-delete / purge / restore / Trash (F24, ADR-037). All owner-gated.
	// deleteMedia soft-deletes (the item moves to Trash, restorable within the grace
	// period); { purge: true } hard-deletes now, bypassing the grace period.
	deleteMedia: (id: number, opts?: { purge?: boolean }) =>
		sendAuthed<Record<string, never>>(
			'DELETE',
			`/media/${id}${opts?.purge ? '?purge=true' : ''}`
		),

	restoreMedia: (id: number) =>
		sendAuthed<{ video: Video }>('POST', `/media/${id}/restore`),

	trash: () => getAuthed<{ items: TrashEntry[]; total: number }>(`/admin/trash`),

	// Metadata writeback — embed curated field values into the media file's tags.
	// Owner-gated. Returns {job_id, queued} when the durable queue is wired (202,
	// F30/ADR-048), or {} on the legacy synchronous path (204, F28). 422 when a
	// field has no tag mapping for the file's container.
	writebackMedia: (id: number, req: WritebackRequest) =>
		sendAuthed<{ job_id?: number; queued?: number }>('POST', `/media/${id}/writeback`, req),

	// Value-level curation (F30, ADR-048). curateMedia records a manual add /
	// suppress / nowrite decision; clearMediaCuration removes one (restoring the
	// underlying source value). Owner-gated; 204 on success.
	curateMedia: (id: number, req: CurationRequest) =>
		sendAuthed<Record<string, never>>('POST', `/media/${id}/curation`, req),
	clearMediaCuration: (id: number, req: CurationRequest) =>
		sendAuthed<Record<string, never>>('POST', `/media/${id}/curation/clear`, req),

	// Per-field source-of-truth decision (F36, ADR-051 §7). Owner-gated. setFieldDecision
	// pins a replace field to a source (file / provider:<name> / manual + literal);
	// clearFieldDecision removes the decision, reverting the field to the file default.
	// Both are DB-only — they never touch the file (RD5); the file changes solely via
	// writebackMedia ("Write decisions to file").
	setFieldDecision: (id: number, canonical: string, req: DecisionRequest) =>
		sendAuthed<Record<string, never>>(
			'PUT',
			`/media/${id}/fields/${encodeURIComponent(canonical)}/decision`,
			req
		),
	clearFieldDecision: (id: number, canonical: string) =>
		sendAuthed<Record<string, never>>(
			'DELETE',
			`/media/${id}/fields/${encodeURIComponent(canonical)}/decision`
		)
};
