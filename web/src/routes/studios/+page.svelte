<script lang="ts">
	import { tick } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { navSearch } from '$lib/navSearch.svelte';
	import { toMessage, monogram, filterByName } from '$lib/format';
	import { PEOPLE_TAG_SORTS, type PeopleTagSort, type Studio } from '$lib/types';
	import SortToggle from '$lib/components/sort/SortToggle.svelte';
	import SortReroll from '$lib/components/sort/SortReroll.svelte';
	import CompletenessSortToggle from '$lib/components/entity/CompletenessSortToggle.svelte';
	import FacetFilter from '$lib/components/curation/FacetFilter.svelte';
	import DuplicatesBanner from '$lib/components/duplicates/DuplicatesBanner.svelte';
	import { firstLetter, letterAnchors as computeLetterAnchors } from '$lib/peopleNav';
	import { listScroll } from '$lib/listScroll.svelte';
	import { readSort, writeSort, shuffleSeed } from '$lib/sortPreference.svelte';
	import { seededShuffle } from '$lib/shuffle';
	import { createMissingFacetOptions } from '$lib/missingFacetOptions.svelte';

	// Studio index (F38, ADR-053) — the People/Tags list pattern, minus avatars and the
	// merge-selection mode (studios have no headshot and no v1 identity ops). Same sort +
	// A–Z jump-nav + scroll-restore behavior.
	let studios = $state<Studio[]>([]);
	let sort = $state<PeopleTagSort>(readSort('studios', PEOPLE_TAG_SORTS, 'name'));
	let loading = $state(true);
	let loadError = $state('');

	$effect(() => {
		writeSort('studios', sort);
	});

	// NS2: `navSearch.inPlace` is only true while this route is mounted AND the box's
	// tab matches this page's own scope (Studios) — otherwise it's previewing another
	// type via the overlay panel and this grid stays unfiltered.
	const q = $derived(navSearch.inPlace ? navSearch.query : '');

	// Completeness sort (F55.5) — owner-only, a separate control/state from `sort`
	// (see CompletenessSortToggle) since PeopleTagSort is shared with the
	// (out-of-scope) Tags page. Declared here (ahead of `sorted` below) so it's
	// initialized before that derived's first read.
	let completenessDir = $state<'' | 'asc' | 'desc'>('');

	// "Random" shuffles the name-ordered list client-side with the session seed (SP2) —
	// a separate $derived from `displayed` so a keystroke's filter pass doesn't also
	// re-shuffle. NS3: filterByName over the already-fetched list, no new fetch.
	// completenessDir overrides the client-side random shuffle — the server has already
	// ordered `studios` by score in that case.
	const sorted = $derived(!completenessDir && sort === 'random' ? seededShuffle(studios, shuffleSeed.value) : studios);
	const displayed = $derived(filterByName(sorted, q));

	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)

	// Missing-facet filter (F55.6) — owner-only, AND semantics across selections.
	// `missingFacetFetched` is a plain (non-reactive) guard, not `$state` — the facet
	// list can legitimately come back empty ([] length 0), and re-assigning a fresh
	// empty array on every response is itself a $state write, so gating on
	// `.length === 0` would refire the fetch forever and hammer the server.
	let missingFacetIDs = $state<string[]>([]);
	const missingFacet = createMissingFacetOptions('studio');
	$effect(() => {
		missingFacet.ensureFetched(isOwner);
	});
	function effectiveSort(): PeopleTagSort | 'completeness_asc' | 'completeness_desc' {
		return completenessDir ? (`completeness_${completenessDir}` as const) : sort;
	}

	const ALPHABET = '#ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('');
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
		// Owner-gated: never send the completeness sort/filter for a non-owner (a
		// transient pre-capabilities-load isOwner=false just falls back to the plain
		// sort, and self-heals into the real request once caps resolve).
		api
			.listStudios(isOwner ? effectiveSort() : sort, undefined, isOwner ? missingFacetIDs : undefined)
			.then((res) => (studios = res.items ?? []))
			.catch((err) => {
				loadError = toMessage(err);
				studios = [];
			})
			.finally(() => {
				loading = false;
				if (firstLoad) {
					firstLoad = false;
					const snap = listScroll.take('studios', scrollKey());
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

	beforeNavigate(() => {
		listScroll.save('studios', { key: scrollKey(), scrollY: window.scrollY });
	});
</script>

<section class="space-y-4">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<h1 class="skin-title text-2xl font-semibold text-ink">Studios</h1>
		<div class="flex items-center gap-2">
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
		</div>
	</div>

	<DuplicatesBanner entityType="studio" />

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if loadError}
		<p class="py-16 text-center text-sm text-warn">Couldn’t load studios: {loadError}</p>
	{:else if studios.length === 0}
		<p class="py-16 text-center text-sm text-muted">No studios indexed yet.</p>
	{:else if displayed.length === 0}
		<p class="py-16 text-center text-sm text-muted">No studios match “{q.trim()}”.</p>
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
		<ul class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
			{#each displayed as s, i (s.id)}
				<li
					id={sort === 'name' && !q.trim() && letterAnchors[firstLetter(s.name)] === i
						? `sl-${firstLetter(s.name)}`
						: undefined}
					class="scroll-mt-16"
				>
					<a
						href={`/studios/${s.id}`}
						class="flex items-center gap-3 rounded-theme border border-rule bg-surface px-4 py-2.5 text-ink hover:border-accent"
					>
						<!-- Leading icon well (HOLODEX-126, generalized to the icon role by F51/
						     ADR-079): a consistent ~40×26 plate keeps rows aligned whether or not
						     the studio has an icon. Enriched/uploaded → real icon; otherwise a
						     monogram (decorative — the name is adjacent). -->
						<span
							class="flex h-[26px] w-10 shrink-0 items-center justify-center overflow-hidden rounded-theme bg-logo-plate"
						>
							{#if s.icon_url}
								<img
									src={s.icon_url}
									alt={`${s.name} icon`}
									class="h-full w-full object-contain p-0.5"
								/>
							{:else}
								<span class="font-display text-sm font-semibold text-logo-plate-ink" aria-hidden="true"
									>{monogram(s.name)}</span
								>
							{/if}
						</span>
						<span class="flex-1 truncate">{s.name}</span>
						<span class="text-xs text-muted">{s.video_count}</span>
					</a>
				</li>
			{/each}
		</ul>
	{/if}
</section>
