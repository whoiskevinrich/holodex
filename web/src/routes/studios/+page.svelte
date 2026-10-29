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

	// Studio index (F38, ADR-053) — the People list pattern, minus the merge-selection mode
	// (studios have no v1 identity ops). Rows are `StudioListRow` (HOLODEX-432: the
	// StudioLinkCard logo box in front of the name). Controls are the F73 list toolbar; the
	// sort is one value — completeness is an owner-only entry in it, never a second control
	// (HOLODEX-473) — held by ListController per ADR-114.
	const list = listController(studiosSchema, '/studios');

	const isOwner = $derived(activity.effectiveOwner); // owner AND Owner view on (F29)
	$effect(() => list.setOwner(isOwner));
	const sortBy = $derived(list.state.sort);

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
			letters: fresh && sortBy === 'name' && !query
		})
	);
	const showIndex = $derived(sortBy === 'name' && !query);
	let listEl = $state<HTMLElement>();
	const virt = new WindowVirtualizer(78);
	const roving = new RovingList(virt, () => studios.total);
	const sm = new MediaQuery('min-width: 640px');
	const lg = new MediaQuery('min-width: 1024px');
	const lanes = $derived(lg.current ? 3 : sm.current ? 2 : 1);
	const activeLetter = $derived(showIndex ? letterAt(virt.firstVisible * virt.lanes, studios.letters) : '');
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
			if (sortBy === 'random') void shuffleSeed.value;
		},
		root: () => listEl
	});
</script>

<section class="space-y-4">
	<h1 class="skin-title text-2xl font-semibold text-ink">Studios</h1>

	<ListToolbar reroll={sortBy === 'random' ? () => shuffleSeed.reroll() : undefined}>
		{#snippet sort()}
			<SortDropdown compact owner={isOwner} options={ENTITY_SORTS} sort={list.state.sort} onchange={(v) => list.setSort(v)} />
		{/snippet}
		{#snippet count()}
			{#if !studios.loading && !studios.error}{studios.total.toLocaleString()}
				{studios.total === 1 ? 'studio' : 'studios'}{/if}<DuplicatesBanner
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
	{:else if studios.total === 0 && !query}
		<p class="py-16 text-center text-sm text-muted">No studios indexed yet.</p>
	{:else if studios.total === 0}
		<p class="py-16 text-center text-sm text-muted">No studios match “{query}”.</p>
	{:else}
		{#if showIndex}
			<AlphaIndex anchors={studios.letters} active={activeLetter} onjump={jumpTo} bind:barHeight />
		{/if}
		<ScrollPositionChip {virt} total={studios.total} />
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
</section>
