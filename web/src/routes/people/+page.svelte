<script lang="ts">
	import { tick } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { toMessage } from '$lib/format';
	import { PEOPLE_TAG_SORTS, type PeopleTagSort, type Person } from '$lib/types';
	import SortToggle from '$lib/components/sort/SortToggle.svelte';
	import SortReroll from '$lib/components/sort/SortReroll.svelte';
	import PersonAvatar from '$lib/components/person/PersonAvatar.svelte';
	import MergeCanonicalDialog from '$lib/components/entity/MergeCanonicalDialog.svelte';
	import DuplicatesBanner from '$lib/components/duplicates/DuplicatesBanner.svelte';
	import { firstLetter, letterAnchors as computeLetterAnchors } from '$lib/peopleNav';
	import { listScroll } from '$lib/listScroll.svelte';
	import { readSort, writeSort, shuffleSeed } from '$lib/sortPreference.svelte';
	import { seededShuffle } from '$lib/shuffle';

	let people = $state<Person[]>([]);
	let sort = $state<PeopleTagSort>(readSort('people', PEOPLE_TAG_SORTS, 'name'));
	let loading = $state(true);
	let loadError = $state('');

	// Persist the chosen sort per page (SP1).
	$effect(() => {
		writeSort('people', sort);
	});

	// "Random" shuffles the name-ordered list client-side with the session seed (SP2,
	// ADR-045) — stable across re-renders, reshuffled only on reroll/new session. The
	// A–Z jump-nav stays tied to sort==='name', where `displayed` equals `people`.
	const displayed = $derived(sort === 'random' ? seededShuffle(people, shuffleSeed.value) : people);

	// Merge selection (F23, owner-only): pick 2+ people, then choose the canonical
	// one to fold the rest into (the choose-survivor step lives in MergeCanonicalDialog).
	// See [[ADR-036]].
	let selecting = $state(false);
	let selectedIds = $state<number[]>([]);
	let choosing = $state(false); // the "Keep which name?" dialog is open

	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)
	const selectedPeople = $derived(people.filter((p) => selectedIds.includes(p.id)));

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
		api
			.listPeople(sort)
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
					const snap = listScroll.take('people', sort);
					if (snap) tick().then(() => window.scrollTo(0, snap.scrollY));
				}
			});
	}

	$effect(() => {
		void sort; // re-run on sort change
		reload();
	});

	// Stash the scroll offset on the way out (e.g. opening a person) so ← Back restores
	// where the list was. Keyed by sort; a sort change invalidates it (listScroll.take).
	beforeNavigate(() => {
		listScroll.save('people', { key: sort, scrollY: window.scrollY });
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
						onclick={() => (selecting = true)}
						class="rounded-theme border border-rule px-3 py-1 text-sm text-ink hover:bg-surface-2"
					>
						Merge people…
					</button>
				{/if}
			{/if}
			{#if sort === 'random'}
				<SortReroll onreroll={() => shuffleSeed.reroll()} />
			{/if}
			<SortToggle bind:sort />
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
	{:else}
		{#if sort === 'name'}
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
					id={sort === 'name' && letterAnchors[firstLetter(p.name)] === i
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
