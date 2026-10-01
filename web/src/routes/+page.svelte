<script lang="ts">
	import { tick } from 'svelte';
	import { afterNavigate, beforeNavigate, goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { forgetPlaylist } from '$lib/playlistContext';
	import SavePlaylistForm from '$lib/components/video/SavePlaylistForm.svelte';
	import { activity } from '$lib/activity.svelte';
	import { browseCache } from '$lib/browse.svelte';
	import { navSearch } from '$lib/navSearch.svelte';
	import { DEFAULT_SORT, filtersToParams } from '$lib/filters';
	import { beginRun, queryLabel } from '$lib/runContext';
	import type { RunMode } from '$lib/run';
	import PlaySplitButton from '$lib/components/video/PlaySplitButton.svelte';
	import { toMessage, videoCount } from '$lib/format';
	import type { Facet, MediaFilters, Resolution, Video } from '$lib/types';
	import VideoGrid from '$lib/components/video/VideoGrid.svelte';
	import ListToolbar from '$lib/components/sort/ListToolbar.svelte';
	import SortDropdown from '$lib/components/sort/SortDropdown.svelte';
	import FilterPanel from '$lib/components/sort/FilterPanel.svelte';
	import FilterChip from '$lib/components/sort/FilterChip.svelte';
	import RecentlyAddedShelf from '$lib/components/video/RecentlyAddedShelf.svelte';
	import MappedFacets from '$lib/components/curation/MappedFacets.svelte';
	import { shuffleSeed } from '$lib/sortPreference.svelte';
	import DensitySlider from '$lib/components/sort/DensitySlider.svelte';
	import { exitAfterRemoval, mediaSchema, type MediaQuery } from '$lib/listState';
	import { listController } from '$lib/listController.svelte';

	const RESOLUTIONS: Resolution[] = ['All', 'SD', 'HD', 'FHD', '4K'];
	const PAGE_SIZE = 50;

	// Sort + filters follow the F73 list contract (ADR-114), held by ListController: the
	// sort is sticky (localStorage) and in the URL; filters and entity scope live in the URL
	// only, so a plain nav click opens the whole library, and Back or a shared link
	// reproduces exactly what was on screen. Every change below goes through `list`.
	const list = listController(mediaSchema, '/');
	const query = $derived(list.state.query);
	const sortBy = $derived(list.state.sort);
	function setQuery(patch: Partial<MediaQuery>) {
		list.setQuery({ ...list.state.query, ...patch });
	}

	// Seeds the shared nav box (not local state, NS4 — there's no page-owned text
	// input anymore) so a "View all N in Videos" deep link (NS1) pre-fills it.
	if (list.state.query.q) navSearch.query = list.state.query.q;
	// NS2: `navSearch.inPlace` is only true while this route is mounted AND the box's
	// tab matches this page's own scope (+layout.svelte owns that match, keyed off
	// the URL) — otherwise the box is previewing another type via the overlay panel
	// and this grid stays unfiltered rather than fighting it.
	const q = $derived(navSearch.inPlace ? navSearch.query : '');
	// The nav box owns the text; mirror it into the list query so the URL carries it. Only
	// while the box is in place: otherwise `q` is '' by definition, and mirroring that would
	// strip a deep link's ?q= before the layout marks the search in-place.
	$effect(() => {
		if (!navSearch.inPlace) return;
		const next = q || undefined;
		if (next !== list.state.query.q) setQuery({ q: next });
	});

	let videos = $state<Video[]>([]);
	let total = $state(0);
	let offset = $state(0);
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state('');

	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)
	$effect(() => list.setOwner(isOwner));

	// "Recently Added" shelf is redundant with the default newest-first sort, so the
	// owner can toggle it off (⋯ menu). Per-browser preference; defaults on.
	const RECENT_KEY = 'holodex:show-recently-added';
	// ssr=false, so localStorage is available at init — seed the saved preference directly
	// instead of true-then-onMount (avoids a show→hide flash).
	let showRecent = $state(localStorage.getItem(RECENT_KEY) !== '0');
	function toggleRecent() {
		showRecent = !showRecent;
		localStorage.setItem(RECENT_KEY, showRecent ? '1' : '0');
	}

	const hasMore = $derived(videos.length < total);

	function currentFilters(): MediaFilters {
		return {
			...query,
			q: q || undefined,
			sort: sortBy,
			// Seed rides the API request (not the shareable URL) so paged "Load more"
			// tiles under one shuffle (ADR-045). Only sent for the random sort.
			seed: sortBy === 'random' ? shuffleSeed.value : undefined,
			limit: PAGE_SIZE
		};
	}

	// Play all / Shuffle (F75): a run over the grid's whole result set in its order. The
	// stored source is the filter alone (ADR-121 D2: sort apart); a random grid passes its
	// seed so the run walks the shuffle on screen, which also makes Play all and Shuffle
	// the same action there (spec RD10).
	// The grid's filter alone (sort apart): a run's source and a smart playlist's query.
	const filterQuery = () => filtersToParams({ ...currentFilters(), sort: undefined }, false).toString();

	function playRun(mode: RunMode): Promise<boolean> {
		const source = filterQuery();
		return beginRun(
			{
				kind: 'query',
				query: source,
				sort: sortBy === DEFAULT_SORT ? undefined : sortBy,
				seed: sortBy === 'random' ? shuffleSeed.value : undefined,
				label: queryLabel(source)
			},
			sortBy === 'random' ? 'in-order' : mode
		);
	}

	// The shareable param set (no paging) doubles as the "any filter active?" check.
	const activeParams = $derived(filtersToParams(currentFilters(), false));
	// The signature as a primitive: the load effect below tracks this, not activeParams,
	// because the controller replaces its state object whenever it re-resolves (owner
	// capabilities arriving, a same-URL arrival). An equal string doesn't propagate, so the
	// effect never re-runs — and never cancels its own pending load — on a no-op change.
	const activeQs = $derived(activeParams.toString());
	const hasFilters = $derived(activeQs !== '');

	// ---- Chips: every active filter and entity scope, removable in one tap (F73 R3/R5) --

	// Mapped-facet labels arrive with the facet list (MappedFacets' onfacets).
	let facets = $state<Facet[]>([]);
	// Entity scope (?person/tag/studio_id/category_id) arrives only from entity-page links,
	// so its names are looked up; an unknown id still gets a removable chip.
	let scopeNames = $state<Record<string, string>>({});
	const SCOPES = [
		{ key: 'person', label: 'Person', load: (id: number) => api.getPerson(id).then((r) => r.person.display_name ?? r.person.name) },
		{ key: 'studio_id', label: 'Studio', load: (id: number) => api.getStudio(id).then((r) => r.studio.name) },
		{ key: 'tag', label: 'Tag', load: (id: number) => api.getTag(id).then((r) => r.tag.name) },
		{ key: 'category', label: 'Category', load: (id: number) => api.getCategory(id).then((r) => r.category.name) }
	] as const;
	$effect(() => {
		for (const s of SCOPES) {
			for (const id of (query[s.key] as number[] | undefined) ?? []) {
				const k = `${s.key}:${id}`;
				if (k in scopeNames) continue;
				scopeNames[k] = '…';
				s.load(id)
					.then((name) => (scopeNames[k] = name))
					.catch(() => (scopeNames[k] = 'unknown'));
			}
		}
	});

	interface Chip {
		/** Stable identity for the keyed each — labels can repeat ("Person: …" while loading). */
		id: string;
		label: string;
		kind: 'filter' | 'scope';
		remove: () => void;
	}
	const range = (a?: number, b?: number, unit = '') =>
		a && b ? `${a}–${b}${unit}` : a ? `≥ ${a}${unit}` : `≤ ${b}${unit}`;
	const activeChips = $derived.by<Chip[]>(() => {
		const out: Chip[] = [];
		for (const s of SCOPES) {
			for (const id of (query[s.key] as number[] | undefined) ?? []) {
				out.push({
					id: `${s.key}:${id}`,
					label: `${s.label}: ${scopeNames[`${s.key}:${id}`] ?? '…'}`,
					kind: 'scope',
					remove: () => setQuery({ [s.key]: ((query[s.key] as number[]) ?? []).filter((x) => x !== id) })
				});
			}
		}
		if (query.resolution && query.resolution !== 'All')
			out.push({ id: 'resolution', label: query.resolution, kind: 'filter', remove: () => setQuery({ resolution: 'All' }) });
		if (query.duration_min || query.duration_max)
			out.push({
				id: 'duration',
				label: `Duration ${range(query.duration_min, query.duration_max, ' min')}`,
				kind: 'filter',
				remove: () => setQuery({ duration_min: undefined, duration_max: undefined })
			});
		if (query.year_min || query.year_max)
			out.push({
				id: 'year',
				label: range(query.year_min, query.year_max),
				kind: 'filter',
				remove: () => setQuery({ year_min: undefined, year_max: undefined })
			});
		for (const [canonical, value] of Object.entries(query.mapped ?? {})) {
			if (!value) continue;
			const label = facets.find((f) => f.canonical === canonical)?.label ?? canonical;
			out.push({
				id: `mapped:${canonical}`,
				label: `${label}: ${value}`,
				kind: 'filter',
				remove: () => setQuery({ mapped: { ...query.mapped, [canonical]: '' } })
			});
		}
		return out;
	});
	// The Filters button counts what its panel holds — not entity scope, which has no field.
	const filterCount = $derived(activeChips.filter((c) => c.kind === 'filter').length);

	const num = (v: string) => (v === '' ? undefined : Number(v) || undefined);

	// Save as playlist… (F75 handoff "Save form"; was F69 P0-8 in the ⋯ menu): the count
	// line's form saves the current filters + sort, smart or snapshot. No filter = the
	// whole library in this sort, a valid smart playlist ("everything, newest first").
	let saveOpen = $state(false);

	// Browse edit mode (F75 handoff §5, P0-8): a smart playlist's *Edit filter* opens
	// `/?<stored query>#edit_playlist=<id>`. The flag is UI state, so it rides the fragment:
	// in the query string the list schema would read it as a mapped filter (a chip, and a
	// key the server refuses on Update). The controller's first URL write drops the
	// fragment, so it's read once here. Update replaces the stored query with the filters
	// on screen (sort apart — it's the playlist's own).
	const editId = Number(new URLSearchParams(location.hash.slice(1)).get('edit_playlist')) || null;
	let editName = $state('');
	let editWas = $state<number | null>(null);
	let editBusy = $state(false);
	let editError = $state('');
	if (editId != null) {
		api
			.getPlaylist(editId, undefined, { limit: 1, offset: 0 })
			.then((res) => {
				editName = res.playlist.name;
				editWas = res.playlist.item_count;
			})
			.catch((err) => (editError = toMessage(err)));
	}
	async function updatePlaylistFilter() {
		if (editId == null || editBusy) return;
		editBusy = true;
		editError = '';
		try {
			await api.updatePlaylist(editId, { query: filterQuery() });
			forgetPlaylist(editId);
			await goto(`/playlists/${editId}`);
		} catch (err) {
			editError = toMessage(err);
		} finally {
			editBusy = false;
		}
	}
	// Cancel returns to the playlist: Back when it's the page we came from, else a plain
	// link (ADR-114 D4, exitAfterRemoval's rule).
	let editFromPlaylist = false;
	afterNavigate(({ from }) => {
		editFromPlaylist ||= from?.url.pathname === `/playlists/${editId}`;
	});
	const cancelEdit = () => exitAfterRemoval(editFromPlaylist, `/playlists/${editId}`);

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
	// The last filter signature we actually loaded, so an effect re-run with no real
	// change (e.g. an equivalent query object) never clobbers a restored page.
	let lastQs: string | null = null;

	// Reading activeQs tracks every filter, so this re-runs on any real change and reloads
	// from page 0 (debounced for the text query, F4.1). The URL is already synced by the
	// list controller.
	$effect(() => {
		const qs = activeQs;

		if (firstLoad) {
			firstLoad = false;
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
			key: activeQs,
			videos,
			total,
			offset,
			scrollY: window.scrollY
		});
	});

	// Clear every filter, scope and the search text. The sort is a preference and stays.
	function clearAll() {
		navSearch.query = '';
		list.setQuery(mediaSchema.parseQuery(new URLSearchParams()));
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
		// (e.g. the nav search panel's own roving-tabindex rows, HOLODEX-249, or the
		// Filters panel's own Escape) — this listener only owns keys nothing else claimed.
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

	const actions = $derived(
		isOwner
			? [{ label: `${showRecent ? 'Hide' : 'Show'} “Recently Added”`, onselect: toggleRecent }]
			: []
	);
	const numberInput =
		'w-20 rounded-theme border border-rule bg-bg px-2 py-1.5 text-sm text-ink focus:border-accent focus:outline-none';
</script>

<section class="space-y-5">
	{#if editId != null && isOwner}
		<!-- Browse edit mode (F75 handoff §5): the filters below are a smart playlist's. -->
		<div class="flex flex-wrap items-center justify-between gap-3 rounded-theme border border-rule bg-surface px-3 py-2 text-sm">
			<p class="flex min-w-0 flex-wrap items-baseline gap-x-2">
				<span class="text-xs uppercase tracking-wide text-muted">Editing filter of</span>
				<span class="max-w-[24ch] truncate text-accent" title={editName}>{editName || '…'}</span>
				{#if !loading}
					<span class="text-muted">
						· {videoCount(total)} match{editWas != null ? ` (was ${editWas.toLocaleString()})` : ''}
					</span>
				{/if}
			</p>
			<div class="flex items-center gap-2">
				<button type="button" onclick={updatePlaylistFilter} disabled={editBusy || !editName} class="btn-accent px-3 py-1.5 text-sm">
					Update “{editName}”
				</button>
				<button type="button" onclick={cancelEdit} disabled={editBusy} class="btn-quiet px-3 py-1.5 text-sm">Cancel</button>
			</div>
			{#if editError}
				<p class="basis-full text-sm text-warn">{editError}</p>
			{/if}
		</div>
	{/if}
	<ListToolbar
		reroll={sortBy === 'random' ? rerollMedia : undefined}
		{actions}
		chipCount={activeChips.length}
		onclear={clearAll}
	>
		{#snippet sort()}
			<SortDropdown compact owner={isOwner} sort={list.state.sort} onchange={(v) => list.setSort(v)} />
		{/snippet}
		{#snippet filters()}
			<FilterPanel count={filterCount} resultLabel={loading ? '' : videoCount(total)} onclear={clearAll}>
				<div>
					<span class="mb-1 block text-xs text-muted">Resolution</span>
					<div class="flex overflow-hidden rounded-theme border border-rule text-sm" role="group" aria-label="Resolution">
						{#each RESOLUTIONS as r (r)}
							<button
								type="button"
								aria-pressed={(query.resolution ?? 'All') === r}
								onclick={() => setQuery({ resolution: r })}
								class="px-3 py-1 {(query.resolution ?? 'All') === r
									? 'bg-accent text-accent-ink'
									: 'text-muted hover:text-ink'}"
							>
								{r}
							</button>
						{/each}
					</div>
				</div>
				<div>
					<span class="mb-1 block text-xs text-muted">Year</span>
					<div class="flex items-center gap-2">
						<input type="number" aria-label="Year from" placeholder="from" class={numberInput}
							value={query.year_min ?? ''} oninput={(e) => setQuery({ year_min: num(e.currentTarget.value) })} />
						<span class="text-xs text-muted">to</span>
						<input type="number" aria-label="Year to" placeholder="to" class={numberInput}
							value={query.year_max ?? ''} oninput={(e) => setQuery({ year_max: num(e.currentTarget.value) })} />
					</div>
				</div>
				<div>
					<span class="mb-1 block text-xs text-muted">Duration (min)</span>
					<div class="flex items-center gap-2">
						<input type="number" min="0" aria-label="Minimum duration in minutes" placeholder="min" class={numberInput}
							value={query.duration_min ?? ''} oninput={(e) => setQuery({ duration_min: num(e.currentTarget.value) })} />
						<span class="text-xs text-muted">to</span>
						<input type="number" min="0" aria-label="Maximum duration in minutes" placeholder="max" class={numberInput}
							value={query.duration_max ?? ''} oninput={(e) => setQuery({ duration_max: num(e.currentTarget.value) })} />
					</div>
				</div>
				<MappedFacets
					bind:mapped={() => query.mapped ?? {}, (m) => setQuery({ mapped: m })}
					onfacets={(f) => (facets = f)}
				/>
			</FilterPanel>
		{/snippet}
		{#snippet view()}
			<DensitySlider />
		{/snippet}
		{#snippet chips()}
			{#each activeChips as c (c.id)}
				<FilterChip label={c.label} kind={c.kind} onremove={c.remove} />
			{/each}
		{/snippet}
	</ListToolbar>

	<!-- Recently Added shelf (F12.3): the default landing view only; hidden once
	     the user filters/sorts so results stay the focus. Sliced from the grid's
	     newest-first page, so it costs no extra request. Below the toolbar (F73), so the
	     toolbar sits in the same place on every list page. -->
	{#if !hasFilters && showRecent}
		<RecentlyAddedShelf {videos} />
	{/if}

	{#if error}
		<p class="rounded-theme border border-accent bg-surface px-3 py-2 text-sm text-ink">{error}</p>
	{:else}
		<!-- Count line (F75 handoff §1): replaces the toolbar's count, so there is one. The
		     total is the query's, never the loaded page length. -->
		<div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
			<p class="text-sm text-muted" aria-live="polite">{loading ? 'Loading…' : videoCount(total)}</p>
			<div class="flex flex-wrap items-center gap-2">
				<PlaySplitButton disabled={loading || total === 0} onplay={playRun} />
				{#if isOwner && editId == null && !saveOpen}
					<button type="button" onclick={() => (saveOpen = true)} class="btn-quiet px-3 py-1.5 text-sm">Save as playlist…</button>
				{/if}
			</div>
		</div>
		{#if saveOpen}
			<SavePlaylistForm
				query={activeQs}
				seed={sortBy === 'random' ? shuffleSeed.value : undefined}
				onclose={() => (saveOpen = false)}
			/>
		{/if}
		<VideoGrid {videos} empty={hasFilters ? 'No videos match these filters.' : 'No videos indexed yet.'} />
		{#if hasFilters && !loading && videos.length === 0}
			<div class="flex justify-center">
				<button type="button" onclick={clearAll} class="btn-ghost px-3 py-1.5 text-sm">Clear filters</button>
			</div>
		{/if}
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
