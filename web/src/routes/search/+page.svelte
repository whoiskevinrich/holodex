<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import type { Person, Tag, Video } from '$lib/types';
	import VideoGrid from '$lib/components/VideoGrid.svelte';

	let videos = $state<Video[]>([]);
	let people = $state<Person[]>([]);
	let tags = $state<Tag[]>([]);
	let loading = $state(true);

	const q = $derived($page.url.searchParams.get('q') ?? '');

	$effect(() => {
		const term = q;
		if (!term) {
			videos = [];
			people = [];
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
				tags = res.tags ?? [];
			})
			.finally(() => (loading = false));
	});

	const empty = $derived(!loading && videos.length === 0 && people.length === 0 && tags.length === 0);
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
