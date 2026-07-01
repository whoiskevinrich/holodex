import { afterEach, describe, it, expect, vi } from 'vitest';
import { api } from './api';

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

	it('renamePerson POSTs the name and resolves empty on 204', async () => {
		const fetchMock = stub(204);
		await expect(api.renamePerson(3, 'New Name')).resolves.toEqual({});
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/people/3/rename',
			expect.objectContaining({ method: 'POST', body: JSON.stringify({ name: 'New Name' }) })
		);
	});

	it('renamePerson surfaces the colliding person on 409 instead of throwing (RD1)', async () => {
		const colliding = { id: 9, name: 'Existing Person', video_count: 14 };
		stub(409, colliding);
		await expect(api.renamePerson(3, 'Existing Person')).resolves.toEqual({
			conflict: colliding
		});
	});

	it('renamePerson throws ApiError on other failures', async () => {
		stub(400);
		await expect(api.renamePerson(3, 'x')).rejects.toMatchObject({ status: 400 });
	});
});
