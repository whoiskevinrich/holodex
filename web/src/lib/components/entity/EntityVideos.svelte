<script lang="ts">
	import { tick, type Snippet } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import type { Video } from '$lib/types';
	import VideoGrid from '../video/VideoGrid.svelte';
	import { listScroll } from '$lib/listScroll.svelte';
	import { filterByTitle } from '$lib/format';
	import { navSearch } from '$lib/navSearch.svelte';

	// Shared body for the person/[id], studio/[id], and tag/[id] detail pages: back-link,
	// hero, and the reused grid. The optional `detail` snippet renders an entity-specific
	// panel (e.g. People enrichment, F22) between the hero and grid; the tag page omits
	// it, keeping this component shared.
	//
	// `hero` renders each page's own title/count block — since HOLODEX-269 gave Studio a
	// hero too (its NameEditControl-based rename control), all three callers supply one;
	// there is no title+count fallback to keep in sync with them.
	//
	// Scroll restoration (HOLODEX-248): every caller reduces to the same (entity kind, id)
	// shape, so the wiring lives here once instead of copy-pasted per page. `scrollKey`
	// (e.g. `person:${id}`) addresses this entity's own listScroll slot. Top-level script
	// code runs once per component instance, which is exactly the "restore once" semantics
	// we want — no manual firstLoad flag needed. This relies on the caller only mounting
	// EntityVideos once real data has loaded (AsyncState renders it only when !loading) and
	// not unmounting it again for a later reload (merge/rename/writeback toggle never flips
	// loading back to true) — otherwise this would re-fire on every reload.
	let {
		backHref,
		backLabel,
		videos,
		empty,
		scrollKey,
		hero,
		detail
	}: {
		backHref: string;
		backLabel: string;
		videos: Video[];
		empty: string;
		scrollKey: string;
		hero: Snippet;
		detail?: Snippet;
	} = $props();

	// A single caller per scrollKey (one entity's own video grid, no sort control), so
	// there's no second axis to invalidate on — the key just has to satisfy Keyed. NS6's
	// in-place filter below doesn't add one either: restoring scroll under a stale filter
	// is fine since the query itself resets on navigation (navSearch is a page-scoped
	// singleton, not persisted), so by the time ← Back lands here the grid is unfiltered
	// again.
	const SCROLL_INVALIDATION_KEY = 'videos';

	// NS6 (HOLODEX-249): the nav search box drives this entity's own video list in
	// place once `pageScopeFor` (navSearch.svelte.ts) has scoped the current detail
	// route to Videos. Lives here rather than in each of the three callers — same
	// "wiring lives here once" reasoning as the scroll restoration above.
	const videoQuery = $derived(navSearch.inPlace ? navSearch.query : '');
	const displayedVideos = $derived(filterByTitle(videos, videoQuery));
	const emptyMessage = $derived(
		videoQuery.trim() ? `No videos match “${videoQuery.trim()}”.` : empty
	);

	tick().then(() => {
		const snap = listScroll.take(scrollKey, SCROLL_INVALIDATION_KEY);
		if (snap) window.scrollTo(0, snap.scrollY);
	});

	// Stash the scroll offset on the way out (e.g. opening a video) so ← Back restores
	// where this entity's video list was.
	beforeNavigate(() => {
		listScroll.save(scrollKey, { key: SCROLL_INVALIDATION_KEY, scrollY: window.scrollY });
	});
</script>

<section class="space-y-4">
	<a href={backHref} class="text-sm text-muted hover:text-ink">← {backLabel}</a>
	{@render hero()}
	{#if detail}{@render detail()}{/if}
	<VideoGrid videos={displayedVideos} empty={emptyMessage} />
</section>
