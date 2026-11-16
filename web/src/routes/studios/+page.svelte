<script lang="ts">
	import { MediaQuery } from 'svelte/reactivity';
	import { api } from '#lib/api.ts';
	import { activity } from '#lib/activity.svelte.ts';
	import { navSearch } from '#lib/navSearch.svelte.ts';
	import type { Studio } from '#lib/types.ts';
	import { settledQuery } from '#lib/pagedList.svelte.ts';
	import { SparseList, letterAt } from '#lib/sparseList.svelte.ts';
	import { RovingList, WindowVirtualizer, wholeListPage } from '#lib/windowVirtualizer.svelte.ts';
	import VirtualRows from '#lib/components/shared/VirtualRows.svelte';
	import ScrollPositionChip from '#lib/components/shared/ScrollPositionChip.svelte';
	import ListToolbar from '#lib/components/sort/ListToolbar.svelte';
	import SortDropdown from '#lib/components/sort/SortDropdown.svelte';
	import StudioListRow from '#lib/components/entity/StudioListRow.svelte';
	import AlphaIndex from '#lib/components/entity/AlphaIndex.svelte';
	import DuplicatesBanner from '#lib/components/duplicates/DuplicatesBanner.svelte';
	import SweepStatusLine from '#lib/components/activity/SweepStatusLine.svelte';
	import { shuffleSeed } from '#lib/sortPreference.svelte.ts';
	import { ENTITY_SORTS, studiosSchema } from '#lib/listState.ts';
	import { listController } from '#lib/listController.svelte.ts';
	import ViewToggle from '#lib/components/sort/ViewToggle.svelte';
	import ListTable from '#lib/components/sort/ListTable.svelte';
	import FilterPanel from '#lib/components/sort/FilterPanel.svelte';
	import FilterChip from '#lib/components/sort/FilterChip.svelte';
	import LinkFieldFilters from '#lib/components/sort/LinkFieldFilters.svelte';
	import { EntityFilters } from '#lib/entityFilters.svelte.ts';
	import { STUDIO_COLUMNS } from '#lib/listColumns.ts';
	import { PAGE_VIEWS, readView, writeView, type PageView } from '#lib/viewPreference.svelte.ts';

	// Studio index (F38, ADR-053) — the People list pattern, minus the merge-selection mode
	// (studios have no v1 identity ops). Rows are `StudioListRow` (HOLODEX-432: the
	// StudioLinkCard logo box in front of the name). Controls are the F73 list toolbar; the
	// sort is one value — the enriched sorts are owner-only entries in it, never a second control
	// (HOLODEX-473) — held by ListController per ADR-114.
	const list = listController(studiosSchema, '/studios', () => activity.effectiveOwner);

	const isOwner = $derived(activity.effectiveOwner); // owner AND Owner view on (F29)
	const sortBy = $derived(list.state.sort);
	// The owner's Filters panel (F90): provider link and field conditions, in the URL.
	const ownerFilters = new EntityFilters('studios', () => list.state.query, (q) => list.setQuery(q), () => isOwner);
	// List or Table (F90 handoff, Studios Table view); only the toggle writes it (F73 R7).
	let activeView = $state<PageView<'studios'>>(readView('studios'));
	const tableColumns = STUDIO_COLUMNS.map((c) => ({
		...c,
		cell: { name: nameCell, videos: videosCell, enriched: enrichedCell }[c.id]!
	}));
	const studioCount = (n: number) => `${n.toLocaleString()} ${n === 1 ? 'studio' : 'studios'}`;

	// NS2: `navSearch.inPlace` is only true while this route is mounted AND the box's
	// tab matches this page's own scope (Studios); the search runs on the server (F80 EL5).
	const search = settledQuery(() => (navSearch.inPlace ? navSearch.query : ''));
	const query = $derived(search.value);

	// The People list's whole-list scroll (F80 EL11, HOLODEX-601): one list as tall as the
	// library, fetched a page at a time where it's viewed, Random with the session seed
	// (SP2), the A–Z index on the name sort with no search.
	const studios = new SparseList<Studio>((offset, limit, fresh) =>
		api.listStudios({
			sort: sortBy,
			q: query,
			seed: shuffleSeed.value,
			limit,
			offset,
			letters: fresh && sortBy === 'name' && !query,
			...ownerFilters.query
		})
	);
	const showIndex = $derived(sortBy === 'name' && !query);
	let listEl = $state<HTMLElement>();
	const virt = new WindowVirtualizer(78);
	const roving = new RovingList(virt, () => studios.total);
	const sm = new MediaQuery('min-width: 640px');
	const lg = new MediaQuery('min-width: 1024px');
	const lanes = $derived(lg.current ? 3 : sm.current ? 2 : 1);
	const activeLetter = $derived(showIndex ? letterAt(roving.anchorEntry, studios.letters) : '');
	let barHeight = $state(0);
	$effect(() => virt.setStickyTop(barHeight));

	const { jumpTo } = wholeListPage({
		id: 'studios',
		list: studios,
		virt,
		roving,
		key: () => `${list.key}:${query}`,
		restartOn: () => {
			void sortBy;
			void query;
			void ownerFilters.query;
			if (sortBy === 'random') void shuffleSeed.value;
		},
		root: () => listEl
	});
