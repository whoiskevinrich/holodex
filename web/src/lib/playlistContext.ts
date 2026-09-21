import { api } from '$lib/api';
import type { PlaylistResponse } from '$lib/types';

// Playlist playback context for /media/[id] (F69 P0-9, ADR-104 D4). `?playlist=<id>`
// (+ `&seed=` for a 'random' sort) puts the page in a context: the playlist's ordered
// ids, where the current video sits in them, and the next/previous hrefs. Pure helpers
// here, page state on the page; the one impure piece is the per-(id, seed) fetch cache.

export interface PlaylistParam {
	id: number;
	seed?: number;
}

/** The `?playlist=` (+ `&seed=`) pair from a URL, or null when the page is not in a context. */
export function parsePlaylistParam(url: URL): PlaylistParam | null {
	const id = Number(url.searchParams.get('playlist'));
	if (!Number.isInteger(id) || id <= 0) return null;
	const rawSeed = url.searchParams.get('seed');
	const seed = rawSeed == null ? NaN : Number(rawSeed);
	return Number.isInteger(seed) ? { id, seed } : { id };
}

/** `/media/{videoId}?playlist=…[&seed=…]` — the deep link that keeps the context. */
export function playlistHref(videoId: number, p: PlaylistParam): string {
	return `/media/${videoId}?playlist=${p.id}${p.seed != null ? `&seed=${p.seed}` : ''}`;
}

export interface Neighbours {
	/** 0-based index of the current video in the order, or -1 when it is not a member. */
	index: number;
	prev: number | null;
	next: number | null;
}

/** Where `videoId` sits in `ids`, and its neighbours in the playlist's order. */
export function neighbours(ids: number[], videoId: number): Neighbours {
	const index = ids.indexOf(videoId);
	if (index === -1) return { index, prev: null, next: null };
	return {
		index,
		prev: index > 0 ? ids[index - 1] : null,
		next: index < ids.length - 1 ? ids[index + 1] : null
	};
}

// Play-on-load intent (spec RD6): set by `ended`, Next, Previous and Start — never by a
// URL — and consumed exactly once, by the page, when the video it names is the one that
// has loaded. Keyed on the target id rather than the element so a stale element (the
// spike's AbortError) is impossible by construction; a reload or a shared link finds it
// empty and waits for a gesture.
let intent: number | null = null;

export function setPlayIntent(videoId: number) {
	intent = videoId;
}

/** True once, when `videoId` is the intended one; any other id clears the intent. */
export function takePlayIntent(videoId: number): boolean {
	const hit = intent === videoId;
	intent = null;
	return hit;
}

// One fetch per (playlist, seed) for the session: a play-through walks one order (a
// 'random' sort's seed is echoed by the server and kept in the URL). A failure — a 404
// for an unknown id or a private playlist seen by a visitor, or a network error —
// resolves to null, makes the param inert for this page view, and is not remembered.
const cache = new Map<string, Promise<PlaylistResponse | null>>();

export function loadPlaylist(p: PlaylistParam): Promise<PlaylistResponse | null> {
	const key = `${p.id}:${p.seed ?? ''}`;
	let pending = cache.get(key);
	if (!pending) {
		pending = api.getPlaylist(p.id, p.seed).catch(() => {
			cache.delete(key);
			return null;
		});
		cache.set(key, pending);
	}
	return pending;
}
