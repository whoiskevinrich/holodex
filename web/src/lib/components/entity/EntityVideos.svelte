<script lang="ts">
	import { tick, type Snippet } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import type { Video } from '$lib/types';
	import VideoGrid from '../video/VideoGrid.svelte';
	import { listScroll } from '$lib/listScroll.svelte';
	import { videoCount } from '$lib/format';

	// Shared body for the person/[id], studio/[id], and tag/[id] detail pages: back-link,
	// title, video count, and the reused grid. The optional `detail` snippet renders an
	// entity-specific panel (e.g. People enrichment, F22) between the count and grid;
	// the tag page omits it, keeping this component shared.
	//
	// The optional `hero` snippet REPLACES the default title+count block — the person
	// page uses it to render its banner with the name beside the portrait (so the name
	// reads as one unit with the face), supplying its own title and count within. When
	// `hero` is omitted (the tag/studio pages) the plain title+count is rendered as before.
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
		name,
		videos,
		empty,
		scrollKey,
		hero,
		detail
	}: {
		backHref: string;
		backLabel: string;
		name: string;
		videos: Video[];
		empty: string;
		scrollKey: string;
		hero?: Snippet;
		detail?: Snippet;
	} = $props();

	// A single caller per scrollKey (one entity's own video grid, not sorted/filtered), so
	// there's no second axis to invalidate on — the key just has to satisfy Keyed.
	const SCROLL_INVALIDATION_KEY = 'videos';

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
	{#if hero}
		{@render hero()}
	{:else}
		<h1 class="skin-title text-2xl font-semibold text-ink">{name}</h1>
		<p class="text-sm text-muted">{videoCount(videos.length)}</p>
	{/if}
	{#if detail}{@render detail()}{/if}
	<VideoGrid {videos} {empty} />
</section>
