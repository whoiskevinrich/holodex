// Browse-grid snapshot (ADR-032, QW4). Holodex is an SPA (ssr=false), so the browse grid
// is destroyed on navigating away and re-created on return — losing scroll position AND
// every "Load more" page (the grid re-fetches only page 0 on mount). A keyed view
// snapshot survives client navigation, so returning to the grid (open item → Back)
// restores the loaded set, pagination offset, filters, and scroll synchronously — no
// re-fetch, no flash. The key is the filter/sort param string; see navSnapshot for the
// shared one-shot/stale-on-mismatch mechanics. Session-scoped: a full reload rebuilds
// from the URL.
import type { Video } from '$lib/types';
import { createNavSnapshot, type Keyed } from '$lib/navSnapshot.svelte';

export interface BrowseSnapshot extends Keyed {
	videos: Video[];
	total: number;
	offset: number;
	scrollY: number;
}

export const browseCache = createNavSnapshot<BrowseSnapshot>();
