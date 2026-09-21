import { describe, expect, it } from 'vitest';
import {
	neighbours,
	parsePlaylistParam,
	playlistHref,
	setPlayIntent,
	takePlayIntent
} from './playlistContext';

describe('parsePlaylistParam', () => {
	it('reads the playlist id and an optional seed', () => {
		expect(parsePlaylistParam(new URL('http://h/media/5?playlist=3'))).toEqual({ id: 3 });
		expect(parsePlaylistParam(new URL('http://h/media/5?playlist=3&seed=42'))).toEqual({
			id: 3,
			seed: 42
		});
	});

	it('is null outside a context and ignores a malformed id or seed', () => {
		expect(parsePlaylistParam(new URL('http://h/media/5'))).toBeNull();
		expect(parsePlaylistParam(new URL('http://h/media/5?playlist=abc'))).toBeNull();
		expect(parsePlaylistParam(new URL('http://h/media/5?playlist=0'))).toBeNull();
		expect(parsePlaylistParam(new URL('http://h/media/5?playlist=3&seed=x'))).toEqual({ id: 3 });
	});
});

describe('playlistHref', () => {
	it('keeps the context, seed included only when present', () => {
		expect(playlistHref(9, { id: 3 })).toBe('/media/9?playlist=3');
		expect(playlistHref(9, { id: 3, seed: 42 })).toBe('/media/9?playlist=3&seed=42');
	});
});

describe('neighbours', () => {
	const ids = [10, 20, 30];
	it('walks the order in both directions', () => {
		expect(neighbours(ids, 20)).toEqual({ index: 1, prev: 10, next: 30 });
	});
	it('has no previous on the first item and no next on the last', () => {
		expect(neighbours(ids, 10)).toEqual({ index: 0, prev: null, next: 20 });
		expect(neighbours(ids, 30)).toEqual({ index: 2, prev: 20, next: null });
	});
	it('reports a non-member (stale link, removed item) as index -1 with no neighbours', () => {
		expect(neighbours(ids, 99)).toEqual({ index: -1, prev: null, next: null });
		expect(neighbours([], 10)).toEqual({ index: -1, prev: null, next: null });
	});
});

describe('play intent', () => {
	it('is consumed once, only by the video it names', () => {
		setPlayIntent(7);
		expect(takePlayIntent(7)).toBe(true);
		expect(takePlayIntent(7)).toBe(false); // consumed
	});
	it('is cleared by any other outcome, so a stale intent never fires later', () => {
		setPlayIntent(7);
		expect(takePlayIntent(8)).toBe(false); // a different video loaded
		expect(takePlayIntent(7)).toBe(false); // and the intent is gone
	});
	it('is empty on a fresh load (a reload or shared link never autoplays)', () => {
		expect(takePlayIntent(1)).toBe(false);
	});
});
