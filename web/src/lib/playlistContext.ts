import { api } from '$lib/api';
import type { PlaylistResponse } from '$lib/types';

// Playback plumbing shared by every run (F69 P0-9 / ADR-104 D4; F75 / ADR-121 D7): the
// play-on-load intent, and the per-(id, seed) playlist fetch cache a playlist-source run
// reads. The run itself (order, URL, toggles) is run.ts; the `?playlist=` URL parse,
// href and neighbours that used to live here moved there when F69's context became a
// run with a playlist source.

export interface PlaylistParam {
	id: number;
	seed?: number;
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

/** Drop a playlist's cached orders after a membership or sort edit, so the next
 *  context load (a Play all right after a removal) reads the current one. */
export function forgetPlaylist(id: number) {
	for (const key of [...cache.keys()]) if (key.startsWith(`${id}:`)) cache.delete(key);
}
