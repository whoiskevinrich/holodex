<script lang="ts">
	import { tick } from 'svelte';
	import { page } from '$app/stores';
	import { beforeNavigate } from '$app/navigation';
	import { api } from '$lib/api';
	import { listScroll } from '$lib/listScroll.svelte';
	import type { Person, Studio, Tag, Video } from '$lib/types';
	import VideoGrid from '$lib/components/video/VideoGrid.svelte';

	let videos = $state<Video[]>([]);
	let people = $state<Person[]>([]);
	let studios = $state<Studio[]>([]);
	let tags = $state<Tag[]>([]);
	let loading = $state(true);

	const q = $derived($page.url.searchParams.get('q') ?? '');

	// On the first load only, restore the scroll position stashed when we last left these
	// results (← Back from a person/studio/tag/video), once the re-fetched results have
	// painted (HOLODEX-248, ADR-032). Keyed by `q` itself — the only thing that
	// determines this result set, and (unlike a plain page $state) it survives Back
	// unchanged since it comes straight from the URL. Later reloads (query edited in
	// place) stay put.
	let firstLoad = true;

	$effect(() => {
		const term = q;
		if (!term) {
			videos = [];
			people = [];
			studios = [];
			tags = [];
			loading = false;
			return;
		}
		loading = true;
		api
			.search(term)
			.then((res) => {
				videos = res.videos ?? [];
				people = res.people ?? [];
				studios = res.studios ?? [];
				tags = res.tags ?? [];
			})
			.finally(() => {
				loading = false;
				if (firstLoad) {
					firstLoad = false;
					const snap = listScroll.take('search', term);
					if (snap) tick().then(() => window.scrollTo(0, snap.scrollY));
				}
			});
	});

	// Stash the scroll offset on the way out (e.g. opening a result) so ← Back restores
	// where these results were.
	beforeNavigate(() => {
		listScroll.save('search', { key: q, scrollY: window.scrollY });
	});

	const empty = $derived(
		!loading && videos.length === 0 && people.length === 0 && studios.length === 0 && tags.length === 0
	);
</script>

<section class="space-y-6">
	<h1 class="skin-title text-xl font-semibold text-ink">
		Results for <span class="text-muted">“{q}”</span>
	</h1>

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Searching…</p>
	{:else if empty}
		<p class="py-16 text-center text-sm text-muted">Nothing matched “{q}”.</p>
	{:else}
		{#if people.length}
			<div class="space-y-2">
				<h2 class="text-xs uppercase tracking-wide text-muted">People</h2>
				<div class="flex flex-wrap gap-2">
					{#each people as p (p.id)}
						<a href={`/people/${p.id}`} class="rounded-full border border-rule px-3 py-1 text-sm text-ink hover:border-accent">{p.name}</a>
					{/each}
				</div>
			</div>
		{/if}

		{#if studios.length}
			<div class="space-y-2">
				<h2 class="text-xs uppercase tracking-wide text-muted">Studios</h2>
				<div class="flex flex-wrap gap-2">
					{#each studios as s (s.id)}
						<a href={`/studios/${s.id}`} class="rounded-full border border-rule px-3 py-1 text-sm text-ink hover:border-accent">{s.name}</a>
					{/each}
				</div>
			</div>
		{/if}

		{#if tags.length}
			<div class="space-y-2">
				<h2 class="text-xs uppercase tracking-wide text-muted">Tags</h2>
				<div class="flex flex-wrap gap-2">
					{#each tags as t (t.id)}
						<a href={`/tags/${t.id}`} class="rounded-theme bg-surface-2 px-2.5 py-1 text-sm text-ink hover:text-accent">{t.name}</a>
					{/each}
				</div>
			</div>
		{/if}

		{#if videos.length}
			<div class="space-y-2">
				<h2 class="text-xs uppercase tracking-wide text-muted">Videos</h2>
				<VideoGrid {videos} />
			</div>
		{/if}
	{/if}
</section>
