<script lang="ts">
	import { tick } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import { api } from '$lib/api';
	import { toMessage, monogram } from '$lib/format';
	import type { Film } from '$lib/types';
	import { listScroll } from '$lib/listScroll.svelte';
	import { readSort, writeSort, shuffleSeed } from '$lib/sortPreference.svelte';
	import { seededShuffle } from '$lib/shuffle';
	import { segmentedToggleClass, segmentedToggleWrapperClass } from '$lib/components/sort/segmentedToggle';
	import SortReroll from '$lib/components/sort/SortReroll.svelte';

	// Films index (F56, design handoff §1): poster-forward grid, closer to the
	// media-browse density than the People/Studio logo-well rows — a film's default
	// image IS the portrait poster. No A–Z jump bar and no "Most videos" sort in v1
	// (ListFilms has no server-side count sort); only name/random, same mechanism as
	// Studio's sort-preference persistence + seeded client shuffle.
	type FilmSort = 'name' | 'random';
	let films = $state<Film[]>([]);
	let sort = $state<FilmSort>(readSort('films', ['name', 'random'] as const, 'name'));
	let loading = $state(true);
	let loadError = $state('');

	$effect(() => {
		writeSort('films', sort);
	});

	const displayed = $derived(sort === 'random' ? seededShuffle(films, shuffleSeed.value) : films);

	let firstLoad = true;
	function reload() {
		loading = true;
		loadError = '';
		api
			.listFilms()
			.then((res) => (films = res.items ?? []))
			.catch((err) => {
				loadError = toMessage(err);
				films = [];
			})
			.finally(() => {
				loading = false;
				if (firstLoad) {
					firstLoad = false;
					const snap = listScroll.take('films', sort);
					if (snap) tick().then(() => window.scrollTo(0, snap.scrollY));
				}
			});
	}

	$effect(() => {
		void sort;
		reload();
	});

	beforeNavigate(() => {
		listScroll.save('films', { key: sort, scrollY: window.scrollY });
	});

	function sceneCountLabel(f: Film): string {
		const n = f.video_count ?? 0;
		return `${n} ${n === 1 ? 'scene' : 'scenes'}`;
	}
</script>

<section class="space-y-4">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<h1 class="skin-title text-2xl font-semibold text-ink">Films</h1>
		<div class="flex items-center gap-2">
			{#if sort === 'random'}
				<SortReroll onreroll={() => shuffleSeed.reroll()} />
			{/if}
			<div class={segmentedToggleWrapperClass}>
				<button onclick={() => (sort = 'name')} class={segmentedToggleClass(sort === 'name')}>A–Z</button>
				<button onclick={() => (sort = 'random')} class={segmentedToggleClass(sort === 'random')}
					>Random</button
				>
			</div>
		</div>
	</div>

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if loadError}
		<p class="py-16 text-center text-sm text-warn">Couldn’t load films: {loadError}</p>
	{:else if films.length === 0}
		<p class="py-16 text-center text-sm text-muted">
			No films yet — attach a video to a film from its media page to get started.
		</p>
	{:else}
		<ul class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
			{#each displayed as f (f.id)}
				<li>
					<a
						href={`/films/${f.id}`}
						class="block overflow-hidden rounded-theme border border-rule bg-surface hover:border-accent"
					>
						<span class="flex aspect-[2/3] items-center justify-center bg-logo-plate">
							<span class="font-display text-3xl font-semibold text-logo-plate-ink" aria-hidden="true"
								>{monogram(f.name)}</span
							>
						</span>
						<div class="p-2">
							<p class="truncate text-sm text-ink">{f.name}</p>
							<p class="text-xs text-muted">
								{#if f.year}{f.year} · {/if}{sceneCountLabel(f)}
							</p>
						</div>
					</a>
				</li>
			{/each}
		</ul>
	{/if}
</section>
