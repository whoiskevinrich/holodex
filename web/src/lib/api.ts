// Typed client for the Holodex REST API. In dev, Vite proxies /api -> :7800;
// in production the Go binary serves both from the same origin (ADR-007).
import { filtersToParams } from './filters';
import type {
	Activity,
	Capabilities,
	EnrichCandidate,
	EnrichedField,
	EnrichSource,
	EntityKind,
	EntityRef,
	Facet,
	JobRun,
	MediaDetailResponse,
	MediaFilters,
	MediaListResponse,
	MetadataKey,
	Person,
	PersonAlias,
	PersonDetailResponse,
	PersonRenameConflict,
	PeopleTagSort,
	PersonImageRole,
	PersonImageSet,
	RefreshReport,
	RelatedResponse,
	SearchResponse,
	Studio,
	StudioDetailResponse,
	Tag,
	TrashEntry,
	Video,
	WritebackRequest,
	CurationRequest,
	DecisionRequest
} from './types';

const BASE = '/api/v1';

// The REST base segment for each identity entity (F43, ADR-061) — the shared
// alias/merge/rename client trio routes through this map instead of hardcoding a path
// per entity.
const ENTITY_BASE: Record<EntityKind, string> = {
	person: 'people',
	studio: 'studios',
	tag: 'tags'
};

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

// ReauthError signals that an upstream auth proxy (Authentik ForwardAuth) expired
// and 302-redirected our request cross-origin to the IdP (HOLODEX-127). It is
// deliberately distinct from ApiError 401 — that is Holodex's *own* owner-session
// expiry (ADR-046). A background fetch cannot follow the cross-origin redirect (it
// hits a CORS wall and rejects with an opaque TypeError), so we detect the redirect
// with `redirect: 'manual'` and recover via a top-level navigation instead.
export class ReauthError extends Error {
	constructor() {
		super('auth session expired (upstream redirect)');
		this.name = 'ReauthError';
	}
}

// triggerReauth recovers from a ForwardAuth expiry with a *top-level* navigation:
// only a document-level request can follow the cross-origin 302, silently
// re-establishing the outpost session via the still-valid Authentik SSO cookie (or
// cleanly landing on the login flow if that has lapsed too). Guarded so the many
// concurrent authed requests (the 3 s poll, in-flight loads) trigger at most one
// reload; the flag resets naturally when the fresh document loads.
let reauthTriggered = false;
export function triggerReauth(): void {
	if (reauthTriggered || typeof window === 'undefined') return;
	reauthTriggered = true;
	window.location.assign(window.location.href);
}

// checkRedirect turns a manually-handled ForwardAuth redirect into a recoverable
// signal. With `redirect: 'manual'` the browser returns an opaque redirect
// (type 'opaqueredirect', status 0) rather than following it into a CORS failure;
// we kick off the top-level re-auth and throw ReauthError so callers can suppress
// their transient error UI while the document reloads. Called before res.ok/json()
// (json() would throw on an opaque response).
function checkRedirect(res: Response): void {
	if (res.type === 'opaqueredirect') {
		triggerReauth();
		throw new ReauthError();
	}
}

