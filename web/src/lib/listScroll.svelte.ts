// Scroll-position snapshot for any list-like page (ADR-032, HOLODEX-248). A list page is
// destroyed on navigating away (into a detail page) and re-created on Back; SvelteKit
// resets scroll to top for in-app (push) navigation, so each list stashes its own scroll
// offset — keyed by list identity (`id`, e.g. 'people', 'studios', `person:${id}`) and an
// invalidation signature (`key`, e.g. the sort value) — and restores it once the
// re-fetched list paints. One shared registry for every scroll-only list, so a new list
// page just picks its own `id`; see navSnapshot for the one-shot/stale-on-mismatch/
// per-id-isolation mechanics. The browse grid is the one exception (its own heavier
// browseCache, browse.svelte.ts) — it also caches the loaded page set, not just scroll.
import { createNavSnapshotRegistry, type Keyed } from '$lib/navSnapshot.svelte';

export interface ListScrollSnapshot extends Keyed {
	scrollY: number;
}

export const listScroll = createNavSnapshotRegistry<ListScrollSnapshot>();
