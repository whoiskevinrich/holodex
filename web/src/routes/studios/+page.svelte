<script lang="ts">
	import { tick } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { navSearch } from '$lib/navSearch.svelte';
	import { toMessage, filterByName } from '$lib/format';
	import type { Studio } from '$lib/types';
	import ListToolbar from '$lib/components/sort/ListToolbar.svelte';
	import SortDropdown from '$lib/components/sort/SortDropdown.svelte';
	import StudioListRow from '$lib/components/entity/StudioListRow.svelte';
	import AlphaIndex from '$lib/components/entity/AlphaIndex.svelte';
	import DuplicatesBanner from '$lib/components/duplicates/DuplicatesBanner.svelte';
	import SweepStatusLine from '$lib/components/activity/SweepStatusLine.svelte';
	import { firstLetter, letterAnchors as computeLetterAnchors } from '$lib/peopleNav';
	import { listScroll } from '$lib/listScroll.svelte';
	import { shuffleSeed } from '$lib/sortPreference.svelte';
	import { seededShuffle } from '$lib/shuffle';
	import { ENTITY_SORTS, studiosSchema } from '$lib/listState';
	import { listController } from '$lib/listController.svelte';

	// Studio index (F38, ADR-053) — the People list pattern, minus the merge-selection mode
	// (studios have no v1 identity ops). Rows are `StudioListRow` (HOLODEX-432: the
	// StudioLinkCard logo box in front of the name). Controls are the F73 list toolbar; the
	// sort is one value — completeness is an owner-only entry in it, never a second control
	// (HOLODEX-473) — held by ListController per ADR-114.
	const list = listController(studiosSchema, '/studios');
	let studios = $state<Studio[]>([]);
	let loading = $state(true);
	let loadError = $state('');

	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)
	$effect(() => list.setOwner(isOwner));
	const sortBy = $derived(list.state.sort);

	// NS2: `navSearch.inPlace` is only true while this route is mounted AND the box's
	// tab matches this page's own scope (Studios) — otherwise it's previewing another
	// type via the overlay panel and this grid stays unfiltered.
	const q = $derived(navSearch.inPlace ? navSearch.query : '');

	// "Random" shuffles the name-ordered list client-side with the session seed (SP2) —
	// a separate $derived from `displayed` so a keystroke's filter pass doesn't also
	// re-shuffle. NS3: filterByName over the already-fetched list, no new fetch. A
	// completeness sort is ordered by the server.
	const sorted = $derived(sortBy === 'random' ? seededShuffle(studios, shuffleSeed.value) : studios);
	const displayed = $derived(filterByName(sorted, q));
	const showIndex = $derived(sortBy === 'name' && !q.trim());

	const letterAnchors = $derived(computeLetterAnchors(studios.map((s) => s.name)));
	function jumpTo(letter: string) {
		const el = document.getElementById(`sl-${letter}`);
		if (!el) return;
		const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
		el.scrollIntoView({ behavior: reduce ? 'auto' : 'smooth', block: 'start' });
	}

	let firstLoad = true;
	function reload() {
		loading = true;
		loadError = '';
		api
			.listStudios(sortBy)
			.then((res) => (studios = res.items ?? []))
			.catch((err) => {
				loadError = toMessage(err);
				studios = [];
			})
			.finally(() => {
				loading = false;
				if (firstLoad) {
					firstLoad = false;
					const snap = listScroll.take('studios', list.key);
					if (snap) tick().then(() => window.scrollTo(0, snap.scrollY));
				}
			});
	}

	$effect(() => {
		void sortBy; // re-run on a sort change (the controller already withholds owner-only sorts)
		reload();
	});

	beforeNavigate(() => {
		listScroll.save('studios', { key: list.key, scrollY: window.scrollY });
	});
</script>

<section class="space-y-4">
	<h1 class="skin-title text-2xl font-semibold text-ink">Studios</h1>

	<ListToolbar reroll={sortBy === 'random' ? () => shuffleSeed.reroll() : undefined}>
		{#snippet sort()}
			<SortDropdown compact owner={isOwner} options={ENTITY_SORTS} sort={list.state.sort} onchange={(v) => list.setSort(v)} />
		{/snippet}
		{#snippet count()}
			{#if !loading && !loadError}{studios.length} {studios.length === 1 ? 'studio' : 'studios'}{/if}<DuplicatesBanner
				entityType="studio"
				inline
			/>
		{/snippet}
	</ListToolbar>

	<!-- Entity refresh sweep (F66 RD10): this kind only; reloads once on running->idle.
	     Shown only while a sweep runs or just finished, so it isn't a standing row. -->
	<SweepStatusLine kind="studio" onfinished={reload} />

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if loadError}
		<p class="py-16 text-center text-sm text-warn">Couldn’t load studios: {loadError}</p>
	{:else if studios.length === 0}
		<p class="py-16 text-center text-sm text-muted">No studios indexed yet.</p>
	{:else if displayed.length === 0}
		<p class="py-16 text-center text-sm text-muted">No studios match “{q.trim()}”.</p>
	{:else}
		{#if showIndex}
			<AlphaIndex anchors={letterAnchors} onjump={jumpTo} />
		{/if}
		<ul class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3 {showIndex ? 'pr-5 sm:pr-0' : ''}">
			{#each displayed as s, i (s.id)}
				<li
					id={showIndex && letterAnchors[firstLetter(s.name)] === i ? `sl-${firstLetter(s.name)}` : undefined}
					class="scroll-mt-16"
				>
					<StudioListRow studio={s} eager={i < 6} />
				</li>
			{/each}
		</ul>
	{/if}
</section>
