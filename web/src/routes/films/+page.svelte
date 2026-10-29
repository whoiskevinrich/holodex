<script lang="ts">
	import { MediaQuery } from 'svelte/reactivity';
	import { api } from '#lib/api.ts';
	import { monogram } from '#lib/format.ts';
	import type { Film } from '#lib/types.ts';
	import { shuffleSeed } from '#lib/sortPreference.svelte.ts';
	import { FILM_SORTS, filmsSchema } from '#lib/listState.ts';
	import { listController } from '#lib/listController.svelte.ts';
	import { SparseList } from '#lib/sparseList.svelte.ts';
	import { RovingList, WindowVirtualizer, wholeListPage } from '#lib/windowVirtualizer.svelte.ts';
	import VirtualRows from '#lib/components/shared/VirtualRows.svelte';
	import ScrollPositionChip from '#lib/components/shared/ScrollPositionChip.svelte';
	import ListToolbar from '#lib/components/sort/ListToolbar.svelte';
	import SortDropdown from '#lib/components/sort/SortDropdown.svelte';

	// Films index (F56, design handoff §1): poster-forward grid, closer to the
	// media-browse density than the People/Studio logo-well rows — a film's default
	// image IS the portrait poster. No A–Z jump bar, no search and no "Most videos" sort;
	// only name/random. Sort state follows the F73 list contract (ADR-114): URL + saved
	// preference, via ListController. The whole-list scroll (F80 EL11/EL14, HOLODEX-604):
	// one list as tall as every film, read a page at a time where it's viewed, Random with
	// the session seed so pages tile (SP2).
	const list = listController(filmsSchema, '/films');
	const sortBy = $derived(list.state.sort);
	const films = new SparseList<Film>((offset, limit) =>
		api.pageFilms({ sort: sortBy, seed: shuffleSeed.value, limit, offset })
	);
	let listEl = $state<HTMLElement>();
	const virt = new WindowVirtualizer(290);
	const roving = new RovingList(virt, () => films.total);
	// The poster grid's columns, as before: 2 / 3 / 4 / 6 at base / sm / md / lg.
	const sm = new MediaQuery('min-width: 640px');
	const md = new MediaQuery('min-width: 768px');
	const lg = new MediaQuery('min-width: 1024px');
	const lanes = $derived(lg.current ? 6 : md.current ? 4 : sm.current ? 3 : 2);

	wholeListPage({
		id: 'films',
		list: films,
		virt,
		roving,
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
			{#if !films.loading && !films.error}{films.total.toLocaleString()}
				{films.total === 1 ? 'film' : 'films'}{/if}
		{/snippet}
	</ListToolbar>

	{#if films.loading && films.total === 0}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if films.error}
		<p class="py-16 text-center text-sm text-warn">Couldn’t load films: {films.error}</p>
	{:else if films.total === 0}
		<p class="py-16 text-center text-sm text-muted">
			No films yet — attach a video to a film from its media page to get started.
		</p>
	{:else}
		<ScrollPositionChip {virt} total={films.total} />
		<VirtualRows list={films} {virt} {roving} {lanes} label="Films" noun="films" rowClass="grid gap-3 pb-3" bind:root={listEl}>
			{#snippet entry(f)}
				<a
					href={`/films/${f.id}`}
					class="block overflow-hidden rounded-theme border border-rule bg-surface hover:border-accent group-focus-visible:border-accent"
				>
					<!-- HOLODEX-318: this rendered the monogram unconditionally, so every film
					     browsed as a lettered plate even with a poster downloaded — the same
					     film showed art on its detail page and a letter here. `poster_url` is
					     populated on the list read too (types.ts), so the monogram is the
					     empty state, not the default. -->
					<span class="flex aspect-[2/3] items-center justify-center overflow-hidden bg-logo-plate">
						{#if f.poster_url}
							<img src={f.poster_url} alt="" loading="lazy" class="h-full w-full object-cover" />
						{:else}
							<span class="font-display text-3xl font-semibold text-logo-plate-ink" aria-hidden="true"
								>{monogram(f.name)}</span
							>
						{/if}
					</span>
					<div class="p-2">
						<p class="truncate text-sm text-ink">{f.name}</p>
						<p class="truncate text-xs text-muted">
							{#if f.year}{f.year} · {/if}{sceneCountLabel(f)}
						</p>
					</div>
				</a>
			{/snippet}
			{#snippet placeholder()}
				<!-- The card's own frame, still (handoff, "Placeholders, continued"). -->
				<div class="overflow-hidden rounded-theme border border-rule bg-surface">
					<span class="block aspect-[2/3] bg-surface-2"></span>
					<div class="p-2">
						<span class="flex h-5 items-center"><span class="h-2.5 w-3/4 rounded-theme bg-rule"></span></span>
						<span class="flex h-4 items-center"><span class="h-2 w-1/2 rounded-theme bg-rule"></span></span>
					</div>
				</div>
			{/snippet}
		</VirtualRows>
	{/if}
</section>