</script>

<section class="space-y-4">
	<h1 class="skin-title text-2xl font-semibold text-ink">Studios</h1>

	<ListToolbar
		reroll={sortBy === 'random' ? () => shuffleSeed.reroll() : undefined}
		chipCount={ownerFilters.chips.length}
		onclear={ownerFilters.clear}
	>
		{#snippet sort()}
			<SortDropdown compact owner={isOwner} options={ENTITY_SORTS} sort={list.state.sort} onchange={(v) => list.setSort(v)} />
		{/snippet}
		{#snippet filters()}
			{#if isOwner}
				<FilterPanel count={ownerFilters.count} resultLabel={studios.loading ? '' : studioCount(studios.total)} onclear={ownerFilters.clear}>
					<LinkFieldFilters list="studios" link={ownerFilters.link} conds={ownerFilters.conds} onlink={ownerFilters.setLink} onconds={ownerFilters.setConds} />
				</FilterPanel>
			{/if}
		{/snippet}
		{#snippet view()}
			<ViewToggle options={PAGE_VIEWS.studios} bind:view={activeView} onchange={(v) => writeView('studios', v)} />
		{/snippet}
		{#snippet chips()}
			{#each ownerFilters.chips as c (c.id)}
				<FilterChip label={c.label} negated={c.negated} ownerOnly onremove={c.remove} />
			{/each}
		{/snippet}
		{#snippet count()}
			{#if !studios.loading && !studios.error}{studioCount(studios.total)}{/if}<DuplicatesBanner
				entityType="studio"
				inline
			/>
		{/snippet}
	</ListToolbar>

	<!-- Entity refresh sweep (F66 RD10): this kind only; reloads once on running->idle.
	     Shown only while a sweep runs or just finished, so it isn't a standing row. -->
	<SweepStatusLine kind="studio" onfinished={() => studios.refresh()} />

	{#if studios.loading && studios.total === 0}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if studios.error}
		<p class="py-16 text-center text-sm text-warn">Couldn’t load studios: {studios.error}</p>
	{:else if studios.total === 0 && ownerFilters.count}
		<div class="py-16 text-center text-sm text-muted">
			<p>No studios match these filters.</p>
			<button type="button" class="btn-quiet mt-2 px-3 py-1.5 text-sm" onclick={ownerFilters.clear}>Clear filters</button>
		</div>
	{:else if studios.total === 0 && !query}
		<p class="py-16 text-center text-sm text-muted">No studios indexed yet.</p>
	{:else if studios.total === 0}
		<p class="py-16 text-center text-sm text-muted">No studios match “{query}”.</p>
	{:else}
		{#if showIndex}
			<AlphaIndex anchors={studios.letters} active={activeLetter} onjump={jumpTo} bind:barHeight />
		{/if}
		<ScrollPositionChip {virt} total={studios.total} />
		{#if activeView === 'table'}
			<div class={showIndex ? 'pr-6 sm:pr-0' : ''}>
				<ListTable
					columns={tableColumns}
					key={(s) => s.id}
					href={(s) => `/studios/${s.id}`}
					sort={sortBy}
					sorts={ENTITY_SORTS}
					owner={isOwner}
					onsort={(s) => list.setSort(s)}
					window={{ list: studios, virt, roving, label: 'Studios', noun: 'studios' }}
					bind:root={listEl}
				/>
			</div>
		{:else}
		<VirtualRows
			list={studios}
			{virt}
			{roving}
			{lanes}
			label="Studios"
			noun="studios"
			rowClass="grid gap-2 pb-2 {showIndex ? 'pr-5 sm:pr-0' : ''}"
			bind:root={listEl}
		>
			{#snippet entry(s, i)}<StudioListRow studio={s} eager={i < 6} />{/snippet}
			{#snippet placeholder()}
				<!-- The row's own frame, still (handoff, "Placeholders"): StudioLogoBox is h-12. -->
				<div class="flex items-center gap-3 rounded-theme border border-rule bg-surface px-4 py-2.5">
					<span class="block h-12 w-12 shrink-0 border border-rule bg-surface-2"></span>
					<span class="h-2.5 w-2/5 rounded-theme bg-rule"></span>
					<span class="ml-auto h-2.5 w-3.5 rounded-theme bg-rule"></span>
				</div>
			{/snippet}
		</VirtualRows>
		{/if}
	{/if}
</section>

<!-- Table view cells (F90 handoff, Studios Table view); the column list is STUDIO_COLUMNS. -->
{#snippet nameCell(s: Studio)}<span class="truncate">{s.name}</span>{/snippet}
{#snippet videosCell(s: Studio)}{s.video_count}{/snippet}
{#snippet enrichedCell(s: Studio)}<span class="tabular-nums {s.enriched ? 'text-ink' : 'text-muted'}">{s.enriched ?? ''}</span>{/snippet}
