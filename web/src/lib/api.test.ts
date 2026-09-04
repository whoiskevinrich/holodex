import { afterEach, describe, it, expect, vi } from 'vitest';
import { api, ENRICH_ENTITY_BASE } from './api';
import { runEnrichRefresh, runEnrichRefreshAll } from './enrichRefresh';

// Person image serving URLs (F25, ADR-038). The frontend always appends the active
// skin (so the backend's empty-slot placeholder matches the current skin) and the
// `?v=` cache-buster only when a version is known — a replaced core slot gets a new
// version and so a new, non-cached URL.
describe('personImageURL', () => {
	it('builds skin + version query params', () => {
		expect(api.personImageURL(7, 'headshot', { skin: 'broadcast', version: 42 })).toBe(
			'/api/v1/people/7/image/headshot?skin=broadcast&v=42'
		);
	});

	it('omits v when no version is given', () => {
		expect(api.personImageURL(3, 'banner', { skin: 'cinematheque' })).toBe(
			'/api/v1/people/3/image/banner?skin=cinematheque'
		);
	});

	it('omits the query entirely with no opts', () => {
		expect(api.personImageURL(9, 'poster')).toBe('/api/v1/people/9/image/poster');
	});

	it('omits v when version is 0 (an unfilled slot)', () => {
		expect(api.personImageURL(1, 'headshot', { skin: 'brutalist', version: 0 })).toBe(
			'/api/v1/people/1/image/headshot?skin=brutalist'
		);
	});
});

describe('personGalleryImageURL', () => {
	it('stamps version and skin when present', () => {
		expect(api.personGalleryImageURL(5, 88, { version: 88, skin: 'broadcast' })).toBe(
			'/api/v1/people/5/images/88?skin=broadcast&v=88'
		);
	});

	it('omits the query when no opts', () => {
		expect(api.personGalleryImageURL(5, 88)).toBe('/api/v1/people/5/images/88');
	});
});

// F37 person clients (RD7 endpoint parity). fetch is stubbed so the paths, verbs, and the
// rename 409→conflict contract are pinned without a server.
describe('person source-of-truth clients (F37)', () => {
	afterEach(() => vi.unstubAllGlobals());

	function stub(status: number, body?: unknown) {
		const fetchMock = vi
			.fn()
			.mockResolvedValue(
				new Response(body === undefined ? null : JSON.stringify(body), { status })
			);
		vi.stubGlobal('fetch', fetchMock);
		return fetchMock;
	}

	it('setPersonFieldDecision PUTs the person decision path with the record source', async () => {
		const fetchMock = stub(204);
		await api.setPersonFieldDecision(7, 'bio', { source: 'record' });
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/people/7/fields/bio/decision',
			expect.objectContaining({ method: 'PUT', body: JSON.stringify({ source: 'record' }) })
		);
	});

	it('clearPersonFieldDecision DELETEs the same path', async () => {
		const fetchMock = stub(204);
		await api.clearPersonFieldDecision(7, 'bio');
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/people/7/fields/bio/decision',
			expect.objectContaining({ method: 'DELETE' })
		);
	});

	it('curatePerson / clearPersonCuration POST the person curation paths', async () => {
		const req = { field: 'aliases', value: 'J Law', action: 'suppress' as const };
		let fetchMock = stub(204);
		await api.curatePerson(7, req);
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/people/7/curation',
			expect.objectContaining({ method: 'POST', body: JSON.stringify(req) })
		);
		fetchMock = stub(204);
		await api.clearPersonCuration(7, req);
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/people/7/curation/clear',
			expect.objectContaining({ method: 'POST' })
		);
	});

	// HOLODEX-269: person rename now goes through the shared renameEntity trio (retiring
	// the person-only renamePerson, whose 409 handling didn't unwrap `body.conflict` and
	// was masked by a mock using the wrong, flat response shape — the real backend always
	// wraps it, pinned below).
	it('renameEntity(person) POSTs the name and resolves empty on 204', async () => {
		const fetchMock = stub(204);
		await expect(api.renameEntity('person', 3, 'New Name')).resolves.toEqual({});
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/people/3/rename',
			expect.objectContaining({ method: 'POST', body: JSON.stringify({ name: 'New Name' }) })
		);
	});

	it('renameEntity(person) surfaces the colliding person on 409 instead of throwing', async () => {
		const colliding = { id: 9, name: 'Existing Person', video_count: 14 };
		stub(409, { conflict: colliding });
		await expect(api.renameEntity('person', 3, 'Existing Person')).resolves.toEqual({
			conflict: colliding
		});
	});

	it('renameEntity(person) throws ApiError on other failures', async () => {
		stub(400);
		await expect(api.renameEntity('person', 3, 'x')).rejects.toMatchObject({ status: 400 });
	});
});

