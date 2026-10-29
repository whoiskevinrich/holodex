<script lang="ts">
	import { untrack } from 'svelte';
	import { afterNavigate, beforeNavigate, goto } from '$app/navigation';
	import { api } from '#lib/api.ts';
	import { forgetPlaylist } from '#lib/playlistContext.ts';
	import SavePlaylistForm from '#lib/components/video/SavePlaylistForm.svelte';
	import { activity } from '#lib/activity.svelte.ts';
	import { browseCache } from '#lib/browse.svelte.ts';
	import { navSearch } from '#lib/navSearch.svelte.ts';
	import { DEFAULT_SORT, MEDIA_SORTS, filtersToParams } from '#lib/filters.ts';
	import { filterLabels, loadScopeNames } from '#lib/filterLabels.ts';
	import { beginRun, queryLabel } from '#lib/runContext.ts';
	import type { RunMode } from '#lib/run.ts';
	import PlaySplitButton from '#lib/components/video/PlaySplitButton.svelte';
	import { formatBytes, formatDuration, resolutionBucket, toMessage, videoCount } from '#lib/format.ts';
	import type { Facet, MediaFilters, Resolution, Video } from '#lib/types.ts';
	import VirtualVideoGrid from '#lib/components/video/VirtualVideoGrid.svelte';
	import ScrollPositionChip from '#lib/components/shared/ScrollPositionChip.svelte';
	import { SparseList } from '#lib/sparseList.svelte.ts';
	import { RovingList, WindowVirtualizer, holdAnchorOnViewChange, wholeListPage } from '#lib/windowVirtualizer.svelte.ts';
	import ListToolbar from '#lib/components/sort/ListToolbar.svelte';
	import SortDropdown from '#lib/components/sort/SortDropdown.svelte';
	import FilterPanel from '#lib/components/sort/FilterPanel.svelte';
	import FilterChip from '#lib/components/sort/FilterChip.svelte';
	import RecentlyAddedShelf from '#lib/components/video/RecentlyAddedShelf.svelte';
	import MappedFacets from '#lib/components/curation/MappedFacets.svelte';
	import { shuffleSeed } from '#lib/sortPreference.svelte.ts';
	import DensitySlider from '#lib/components/sort/DensitySlider.svelte';
	import ViewToggle from '#lib/components/sort/ViewToggle.svelte';
	import ListTable from '#lib/components/sort/ListTable.svelte';
	import VideoThumb from '#lib/components/video/VideoThumb.svelte';
	import CompletenessRing from '#lib/components/completeness/CompletenessRing.svelte';
	import { exitAfterRemoval, mediaSchema, type MediaQuery } from '#lib/listState.ts';
	import { listController } from '#lib/listController.svelte.ts';
	import { MEDIA_COLUMNS } from '#lib/listColumns.ts';
	import { shortDate } from '#lib/maintenance.ts';
	import { PAGE_VIEWS, readView, writeView } from '#lib/viewPreference.svelte.ts';

	const RESOLUTIONS: Resolution[] = ['All', 'SD', 'HD', 'FHD', '4K'];

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
	// input anymore) so a "View all N in Videos" deep link (NS1) pre-fills it. Seeded
	// whenever the URL's q changes, not just at init: a same-route nav (`/` → `/?q=…`) and
	// Back/Forward between such entries keep this component, and an unseeded box would
	// mirror its stale text over the arrived q (HOLODEX-404). A derived, so an equal q from
	// the controller replacing its state doesn't re-run it; declared before the mirror below
	// so it runs first on an arrival.
	const urlQ = $derived(list.state.query.q);
	$effect(() => {
		if (urlQ) navSearch.query = urlQ;
	});
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

	const isOwner = $derived(activity.effectiveOwner); // owner AND Owner view on (F29)
	$effect(() => list.setOwner(isOwner));

	// Grid or Table (F82), remembered per page; only the toggle writes it (F73 R7).
	let activeView = $state(readView('media'));
	const tableColumns = MEDIA_COLUMNS.map((c) => ({
		...c,
		cell: { title: titleCell, duration: durationCell, res: resCell, added: addedCell, size: sizeCell, done: doneCell }[
			c.id
		]!
	}));

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

	function currentFilters(): MediaFilters {
		return {
			...query,
			q: q || undefined,
			sort: sortBy,
			// Seed rides the API request (not the shareable URL) so every page tiles under
			// one shuffle (ADR-045). Only sent for the random sort.
			seed: sortBy === 'random' ? shuffleSeed.value : undefined
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
	$effect(() => loadScopeNames(query, scopeNames));

	const activeChips = $derived(
		filterLabels(query, scopeNames, (c) => facets.find((f) => f.canonical === c)?.label).map((l) => ({
			...l,
			remove: () => setQuery(l.clear)
		}))
	);
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
		// SvelteKit 3 hands the initial `enter` navigation a `from` whose url is null.
		editFromPlaylist ||= from?.url?.pathname === `/playlists/${editId}`;
	});
	const cancelEdit = () => exitAfterRemoval(editFromPlaylist, `/playlists/${editId}`);

	// The whole-list scroll (F80 EL11/EL14, HOLODEX-604): the grid is one list as tall as
	// every match, read a page at a time from GET /media where it's viewed. The list reads
	// with `loadFilters`, which follows the filters at once, or 200ms after typing in the
	// search (F4.1), so a page never reads under a query the list hasn't restarted for.
	let loadFilters = $state(untrack(currentFilters));
	const loadQs = $derived(filtersToParams(loadFilters, false).toString());
	$effect(() => {
		const next = currentFilters();
		if (filtersToParams(next, false).toString() === untrack(() => loadQs) && next.seed === untrack(() => loadFilters.seed)) return;
		const t = setTimeout(() => (loadFilters = next), q ? 200 : 0);
		return () => clearTimeout(t);
	});
	const videos = new SparseList<Video>((offset, limit) => api.listMedia({ ...loadFilters, limit, offset }));
	const total = $derived(videos.total);
	const loading = $derived(videos.loading && videos.total === 0);
	let listEl = $state<HTMLElement>();
	const virt = new WindowVirtualizer(240);
	const roving = new RovingList(virt, () => videos.total);

	// The Recently Added shelf's videos: the unfiltered grid's first page, held apart so the
	// shelf above the grid stays put when that page leaves the cache deep in the list. The
	// pages are loadFilters', so their own signature says whether they're unfiltered.
	let recent = $state<Video[]>([]);
	$effect(() => {
		const first = videos.pages.get(0);
		if (first?.status === 'loaded' && loadQs === '') recent = first.items.slice(0, 20);
	});
	// Grid ↔ Table keeps the first video in view (handoff, "The scroll").
	holdAnchorOnViewChange(() => activeView, virt, roving);

	wholeListPage({
		id: 'media',
		list: videos,
		virt,
		roving,
		key: () => loadQs,
		// The seed isn't part of the filter signature, so a reroll restarts through it here.
		restartOn: () => {
			void loadQs;
			void loadFilters.seed;
		},
		root: () => listEl,
		// Back (QW5): the pages that were in view show at once; a write since re-reads them.
		cache: (key) => {
			const held = browseCache.take(key);
			if (held) recent = held.recent;
			return held;
		}
	});

	beforeNavigate(() => browseCache.save({ key: loadQs, recent, ...videos.snapshot() }));

	// Reroll the random shuffle: a new seed restarts the list at its top.
	const rerollMedia = () => shuffleSeed.reroll();

	// Refresh the unfiltered grid when a background scan finishes (running -> idle) so
	// the count + list reflect newly indexed files without a manual reload — fixes the
	// stale count seen during the initial scan. It re-reads in place, keeping the scroll
	// (EL9). The activity feed is owner-gated, so non-owners pick up changes on their next
	// reload (acceptable).
	let prevScanState: string | undefined;
	$effect(() => {
		const s = activity.data?.scan.state;
		if (prevScanState === 'running' && s === 'idle' && !hasFilters && !untrack(() => videos.loading)) {
			void videos.refresh();
		}
		prevScanState = s;
	});

	// Clear every filter, scope and the search text. The sort is a preference and stays.
	function clearAll() {
		navSearch.query = '';
		list.setQuery(mediaSchema.parseQuery(new URLSearchParams()));
	}

	// Keyboard navigation (F12.5): `/` focuses search and Escape clears filters, bound on
	// the window for the lifetime of the page. The arrow keys between cards are the
	// whole-list scroll's roving list now (EL13), which also reaches cards not yet drawn.
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
		// An arrow from outside the grid enters it at its roving stop (F12.5).
		if (e.key.startsWith('Arrow') && !typing && listEl && !listEl.contains(document.activeElement)) {
			const stop = listEl.querySelector<HTMLElement>('[data-index][tabindex="0"]');
			if (stop) {
				e.preventDefault();
				stop.focus();
			}
		}
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
			<ViewToggle options={PAGE_VIEWS.media} bind:view={activeView} onchange={(v) => writeView('media', v)} />
			{#if activeView === 'grid'}
				<DensitySlider />
			{/if}
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
		<RecentlyAddedShelf videos={recent} />
	{/if}

	{#if videos.error}
		<p class="rounded-theme border border-accent bg-surface px-3 py-2 text-sm text-ink">{videos.error}</p>
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
		{#if total === 0 && !loading}
			<p class="py-16 text-center text-sm text-muted">{hasFilters ? 'No videos match these filters.' : 'No videos indexed yet.'}</p>
			{#if hasFilters}
				<div class="flex justify-center">
					<button type="button" onclick={clearAll} class="btn-ghost px-3 py-1.5 text-sm">Clear filters</button>
				</div>
			{/if}
		{:else if total > 0}
			<ScrollPositionChip {virt} total={videos.total} />
			{#if activeView === 'table'}
				<ListTable
					columns={tableColumns}
					key={(v) => v.id}
					href={(v) => `/media/${v.id}`}
					sort={sortBy}
					sorts={MEDIA_SORTS}
					owner={isOwner}
					onsort={(s) => list.setSort(s)}
					window={{ list: videos, virt, roving, label: 'Videos', noun: 'videos' }}
					bind:root={listEl}
				/>
			{:else}
				<VirtualVideoGrid list={videos} {virt} {roving} bind:root={listEl} />
			{/if}
		{/if}
	{/if}
</section>

<!-- Table view cells (F82 handoff, "Columns"); the column list is MEDIA_COLUMNS. -->
{#snippet titleCell(v: Video)}
	<span class="flex min-w-0 items-center gap-3">
		<VideoThumb video={v} />
		<span class="truncate">{v.title}</span>
	</span>
{/snippet}
{#snippet durationCell(v: Video)}{formatDuration(v.duration_sec)}{/snippet}
{#snippet resCell(v: Video)}
	{#if v.width > 0}
		<span class="rounded-theme bg-accent px-1.5 py-0.5 text-[10px] font-semibold text-accent-ink">{resolutionBucket(v.width)}</span>
	{/if}
{/snippet}
{#snippet addedCell(v: Video)}{shortDate(v.indexed_at)}{/snippet}
{#snippet sizeCell(v: Video)}{v.file_size ? formatBytes(v.file_size) : ''}{/snippet}
{#snippet doneCell(v: Video)}
	{#if v.completeness}
		<CompletenessRing
			required={v.completeness.required}
			extras={v.completeness.extras}
			size="card"
			entity={{ kind: 'video', id: v.id }}
		/>
	{/if}
{/snippet}
