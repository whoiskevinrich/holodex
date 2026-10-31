// Browse-grid snapshot (ADR-032, QW4). Holodex is an SPA (ssr=false), so the browse grid
// is destroyed on navigating away and re-created on return. Since the whole-list scroll
// (F80 EL14, HOLODEX-604) the position rides listScroll like every other whole-list page;
// this keeps the grid's loaded pages beside it, so Back shows the videos that were in view
// at once instead of as placeholders, plus the Recently added shelf's videos, so the shelf
// above the grid doesn't arrive late and push the grid down. The key is the filter/sort
// param string; see navSnapshot for the shared one-shot/stale-on-mismatch mechanics.
// Session-scoped: a full reload rebuilds from the URL.
import type { Video } from '#lib/types.ts';
import type { SparseSnapshot } from '#lib/sparseList.svelte.ts';
import { createNavSnapshot, type Keyed } from '#lib/navSnapshot.svelte.ts';

export interface BrowseSnapshot extends Keyed, SparseSnapshot<Video> {
	recent: Video[];
}

export const browseCache = createNavSnapshot<BrowseSnapshot>();
