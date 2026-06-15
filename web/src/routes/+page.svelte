<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { beforeNavigate, replaceState } from '$app/navigation';
	import { api } from '$lib/api';
	import { browseCache } from '$lib/browse.svelte';
	import { DEFAULT_SORT, filtersToParams, mappedFromParams, paramsToFilters } from '$lib/filters';
	import { toMessage, videoCount } from '$lib/format';
	import type { MediaFilters, Person, Resolution, SortOrder, Tag, Video } from '$lib/types';
	import VideoGrid from '$lib/components/VideoGrid.svelte';
	import FacetFilter from '$lib/components/FacetFilter.svelte';
	import SortDropdown from '$lib/components/SortDropdown.svelte';
	import RecentlyAddedShelf from '$lib/components/RecentlyAddedShelf.svelte';
	import MappedFacets from '$lib/components/MappedFacets.svelte';

	const RESOLUTIONS: Resolution[] = ['All', 'SD', 'HD', 'FHD', '4K'];
	const PAGE_SIZE = 50;

	// Initialize filter state from the URL once, so shared links reproduce it
	// (F4.7). SPA-only (ssr=false), so `location` is always available here.
	const init = paramsToFilters(new URLSearchParams(location.search));
	let q = $state(init.q ?? '');
	let resolution = $state<Resolution>(init.resolution ?? 'All');
	let durationMin = $state<number | ''>(init.duration_min ?? '');
	let durationMax = $state<number | ''>(init.duration_max ?? '');
	let yearMin = $state<number | ''>(init.year_min ?? '');
	let yearMax = $state<number | ''>(init.year_max ?? '');
	let personIDs = $state<number[]>(init.person ?? []);
	let tagIDs = $state<number[]>(init.tag ?? []);
	let sort = $state<SortOrder>(init.sort ?? DEFAULT_SORT);
	let mapped = $state<Record<string, string>>({}); // configurable mapped-field filters (F20.5)

	let videos = $state<Video[]>([]);
	let total = $state(0);
	let offset = $state(0);
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state('');

	// Facet options for the people/tag autocomplete (F4.2/F4.3), fetched once.
	let peopleOptions = $state<Person[]>([]);
	let tagOptions = $state<Tag[]>([]);
	onMount(() => {
		api.listPeople('count').then((r) => (peopleOptions = r.items ?? [])).catch(() => {});
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
			sort,
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
	// in-memory grid/scroll cache. A filter change invalidates it via the signature.
	beforeNavigate(() => {
		browseCache.save({
			signature: activeParams.toString(),
			videos,
			total,
			offset,
			scrollY: window.scrollY
		});
	});

	function clearAll() {
		q = '';
		resolution = 'All';
		durationMin = durationMax = yearMin = yearMax = '';
		personIDs = [];
		tagIDs = [];
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
		const target = e.target as HTMLElement | null;
		const typing = target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA' || target?.tagName === 'SELECT';

		if (e.key === '/' && !typing) {
			e.preventDefault();
			document.getElementById('q')?.focus();
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
	{#if !hasFilters}
		<RecentlyAddedShelf {videos} />
	{/if}

	<div class="flex flex-wrap items-end gap-3">
		<div class="min-w-[12rem] flex-1">
			<label class="mb-1 block text-xs text-muted" for="q">Search title</label>
			<input
				id="q"
				bind:value={q}
				placeholder="Type to search…"
				class="w-full rounded-theme border border-rule bg-surface px-3 py-2 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
			/>
		</div>

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

		<FacetFilter label="People" items={peopleOptions} bind:selected={personIDs} />
		<FacetFilter label="Tags" items={tagOptions} bind:selected={tagIDs} />

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

		<SortDropdown bind:sort />

		{#if hasFilters}
			<button onclick={clearAll} class="rounded-theme border border-rule px-3 py-2 text-sm text-muted hover:text-ink">
				Clear filters
			</button>
		{/if}
	</div>

	<div class="flex items-center justify-between text-sm text-muted">
		<span>{loading ? 'Loading…' : videoCount(total)}</span>
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
					class="rounded-theme border border-rule px-4 py-2 text-sm text-muted hover:text-ink disabled:opacity-50"
				>
					{loadingMore ? 'Loading…' : `Load more (${total - videos.length} left)`}
				</button>
			</div>
		{/if}
	{/if}
</section>