// HOLODEX-127: when the upstream Authentik ForwardAuth session lapses, an authed
// request is 302-redirected cross-origin to the IdP. `redirect: 'manual'` turns that
// into an opaque redirect (not a CORS-blocked follow); we detect it, raise
// ReauthError (distinct from a 401 owner-expiry), and recover with one top-level
// reload. A fresh module per test resets the one-shot reauth guard.
describe('ForwardAuth re-auth handling (HOLODEX-127)', () => {
	afterEach(() => vi.unstubAllGlobals());

	async function freshApi() {
		vi.resetModules();
		return import('./api');
	}

	// A `redirect: 'manual'` fetch returns an opaque redirect for a cross-origin 302;
	// there's no public constructor, so fake the shape checkRedirect() reads.
	function opaqueRedirect(): Response {
		return {
			type: 'opaqueredirect',
			ok: false,
			status: 0,
			json: () => Promise.reject(new Error('opaque'))
		} as unknown as Response;
	}

	function stubWindow(href = 'https://barclay.example/owner/status') {
		const assign = vi.fn();
		vi.stubGlobal('window', { location: { href, assign } });
		return assign;
	}

	it('sends authed reads with redirect:manual and raises ReauthError on a ForwardAuth 302', async () => {
		const { api, ReauthError } = await freshApi();
		stubWindow();
		const fetchMock = vi.fn().mockResolvedValue(opaqueRedirect());
		vi.stubGlobal('fetch', fetchMock);

		await expect(api.activity()).rejects.toBeInstanceOf(ReauthError);
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/admin/activity',
			expect.objectContaining({ redirect: 'manual', credentials: 'same-origin' })
		);
	});

	it('raises ReauthError on an authed write too (not just the poll)', async () => {
		const { api, ReauthError } = await freshApi();
		stubWindow();
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(opaqueRedirect()));
		await expect(api.rescan()).rejects.toBeInstanceOf(ReauthError);
	});

	it('recovers via exactly one top-level reload across concurrent authed failures', async () => {
		const { api } = await freshApi();
		const assign = stubWindow('https://barclay.example/x');
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(opaqueRedirect()));

		await Promise.allSettled([api.activity(), api.capabilities(), api.trash()]);
		expect(assign).toHaveBeenCalledTimes(1);
		expect(assign).toHaveBeenCalledWith('https://barclay.example/x');
	});

	it('a normal 200 is unaffected by redirect:manual', async () => {
		const { api } = await freshApi();
		stubWindow();
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(new Response(JSON.stringify({ owner: true }), { status: 200 }))
		);
		await expect(api.capabilities()).resolves.toEqual({ owner: true });
	});
});

// Alias clients (F23/F43, extended by F58/ADR-088). Untested until now, unlike their
// curatePerson/renameEntity neighbours above — and the collapse makes them the only
// remaining write path for alternate names, so the 409-conflict contract and the
// entity-generic path mapping are worth pinning.
describe('entity alias clients', () => {
	afterEach(() => vi.unstubAllGlobals());

	function stub(status: number, body?: unknown) {
		const fetchMock = vi
			.fn()
			.mockResolvedValue(
				new Response(body === undefined ? null : JSON.stringify(body), { status })
			);
		vi.stubGlobal('fetch', fetchMock);
		return fetchMock;
	}

	it('addEntityAlias POSTs the alias and returns the updated list', async () => {
		const fetchMock = stub(200, { aliases: [{ id: 1, alias: 'J Law' }] });
		const res = await api.addEntityAlias('person', 7, 'J Law');
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/people/7/aliases',
			expect.objectContaining({ method: 'POST', body: JSON.stringify({ alias: 'J Law' }) })
		);
		expect(res.aliases).toEqual([{ id: 1, alias: 'J Law' }]);
		expect(res.conflict).toBeUndefined();
	});

	it('carries source through, so the chip badge has provenance to render', async () => {
		stub(200, { aliases: [{ id: 2, alias: '宮崎駿', source: 'tmdb' }] });
		const res = await api.addEntityAlias('person', 7, '宮崎駿');
		expect(res.aliases?.[0].source).toBe('tmdb');
	});

	it('surfaces a 409 as a conflict rather than throwing — never a silent merge', async () => {
		stub(409, { conflict: { id: 42, name: 'Jennifer Lawrence' } });
		const res = await api.addEntityAlias('person', 7, 'J Law');
		expect(res.conflict).toEqual({ id: 42, name: 'Jennifer Lawrence' });
		expect(res.aliases).toBeUndefined();
	});

	it('throws on a non-409 error instead of reporting a phantom conflict', async () => {
		stub(500);
		await expect(api.addEntityAlias('person', 7, 'J Law')).rejects.toThrow();
	});

	it('deleteEntityAlias DELETEs the scoped alias path', async () => {
		const fetchMock = stub(204);
		await api.deleteEntityAlias('person', 7, 3);
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/people/7/aliases/3',
			expect.objectContaining({ method: 'DELETE' })
		);
	});

	it('maps studio to its own base — AliasPanel is reused verbatim there', async () => {
		const fetchMock = stub(200, { aliases: [] });
		await api.addEntityAlias('studio', 4, 'Ghibli');
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/studios/4/aliases',
			expect.objectContaining({ method: 'POST' })
		);
	});
});

