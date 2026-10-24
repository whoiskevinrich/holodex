<script lang="ts">
	import { untrack } from 'svelte';
	import { api } from '#lib/api.ts';
	import { activity } from '#lib/activity.svelte.ts';
	import { navSearch } from '#lib/navSearch.svelte.ts';
	import type { Person } from '#lib/types.ts';
	import { PagedList, pagedListPage, settledQuery } from '#lib/pagedList.svelte.ts';
	import LoadMoreButton from '#lib/components/shared/LoadMoreButton.svelte';
	import ListToolbar from '#lib/components/sort/ListToolbar.svelte';
	import SortDropdown from '#lib/components/sort/SortDropdown.svelte';
	import PersonAvatar from '#lib/components/person/PersonAvatar.svelte';
	import PersonPosterGrid from '#lib/components/person/PersonPosterGrid.svelte';
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

	// One page at a time from the server (F80): name, count, completeness, or Random with
	// the session seed so pages tile (SP2, ADR-045). The A–Z index needs the name sort, no
	// search and List view, as before; its letters come with each fresh load.
	const people = new PagedList<Person>((offset, limit, fresh) =>
		api.listPeople({
			sort: sortBy,
			q: query,
			seed: shuffleSeed.value,
			limit,
			offset,
			letters: fresh && sortBy === 'name' && !query
		})
	);
	const showIndex = $derived(sortBy === 'name' && !query && activeView !== 'poster');
	let listEl = $state<HTMLElement>();

	// Merge selection (F23, owner-only): pick 2+ people, then choose the canonical
	// one to fold the rest into (the choose-survivor step lives in MergeCanonicalDialog).
	// See [[ADR-036]]. Entered from the toolbar's ⋯ page-actions menu. The picked people
	// are kept whole, so a selection survives Load more and a letter restart (EL9).
	let selecting = $state(false);
	let selectedPeople = $state<Person[]>([]);
	let choosing = $state(false); // the "Keep which name?" dialog is open
	const selectedIds = $derived(selectedPeople.map((p) => p.id));

	// A sort, search or reshuffle starts the list over; Back restores the run (EL7). The
	// scroll key is the canonical view plus the search (ADR-114 D5).
	const { jumpTo, loadMore } = pagedListPage({
		id: 'people',
		list: people,
		key: () => `${list.key}:${query}`,
		restartOn: () => {
			void sortBy;
			void query;
			if (sortBy === 'random') void shuffleSeed.value;
		},
		root: () => listEl
	});

	// Poster view has no A–Z index, so switching to it drops a letter restart (handoff, A–Z).
	// Table keeps the index as List does (F82 P0-7).
	$effect(() => {
		if (activeView === 'poster' && untrack(() => people.letter)) people.jump('');
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
		{#if !people.loading && !(people.error && people.items.length === 0)}{people.total.toLocaleString()}
			{people.total === 1 ? 'person' : 'people'}{#if people.letter}
				{' '}· <span class="text-ink">from {people.letter}</span> ·
				<button type="button" class="text-accent underline" onclick={() => jumpTo('')}>Back to start</button>
			{/if}{/if}<DuplicatesBanner
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

	{#if people.loading && people.items.length === 0}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if people.error && people.items.length === 0}
		<p class="py-16 text-center text-sm text-warn">Couldn’t load people: {people.error}</p>
	{:else if people.items.length === 0 && !query}
		<p class="py-16 text-center text-sm text-muted">No people indexed yet.</p>
	{:else if people.items.length === 0}
		<p class="py-16 text-center text-sm text-muted">No people match “{query}”.</p>
	{:else}
		<!-- A failed Load more keeps the loaded list and says so above it. -->
		{#if people.error}<p class="text-sm text-warn">{people.error}</p>{/if}
		{#if showIndex}
			<AlphaIndex anchors={people.letters} active={people.letter} onjump={jumpTo} />
		{/if}
		<div bind:this={listEl} class="scroll-mt-16">
		{#if activeView === 'poster'}
			<PersonPosterGrid people={people.items} />
		{:else if activeView === 'table' && !selecting}
			<!-- pr-6 keeps the phone A–Z rail clear of the Done ring (handoff, "At 375px"). -->
			<div class={showIndex ? 'pr-6 sm:pr-0' : ''}>
				<ListTable
					columns={tableColumns}
					items={people.items}
					key={(p) => p.id}
					href={(p) => `/people/${p.id}`}
					sort={sortBy}
					sorts={ENTITY_SORTS}
					owner={isOwner}
					onsort={(s) => list.setSort(s)}
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
		<ul class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3 {showIndex ? 'pr-5 sm:pr-0' : ''}">
			{#each people.items as p, i (p.id)}
				<li>
					{#if selecting}
						<div
							class="relative flex items-center gap-3 rounded-theme border bg-surface px-4 py-2.5 text-ink {selectedIds.includes(
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
							class="relative flex items-center gap-3 rounded-theme border border-rule bg-surface px-4 py-2.5 text-ink hover:border-accent has-[a:focus-visible]:border-accent"
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
				</li>
			{/each}
		</ul>
		{/if}
		</div>
		<LoadMoreButton left={people.left} loading={people.loadingMore} announce={people.announce} onclick={loadMore} />
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
