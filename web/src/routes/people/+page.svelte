<script lang="ts">
	import { tick } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { navSearch } from '$lib/navSearch.svelte';
	import { toMessage, filterByName } from '$lib/format';
	import type { Person } from '$lib/types';
	import ListToolbar from '$lib/components/sort/ListToolbar.svelte';
	import SortDropdown from '$lib/components/sort/SortDropdown.svelte';
	import PersonAvatar from '$lib/components/person/PersonAvatar.svelte';
	import PersonPosterGrid from '$lib/components/person/PersonPosterGrid.svelte';
	import PersonViewToggle from '$lib/components/person/PersonViewToggle.svelte';
	import MergeCanonicalDialog from '$lib/components/entity/MergeCanonicalDialog.svelte';
	import AlphaIndex from '$lib/components/entity/AlphaIndex.svelte';
	import CompletenessRing from '$lib/components/completeness/CompletenessRing.svelte';
	import DuplicatesBanner from '$lib/components/duplicates/DuplicatesBanner.svelte';
	import SweepStatusLine from '$lib/components/activity/SweepStatusLine.svelte';
	import { firstLetter, letterAnchors as computeLetterAnchors } from '$lib/peopleNav';
	import { listScroll } from '$lib/listScroll.svelte';
	import { shuffleSeed } from '$lib/sortPreference.svelte';
	import { readView, writeView, type PersonView } from '$lib/viewPreference.svelte';
	import { seededShuffle } from '$lib/shuffle';
	import { ENTITY_SORTS, peopleSchema } from '$lib/listState';
	import { listController } from '$lib/listController.svelte';
	import DensitySlider from '$lib/components/sort/DensitySlider.svelte';

	// People index. Controls are the F73 list toolbar; the sort is one value held by
	// ListController (ADR-114) — completeness is an owner-only entry in it, never a second
	// control, which is what let two sorts show as active at once (HOLODEX-473).
	const list = listController(peopleSchema, '/people');
	let people = $state<Person[]>([]);
	let loading = $state(true);
	let loadError = $state('');

	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)
	$effect(() => list.setOwner(isOwner));
	const sortBy = $derived(list.state.sort);

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

	// "Random" shuffles the name-ordered list client-side with the session seed (SP2,
	// ADR-045) — stable across re-renders, reshuffled only on reroll/new session (kept
	// a separate $derived from `displayed` so a keystroke's filter pass doesn't also
	// re-shuffle). A completeness sort is ordered by the server. The A–Z index stays tied
	// to the name sort with no active search (NS3: filterByName over the fetched list).
	const sorted = $derived(sortBy === 'random' ? seededShuffle(people, shuffleSeed.value) : people);
	const displayed = $derived(filterByName(sorted, q));
	const showIndex = $derived(sortBy === 'name' && !q.trim());

	// Merge selection (F23, owner-only): pick 2+ people, then choose the canonical
	// one to fold the rest into (the choose-survivor step lives in MergeCanonicalDialog).
	// See [[ADR-036]]. Entered from the toolbar's ⋯ page-actions menu.
	let selecting = $state(false);
	let selectedIds = $state<number[]>([]);
	let choosing = $state(false); // the "Keep which name?" dialog is open

	const selectedPeople = $derived(people.filter((p) => selectedIds.includes(p.id)));

	// A–Z jump-navigation (alphabetical sort only). Logic lives in $lib/peopleNav.
	const letterAnchors = $derived(computeLetterAnchors(people.map((p) => p.display_name ?? p.name)));
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
		api
			.listPeople(sortBy)
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
					const snap = listScroll.take('people', list.key);
					if (snap) tick().then(() => window.scrollTo(0, snap.scrollY));
				}
			});
	}

	$effect(() => {
		void sortBy; // re-run on a sort change (the controller already withholds owner-only sorts)
		reload();
	});

	// Stash the scroll offset on the way out (e.g. opening a person) so ← Back restores
	// where the list was. Keyed by the canonical view (ADR-114 D5); a change invalidates it.
	beforeNavigate(() => {
		listScroll.save('people', { key: list.key, scrollY: window.scrollY });
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
			<PersonViewToggle bind:view={activeView} />
			{#if activeView === 'poster'}
				<DensitySlider />
			{/if}
		{/snippet}
	</ListToolbar>

	{#snippet countLine()}
		{#if !loading && !loadError}{people.length} {people.length === 1 ? 'person' : 'people'}{/if}<DuplicatesBanner
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
	<SweepStatusLine kind="person" onfinished={reload} />

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
		{#if showIndex}
			<AlphaIndex anchors={letterAnchors} onjump={jumpTo} />
		{/if}
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
			{#each displayed as p, i (p.id)}
				<li
					id={showIndex && letterAnchors[firstLetter(p.display_name ?? p.name)] === i
						? `pl-${firstLetter(p.display_name ?? p.name)}`
						: undefined}
					class="scroll-mt-16"
				>
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
									onchange={() => toggle(p.id)}
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