// Film enrichment clients (F59/ADR-089 D5, HOLODEX-309). The routes have existed since
// ADR-086 but nothing in the SPA called them, so these pin the paths and verbs the film
// detail page and the owner enrich queue now depend on. The `runEnrichRefresh` cases are
// the load-bearing ones: they prove films ride the *generic* helper, so a future
// film-specific branch in enrichRefresh.ts would be a regression, not a fix.
describe('film enrichment clients (F59)', () => {
	afterEach(() => vi.unstubAllGlobals());

	function stub(status: number, body?: unknown) {
		const fetchMock = vi
			.fn()
			.mockResolvedValue(
				new Response(body === undefined ? null : JSON.stringify(body), { status })
			);
		vi.stubGlobal('fetch', fetchMock);
		return fetchMock;
	}

	it('ENRICH_ENTITY_BASE maps film to /films, alongside the three older kinds', () => {
		expect(ENRICH_ENTITY_BASE.film).toBe('films');
		// Pinned as a set: this record is exhaustive over EnrichEntityKind, so a new kind
		// that forgets its base segment silently 404s every refresh for that entity.
		expect(Object.keys(ENRICH_ENTITY_BASE).sort()).toEqual(['film', 'person', 'studio', 'video']);
	});

	it('enrichFilmResolve POSTs the film resolve path with provider and query', async () => {
		const fetchMock = stub(200, { candidates: [] });
		await api.enrichFilmResolve(12, 'tmdb', 'The Matrix');
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/films/12/enrich/resolve',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ provider: 'tmdb', query: 'The Matrix' })
			})
		);
	});

	it('enrichFilmApply POSTs the external id under the snake_case key the API expects', async () => {
		const fetchMock = stub(200, { enriched: [] });
		await api.enrichFilmApply(12, 'tmdb', '603');
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/films/12/enrich',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ provider: 'tmdb', external_id: '603' })
			})
		);
	});

	it('enrichFilmClear DELETEs the provider-scoped path, encoding the provider name', async () => {
		const fetchMock = stub(204);
		await api.enrichFilmClear(12, 'my provider');
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/films/12/enrich/my%20provider',
			expect.objectContaining({ method: 'DELETE' })
		);
	});

	it('enrichDismiss/enrichRefresh route films through the generic kind dispatch', async () => {
		let fetchMock = stub(204);
		await api.enrichDismiss('film', 12, 'tmdb');
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/films/12/enrich/tmdb/dismiss',
			expect.objectContaining({ method: 'POST' })
		);

		fetchMock = stub(200, { results: [] });
		await api.enrichRefresh('film', 12, 'tmdb');
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/films/12/enrich/tmdb/refresh',
			expect.objectContaining({ method: 'POST' })
		);
	});

	it('runEnrichRefresh drives the film path with no film-specific branch of its own', async () => {
		const fetchMock = stub(200, { results: [] });
		const reload = vi.fn().mockResolvedValue(undefined);
		const busy: string[] = [];
		const errors: string[] = [];

		await runEnrichRefresh('film', 12, 'tmdb', (v) => busy.push(v), (v) => errors.push(v), reload);

		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/films/12/enrich/tmdb/refresh',
			expect.objectContaining({ method: 'POST' })
		);
		// Busy is set to the provider then cleared, and the detail reload runs once — the
		// same contract the person/studio/video pages already rely on.
		expect(busy).toEqual(['tmdb', '']);
		expect(errors).toEqual(['']);
		expect(reload).toHaveBeenCalledTimes(1);
	});

	it('runEnrichRefreshAll opens the picker on a needs_review film result', async () => {
		const fetchMock = stub(200, { results: [{ provider: 'tmdb', status: 'needs_review' }] });
		const opened: string[] = [];

		await runEnrichRefreshAll(
			'film',
			12,
			() => {},
			() => {},
			async () => {},
			(p) => opened.push(p)
		);

		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/films/12/enrich/refresh-all',
			expect.objectContaining({ method: 'POST' })
		);
		// An ambiguous provider must never be silently dropped for films either.
		expect(opened).toEqual(['tmdb']);
	});

	it('surfaces a films-disabled 404 as an error instead of a silent no-op', async () => {
		// Film enrich routes are unregistered when films_enabled is off, so the only honest
		// client behaviour is to report the failure — the page renders it inline.
		stub(404);
		const errors: string[] = [];
		const reload = vi.fn().mockResolvedValue(undefined);

		await runEnrichRefresh('film', 12, 'tmdb', () => {}, (v) => errors.push(v), reload);

		expect(errors.at(-1)).toBeTruthy();
		expect(reload).not.toHaveBeenCalled();
	});
});
