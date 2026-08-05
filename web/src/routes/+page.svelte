<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { beforeNavigate, replaceState } from '$app/navigation';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { browseCache } from '$lib/browse.svelte';
	import { navSearch } from '$lib/navSearch.svelte';
	import { DEFAULT_SORT, SORT_ORDERS, filtersToParams, mappedFromParams, paramsToFilters } from '$lib/filters';
	import { toMessage, videoCount } from '$lib/format';
	import type { MediaFilters, Resolution, SortOrder, Tag, Video } from '$lib/types';
	import VideoGrid from '$lib/components/video/VideoGrid.svelte';
	import FacetFilter from '$lib/components/curation/FacetFilter.svelte';
	import SortDropdown from '$lib/components/sort/SortDropdown.svelte';
	import SortReroll from '$lib/components/sort/SortReroll.svelte';
	import RecentlyAddedShelf from '$lib/components/video/RecentlyAddedShelf.svelte';
	import MappedFacets from '$lib/components/curation/MappedFacets.svelte';
	import { readSort, writeSort, shuffleSeed } from '$lib/sortPreference.svelte';
	import { mediaDensity, DENSITY_MIN, DENSITY_MAX, invertDensity } from '$lib/density.svelte';

	const RESOLUTIONS: Resolution[] = ['All', 'SD', 'HD', 'FHD', '4K'];
	const PAGE_SIZE = 50;

	// Initialize filter state from the URL once, so shared links reproduce it
	// (F4.7). SPA-only (ssr=false), so `location` is always available here.
	const initParams = new URLSearchParams(location.search);
	const init = paramsToFilters(initParams);
	// Seeds the shared nav box (not local state, NS4 — there's no page-owned text
	// input anymore) so a "View all N in Videos" deep link (NS1) pre-fills it.
	if (init.q) navSearch.query = init.q;
	// NS2: `navSearch.inPlace` is only true while this route is mounted AND the box's
	// tab matches this page's own scope (+layout.svelte owns that match, keyed off
	// the URL) — otherwise the box is previewing another type via the overlay panel
	// and this grid stays unfiltered rather than fighting it.
	const q = $derived(navSearch.inPlace ? navSearch.query : '');
	let resolution = $state<Resolution>(init.resolution ?? 'All');
	let durationMin = $state<number | ''>(init.duration_min ?? '');
	let durationMax = $state<number | ''>(init.duration_max ?? '');
	let yearMin = $state<number | ''>(init.year_min ?? '');
	let yearMax = $state<number | ''>(init.year_max ?? '');
	let personIDs = $state<number[]>(init.person ?? []);
	let tagIDs = $state<number[]>(init.tag ?? []);
	let studioIDs = $state<number[]>(init.studio_id ?? []);
	let categoryIDs = $state<number[]>(init.category ?? []);
	// SP1 sort precedence: a sort in the URL (shared/deep link) wins; otherwise the
	// per-page saved preference; otherwise the default. An invalid URL value is
	// ignored so a crafted ?sort=bogus can't wedge the control.
	const urlSort = initParams.get('sort');
	let sort = $state<SortOrder>(
		urlSort && SORT_ORDERS.includes(urlSort as SortOrder)
			? (urlSort as SortOrder)
			: readSort('media', SORT_ORDERS, DEFAULT_SORT)
	);
	// Remember the choice for next visit (SP1). Restoring 'random' re-enters random
	// with a fresh session seed (a new shuffle), per spec.
	$effect(() => {
		writeSort('media', sort);
	});
	let mapped = $state<Record<string, string>>({}); // configurable mapped-field filters (F20.5)

	let videos = $state<Video[]>([]);
	let total = $state(0);
	let offset = $state(0);
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state('');

	// Facet options for the tag autocomplete (F4.2), fetched once. People/Studios/Categories
	// no longer have facet controls on this page — person/studio_id/category still
	// round-trip through the URL/filters for shareable links and the REST/MCP API
	// contract, just without an on-page picker.
	let tagOptions = $state<Tag[]>([]);

	// "Recently Added" shelf is redundant with the default newest-first sort, so the
	// owner can toggle it off. Per-browser preference; defaults on.
	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)
	const RECENT_KEY = 'holodex:show-recently-added';
	// ssr=false (see above), so localStorage is available at init — seed the saved
	// preference directly instead of true-then-onMount (avoids a show→hide flash).
	let showRecent = $state(localStorage.getItem(RECENT_KEY) !== '0');
	function toggleRecent() {
		showRecent = !showRecent;
		localStorage.setItem(RECENT_KEY, showRecent ? '1' : '0');
	}

	onMount(() => {
		api.listTags('count').then((r) => (tagOptions = r.items ?? [])).catch(() => {});
	});

	const hasMore = $derived(videos.length < total);

	function currentFilters(): MediaFilters {
		return {
			q: q || undefined,
			resolution,
			duration_min: durationMin || undefined,
			duration_max: durationMax || undefined,
			year_min: yearMin || undefined,
			year_max: yearMax || undefined,
			person: personIDs,
			tag: tagIDs,
			studio_id: studioIDs,
			category: categoryIDs,
			sort,
			// Seed rides the API request (not the shareable URL) so paged "Load more"
			// tiles under one shuffle (ADR-045). Only sent for the random sort.
			seed: sort === 'random' ? shuffleSeed.value : undefined,
			mapped,
			limit: PAGE_SIZE
		};
	}

	// The shareable param set (no paging) doubles as the "any filter active?" check.
	const activeParams = $derived(filtersToParams(currentFilters(), false));
	const hasFilters = $derived(activeParams.toString() !== '');

	let debounce: ReturnType<typeof setTimeout>;
	// loadPage(true) replaces the grid (filter change); loadPage(false) appends the
	// next page (F3.1 pagination). offset is intentionally not a tracked dep of the
	// re-fetch effect, so paging doesn't re-trigger a full reload.
	async function loadPage(reset: boolean) {
		if (reset) {
			offset = 0;
			loading = true;
		} else {
			loadingMore = true;
		}
		error = '';
		try {
			const res = await api.listMedia({ ...currentFilters(), offset });
			const items = res.items ?? [];
			videos = reset ? items : [...videos, ...items];
			total = res.total;
		} catch (e) {
			error = toMessage(e);
		} finally {
			loading = false;
			loadingMore = false;
		}
	}

	function loadMore() {
		offset += PAGE_SIZE;
		loadPage(false);
	}

	// Reroll the random shuffle: draw a new seed and refetch from page 0. The seed
	// isn't part of the URL/filter signature, so the filter effect won't react —
	// this explicit refetch is the single reload (no double fetch).
	function rerollMedia() {
		shuffleSeed.reroll();
		window.scrollTo(0, 0);
		loadPage(true);
	}

	// Refresh the unfiltered grid when a background scan finishes (running -> idle) so
	// the count + list reflect newly indexed files without a manual reload — fixes the
	// stale count seen during the initial scan. The activity feed is owner-gated, so
	// non-owners pick up changes on their next reload (acceptable).
	let prevScanState: string | undefined;
	$effect(() => {
		const s = activity.data?.scan.state;
		if (prevScanState === 'running' && s === 'idle' && !hasFilters && !loading) {
			loadPage(true);
		}
		prevScanState = s;
	});

	// Restore the grid from the browse cache exactly once, on mount, when returning
	// from a detail page with the same filters (QW4 / ADR-032). Seeds synchronously so
	// the content height is correct and scroll can be restored without a re-fetch flash.
	let firstLoad = true;
	// The last filter signature we actually loaded. Guards against reloading when the
	// effect re-runs but the filters didn't truly change — e.g. MappedFacets loading
	// rewrites `mapped` to an equivalent value (which would otherwise clobber a restored
	// page with a fresh page-0 fetch). Also trims a redundant fetch on every grid mount.
	let lastQs: string | null = null;

	// Reading activeParams tracks every filter var, so this re-runs on any change:
	// sync the URL and reload from page 0 (debounced for the text query, F4.1).
	$effect(() => {
		const qs = activeParams.toString();

		if (firstLoad) {
			firstLoad = false;
			// On mount the URL already reflects the initial filters, so don't touch
			// history here. Try the browse cache first (QW4): if we're returning to the
			// grid with the same filters, seed synchronously and skip the page-0 fetch.
			const cached = browseCache.take(qs);
			if (cached) {
				videos = cached.videos;
				total = cached.total;
				offset = cached.offset;
				loading = false;
				lastQs = qs;
				// Restore scroll once the seeded grid paints (correct height by then).
				tick().then(() => window.scrollTo(0, cached.scrollY));
				return;
			}
		} else if (qs !== lastQs) {
			// Real filter/sort change: sync the URL via SvelteKit's router (not raw
			// history.replaceState, which wipes the router state and breaks back-nav),
			// and show the new result set from the top.
			replaceState(qs ? `/?${qs}` : '/', {});
			window.scrollTo(0, 0);
		}

		if (qs === lastQs) return; // no actual filter change — don't reload
		lastQs = qs;
		clearTimeout(debounce);
		debounce = setTimeout(() => loadPage(true), q ? 200 : 0);
		return () => clearTimeout(debounce);
	});

	// Snapshot the grid (loaded set + paging + scroll) when navigating away, so Back
	// restores it. Filters round-trip through the URL already; this adds only the
	// in-memory grid/scroll cache. A filter change invalidates it via the key.
	beforeNavigate(() => {
		browseCache.save({
			key: activeParams.toString(),
			videos,
			total,
			offset,
			scrollY: window.scrollY
		});
	});

	function clearAll() {
		navSearch.query = '';
		resolution = 'All';
		durationMin = durationMax = yearMin = yearMax = '';
		personIDs = [];
		tagIDs = [];
		studioIDs = [];
		categoryIDs = [];
		sort = DEFAULT_SORT;
		mapped = {};
	}

	// Keyboard navigation (F12.5): `/` focuses search, arrow keys move between grid
	// cards (the cards are <a> links, so Enter follows natively), Escape clears
	// filters. Bound on the window for the lifetime of the page.
	function gridCards(): { grid: Element | null; cards: HTMLElement[] } {
		const grid = document.querySelector('.video-grid');
		const cards = grid ? Array.from(grid.querySelectorAll<HTMLElement>('a[href^="/media/"]')) : [];
		return { grid, cards };
	}

	function onKeydown(e: KeyboardEvent) {
		// Respect a handler closer to the event target that already claimed this key
		// (e.g. the nav search panel's own roving-tabindex rows, HOLODEX-249) — this
		// listener only owns arrow keys when nothing else does.
		if (e.defaultPrevented) return;
		const target = e.target as HTMLElement | null;
		const typing = target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA' || target?.tagName === 'SELECT';

		if (e.key === '/' && !typing) {
			e.preventDefault();
			document.getElementById('global-search-input')?.focus();
			return;
		}
		if (e.key === 'Escape') {
			if (typing) target?.blur();
			if (hasFilters) clearAll();
			return;
		}
		if (!e.key.startsWith('Arrow')) return;

		const { grid, cards } = gridCards();
		if (!grid || cards.length === 0) return;
		const idx = cards.indexOf(document.activeElement as HTMLElement);
		if (idx === -1) {
			if (typing) return; // don't steal arrows from a text field
			e.preventDefault();
			cards[0].focus();
			return;
		}
		e.preventDefault();
		// Column count only matters for vertical movement.
		const cols =
			e.key === 'ArrowDown' || e.key === 'ArrowUp'
				? getComputedStyle(grid).gridTemplateColumns.split(' ').length
				: 1;
		const delta =
			({ ArrowRight: 1, ArrowLeft: -1, ArrowDown: cols, ArrowUp: -cols } as Record<string, number>)[
				e.key
			] ?? 0;
		cards[Math.max(0, Math.min(idx + delta, cards.length - 1))].focus();
	}

	$effect(() => {
		window.addEventListener('keydown', onKeydown);
		return () => window.removeEventListener('keydown', onKeydown);
	});