async function get<T>(path: string, fetchFn: typeof fetch = fetch, init?: RequestInit): Promise<T> {
	const res = await fetchFn(`${BASE}${path}`, { redirect: 'manual', ...init });
	checkRedirect(res);
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
		redirect: 'manual',
		headers: { ...(body ? { 'Content-Type': 'application/json' } : {}) },
		body: body ? JSON.stringify(body) : undefined
	});
	checkRedirect(res);
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
		redirect: 'manual',
		body: form
	});
	checkRedirect(res);
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

	// The image read model alone — the post-mutation refresh, so an upload or
	// gallery edit never refetches the full detail payload.
	getPersonImages: (id: number, fetchFn?: typeof fetch) =>
		get<PersonImageSet>(`/people/${id}/images`, fetchFn),

	listTags: (sort: PeopleTagSort = 'name', fetchFn?: typeof fetch) =>
		get<{ items: Tag[] }>(`/tags${sort === 'name' ? '' : `?sort=${sort}`}`, fetchFn),

	getTag: (id: number, fetchFn?: typeof fetch) =>
		get<{ tag: Tag; items: Video[]; total: number }>(`/tags/${id}`, fetchFn),

	// Studio entities (F38, ADR-053). Same list contract as people/tags (name|count|
	// random; random shuffled client-side). Detail carries resolved[] in the record
	// vocabulary (no in_sync) plus the studio's videos.
	listStudios: (sort: PeopleTagSort = 'name', fetchFn?: typeof fetch) =>
		get<{ items: Studio[] }>(`/studios${sort === 'name' ? '' : `?sort=${sort}`}`, fetchFn),

	getStudio: (id: number, fetchFn?: typeof fetch) =>
		get<StudioDetailResponse>(`/studios/${id}`, fetchFn),

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

	// Public provider directory (ADR-059): name → brand icon URL, for provenance badges
	// and the website label that render for everyone (unlike owner-gated enrichSources).
	providers: () => get<{ providers: EnrichSource[] }>(`/providers`),

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

	// Studio (company) enrichment (F38 S3). Mirrors the person enrich trio; all
	// owner-gated. Studios have no file → no writeback and no relink (a studio-entity
	// enrich changes the studio's own fields, not the video → studio links).
	enrichStudioResolve: (studioId: number, provider: string, query: string) =>
		sendAuthed<{ candidates: EnrichCandidate[] }>('POST', `/studios/${studioId}/enrich/resolve`, {
			provider,
			query
		}),

	enrichStudioApply: (studioId: number, provider: string, externalId: string) =>
		sendAuthed<{ enriched: EnrichedField[] }>('POST', `/studios/${studioId}/enrich`, {
			provider,
			external_id: externalId
		}),

	enrichStudioClear: (studioId: number, provider: string) =>
		sendAuthed<Record<string, never>>('DELETE', `/studios/${studioId}/enrich/${encodeURIComponent(provider)}`),

	// Per-item metadata refresh (F31, ADR-047) — forced file re-extract + re-enrich
	// of the item's linked providers. Owner-gated; returns the combined report.
	refreshMedia: (videoId: number) =>
		sendAuthed<RefreshReport>('POST', `/media/${videoId}/refresh`),

	// Shared entity name-identity mutations (F43, ADR-061) — one owner-gated client trio
	// over the per-entity routes (people | studios | tags), mirroring the F23 person shape.
	// Person uses these too, so the AliasPanel/EntityPicker are entity-uniform; the person
	// page's rename flow keeps its dedicated renamePerson (it carries the F37 name-chip UX).

	// addEntityAlias adds an alias, returning the updated list — or, when the name already
	// belongs to another entity of this kind, that entity as a `conflict` (409), so the UI
	// offers a merge instead of silently folding two possibly-distinct entities.
	addEntityAlias: async (
		kind: EntityKind,
		id: number,
		alias: string
	): Promise<{ aliases?: PersonAlias[]; conflict?: EntityRef }> => {
		const path = `/${ENTITY_BASE[kind]}/${id}/aliases`;
		const res = await fetch(`${BASE}${path}`, {
			method: 'POST',
			credentials: CREDS,
			redirect: 'manual',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ alias })
		});
		checkRedirect(res);
		if (res.status === 409) {
			const body = (await res.json().catch(() => ({}))) as { conflict?: EntityRef };
			return { conflict: body.conflict };
		}
		if (!res.ok) {
			throw new ApiError(res.status, path);
		}
		return { aliases: ((await res.json()) as { aliases: PersonAlias[] }).aliases };
	},

	deleteEntityAlias: (kind: EntityKind, id: number, aliasId: number) =>
		sendAuthed<Record<string, never>>('DELETE', `/${ENTITY_BASE[kind]}/${id}/aliases/${aliasId}`),

	// Merge `fromId` into `canonicalId` for any identity entity. For studios the loser's
	// name is registered as an alias so RelinkVideoStudios re-derivation won't resurrect it
	// (RD6). The response wraps the survivor under a per-entity key; callers reload, so the
	// decoded body is returned as-is.
	mergeEntities: (kind: EntityKind, canonicalId: number, fromId: number) =>
		sendAuthed<Record<string, unknown>>('POST', `/${ENTITY_BASE[kind]}/${canonicalId}/merge`, {
			from_id: fromId
		}),

	// Rename an entity, keeping the old name as an alias (one transaction). 204 on success;
	// a 409 (the name already belongs to another entity of this kind) returns that entity as
	// `conflict` so the UI can offer merge instead — never an auto-merge. Studio/tag wrap the
	// conflict under `conflict`; the person route returns it at top level — accept both.
	renameEntity: async (
		kind: EntityKind,
		id: number,
		name: string
	): Promise<{ conflict?: EntityRef }> => {
		const path = `/${ENTITY_BASE[kind]}/${id}/rename`;
		const res = await fetch(`${BASE}${path}`, {
			method: 'POST',
			credentials: CREDS,
			redirect: 'manual',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ name })
		});
		checkRedirect(res);
		if (res.status === 409) {
			const body = (await res.json().catch(() => ({}))) as { conflict?: EntityRef } & EntityRef;
			return { conflict: body.conflict ?? body };
		}
		if (!res.ok && res.status !== 204) {
			throw new ApiError(res.status, path);
		}
		return {};
	},

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
		),

	// Person per-field source decisions (F37, RD7) — the media pair mirrored onto
	// /people/{id}. source ∈ record | provider:<name> | manual (RD4 — the person baseline
	// is `record`); `name` is rejected server-side (400, RD1 — it renames, never pins).
	// DB-only either way: persons have no writeback.
	setPersonFieldDecision: (id: number, canonical: string, req: DecisionRequest) =>
		sendAuthed<Record<string, never>>(
			'PUT',
			`/people/${id}/fields/${encodeURIComponent(canonical)}/decision`,
			req
		),
	clearPersonFieldDecision: (id: number, canonical: string) =>
		sendAuthed<Record<string, never>>(
			'DELETE',
			`/people/${id}/fields/${encodeURIComponent(canonical)}/decision`
		),

	// Person value-level curation (F37, RD2/RD7) — the media pair mirrored onto
	// /people/{id}; drives the "Also known as" merge row (add / suppress).
	curatePerson: (id: number, req: CurationRequest) =>
		sendAuthed<Record<string, never>>('POST', `/people/${id}/curation`, req),
	clearPersonCuration: (id: number, req: CurationRequest) =>
		sendAuthed<Record<string, never>>('POST', `/people/${id}/curation/clear`, req),

	// Studio per-field source decisions + curation (F38, RD5) — the person pair
	// mirrored onto /studios/{id}. source ∈ record | provider:<name> | manual; `name`
	// is rejected server-side (read-only identity). DB-only: studios have no writeback.
	setStudioFieldDecision: (id: number, canonical: string, req: DecisionRequest) =>
		sendAuthed<Record<string, never>>(
			'PUT',
			`/studios/${id}/fields/${encodeURIComponent(canonical)}/decision`,
			req
		),
	clearStudioFieldDecision: (id: number, canonical: string) =>
		sendAuthed<Record<string, never>>(
			'DELETE',
			`/studios/${id}/fields/${encodeURIComponent(canonical)}/decision`
		),
	curateStudio: (id: number, req: CurationRequest) =>
		sendAuthed<Record<string, never>>('POST', `/studios/${id}/curation`, req),
	clearStudioCuration: (id: number, req: CurationRequest) =>
		sendAuthed<Record<string, never>>('POST', `/studios/${id}/curation/clear`, req),

	// Rename a person, keeping the old name as an F23 alias (one transaction — search
	// and scan routing keep matching it; F37 RD1). 204 on success. A 409 (the name
	// already belongs to another person) returns that person as `conflict` so the UI
	// can offer the existing merge flow instead — never an auto-merge.
	renamePerson: async (
		personId: number,
		name: string
	): Promise<{ conflict?: PersonRenameConflict }> => {
		const path = `/people/${personId}/rename`;
		const res = await fetch(`${BASE}${path}`, {
			method: 'POST',
			credentials: CREDS,
			redirect: 'manual',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ name })
		});
		checkRedirect(res);
		if (res.status === 409) {
			const body = (await res.json().catch(() => ({}))) as PersonRenameConflict;
			return { conflict: body };
		}
		if (!res.ok && res.status !== 204) {
			throw new ApiError(res.status, path);
		}
		return {};
	}
};
