// One-shot, key-validated view snapshot for restoring state across SPA navigation
// (ADR-032). Holodex is an SPA (ssr=false): a list/grid is destroyed on navigating away
// and re-created on Back, losing scroll position (and, for the browse grid, every loaded
// "Load more" page). The JS module isn't torn down, so a snapshot saved on
// `beforeNavigate` survives and is restored on return — but only while the view's KEY
// still matches (the filter/sort signature unchanged); a key change invalidates it.
// One-shot: `take` clears the snapshot, so each navigate-away must re-save and a snapshot
// can't be reused across visits. Session-scoped: a full reload starts empty.
//
// Used by browse.svelte.ts (full grid snapshot) and peopleScroll.svelte.ts (scroll only).

export interface Keyed {
	/** The filter/sort signature that produced this snapshot; restore only if it matches. */
	key: string;
}

export function createNavSnapshot<T extends Keyed>() {
	let snap: T | null = null;
	return {
		save(s: T) {
			snap = s;
		},
		// Return the snapshot iff its key matches the current view, else null; clears on
		// read either way (consumed once; stale-on-mismatch).
		take(key: string): T | null {
			const s = snap;
			snap = null;
			return s && s.key === key ? s : null;
		},
		clear() {
			snap = null;
		}
	};
}
