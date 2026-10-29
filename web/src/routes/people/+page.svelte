<script lang="ts">
	import { untrack } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { api } from '#lib/api.ts';
	import { activity } from '#lib/activity.svelte.ts';
	import { navSearch } from '#lib/navSearch.svelte.ts';
	import type { Person } from '#lib/types.ts';
	import { settledQuery } from '#lib/pagedList.svelte.ts';
	import { SparseList, letterAt } from '#lib/sparseList.svelte.ts';
	import { RovingList, WindowVirtualizer, wholeListPage } from '#lib/windowVirtualizer.svelte.ts';
	import { posterColumns } from '#lib/density.svelte.ts';
	import VirtualRows from '#lib/components/shared/VirtualRows.svelte';
	import ScrollPositionChip from '#lib/components/shared/ScrollPositionChip.svelte';
	import ListToolbar from '#lib/components/sort/ListToolbar.svelte';
	import SortDropdown from '#lib/components/sort/SortDropdown.svelte';
	import PersonAvatar from '#lib/components/person/PersonAvatar.svelte';
	import PersonPosterCard from '#lib/components/person/PersonPosterCard.svelte';
	import ViewToggle from '#lib/components/sort/ViewToggle.svelte';
	import ListTable from '#lib/components/sort/ListTable.svelte';
	import { PEOPLE_COLUMNS } from '#lib/listColumns.ts';
	import MergeCanonicalDialog from '#lib/components/entity/MergeCanonicalDialog.svelte';
	import AlphaIndex from '#lib/components/entity/AlphaIndex.svelte';
	import CompletenessRing from '#lib/components/completeness/CompletenessRing.svelte';
	import DuplicatesBanner from '#lib/components/duplicates/DuplicatesBanner.svelte';
	import SweepStatusLine from '#lib/components/activity/SweepStatusLine.svelte';
	import { shuffleSeed } from '#lib/sortPreference.svelte.ts';
	import { PAGE_VIEWS, readView, writeView, type PersonView } from '#lib/viewPreference.svelte.ts';
	import { ENTITY_SORTS, peopleSchema } from '#lib/listState.ts';
	import { listController } from '#lib/listController.svelte.ts';
	import DensitySlider from '#lib/components/sort/DensitySlider.svelte';

	// People index. Controls are the F73 list toolbar; the sort is one value held by
	// ListController (ADR-114) — completeness is an owner-only entry in it, never a second
	// control, which is what let two sorts show as active at once (HOLODEX-473).
	const list = listController(peopleSchema, '/people');

	const isOwner = $derived(activity.effectiveOwner); // owner AND Owner view on (F29)
	$effect(() => list.setOwner(isOwner));
	const sortBy = $derived(list.state.sort);

	// List/Poster display mode, persisted per page (F55 RD1) — a sibling key to sort's own
	// holodex:sort:people. Only the toggle's change handler writes it (F73 R7).
	let activeView = $state<PersonView>(readView('people'));
	const tableColumns = PEOPLE_COLUMNS.map((c) => ({
		...c,
		cell: { name: nameCell, videos: videosCell, done: doneCell }[c.id]!
	}));

	// NS2: `navSearch.inPlace` is only true while this route is mounted AND the box's
	// tab matches this page's own scope (People) — otherwise it's previewing another
	// type via the overlay panel and this grid stays unfiltered.
	const search = settledQuery(() => (navSearch.inPlace ? navSearch.query : ''));
	const query = $derived(search.value);

	// One list as tall as the whole library, fetched a page at a time where it's viewed
	// (F80 EL11, HOLODEX-601): name, count, completeness, or Random with the session seed so
	// pages tile (SP2, ADR-045). The A–Z index needs the name sort and no search, in every
	// view; its letters come with the first read of each list.
	const people = new SparseList<Person>((offset, limit, fresh) =>
		api.listPeople({
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
	const roving = new RovingList(virt, () => people.total);
	// The List view's columns follow its breakpoints (1 / 2 / 3 at base / sm / lg).
	const sm = new MediaQuery('min-width: 640px');
	const lg = new MediaQuery('min-width: 1024px');
	const listLanes = $derived(lg.current ? 3 : sm.current ? 2 : 1);
	const cols = $derived(posterColumns());
	// One poster column's width for the posters' `sizes` (HOLODEX-332): the viewport less the
	// layout's `<main class="px-6">` gutters (48px) and the grid's `gap-4` gaps, split N ways.
	const posterSizes = $derived(`calc((100vw - ${48 + 16 * (cols - 1)}px) / ${cols})`);
	// The A–Z letter that follows the scroll (EL6): the first entry in view's.
	const activeLetter = $derived(showIndex ? letterAt(virt.firstVisible * virt.lanes, people.letters) : '');
	let barHeight = $state(0);
	$effect(() => virt.setStickyTop(barHeight));

	// A view switch keeps the first entry in view (handoff, "The scroll"). Read it before
	// the old view unmounts (the page collapses and the browser clamps the scroll), and
	// land on it once the new view has measured its rows.
	let shownView = untrack(() => activeView);
	$effect.pre(() => {
		if (activeView === shownView) return;
		shownView = activeView;
		const anchor = untrack(() => virt.firstVisible * virt.lanes);
		if (anchor > 0) virt.landOn(anchor);
	});

	// Merge selection (F23, owner-only): pick 2+ people, then choose the canonical
	// one to fold the rest into (the choose-survivor step lives in MergeCanonicalDialog).
	// See [[ADR-036]]. Entered from the toolbar's ⋯ page-actions menu. The picked people
	// are kept whole, so a selection survives scrolling them far out of view (EL9).
	let selecting = $state(false);
	let selectedPeople = $state<Person[]>([]);
	let choosing = $state(false); // the "Keep which name?" dialog is open
	const selectedIds = $derived(selectedPeople.map((p) => p.id));

	// A sort, search or reshuffle starts the list over at the top; Back restores the entry
	// that was first in view (EL7). The scroll key is the canonical view plus the search
	// (ADR-114 D5).
	const { jumpTo } = wholeListPage({
		id: 'people',
		list: people,
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

	function toggle(p: Person) {
		selectedPeople = selectedIds.includes(p.id)
			? selectedPeople.filter((x) => x.id !== p.id)
			: [...selectedPeople, p];
	}

	function cancelSelect() {
		selecting = false;
		selectedPeople = [];
		choosing = false;
	}

	// Poster view has no select-mode checkbox affordance (F55 RD2) — entering select mode
	// switches to List so the checkbox is visible, rather than leaving the click a dead end
	// or hiding the button entirely.
	function startSelect() {
		activeView = 'list';
		selecting = true;
	}
</script>

<section class="space-y-4">
	<h1 class="skin-title text-2xl font-semibold text-ink">People</h1>

	<ListToolbar
		reroll={sortBy === 'random' ? () => shuffleSeed.reroll() : undefined}
		actions={isOwner && !selecting ? [{ label: 'Merge people…', onselect: startSelect }] : []}
		count={selecting ? selectHint : countLine}
		mode={selecting ? selectBar : undefined}
	>
		{#snippet sort()}
			<SortDropdown compact owner={isOwner} options={ENTITY_SORTS} sort={list.state.sort} onchange={(v) => list.setSort(v)} />
		{/snippet}
		{#snippet view()}
			<ViewToggle options={PAGE_VIEWS.people} bind:view={activeView} onchange={(v) => writeView('people', v)} />
			{#if activeView === 'poster'}
				<DensitySlider />
			{/if}
		{/snippet}
	</ListToolbar>

	{#snippet countLine()}
		{#if !people.loading && !people.error}{people.total.toLocaleString()}
			{people.total === 1 ? 'person' : 'people'}{/if}<DuplicatesBanner
			entityType="person"
			inline
		/>
	{/snippet}

	<!-- Select mode (F73 D2, the owner's pick): the toolbar row becomes the mode bar, and
	     the hint takes the count line's place. -->
	{#snippet selectBar()}
		<span class="min-w-0 flex-1 truncate text-sm text-ink" aria-live="polite">{selectedIds.length} selected</span>
		<button
			onclick={() => (choosing = true)}
			disabled={selectedIds.length < 2}
			class="h-8 shrink-0 rounded-theme bg-accent px-3 text-sm font-semibold text-accent-ink disabled:opacity-60"
		>
			Merge
		</button>
		<button onclick={cancelSelect} class="btn-ghost h-8 shrink-0 px-3 text-sm">Cancel</button>
	{/snippet}
	{#snippet selectHint()}Select two or more people, then choose which name to keep.{/snippet}

	<!-- Entity refresh sweep (F66 RD10): this kind only; reloads once on running->idle.
	     Shown only while a sweep runs or just finished, so it isn't a standing row. -->
	<SweepStatusLine kind="person" onfinished={() => people.refresh()} />

	{#if people.loading && people.total === 0}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if people.error}
		<p class="py-16 text-center text-sm text-warn">Couldn’t load people: {people.error}</p>
	{:else if people.total === 0 && !query}
		<p class="py-16 text-center text-sm text-muted">No people indexed yet.</p>
	{:else if people.total === 0}
		<p class="py-16 text-center text-sm text-muted">No people match “{query}”.</p>
	{:else}
		{#if showIndex}
			<AlphaIndex anchors={people.letters} active={activeLetter} onjump={jumpTo} bind:barHeight />
		{/if}
		<ScrollPositionChip {virt} total={people.total} />
		{#if activeView === 'poster'}
			<VirtualRows
				list={people}
				{virt}
				{roving}
				lanes={cols}
				label="People"
				noun="people"
				rowClass="people-poster-grid grid gap-4 pb-4 {showIndex ? 'pr-5 sm:pr-0' : ''}"
				bind:root={listEl}
			>
				{#snippet entry(p, i)}<PersonPosterCard person={p} eager={i < cols} sizes={posterSizes} />{/snippet}
				{#snippet placeholder()}
					<div>
						<span class="portrait-frame--2x3 poster-card-frame block w-full border border-rule bg-surface-2"></span>
						<span class="block pt-1.5"><span class="block h-5 py-1.5"><span class="block h-full w-2/3 rounded-theme bg-rule"></span></span></span>
						<span class="block pt-0.5"><span class="block h-4 py-1"><span class="block h-full w-4 rounded-theme bg-rule"></span></span></span>
					</div>
				{/snippet}
			</VirtualRows>
		{:else if activeView === 'table' && !selecting}
			<!-- pr-6 keeps the phone A–Z rail clear of the Done ring (handoff, "At 375px"). -->
			<div class={showIndex ? 'pr-6 sm:pr-0' : ''}>
				<ListTable
					columns={tableColumns}
					key={(p) => p.id}
					href={(p) => `/people/${p.id}`}
					sort={sortBy}
					sorts={ENTITY_SORTS}
					owner={isOwner}
					onsort={(s) => list.setSort(s)}
					window={{ list: people, virt, roving, label: 'People', noun: 'people' }}
					bind:root={listEl}
				/>
			</div>
		{:else}
			<!-- Shared row body for both modes — the only difference between select mode and
			     nav mode is the wrapper (checkbox label vs link), so the avatar/name live here
			     once. The trailing ring + count sit OUTSIDE the wrapper (rowTrail): since F65.8
			     the ring is a <button>, which may neither nest in the link nor toggle the
			     checkbox, so the wrapper is a stretched link/label (its ::after covers the
			     row) and the ring rides above the stretch as a sibling. -->
			{#snippet personRow(p: Person, i: number)}
				<PersonAvatar personId={p.id} name={p.display_name ?? p.name} version={p.headshot_version} size="sm" eager={i < 6} />
				<span class="flex-1 truncate">{p.display_name ?? p.name}</span>
			{/snippet}
			{#snippet rowTrail(p: Person)}
				{#if p.completeness}
					<!-- Owner-only by payload (F65.4/F65.5): trailing, before the count, so names
					     don't move between visitor and owner mode. -->
					<span class="relative z-[1] inline-flex">
						<CompletenessRing
							required={p.completeness.required}
							extras={p.completeness.extras}
							size="row"
							entity={{ kind: 'person', id: p.id }}
						/>
					</span>
				{/if}
				<span class="text-xs text-muted">{p.video_count}</span>
			{/snippet}
			<VirtualRows
				list={people}
				{virt}
				{roving}
				lanes={listLanes}
				label="People"
				noun="people"
				rowClass="grid gap-2 pb-2 {showIndex ? 'pr-5 sm:pr-0' : ''}"
				bind:root={listEl}
			>
				{#snippet entry(p, i)}
					{#if selecting}
						<div
							class="relative flex items-center gap-3 rounded-theme border bg-surface px-4 py-2.5 text-ink group-focus-visible:border-accent {selectedIds.includes(
								p.id
							)
								? 'border-accent'
								: 'border-rule hover:border-accent'}"
						>
							<label class="flex min-w-0 flex-1 cursor-pointer items-center gap-3 after:absolute after:inset-0 after:content-['']">
								<input
									type="checkbox"
									class="accent-accent"
									checked={selectedIds.includes(p.id)}
									onchange={() => toggle(p)}
								/>
								{@render personRow(p, i)}
							</label>
							{@render rowTrail(p)}
						</div>
					{:else}
						<div
							class="relative flex items-center gap-3 rounded-theme border border-rule bg-surface px-4 py-2.5 text-ink hover:border-accent group-focus-visible:border-accent"
						>
							<a
								href={`/people/${p.id}`}
								class="flex min-w-0 flex-1 items-center gap-3 after:absolute after:inset-0 after:content-['']"
							>
								{@render personRow(p, i)}
							</a>
							{@render rowTrail(p)}
						</div>
					{/if}
				{/snippet}
				{#snippet placeholder()}
					<!-- The row's own frame, still (handoff, "Placeholders"). -->
					<div class="flex items-center gap-3 rounded-theme border border-rule bg-surface px-4 py-2.5">
						<span class="portrait-frame--1x1 block w-12 shrink-0 border border-rule bg-surface-2"></span>
						<span class="h-2.5 w-2/5 rounded-theme bg-rule"></span>
						<span class="ml-auto h-2.5 w-3.5 rounded-theme bg-rule"></span>
					</div>
				{/snippet}
			</VirtualRows>
		{/if}
	{/if}
</section>

<!-- Table view cells (F82 handoff, "Columns"); the column list is PEOPLE_COLUMNS. -->
{#snippet nameCell(p: Person)}
	<span class="flex min-w-0 items-center gap-3">
		<span class="shrink-0">
			<PersonAvatar personId={p.id} name={p.display_name ?? p.name} version={p.headshot_version} size="xs" />
		</span>
		<span class="truncate">{p.display_name ?? p.name}</span>
	</span>
{/snippet}
{#snippet videosCell(p: Person)}{p.video_count}{/snippet}
{#snippet doneCell(p: Person)}
	{#if p.completeness}
		<CompletenessRing
			required={p.completeness.required}
			extras={p.completeness.extras}
			size="card"
			entity={{ kind: 'person', id: p.id }}
		/>
	{/if}
{/snippet}

<!-- "Keep which name?" — pick the survivor for a multi-select merge, then fold the rest in. -->
{#if choosing}
	<MergeCanonicalDialog
		kind="person"
		items={selectedPeople}
		onclose={() => (choosing = false)}
		onmerged={() => {
			cancelSelect();
			people.refresh();
		}}
	/>
{/if}
