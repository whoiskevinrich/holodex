// Typed client for the Holodex REST API. In dev, Vite proxies /api -> :7800;
// in production the Go binary serves both from the same origin (ADR-007).
import { filtersToParams } from './filters';
import type {
	Activity,
	Capabilities,
	Category,
	EnrichCandidate,
	EnrichedField,
	EnrichEntityKind,
	EnrichQueueRow,
	EnrichSource,
	DuplicatePair,
	DeniedTag,
	EntityKind,
	EntityRef,
	ExtractionQueueRow,
	ExtractionResolveAction,
	ExtractionResult,
	Facet,
	FacetSummary,
	CompletenessFacetGroup,
	Film,
	FilmDetailResponse,
	FilmSceneCollision,
	FilmStudioCascadeResult,
	FilmVideoCandidate,
	JobRun,
	JobDigest,
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
	RefreshAllResult,
	RefreshReport,
	RelatedResponse,
	SearchResponse,
	Studio,
	StudioDetailResponse,
	StudioImageRole,
	FilmImageRole,
	Tag,
	TrashEntry,
	Video,
	VideoCollisionRef,
	WritebackRequest,
	CurationRequest,
	DecisionRequest,
	DecisionSource,
	FieldClaim,
	FieldPromotionRequest,
	FieldPromotionView,
	FieldTarget,
	PromotionEntityType
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

// The REST base segment for each enrichment entity (F47, ADR-066) — 'video' rides
// /media, so this can't reuse ENTITY_BASE (F43's alias/merge/rename spine has no video).
// Exported for CompletenessQueueRow, which links to the same entity pages by kind.
export const ENRICH_ENTITY_BASE: Record<EnrichEntityKind, string> = {
	person: 'people',
	studio: 'studios',
	video: 'media'
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
		redirect: 'manual',
		headers: { 'X-Admin-Token': token.trim() }
	});
	checkRedirect(res);
	if (!res.ok) {
		throw new ApiError(res.status, '/session');
	}
}

// endSession signs the owner out (clears the cookie). Best-effort: a network
// failure is swallowed. A lapsed ForwardAuth session still triggers the usual
// top-level reauth reload (checkRedirect/triggerReauth) — only the resulting
// ReauthError is swallowed here so the caller doesn't also see an error.
export async function endSession(): Promise<void> {
	try {
		const res = await fetch(`${BASE}/session`, {
			method: 'DELETE',
			credentials: CREDS,
			redirect: 'manual'
		});
		checkRedirect(res);
	} catch {
		// best-effort — ignore
	}
}

// getAuthed is get() sending the session cookie for the owner surface.
const getAuthed = <T>(path: string): Promise<T> => get<T>(path, fetch, { credentials: CREDS });

