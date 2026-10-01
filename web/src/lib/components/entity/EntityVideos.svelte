<script lang="ts">
	import { tick, untrack, type Snippet } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import type { MediaFilters, Video } from '$lib/types';
	import VideoGrid from '../video/VideoGrid.svelte';
	import { api } from '$lib/api';
	import { listScroll } from '$lib/listScroll.svelte';
	import { toMessage } from '$lib/format';
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
	// The grid is query-backed (HOLODEX-501, ADR-121 D6): it pages through GET /media
	// with the caller's entity `facet` (e.g. `{ person: [id] }`), exactly as browse does,
	// so there is no cap and the query a run or a save reads is the one shown. `total`
	// binds back up for the caller's hero count: the entity's whole count, unaffected by
	// the title filter, and null until the first unfiltered load lands. Bump `refreshKey`
	// after a mutation that can change the set (a merge-in, say) to re-read it in place.
	//
	// Scroll restoration (HOLODEX-248): every caller reduces to the same (entity kind, id)
	// shape, so the wiring lives here once instead of copy-pasted per page. `scrollKey`
	// (e.g. `person:${id}`) addresses this entity's own listScroll slot. Top-level script
	// code runs once per component instance, which is exactly the "restore once" semantics
	// we want — no manual firstLoad flag needed. This relies on the caller only mounting
	// EntityVideos once the entity has loaded (AsyncState renders it only when !loading)
	// and not unmounting it again for a later reload. The snapshot also records how many
	// videos were loaded, so the first fetch brings back every page the scroll offset
	// sits in.
	let {
		backHref,
		backLabel,
		facet,
		refreshKey = 0,
		total = $bindable(null),
		empty,
		scrollKey,
		hero,
		detail,
		footer
	}: {
		backHref?: string;
		backLabel?: string;
		facet: MediaFilters;
		refreshKey?: number;
		total?: number | null;
		empty: string;
		scrollKey: string;
		hero: Snippet;
		detail?: Snippet;
		footer?: Snippet;
	} = $props();

	const PAGE_SIZE = 50;
	// The server caps a page at 1000 (repo maxListLimit).
	const MAX_LIMIT = 1000;

	// A single caller per scrollKey (one entity's own video grid, no sort control), so
	// there's no second axis to invalidate on — the key just has to satisfy Keyed. NS6's
	// in-place filter below doesn't add one either: restoring scroll under a stale filter
	// is fine since the query itself resets on navigation (navSearch is a page-scoped
	// singleton, not persisted), so by the time ← Back lands here the grid is unfiltered
	// again.
	const SCROLL_INVALIDATION_KEY = 'videos';
	// Taken once per mount, deliberately: the restore is one-shot.
	const restore = untrack(() => listScroll.take(scrollKey, SCROLL_INVALIDATION_KEY));

	let videos = $state<Video[]>([]);
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state('');

	// NS6 (HOLODEX-249): the nav search box drives this entity's own video list in
	// place once `pageScopeFor` (navSearch.svelte.ts) has scoped the current detail
	// route to Videos. Since HOLODEX-501 it is the query's `q` (title prefix match, as
	// on browse), so the server filters and `matched` counts the filtered set (F75 RD7).
	const videoQuery = $derived((navSearch.inPlace ? navSearch.query : '').trim());
	const filters = $derived<MediaFilters>({ ...facet, q: videoQuery || undefined });
	const emptyMessage = $derived(videoQuery ? `No videos match “${videoQuery}”.` : empty);
	// The current query's match count; equals `total` when no title filter is set.
	let matched = $state(0);
	const hasMore = $derived(videos.length < matched);

	function applyCount(f: MediaFilters, n: number) {
		matched = n;
		if (!f.q) total = n;
	}

	// seq drops a response that a newer request has superseded (typing in the title box).
	let seq = 0;
	let started = false;
	let restorePending = restore != null;
	let lastQuery = '';

	async function load(f: MediaFilters, limit: number) {
		const mine = ++seq;
		error = '';
		try {
			const res = await api.listMedia({ ...f, limit, offset: 0 });
			if (mine !== seq) return;
			videos = res.items ?? [];
			applyCount(f, res.total);
		} catch (e) {
			if (mine === seq) error = toMessage(e);
		} finally {
			if (mine === seq) loading = false;
		}
		if (restorePending && mine === seq && restore) {
			restorePending = false;
			await tick();
			window.scrollTo(0, restore.scrollY);
		}
	}

	let debounce: ReturnType<typeof setTimeout>;
	$effect(() => {
		const f = filters;
		void refreshKey;
		const query = JSON.stringify(f);
		clearTimeout(debounce);
		if (!started) {
			started = true;
			lastQuery = query;
			load(f, Math.min(Math.max(restore?.loaded ?? 0, PAGE_SIZE), MAX_LIMIT));
		} else if (query === lastQuery) {
			// A refresh of the same set keeps every page already loaded. untrack: the
			// loaded count must not become a dependency, or each load would re-fire this.
			load(f, Math.min(Math.max(untrack(() => videos.length), PAGE_SIZE), MAX_LIMIT));
		} else {
			lastQuery = query;
			debounce = setTimeout(() => load(f, PAGE_SIZE), f.q ? 200 : 0);
		}
		return () => clearTimeout(debounce);
	});

	async function loadMore() {
		// A title-box change still inside its debounce means `videos` belongs to the old
		// query; paging it with the new one would append the wrong set.
		const f = filters;
		if (JSON.stringify(f) !== lastQuery) return;
		const mine = seq;
		loadingMore = true;
		error = '';
		try {
			const res = await api.listMedia({ ...f, limit: PAGE_SIZE, offset: videos.length });
			if (mine !== seq) return;
			videos = [...videos, ...(res.items ?? [])];
			applyCount(f, res.total);
		} catch (e) {
			if (mine === seq) error = toMessage(e);
		} finally {
			loadingMore = false;
		}
	}

	// Stash the scroll offset on the way out (e.g. opening a video) so ← Back restores
	// where this entity's video list was.
	beforeNavigate(() => {
		listScroll.save(scrollKey, {
			key: SCROLL_INVALIDATION_KEY,
			scrollY: window.scrollY,
			loaded: videos.length
		});
	});
</script>

<section class="space-y-4">
	{#if backHref && backLabel}
		<a href={backHref} class="text-sm text-muted hover:text-ink">← {backLabel}</a>
	{/if}
	{@render hero()}
	{#if detail}{@render detail()}{/if}
	<!-- id="videos": the F68 hover card's "Videos" link lands here. -->
	<div id="videos" class="scroll-mt-16">
		{#if loading}
			<p class="text-sm text-muted">Loading videos…</p>
		{:else if error && videos.length === 0}
			<p class="text-sm text-warn">{error}</p>
		{:else}
			<!-- A failed refresh or Load more keeps the loaded grid and says so above it. -->
			{#if error}<p class="pb-2 text-sm text-warn">{error}</p>{/if}
			<VideoGrid {videos} empty={emptyMessage} />
			{#if hasMore}
				<div class="flex justify-center pt-4">
					<button onclick={loadMore} disabled={loadingMore} class="btn-ghost px-4 py-2 text-sm">
						{loadingMore ? 'Loading…' : `Load more (${matched - videos.length} left)`}
					</button>
				</div>
			{/if}
		{/if}
	</div>
	{#if footer}{@render footer()}{/if}
</section>
