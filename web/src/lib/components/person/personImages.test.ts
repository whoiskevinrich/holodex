import { afterEach, describe, expect, it, vi } from 'vitest';
import type { PersonImage, PersonImageSet } from '$lib/types';
import { loadPersonImages, stripGallery, STRIP_GALLERY_SLOTS } from './personImages';

const img = (id: number, role: PersonImage['role'], sort_order = id): PersonImage => ({
	id,
	role,
	source: 'enrichment',
	version: 1,
	width: 400,
	height: 400,
	sort_order,
	created_at: '2026-09-23T00:00:00Z'
});

const set = (gallery: PersonImage[]): PersonImageSet => ({ roles: {}, gallery });

// P0-5: the panel fetches per pair on expand and reopening issues no second request.
// `fetch` is stubbed rather than `api`, because the criterion is about REQUESTS —
// mocking the client would pin the call count and miss a bespoke fetch added later.
// The cache is module-level and shared across the whole suite, so each case uses its
// own person id; that is also what the real surface does (one entry per person).
describe('loadPersonImages', () => {
	afterEach(() => vi.unstubAllGlobals());

	// A fresh Response per call: a body can only be read once, and these cases
	// deliberately make two requests.
	function stub(body: unknown, status = 200) {
		const fetchMock = vi.fn(async () => new Response(JSON.stringify(body), { status }));
		vi.stubGlobal('fetch', fetchMock);
		return fetchMock;
	}

	it('reads /people/{id}/images', async () => {
		const fetchMock = stub(set([]));
		await loadPersonImages(101);
		expect(fetchMock).toHaveBeenCalledWith('/api/v1/people/101/images', expect.anything());
	});

	it('issues no second request when the same person is loaded again', async () => {
		const fetchMock = stub(set([img(1, 'headshot')]));
		const first = await loadPersonImages(102);
		const second = await loadPersonImages(102);
		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(second).toBe(first);
	});

	it('shares one in-flight request between both panels asking at once', async () => {
		const fetchMock = stub(set([]));
		const [a, b] = await Promise.all([loadPersonImages(103), loadPersonImages(103)]);
		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(a).toBe(b);
	});

	it('keys by person, so the two sides of a pair are separate entries', async () => {
		const fetchMock = stub(set([]));
		await Promise.all([loadPersonImages(104), loadPersonImages(105)]);
		expect(fetchMock).toHaveBeenCalledTimes(2);
	});

	it("does not cache a failure — a failed side's Retry re-requests", async () => {
		const failing = stub({ error: 'boom' }, 500);
		await expect(loadPersonImages(106)).rejects.toThrow();
		expect(failing).toHaveBeenCalledTimes(1);

		const ok = stub(set([img(1, 'headshot')]));
		await expect(loadPersonImages(106)).resolves.toBeDefined();
		expect(ok).toHaveBeenCalledTimes(1);
	});
});

// P0-3. Slot 1 is always the headshot role, so the gallery slots must exclude it or
// the same face renders twice; and the cap is the whole strip — there is no `+N`.
describe('stripGallery', () => {
	it('drops the headshot, which slot 1 already serves', () => {
		const out = stripGallery(set([img(1, 'headshot'), img(2, 'poster'), img(3, 'banner')]));
		expect(out.map((i) => i.id)).toEqual([2, 3]);
	});

	// 50 images is the live median (the 2026-09-22 probe: most people carry 40–50), and
	// the headshot plus these four are the five 44px frames OQ4 settled on.
	it('caps at four gallery slots, and says nothing about the rest', () => {
		const many = set(Array.from({ length: 50 }, (_, i) => img(i + 1, 'poster')));
		expect(stripGallery(many)).toHaveLength(STRIP_GALLERY_SLOTS);
		expect(STRIP_GALLERY_SLOTS).toBe(4);
	});

	it('keeps the order the API returned — sort_order, not id', () => {
		const out = stripGallery(set([img(9, 'poster', 0), img(4, 'banner', 1), img(7, 'poster', 2)]));
		expect(out.map((i) => i.id)).toEqual([9, 4, 7]);
	});

	it('reserves nothing for a person with one image', () => {
		expect(stripGallery(set([img(1, 'headshot')]))).toEqual([]);
	});

	it('reserves nothing for a person with no images at all', () => {
		expect(stripGallery(set([]))).toEqual([]);
	});
});