// sendAuthed issues a write (POST/PUT/DELETE) on the owner surface carrying the
// session cookie and an optional JSON body, returning the decoded response (or {}).
async function sendAuthed<T>(method: 'POST' | 'PUT' | 'PATCH' | 'DELETE', path: string, body?: unknown): Promise<T> {
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

// sendConflictable is sendAuthed's sibling for owner mutations that can resolve a
// composite-key collision as `{conflict}` instead of throwing: curateMedia and
// setFieldDecision (HOLODEX-270/272, VideoCollisionRef), and the film attach
// endpoints (F56, FilmSceneCollision naming the scene-number occupant). Any other
// 409 (e.g. "item deleted") still throws, or a caller checking only
// `if (res.conflict)` would fall through to its success path.
async function sendConflictable<TReq, TConflict = VideoCollisionRef>(
	method: 'POST' | 'PUT',
	path: string,
	body: TReq
): Promise<{ conflict?: TConflict }> {
	const res = await fetch(`${BASE}${path}`, {
		method,
		credentials: CREDS,
		redirect: 'manual',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	checkRedirect(res);
	if (res.status === 409) {
		const conflictBody = (await res.json().catch(() => ({}))) as { conflict?: TConflict };
		if (conflictBody.conflict) return { conflict: conflictBody.conflict };
		throw new ApiError(res.status, path);
	}
	if (!res.ok && res.status !== 204) {
		throw new ApiError(res.status, path);
	}
	return {};
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

// uploadEntityImage builds the multipart body shared by every single-slot entity
// image upload (Studio F51/Film F56 — HOLODEX-286); only the URL path segment and
// role type differ per entity.
function uploadEntityImage(kind: 'studios' | 'films', id: number, file: File, role: string) {
	const form = new FormData();
	form.append('image', file);
	return uploadAuthed<{ id: number; version: number }>(`/${kind}/${id}/images/${role}`, form);
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

	// Video poster upload (F52, HOLODEX-252). The poster IS the existing thumbnail
	// (ADR-009) — upload adds a new highest-precedence tier to that same pipeline,
	// so the caller bumps its existing thumbnail cache-bust counter on success,
	// exactly like regenerateThumbnail.
	uploadVideoPoster: (id: number, file: File) => {
		const form = new FormData();
		form.append('image', file);
		return uploadAuthed<Record<string, never>>(`/media/${id}/poster`, form);
	},

	deleteVideoPoster: (id: number) => sendAuthed<Record<string, never>>('DELETE', `/media/${id}/poster`),

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
	// completeness_asc/desc + missingFacet (F55.5/F55.6) are owner-only — the server
	// 401s a non-owner request using either; callers gate on isOwner before passing them.
	listPeople: (
		sort: PeopleTagSort | 'completeness_asc' | 'completeness_desc' = 'name',
		fetchFn?: typeof fetch,
		missingFacet?: string[]
	) => {
		const p = new URLSearchParams();
		if (sort !== 'name') p.set('sort', sort);
		for (const canonical of missingFacet ?? []) p.append('missing_facet', canonical);
		const qs = p.toString();
		return get<{ items: Person[] }>(`/people${qs ? `?${qs}` : ''}`, fetchFn);
	},

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

	// Tag hierarchy (F50, ADR-075 D1) — the /tags pill-menu "Set parent…"/"Clear
	// parent" action. parentId: null clears to root. A 400 with {cycle: true}
	// means the proposed parent is the tag itself or one of its own descendants
	// — surfaced as `cycle` (not thrown) so the caller can show the ADR-075
	// D1 cycle message inline, mirroring addEntityAlias/renameEntity's own
	// conflict-as-return-value shape rather than exception-based branching.
	// The caller always reloads the tag list on success, so the mutated tag
	// itself isn't parsed out of the response body.
	setTagParent: async (id: number, parentId: number | null): Promise<{ cycle?: boolean }> => {
		const path = `/tags/${id}/parent`;
		const res = await fetch(`${BASE}${path}`, {
			method: 'POST',
			credentials: CREDS,
			redirect: 'manual',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ parent_id: parentId })
		});
		checkRedirect(res);
		const body = (await res.json().catch(() => ({}))) as { cycle?: boolean };
		if (res.status === 400) {
			if (body.cycle) return { cycle: true };
			throw new ApiError(res.status, path);
		}
		if (!res.ok) {
			throw new ApiError(res.status, path);
		}
		return {};
	},

	// Tag Categories (HOLODEX-240, ADR-078) — hand-curated grouping, no alias/merge
	// spine, so this is its own small trio rather than riding ENTITY_BASE. Reads
	// are public; mutations owner-gated. A 409 from create/rename means the name
	// collides with a tag or another category — the caller reads `err.status` off
	// the thrown ApiError (same convention as addVideoTag/attachVideoTag's 422/400).
	listCategories: (fetchFn?: typeof fetch) => get<{ items: Category[] }>(`/categories`, fetchFn),

	getCategory: (id: number, fetchFn?: typeof fetch) =>
		get<{ category: Category }>(`/categories/${id}`, fetchFn),

	createCategory: (name: string) => sendAuthed<{ category: Category }>('POST', `/categories`, { name }),

	renameCategory: (id: number, name: string) =>
		sendAuthed<{ category: Category }>('POST', `/categories/${id}/rename`, { name }),

	deleteCategory: (id: number) => sendAuthed<Record<string, never>>('DELETE', `/categories/${id}`),

	assignCategoryTags: (id: number, tagIds: number[]) =>
		sendAuthed<{ category: Category }>('POST', `/categories/${id}/tags`, { tag_ids: tagIds }),

	unassignCategoryTags: (id: number, tagIds: number[]) =>
		sendAuthed<{ category: Category }>('DELETE', `/categories/${id}/tags`, { tag_ids: tagIds }),

	// Resolve-or-create a tag by name with no video attach (HOLODEX-240) — the
	// /categories/{id} "+ Add tag" control's first step; the caller then assigns
	// the returned id via assignCategoryTags. Same 422/400/409 status contract as
	// addVideoTag (deny-list / too long / collides with a category).
	resolveOrCreateTag: (name: string) => sendAuthed<{ tag: Tag }>('POST', `/tags`, { name }),

	// Studio entities (F38, ADR-053). Same list contract as people/tags (name|count|
	// random; random shuffled client-side). Detail carries resolved[] in the record
	// vocabulary (no in_sync) plus the studio's videos.
	// completeness_asc/desc + missingFacet (F55.5/F55.6) mirror listPeople — owner-only,
	// callers gate on isOwner before passing them.
	listStudios: (
		sort: PeopleTagSort | 'completeness_asc' | 'completeness_desc' = 'name',
		fetchFn?: typeof fetch,
		missingFacet?: string[]
	) => {
		const p = new URLSearchParams();
		if (sort !== 'name') p.set('sort', sort);
		for (const canonical of missingFacet ?? []) p.append('missing_facet', canonical);
		const qs = p.toString();
		return get<{ items: Studio[] }>(`/studios${qs ? `?${qs}` : ''}`, fetchFn);
	},

	getStudio: (id: number, fetchFn?: typeof fetch) =>
		get<StudioDetailResponse>(`/studios/${id}`, fetchFn),

	// Studio images (F51, ADR-079): three owner-editable core roles (icon/logo/
	// poster), no gallery. Reads are public; a filled role serves the real JPEG, an
	// empty one 404s (the SPA renders its own fallback — no server-side placeholder,
	// unlike Person). The served, cache-busted URL is already embedded in the Studio
	// object (icon_url/logo_url/poster_url) — no client-side URL builder needed, mirrors
	// how Studio.logo_url worked pre-F51.
	uploadStudioImage: (id: number, file: File, role: StudioImageRole) =>
		uploadEntityImage('studios', id, file, role),

	deleteStudioImage: (id: number, role: StudioImageRole) =>
		sendAuthed<Record<string, never>>('DELETE', `/studios/${id}/images/${role}`),

	// Film entities (F56, ADR-085): the first entity whose video membership is an owner
	// assertion, not a derived link — see docs/architecture/ADR-085-films-entity.md.
	// Reads are public (gated on films_enabled server-side at Mount); mutations are
	// owner-gated. ?q= name-searches (FTS); ?person_id=/?studio_id=/?tag_id= filter to
	// films whose video union includes that entity (mutually exclusive with ?q).
	listFilms: (
		opts: { q?: string; personId?: number; studioId?: number; tagId?: number } = {},
		fetchFn?: typeof fetch
	) => {
		const p = new URLSearchParams();
		if (opts.q) p.set('q', opts.q);
		if (opts.personId) p.set('person_id', String(opts.personId));
		if (opts.studioId) p.set('studio_id', String(opts.studioId));
		if (opts.tagId) p.set('tag_id', String(opts.tagId));
		const qs = p.toString();
		return get<{ items: Film[] }>(`/films${qs ? `?${qs}` : ''}`, fetchFn);
	},

	getFilm: (id: number, fetchFn?: typeof fetch) => get<FilmDetailResponse>(`/films/${id}`, fetchFn),

	// createFilm is get-or-create on (name, year): a duplicate submit returns the
	// existing film rather than a 409, so the video→film picker's "create new" action
	// is idempotent.
	createFilm: (name: string, year?: number) =>
		sendAuthed<{ film: Film }>('POST', `/films`, { name, year: year ?? 0 }),

	// Film images (F56/HOLODEX-280, ADR-086): poster/thumb, owner upload/replace/
	// remove — mirrors uploadStudioImage/deleteStudioImage exactly.
	uploadFilmImage: (id: number, file: File, role: FilmImageRole) =>
		uploadEntityImage('films', id, file, role),

	deleteFilmImage: (id: number, role: FilmImageRole) =>
		sendAuthed<Record<string, never>>('DELETE', `/films/${id}/images/${role}`),

	// Film↔video attach/detach (owner-gated). scene_number null = unnumbered.
	// attachFilmVideo/bulkAttachFilmVideos surface a scene-number collision as
	// `{conflict}` (naming the occupant) instead of throwing, mirroring curateMedia's
	// composite-key-collision contract.
	attachFilmVideo: (filmId: number, videoId: number, sceneNumber: number | null, isFullFilm: boolean) =>
		sendConflictable<
			{ video_id: number; scene_number: number | null; is_full_film: boolean },
			FilmSceneCollision
		>('POST', `/films/${filmId}/videos`, {
			video_id: videoId,
			scene_number: sceneNumber,
			is_full_film: isFullFilm
		}),

	bulkAttachFilmVideos: (filmId: number, videoIds: number[], startingSceneNumber: number) =>
		sendConflictable<{ video_ids: number[]; starting_scene_number: number }, FilmSceneCollision>(
			'POST',
			`/films/${filmId}/videos/bulk`,
			{ video_ids: videoIds, starting_scene_number: startingSceneNumber }
		),

	detachFilmVideo: (filmId: number, videoId: number) =>
		sendAuthed<Record<string, never>>('DELETE', `/films/${filmId}/videos/${videoId}`),

	// filmVideoCandidates is the film→video picker's search (owner-gated): default
	// scope excludes videos attached to ANY film; unattached:false widens to the whole
	// library and flags already-attached-elsewhere via each row's already_attached.
	filmVideoCandidates: (
		filmId: number,
		opts: { q?: string; studioId?: number; personId?: number; unattached?: boolean } = {}
	) => {
		const p = new URLSearchParams();
		if (opts.q) p.set('q', opts.q);
		if (opts.studioId) p.set('studio_id', String(opts.studioId));
		if (opts.personId) p.set('person', String(opts.personId));
		if (opts.unattached === false) p.set('unattached', 'false');
		const qs = p.toString();
		return getAuthed<{ items: FilmVideoCandidate[]; total: number }>(
			`/films/${filmId}/video-candidates${qs ? `?${qs}` : ''}`
		);
	},

	// Film per-field source decisions (F56, mirrors the studio pair) — DB-only, no
	// writeback/rename (name is baseline-backed and read-only in v1).
	setFilmFieldDecision: (id: number, canonical: string, req: DecisionRequest) =>
		sendAuthed<Record<string, never>>(
			'PUT',
			`/films/${id}/fields/${encodeURIComponent(canonical)}/decision`,
			req
		),
	clearFilmFieldDecision: (id: number, canonical: string) =>
		sendAuthed<Record<string, never>>(
			'DELETE',
			`/films/${id}/fields/${encodeURIComponent(canonical)}/decision`
		),

	// Film-studio cascade (F57, HOLODEX-285, ADR-087): one owner action that sets a new
	// manual Studio decision AND enqueues a file writeback across every video attached
	// to the film. Owner-gated. results is best-effort per video (D2) -- a collision or
	// error on one video never blocks the others. batch_id is "" when nothing enqueued
	// (every video collided/errored, or the film has no attached videos).
	cascadeFilmStudio: (id: number, req: { source: DecisionSource; manual_value?: string }) =>
		sendAuthed<{ batch_id: string; results: FilmStudioCascadeResult[] }>(
			'POST',
			`/films/${id}/studio/cascade`,
			req
		),

	// GET /search returns films alongside people/videos/tags/studios natively
	// (HOLODEX-283) — the backend gates the films group on films_enabled itself, so
	// callers don't need a separate films fetch or a films_enabled check.
	search: (q: string, fetchFn?: typeof fetch) =>
		get<SearchResponse>(`/search?q=${encodeURIComponent(q)}`, fetchFn),

	// Configurable metadata fields (F20): filterable facets + key-discovery view.
	facets: (fetchFn?: typeof fetch) => get<{ facets: Facet[] }>(`/facets`, fetchFn),

	// Missing-facet summary (F55.6, ADR-081 D4) — canonical facets + how many
	// entities of this type are currently missing each, for the browse page's
	// Missing-facet filter chip options. Owner-gated (score/actionability are
	// never exposed to non-owners, even as an aggregate count).
	completenessFacets: (entityType: 'video' | 'person' | 'studio') =>
		getAuthed<{ facets: FacetSummary[] }>(`/completeness/facets?entity_type=${entityType}`),

	// Facet-first remediation queue (F55.7) — every missing scored facet across
	// the whole library, grouped by facet and split candidate-ready/needs-research.
	// Owner-gated; a pure DB read (no writes on load).
	completenessQueue: () =>
		getAuthed<{ groups: CompletenessFacetGroup[] }>(`/owner/completeness-queue`),

	metadataKeys: (fetchFn?: typeof fetch) =>
		get<{ keys: MetadataKey[] }>(`/metadata-keys`, fetchFn),

	// System Activity (F21). capabilities is ungated, but it carries the session
	// cookie (credentials) so the server reports owner:true once authenticated —
	// including across reloads, since the cookie persists (ADR-046).
	capabilities: () => getAuthed<Capabilities>(`/capabilities`),

	activity: () => getAuthed<Activity>(`/admin/activity`),

	activityHistory: (days = 30) =>
		getAuthed<{ runs: JobRun[] }>(`/admin/activity/history?days=${days}`),

	// Per-kind digest of the same window (ADR-071): a fixed-size summary that
	// answers "did anything fail" without loading every run.
	activityDigest: (days = 30) =>
		getAuthed<JobDigest>(`/admin/activity/digest?days=${days}`),

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

	// Video↔tag attach/detach (F50, ADR-075 P0-7) — the media-page add/remove chips.
	// addVideoTag resolves-or-creates by name (source='manual'); a 422 means the term
	// is on the deny-list, a 400 means it's over the length cap — the caller reads
	// `err.status` off the thrown ApiError to tell those apart (no structured body).
	addVideoTag: (videoId: number, name: string) =>
		sendAuthed<{ tag: Tag }>('POST', `/media/${videoId}/tags`, { name }),

	removeVideoTag: (videoId: number, tagId: number) =>
		sendAuthed<Record<string, never>>('DELETE', `/media/${videoId}/tags/${tagId}`),

	// Shared entity name-identity mutations (F43, ADR-061) — one owner-gated client trio
	// over the per-entity routes (people | studios | tags), mirroring the F23 person shape.
	// Person uses these too (HOLODEX-269 retired the person-only renamePerson, which had a
	// conflict-parsing bug this shared renameEntity doesn't have), so AliasPanel/EntityPicker/
	// NameEditControl are all entity-uniform.

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

	// Editor near-miss soft-warning lookup (F43 P1-5). Owner-gated read: a candidate name
	// → the fuzzy near-miss entity (loose-key match, not an exact collision, not
	// kept-separate) or null. kind is 'studio' | 'tag'.
	nearMiss: (kind: Exclude<EntityKind, 'person'>, id: number, name: string) =>
		getAuthed<{ near_miss: EntityRef | null }>(
			`/${ENTITY_BASE[kind]}/${id}/near-miss?name=${encodeURIComponent(name)}`
		),

	// Near-miss review queue (F43 S5). Owner-gated. duplicates() lists every flagged
	// possible-duplicate pair (grouped tags-first); dismissDuplicate records the pair
	// keep-separate and removes it — it never re-surfaces.
	duplicates: () => getAuthed<{ pairs: DuplicatePair[] }>(`/owner/duplicates`),

	dismissDuplicate: (kind: EntityKind, idA: number, idB: number) =>
		sendAuthed<Record<string, never>>('POST', `/owner/duplicates/dismiss`, {
			entity_type: kind,
			id_a: idA,
			id_b: idB
		}),

	// Tag deny-list (F50, ADR-075 D2) — the owner's /owner/tags "Deny-list" tab.
	// A denied term is blocked from becoming a tag from any origin (scanner,
	// manual attach, materialization). Denying is idempotent; removing an
	// unknown term 404s. `existing_tag` reports whether the term already names
	// a live tag (server-computed, so the UI doesn't re-fetch and scan the full
	// tag list itself for the same answer) — denying is forward-only, so the
	// caller surfaces this as a caveat, not a removal.
	deniedTags: () => getAuthed<{ terms: DeniedTag[] }>(`/owner/tags/denylist`),

	denyTag: (term: string) =>
		sendAuthed<{ existing_tag?: boolean }>('POST', `/owner/tags/denylist`, { term }),

	removeDeniedTag: (term: string) =>
		sendAuthed<Record<string, never>>(
			'DELETE',
			`/owner/tags/denylist?term=${encodeURIComponent(term)}`
		),

	// Filename metadata extraction (F48, ADR-067). All owner-gated.

	// Extraction review queue (F48.6). Zero-cost load, same contract as
	// enrichQueue: opening the tab performs no writes.
	extractionQueue: () => getAuthed<{ rows: ExtractionQueueRow[] }>(`/owner/extraction-queue`),

	// Resolve one pending field (F48.6c): action='filename'|'tag' keeps that side's
	// existing value; action='manual' writes the given value (freeform edit, or an
	// entity name picked from search). Enqueues a write except for 'tag' (the file
	// already holds that value).
	resolveExtractionReview: (id: number, action: ExtractionResolveAction, value?: string) =>
		sendAuthed<Record<string, never>>('POST', `/owner/extraction-review/${id}/resolve`, {
			action,
			value
		}),

	// Durable "not this field" verdict (F48.6d) — the row disappears until the owner
	// re-triggers extraction for the video.
	dismissExtractionReview: (id: number) =>
		sendAuthed<Record<string, never>>('POST', `/owner/extraction-review/${id}/dismiss`),

	// On-demand single-video extraction (F48.5a) — synchronous, reflects the match/
	// route result immediately (no queue, no preview).
	extractVideo: (id: number) => sendAuthed<ExtractionResult>('POST', `/media/${id}/extract`),

	// Library-wide batch extraction (F48.5b) — 202 immediately; progress is tracked
	// via System Activity (kind=extraction). started:false means a pass was already
	// running, which already satisfies the request.
	extractAll: () => sendAuthed<{ status: string; started: boolean }>('POST', `/admin/extract-all`),

	// Rollback (F48.9d) — restores every field snapshotted under batchID to its
	// pre-write value; itself a normal (re-snapshotted) writeback job per video.
	revertWritebackBatch: (batchId: string) =>
		sendAuthed<{ job_ids: number[] }>('POST', `/writeback/batches/${batchId}/revert`),

	// Enrichment review queue (F47 S2, ADR-066). Owner-gated; a pure DB read — opening
	// the tab makes zero provider calls (RD2/RD3). A row's `providers` lists only
	// outstanding (not-yet-linked) providers.
	enrichQueue: () => getAuthed<{ rows: EnrichQueueRow[] }>(`/owner/enrich-queue`),

	// Records a durable "not matched" verdict for one (entity, provider) — EnrichPicker's
	// "None of these match" (RD4). Blocks a future /resolve for the pair until undismissed.
	enrichDismiss: (kind: EnrichEntityKind, id: number, provider: string) =>
		sendAuthed<Record<string, never>>(
			'POST',
			`/${ENRICH_ENTITY_BASE[kind]}/${id}/enrich/${encodeURIComponent(provider)}/dismiss`
		),

	// Clears a "not matched" dismissal for one (entity, provider) — the queue row's
	// "Try again" action (RD4). A future /resolve for the pair is unblocked.
	enrichUndismiss: (kind: EnrichEntityKind, id: number, provider: string) =>
		sendAuthed<Record<string, never>>(
			'DELETE',
			`/${ENRICH_ENTITY_BASE[kind]}/${id}/enrich/${encodeURIComponent(provider)}/dismiss`
		),

	// Re-fetches a linked provider's data using its stored external_id — no /resolve,
	// no picker (RD7/P0-5, EnrichProviderChips' "Refresh" primary action).
	enrichRefresh: (kind: EnrichEntityKind, id: number, provider: string) =>
		sendAuthed<{ enriched: EnrichedField[] }>(
			'POST',
			`/${ENRICH_ENTITY_BASE[kind]}/${id}/enrich/${encodeURIComponent(provider)}/refresh`
		),

	// Fans out over every configured provider for the entity (RD8/P1-2): linked
	// providers refresh directly, unlinked ones resolve-and-route (auto-apply a single
	// strong match, else needs_review) — entirely server-side, one round trip.
	enrichRefreshAll: (kind: EnrichEntityKind, id: number) =>
		sendAuthed<{ results: RefreshAllResult[] }>(
			'POST',
			`/${ENRICH_ENTITY_BASE[kind]}/${id}/enrich/refresh-all`
		),

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
	// F30/ADR-048) — no per-field outcome yet, wait on writebackJobStatus. On the
	// legacy synchronous path (F28) returns {written, skipped} (200, HOLODEX-216)
	// naming exactly which submitted fields landed; 422 only when every field has
	// no tag mapping for the file's container.
	writebackMedia: (id: number, req: WritebackRequest) =>
		sendAuthed<{ job_id?: number; queued?: number; written?: string[]; skipped?: string[] }>(
			'POST',
			`/media/${id}/writeback`,
			req
		),

	// One queued write's state, for polling it to completion (ADR-073): "pending" /
	// "running" while in flight, "done" once the queue row is gone (it deletes on
	// success), "failed" + `error` when it gave up. See $lib/writebackJob.
	writebackJobStatus: (jobId: number) =>
		getAuthed<{ status: string; error?: string }>(`/writeback/jobs/${jobId}`),

	// Aggregate progress across every job sharing a batchID (HOLODEX-239, ADR-077
	// D3) — the tag-scoped manual-sync dialog polls this instead of fanning out to
	// one /writeback/jobs/{id} call per video. See $lib/writebackJob.
	writebackBatchStatus: (batchId: string) =>
		getAuthed<{ pending: number; running: number; done: number; failed: number }>(
			`/writeback/batches/${batchId}/status`
		),

	// Tag writeback exclusion (HOLODEX-239, ADR-077 D1/D2). Toggling the flag alone
	// never enqueues a write; the sync endpoints batch-enqueue via the durable
	// queue and return 202 the moment the batch is enqueued (nothing written yet).
	setTagWriteback: (id: number, enabled: boolean) =>
		sendAuthed<{ tag: Tag }>('PATCH', `/tags/${id}/writeback`, { enabled }),

	syncTagWriteback: (id: number) =>
		sendAuthed<{ batch_id: string; enqueued: number }>('POST', `/tags/${id}/writeback/sync`),

	setTagsWriteback: (tagIds: number[], enabled: boolean) =>
		sendAuthed<Record<string, never>>('PATCH', `/tags/writeback`, { tag_ids: tagIds, enabled }),

	syncTagsWriteback: (tagIds: number[]) =>
		sendAuthed<{ batch_id: string; enqueued: number }>('POST', `/tags/writeback/sync`, {
			tag_ids: tagIds
		}),

	// Value-level curation (F30, ADR-048). curateMedia records a manual add /
	// suppress / nowrite decision; clearMediaCuration removes one (restoring the
	// underlying source value). Owner-gated; 204 on success. A person-typed field
	// add/suppress may 409 on the People composite-key collision gate (HOLODEX-272);
	// like setFieldDecision, that returns as `conflict` (conflict-as-return-value, not
	// an exception) — every other field/action combination never produces one.
	curateMedia: (id: number, req: CurationRequest) =>
		sendConflictable('POST', `/media/${id}/curation`, req),
	clearMediaCuration: (id: number, req: CurationRequest) =>
		sendAuthed<Record<string, never>>('POST', `/media/${id}/curation/clear`, req),

	// Per-field source-of-truth decision (F36, ADR-051 §7). Owner-gated. setFieldDecision
	// pins a replace field to a source (file / provider:<name> / manual + literal);
	// clearFieldDecision removes the decision, reverting the field to the file default.
	// Both are DB-only — they never touch the file (RD5); the file changes solely via
	// writebackMedia ("Write decisions to file"). A manual title edit may 409 on a
	// composite-key collision (HOLODEX-270); like renameEntity, that returns as `conflict`
	// (conflict-as-return-value, not an exception) rather than throwing — every other
	// field/source combination never produces one.
	setFieldDecision: (id: number, canonical: string, req: DecisionRequest) =>
		sendConflictable('PUT', `/media/${id}/fields/${encodeURIComponent(canonical)}/decision`, req),
	clearFieldDecision: (id: number, canonical: string) =>
		sendAuthed<Record<string, never>>(
			'DELETE',
			`/media/${id}/fields/${encodeURIComponent(canonical)}/decision`
		),

	// Per-facet not-applicable exclusion (F55.10, ADR-081 D2) — the completeness
	// breakdown panel's DD8 toggle. Video-only: it is independent of any
	// field_source_decisions row for the same field, and the mutation validates
	// against the full registry (not just video-mapped fields), but v1's only UI
	// target is external_provider_id, a video-only registry field. Owner-gated;
	// 204 on success.
	setFacetNotApplicable: (id: number, canonical: string) =>
		sendAuthed<Record<string, never>>(
			'PUT',
			`/media/${id}/fields/${encodeURIComponent(canonical)}/not-applicable`
		),
	clearFacetNotApplicable: (id: number, canonical: string) =>
		sendAuthed<Record<string, never>>(
			'DELETE',
			`/media/${id}/fields/${encodeURIComponent(canonical)}/not-applicable`
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

	// In-app field promotion (F44, ADR-062). Owner-gated, and — unlike decisions/curation —
	// GLOBAL per (entity_type, field_key): promoting an auto-registered non-canonical field
	// makes it first-class curatable for every entity of that type that has the key, with no
	// metadata-mappings.yaml editing. `promoteField` upserts the presentation override;
	// `unpromoteField` de-promotes (reverts to the F39 display-only row; the shadow value and
	// any prior decisions/curation are untouched and re-apply on re-promotion).
	promoteField: (entityType: PromotionEntityType, fieldKey: string, req: FieldPromotionRequest) =>
		sendAuthed<Record<string, never>>(
			'PUT',
			`/admin/field-promotions/${entityType}/${encodeURIComponent(fieldKey)}`,
			req
		),
	unpromoteField: (entityType: PromotionEntityType, fieldKey: string) =>
		sendAuthed<Record<string, never>>(
			'DELETE',
			`/admin/field-promotions/${entityType}/${encodeURIComponent(fieldKey)}`
		),
	listFieldPromotions: (entityType: PromotionEntityType) =>
		getAuthed<FieldPromotionView[]>(`/admin/field-promotions/${entityType}`),

	// In-app field claims (F49, ADR-074). Also owner-gated and GLOBAL per (entity_type,
	// provider, field_key): attaching a provider key to a canonical field makes it a
	// candidate source of that field everywhere, and stops it auto-registering as its own
	// display-only row (the GH #178 duplicate-paragraph fix). `claimField` upserts;
	// `unclaimField` returns the key to auto-registration. `listFieldTargets` is the
	// picker's option list — the entity type's effective field set, which the page cannot
	// derive because empty undecided fields never render.
	claimField: (
		entityType: PromotionEntityType,
		provider: string,
		fieldKey: string,
		canonical: string
	) =>
		sendAuthed<Record<string, never>>(
			'PUT',
			`/admin/field-claims/${entityType}/${encodeURIComponent(provider)}/${encodeURIComponent(fieldKey)}`,
			{ canonical }
		),
	unclaimField: (entityType: PromotionEntityType, provider: string, fieldKey: string) =>
		sendAuthed<Record<string, never>>(
			'DELETE',
			`/admin/field-claims/${entityType}/${encodeURIComponent(provider)}/${encodeURIComponent(fieldKey)}`
		),
	listFieldClaims: (entityType: PromotionEntityType) =>
		getAuthed<FieldClaim[]>(`/admin/field-claims/${entityType}`),
	listFieldTargets: (entityType: PromotionEntityType) =>
		getAuthed<FieldTarget[]>(`/admin/field-targets/${entityType}`)
};
