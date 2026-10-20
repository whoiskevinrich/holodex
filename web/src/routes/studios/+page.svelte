<script lang="ts">
	import { api } from '#lib/api.ts';
	import { activity } from '#lib/activity.svelte.ts';
	import { navSearch } from '#lib/navSearch.svelte.ts';
	import type { Studio } from '#lib/types.ts';
	import { PagedList, pagedListPage, settledQuery } from '#lib/pagedList.svelte.ts';
	import LoadMoreButton from '#lib/components/shared/LoadMoreButton.svelte';
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

	// One page at a time (F80), the People list's paging: Random with the session seed
	// (SP2), the A–Z index on the name sort with no search, letters on each fresh load.
	const studios = new PagedList<Studio>((offset, limit, fresh) =>
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

	const { jumpTo, loadMore } = pagedListPage({
		id: 'studios',
		list: studios,
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
			{#if !studios.loading && !(studios.error && studios.items.length === 0)}{studios.total.toLocaleString()}
				{studios.total === 1 ? 'studio' : 'studios'}{#if studios.letter}
					{' '}· <span class="text-ink">from {studios.letter}</span> ·
					<button type="button" class="text-accent underline" onclick={() => jumpTo('')}>Back to start</button>
				{/if}{/if}<DuplicatesBanner
				entityType="studio"
				inline
			/>
		{/snippet}
	</ListToolbar>

	<!-- Entity refresh sweep (F66 RD10): this kind only; reloads once on running->idle.
	     Shown only while a sweep runs or just finished, so it isn't a standing row. -->
	<SweepStatusLine kind="studio" onfinished={() => studios.refresh()} />

	{#if studios.loading && studios.items.length === 0}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if studios.error && studios.items.length === 0}
		<p class="py-16 text-center text-sm text-warn">Couldn’t load studios: {studios.error}</p>
	{:else if studios.items.length === 0 && !query}
		<p class="py-16 text-center text-sm text-muted">No studios indexed yet.</p>
	{:else if studios.items.length === 0}
		<p class="py-16 text-center text-sm text-muted">No studios match “{query}”.</p>
	{:else}
		<!-- A failed Load more keeps the loaded list and says so above it. -->
		{#if studios.error}<p class="text-sm text-warn">{studios.error}</p>{/if}
		{#if showIndex}
			<AlphaIndex anchors={studios.letters} active={studios.letter} onjump={jumpTo} />
		{/if}
		<ul
			bind:this={listEl}
			class="grid scroll-mt-16 grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3 {showIndex ? 'pr-5 sm:pr-0' : ''}"
		>
			{#each studios.items as s, i (s.id)}
				<li>
					<StudioListRow studio={s} eager={i < 6} />
				</li>
			{/each}
		</ul>
		<LoadMoreButton left={studios.left} loading={studios.loadingMore} announce={studios.announce} onclick={loadMore} />
	{/if}
</section>
