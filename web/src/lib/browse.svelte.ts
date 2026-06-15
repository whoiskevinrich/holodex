// Browse-state preservation across SPA navigation (ADR-032, QW4). Holodex is an
// SPA (ssr=false), so the browse grid component is destroyed on navigation away and
// re-created on return — losing scroll position AND every "Load more" page (the grid
// re-fetches only page 0 on mount). This module-scoped cache survives client
// navigation (the JS module isn't torn down), so returning to the grid (open item →
// Back) restores the loaded set, pagination offset, filters, and scroll synchronously
// — no re-fetch, no flash. Session-scoped only: a full reload rebuilds from the URL.
import type { Video } from '$lib/types';

export interface BrowseSnapshot {
	signature: string; // the filter/sort param string that produced this set
	videos: Video[];
	total: number;
	offset: number;
	scrollY: number;
}

// One entry — there is a single browse grid. Shaped so it could become a keyed map
// for entity pages later without changing consumers (ADR-032).
let snapshot: BrowseSnapshot | null = null;

export const browseCache = {
	save(s: BrowseSnapshot) {
		snapshot = s;
	},
	// Consume the snapshot iff its signature matches the current filters; otherwise
	// drop it (a filter change invalidates the cache) and return null. One-shot: each
	// navigate-away re-saves, so reuse can't go stale across visits.
	take(signature: string): BrowseSnapshot | null {
		const s = snapshot;
		snapshot = null;
		return s && s.signature === signature ? s : null;
	},
	clear() {
		snapshot = null;
	}
};
