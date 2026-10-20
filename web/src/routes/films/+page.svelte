<script lang="ts">
	import { api } from '#lib/api.ts';
	import { monogram } from '#lib/format.ts';
	import type { Film } from '#lib/types.ts';
	import { shuffleSeed } from '#lib/sortPreference.svelte.ts';
	import { FILM_SORTS, filmsSchema } from '#lib/listState.ts';
	import { listController } from '#lib/listController.svelte.ts';
	import { PagedList, pagedListPage } from '#lib/pagedList.svelte.ts';
	import ListToolbar from '#lib/components/sort/ListToolbar.svelte';
	import SortDropdown from '#lib/components/sort/SortDropdown.svelte';
	import LoadMoreButton from '#lib/components/shared/LoadMoreButton.svelte';

	// Films index (F56, design handoff §1): poster-forward grid, closer to the
	// media-browse density than the People/Studio logo-well rows — a film's default
	// image IS the portrait poster. No A–Z jump bar, no search and no "Most videos" sort;
	// only name/random. Sort state follows the F73 list contract (ADR-114): URL + saved
	// preference, via ListController. One page at a time from the server (F80), Random
	// with the session seed so pages tile (SP2).
	const list = listController(filmsSchema, '/films');
	const sortBy = $derived(list.state.sort);
	const films = new PagedList<Film>((offset, limit) =>
		api.pageFilms({ sort: sortBy, seed: shuffleSeed.value, limit, offset })
	);
	let listEl = $state<HTMLElement>();

	const { loadMore } = pagedListPage({
		id: 'films',
		list: films,
		key: () => list.key,
		restartOn: () => {
			void sortBy;
			if (sortBy === 'random') void shuffleSeed.value;
		},
		root: () => listEl
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
			{#if !films.loading && !(films.error && films.items.length === 0)}{films.total.toLocaleString()}
				{films.total === 1 ? 'film' : 'films'}{/if}
		{/snippet}
	</ListToolbar>

	{#if films.loading && films.items.length === 0}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if films.error && films.items.length === 0}
		<p class="py-16 text-center text-sm text-warn">Couldn’t load films: {films.error}</p>
	{:else if films.items.length === 0}
		<p class="py-16 text-center text-sm text-muted">
			No films yet — attach a video to a film from its media page to get started.
		</p>
	{:else}
		<!-- A failed Load more keeps the loaded grid and says so above it. -->
		{#if films.error}<p class="text-sm text-warn">{films.error}</p>{/if}
		<ul bind:this={listEl} class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
			{#each films.items as f (f.id)}
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
		<LoadMoreButton left={films.left} loading={films.loadingMore} announce={films.announce} onclick={loadMore} />
	{/if}
</section>
