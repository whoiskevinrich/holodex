<script lang="ts">
	import { tick } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { navSearch } from '$lib/navSearch.svelte';
	import { toMessage, filterByName } from '$lib/format';
	import { PEOPLE_TAG_SORTS, type PeopleTagSort, type Person } from '$lib/types';
	import SortToggle from '$lib/components/sort/SortToggle.svelte';
	import SortReroll from '$lib/components/sort/SortReroll.svelte';
	import PersonAvatar from '$lib/components/person/PersonAvatar.svelte';
	import PersonPosterGrid from '$lib/components/person/PersonPosterGrid.svelte';
	import PersonViewToggle from '$lib/components/person/PersonViewToggle.svelte';
	import MergeCanonicalDialog from '$lib/components/entity/MergeCanonicalDialog.svelte';
	import CompletenessSortToggle from '$lib/components/entity/CompletenessSortToggle.svelte';
	import FacetFilter from '$lib/components/curation/FacetFilter.svelte';
	import DuplicatesBanner from '$lib/components/duplicates/DuplicatesBanner.svelte';
	import { firstLetter, letterAnchors as computeLetterAnchors } from '$lib/peopleNav';
	import { listScroll } from '$lib/listScroll.svelte';
	import { readSort, writeSort, shuffleSeed } from '$lib/sortPreference.svelte';
	import { readView, writeView, type PersonView } from '$lib/viewPreference.svelte';
	import { seededShuffle } from '$lib/shuffle';
	import { mediaDensity, DENSITY_MIN, DENSITY_MAX, invertDensity } from '$lib/density.svelte';
	import { createMissingFacetOptions } from '$lib/missingFacetOptions.svelte';

	let people = $state<Person[]>([]);
	let sort = $state<PeopleTagSort>(readSort('people', PEOPLE_TAG_SORTS, 'name'));
	let loading = $state(true);
	let loadError = $state('');

	// Persist the chosen sort per page (SP1).
	$effect(() => {
		writeSort('people', sort);
	});

	// List/Poster display mode, persisted per page (F55 RD1) — a sibling key to sort's own
	// holodex:sort:people, same validated-read/fallback-on-corrupt shape.
	let activeView = $state<PersonView>(readView());
	$effect(() => {
		writeView(activeView);
	});

	// NS2: `navSearch.inPlace` is only true while this route is mounted AND the box's
	// tab matches this page's own scope (People) — otherwise it's previewing another
	// type via the overlay panel and this grid stays unfiltered.
	const q = $derived(navSearch.inPlace ? navSearch.query : '');

	// Completeness sort (F55.5) — owner-only, a separate control/state from `sort`
	// (see CompletenessSortToggle) since PeopleTagSort is shared with the
	// (out-of-scope) Tags page. Declared here (ahead of `sorted` below) so it's
	// initialized before that derived's first read.
	let completenessDir = $state<'' | 'asc' | 'desc'>('');

	// "Random" shuffles the name-ordered list client-side with the session seed (SP2,
	// ADR-045) — stable across re-renders, reshuffled only on reroll/new session (kept
	// a separate $derived from `displayed` so a keystroke's filter pass doesn't also
	// re-shuffle). The A–Z jump-nav stays tied to sort==='name' with no active filter,
	// where `displayed` equals `people` (NS3: filterByName over the already-fetched
	// list, no new fetch).
	// completenessDir overrides the client-side random shuffle — the server has already
	// ordered `people` by score in that case.
	const sorted = $derived(!completenessDir && sort === 'random' ? seededShuffle(people, shuffleSeed.value) : people);
	const displayed = $derived(filterByName(sorted, q));

	// Merge selection (F23, owner-only): pick 2+ people, then choose the canonical
	// one to fold the rest into (the choose-survivor step lives in MergeCanonicalDialog).
	// See [[ADR-036]].
	let selecting = $state(false);
	let selectedIds = $state<number[]>([]);
	let choosing = $state(false); // the "Keep which name?" dialog is open

	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)
	const selectedPeople = $derived(people.filter((p) => selectedIds.includes(p.id)));

	// Missing-facet filter (F55.6) — owner-only, AND semantics across selections.
	// `missingFacetFetched` is a plain (non-reactive) guard, not `$state` — the person
	// facet list can legitimately come back empty ([] length 0), and re-assigning a
	// fresh empty array on every response is itself a $state write, so gating on
	// `.length === 0` would refire the fetch forever and hammer the server.
	let missingFacetIDs = $state<string[]>([]);
	const missingFacet = createMissingFacetOptions('person');
	$effect(() => {
		missingFacet.ensureFetched(isOwner);
	});
	function effectiveSort(): PeopleTagSort | 'completeness_asc' | 'completeness_desc' {
		return completenessDir ? (`completeness_${completenessDir}` as const) : sort;
	}

	// A–Z jump-navigation (alphabetical sort only): a sticky letter bar that scrolls to
	// the first person under each letter. Logic lives in $lib/peopleNav (unit-tested).
	const ALPHABET = '#ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('');
	const letterAnchors = $derived(computeLetterAnchors(people.map((p) => p.name)));
	function jumpTo(letter: string) {
		const el = document.getElementById(`pl-${letter}`);
		if (!el) return;
		const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
		el.scrollIntoView({ behavior: reduce ? 'auto' : 'smooth', block: 'start' });
	}

	// On the first load only, restore the scroll position stashed when we last left the
	// list (← Back from a person), once the re-fetched list has painted. Later reloads
	// (sort change, post-merge) intentionally stay at the top.
	let firstLoad = true;

	function reload() {
		loading = true;
		loadError = '';
		// Owner-gated: never send the completeness sort/filter for a non-owner (a
		// transient pre-capabilities-load isOwner=false just falls back to the plain
		// sort, and self-heals into the real request once caps resolve).
		api
			.listPeople(isOwner ? effectiveSort() : sort, undefined, isOwner ? missingFacetIDs : undefined)
			.then((res) => (people = res.items ?? []))
			.catch((err) => {
				// Surface a failed fetch instead of masking it as the empty "no people" state.
				loadError = toMessage(err);
				people = [];
			})
			.finally(() => {
				loading = false;
				if (firstLoad) {
					firstLoad = false;
					const snap = listScroll.take('people', scrollKey());
					if (snap) tick().then(() => window.scrollTo(0, snap.scrollY));
				}
			});
	}

	function scrollKey(): string {
		return `${isOwner ? effectiveSort() : sort}|${isOwner ? missingFacetIDs.join(',') : ''}`;
	}

	$effect(() => {
		void sort; void completenessDir; void missingFacetIDs; // re-run on any sort/filter change
		reload();
	});

	// Stash the scroll offset on the way out (e.g. opening a person) so ← Back restores
	// where the list was. Keyed by the effective sort/filter; a change invalidates it.
	beforeNavigate(() => {
		listScroll.save('people', { key: scrollKey(), scrollY: window.scrollY });
	});

	function toggle(id: number) {
		selectedIds = selectedIds.includes(id)
			? selectedIds.filter((x) => x !== id)
			: [...selectedIds, id];
	}

	function cancelSelect() {
		selecting = false;
		selectedIds = [];
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
	<div class="flex flex-wrap items-center justify-between gap-2">
		<h1 class="skin-title text-2xl font-semibold text-ink">People</h1>
		<div class="flex items-center gap-2">
			{#if isOwner}
				{#if selecting}
					<button
						onclick={() => (choosing = true)}
						disabled={selectedIds.length < 2}
						class="rounded-theme bg-accent px-3 py-1 text-sm font-semibold text-accent-ink disabled:opacity-60"
					>
						Merge {selectedIds.length || ''} selected
					</button>
					<button
						onclick={cancelSelect}
						class="rounded-theme border border-rule px-3 py-1 text-sm text-ink hover:bg-surface-2"
					>
						Cancel
					</button>
				{:else}
					<button
						onclick={startSelect}
						class="rounded-theme border border-rule px-3 py-1 text-sm text-ink hover:bg-surface-2"
					>
						Merge people…
					</button>
				{/if}
			{/if}
			{#if !completenessDir && sort === 'random'}
				<SortReroll onreroll={() => shuffleSeed.reroll()} />
			{/if}
			<SortToggle bind:sort />
			{#if isOwner}
				<CompletenessSortToggle bind:dir={completenessDir} />
				<FacetFilter
					label="Missing"
					items={missingFacet.options.map((f) => ({ id: f.canonical, name: f.label, video_count: f.missing_count }))}
					bind:selected={missingFacetIDs}
				/>
			{/if}
			<PersonViewToggle bind:view={activeView} />
			{#if activeView === 'poster'}
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
			{/if}
		</div>
	</div>

	<DuplicatesBanner entityType="person" />

	{#if selecting}
		<p class="text-sm text-muted">Select two or more people, then choose which name to keep.</p>
	{/if}

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if loadError}
		<p class="py-16 text-center text-sm text-warn">Couldn’t load people: {loadError}</p>
	{:else if people.length === 0}
		<p class="py-16 text-center text-sm text-muted">No people indexed yet.</p>
	{:else if displayed.length === 0}
		<p class="py-16 text-center text-sm text-muted">No people match “{q.trim()}”.</p>
	{:else if activeView === 'poster'}
		<PersonPosterGrid people={displayed} />
	{:else}
		{#if !completenessDir && sort === 'name' && !q.trim()}
			<nav
				aria-label="Jump to letter"
				class="sticky top-0 z-10 -mx-1 flex flex-wrap gap-0.5 bg-bg/85 px-1 py-1.5 backdrop-blur"
			>
				{#each ALPHABET as L (L)}
					{#if L in letterAnchors}
						<button
							onclick={() => jumpTo(L)}
							aria-label={`Jump to ${L === '#' ? 'non-alphabetic names' : L}`}
							class="rounded-theme px-1.5 py-0.5 text-xs font-medium text-muted hover:bg-surface-2 hover:text-accent"
						>
							{L}
						</button>
					{:else}
						<span class="px-1.5 py-0.5 text-xs text-muted opacity-30" aria-hidden="true">{L}</span>
					{/if}
				{/each}
			</nav>
		{/if}
		<!-- Shared row body for both modes — the only difference between select mode and
		     nav mode is the wrapper (checkbox label vs link), so the avatar/name/count live
		     here once. -->
		{#snippet personRow(p: Person, i: number)}
			<PersonAvatar personId={p.id} name={p.name} version={p.headshot_version} size="sm" eager={i < 6} />
			<span class="flex-1 truncate">{p.name}</span>
			<span class="text-xs text-muted">{p.video_count}</span>
		{/snippet}
		<ul class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
			{#each displayed as p, i (p.id)}
				<li
					id={sort === 'name' && !q.trim() && letterAnchors[firstLetter(p.name)] === i
						? `pl-${firstLetter(p.name)}`
						: undefined}
					class="scroll-mt-16"
				>
					{#if selecting}
						<label
							class="flex cursor-pointer items-center gap-3 rounded-theme border bg-surface px-4 py-2.5 text-ink {selectedIds.includes(
								p.id
							)
								? 'border-accent'
								: 'border-rule hover:border-accent'}"
						>
							<input
								type="checkbox"
								class="accent-accent"
								checked={selectedIds.includes(p.id)}
								onchange={() => toggle(p.id)}
							/>
							{@render personRow(p, i)}
						</label>
					{:else}
						<a
							href={`/people/${p.id}`}
							class="flex items-center gap-3 rounded-theme border border-rule bg-surface px-4 py-2.5 text-ink hover:border-accent"
						>
							{@render personRow(p, i)}
						</a>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</section>

<!-- "Keep which name?" — pick the survivor for a multi-select merge, then fold the rest in. -->
{#if choosing}
	<MergeCanonicalDialog
		kind="person"
		items={selectedPeople}
		onclose={() => (choosing = false)}
		onmerged={() => {
			cancelSelect();
			reload();
		}}
	/>
{/if}