</script>

<section class="space-y-5">
	<!-- Recently Added shelf (F12.3): the default landing view only; hidden once
	     the user filters/sorts so results stay the focus. Sliced from the grid's
	     newest-first page, so it costs no extra request. -->
	{#if !hasFilters && showRecent}
		<RecentlyAddedShelf {videos} />
	{/if}

	<div class="flex flex-wrap items-end gap-3">
		<div>
			<span class="mb-1 block text-xs text-muted">Resolution</span>
			<div class="flex overflow-hidden rounded-theme border border-rule">
				{#each RESOLUTIONS as r (r)}
					<button
						onclick={() => (resolution = r)}
						class={`px-3 py-2 text-sm ${resolution === r ? 'bg-accent text-accent-ink' : 'bg-surface text-muted hover:text-ink'}`}
					>
						{r}
					</button>
				{/each}
			</div>
		</div>

		<div>
			<span class="mb-1 block text-xs text-muted">Duration (min)</span>
			<div class="flex items-center gap-1">
				<input type="number" min="0" bind:value={durationMin} placeholder="min"
					class="w-20 rounded-theme border border-rule bg-surface px-2 py-2 text-sm text-ink" />
				<span class="text-muted">–</span>
				<input type="number" min="0" bind:value={durationMax} placeholder="max"
					class="w-20 rounded-theme border border-rule bg-surface px-2 py-2 text-sm text-ink" />
			</div>
		</div>

		<div>
			<span class="mb-1 block text-xs text-muted">Year</span>
			<div class="flex items-center gap-1">
				<input type="number" bind:value={yearMin} placeholder="from"
					class="w-20 rounded-theme border border-rule bg-surface px-2 py-2 text-sm text-ink" />
				<span class="text-muted">–</span>
				<input type="number" bind:value={yearMax} placeholder="to"
					class="w-20 rounded-theme border border-rule bg-surface px-2 py-2 text-sm text-ink" />
			</div>
		</div>

		<FacetFilter label="Tags" items={tagOptions} bind:selected={tagIDs} />

		<div class="min-w-[160px]">
			<span class="mb-1 block text-xs text-muted">Density</span>
			<div class="flex items-center gap-2">
				<svg class="h-4 w-4 shrink-0 text-muted" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
					<rect x="3" y="3" width="7" height="7" rx="1" />
					<rect x="14" y="3" width="7" height="7" rx="1" />
					<rect x="3" y="14" width="7" height="7" rx="1" />
					<rect x="14" y="14" width="7" height="7" rx="1" />
				</svg>
				<input
					type="range"
					min={DENSITY_MIN}
					max={DENSITY_MAX}
					step="1"
					aria-label="Grid density"
					value={invertDensity(mediaDensity.value)}
					oninput={(e) => (mediaDensity.value = invertDensity(Number(e.currentTarget.value)))}
					class="accent-accent"
				/>
				<svg class="h-4 w-4 shrink-0 text-muted" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
					<rect x="4" y="4" width="16" height="16" rx="2" />
				</svg>
			</div>
		</div>

		<MappedFacets
			bind:mapped
			onfacets={(facets) =>
				(mapped = {
					...mapped,
					...mappedFromParams(
						new URLSearchParams(location.search),
						facets.map((f) => f.canonical)
					)
				})}
		/>

		<div class="flex items-end gap-2">
			<SortDropdown bind:sort />
			{#if sort === 'random'}
				<SortReroll onreroll={rerollMedia} />
			{/if}
		</div>

		{#if hasFilters}
			<button onclick={clearAll} class="rounded-theme border border-rule px-3 py-2 text-sm text-muted hover:text-ink">
				Clear filters
			</button>
		{/if}
	</div>

	<div class="flex items-center justify-between text-sm text-muted">
		<span>{loading ? 'Loading…' : videoCount(total)}</span>
		{#if isOwner && !hasFilters}
			<button
				onclick={toggleRecent}
				class="rounded-theme border border-rule px-2 py-1 text-xs text-muted hover:text-ink"
			>
				{showRecent ? 'Hide' : 'Show'} “Recently Added”
			</button>
		{/if}
	</div>

	{#if error}
		<p class="rounded-theme border border-accent bg-surface px-3 py-2 text-sm text-ink">{error}</p>
	{:else}
		<VideoGrid {videos} empty={hasFilters ? 'No videos match these filters.' : 'No videos indexed yet.'} />
		{#if hasMore}
			<div class="flex justify-center pt-2">
				<button
					onclick={loadMore}
					disabled={loadingMore}
					class="btn-ghost px-4 py-2 text-sm"
				>
					{loadingMore ? 'Loading…' : `Load more (${total - videos.length} left)`}
				</button>
			</div>
		{/if}
	{/if}
</section>
