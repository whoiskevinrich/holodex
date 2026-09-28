<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import { api } from '$lib/api';
	import { toMessage, monogram } from '$lib/format';
	import type { Film } from '$lib/types';
	import { listScroll } from '$lib/listScroll.svelte';
	import { shuffleSeed } from '$lib/sortPreference.svelte';
	import { seededShuffle } from '$lib/shuffle';
	import { FILM_SORTS, filmsSchema } from '$lib/listState';
	import { listController } from '$lib/listController.svelte';
	import ListToolbar from '$lib/components/sort/ListToolbar.svelte';
	import SortDropdown from '$lib/components/sort/SortDropdown.svelte';

	// Films index (F56, design handoff §1): poster-forward grid, closer to the
	// media-browse density than the People/Studio logo-well rows — a film's default
	// image IS the portrait poster. No A–Z jump bar and no "Most videos" sort in v1
	// (ListFilms has no server-side count sort); only name/random. Sort state follows the
	// F73 list contract (ADR-114): URL + saved preference, via ListController.
	const list = listController(filmsSchema, '/films');
	let films = $state<Film[]>([]);
	let loading = $state(true);
	let loadError = $state('');

	const displayed = $derived(list.state.sort === 'random' ? seededShuffle(films, shuffleSeed.value) : films);

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
					const snap = listScroll.take('films', list.key);
					if (snap) tick().then(() => window.scrollTo(0, snap.scrollY));
				}
			});
	}

	// Sorting is entirely client-derived (`displayed` above) -- this only needs to run
	// once on mount, not on every `sort` change (a $effect reading `sort` would refire
	// reload() and refetch from the server on every A–Z/Random toggle for no reason).
	onMount(reload);

	beforeNavigate(() => {
		listScroll.save('films', { key: list.key, scrollY: window.scrollY });
	});

	function sceneCountLabel(f: Film): string {
		const n = f.video_count ?? 0;
		return `${n} ${n === 1 ? 'scene' : 'scenes'}`;
	}
</script>

<section class="space-y-4">
	<h1 class="skin-title text-2xl font-semibold text-ink">Films</h1>

	<ListToolbar reroll={list.state.sort === 'random' ? () => shuffleSeed.reroll() : undefined}>
		{#snippet sort()}
			<SortDropdown compact options={FILM_SORTS} sort={list.state.sort} onchange={(v) => list.setSort(v)} />
		{/snippet}
		{#snippet count()}
			{#if !loading && !loadError}{films.length} {films.length === 1 ? 'film' : 'films'}{/if}
		{/snippet}
	</ListToolbar>

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
						<!-- HOLODEX-318: this rendered the monogram unconditionally, so every film
						     browsed as a lettered plate even with a poster downloaded — the same
						     film showed art on its detail page and a letter here. `poster_url` is
						     populated on the list read too (types.ts), so the monogram is the
						     empty state, not the default. -->
						<span class="flex aspect-[2/3] items-center justify-center overflow-hidden bg-logo-plate">
							{#if f.poster_url}
								<img
									src={f.poster_url}
									alt=""
									loading="lazy"
									class="h-full w-full object-cover"
								/>
							{:else}
								<span class="font-display text-3xl font-semibold text-logo-plate-ink" aria-hidden="true"
									>{monogram(f.name)}</span
								>
							{/if}
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
