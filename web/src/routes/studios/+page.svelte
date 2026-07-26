<script lang="ts">
	import { tick } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import { api } from '$lib/api';
	import { toMessage, monogram } from '$lib/format';
	import { PEOPLE_TAG_SORTS, type PeopleTagSort, type Studio } from '$lib/types';
	import SortToggle from '$lib/components/sort/SortToggle.svelte';
	import SortReroll from '$lib/components/sort/SortReroll.svelte';
	import DuplicatesBanner from '$lib/components/duplicates/DuplicatesBanner.svelte';
	import { firstLetter, letterAnchors as computeLetterAnchors } from '$lib/peopleNav';
	import { peopleScroll } from '$lib/peopleScroll.svelte';
	import { readSort, writeSort, shuffleSeed } from '$lib/sortPreference.svelte';
	import { seededShuffle } from '$lib/shuffle';

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

	// "Random" shuffles the name-ordered list client-side with the session seed (SP2).
	const displayed = $derived(sort === 'random' ? seededShuffle(studios, shuffleSeed.value) : studios);

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
		api
			.listStudios(sort)
			.then((res) => (studios = res.items ?? []))
			.catch((err) => {
				loadError = toMessage(err);
				studios = [];
			})
			.finally(() => {
				loading = false;
				if (firstLoad) {
					firstLoad = false;
					const snap = peopleScroll.take(`studios:${sort}`);
					if (snap) tick().then(() => window.scrollTo(0, snap.scrollY));
				}
			});
	}

	$effect(() => {
		void sort; // re-run on sort change
		reload();
	});

	beforeNavigate(() => {
		peopleScroll.save({ key: `studios:${sort}`, scrollY: window.scrollY });
	});
</script>

<section class="space-y-4">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<h1 class="skin-title text-2xl font-semibold text-ink">Studios</h1>
		<div class="flex items-center gap-2">
			{#if sort === 'random'}
				<SortReroll onreroll={() => shuffleSeed.reroll()} />
			{/if}
			<SortToggle bind:sort />
		</div>
	</div>

	<DuplicatesBanner entityType="studio" />

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if loadError}
		<p class="py-16 text-center text-sm text-warn">Couldn’t load studios: {loadError}</p>
	{:else if studios.length === 0}
		<p class="py-16 text-center text-sm text-muted">No studios indexed yet.</p>
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
		<ul class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
			{#each displayed as s, i (s.id)}
				<li
					id={sort === 'name' && letterAnchors[firstLetter(s.name)] === i
						? `sl-${firstLetter(s.name)}`
						: undefined}
					class="scroll-mt-16"
				>
					<a
						href={`/studios/${s.id}`}
						class="flex items-center gap-3 rounded-theme border border-rule bg-surface px-4 py-2.5 text-ink hover:border-accent"
					>
						<!-- Leading logo well (HOLODEX-126): a consistent ~40×26 plate keeps rows
						     aligned whether or not the studio has a logo. Enriched → real logo;
						     otherwise a monogram (decorative — the name is adjacent). -->
						<span
							class="flex h-[26px] w-10 shrink-0 items-center justify-center overflow-hidden rounded-theme bg-logo-plate"
						>
							{#if s.logo_url}
								<img
									src={s.logo_url}
									alt={`${s.name} logo`}
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
