<script lang="ts">
	import { api } from '$lib/api';
	import { PEOPLE_TAG_SORTS, type PeopleTagSort, type Tag } from '$lib/types';
	import SortToggle from '$lib/components/SortToggle.svelte';
	import SortReroll from '$lib/components/SortReroll.svelte';
	import { readSort, writeSort, shuffleSeed } from '$lib/sortPreference.svelte';
	import { seededShuffle } from '$lib/shuffle';

	let tags = $state<Tag[]>([]);
	let sort = $state<PeopleTagSort>(readSort('tags', PEOPLE_TAG_SORTS, 'name'));
	let loading = $state(true);

	// Persist the chosen sort per page (SP1).
	$effect(() => {
		writeSort('tags', sort);
	});

	$effect(() => {
		const by = sort;
		loading = true;
		api
			.listTags(by)
			.then((res) => (tags = res.items ?? []))
			.finally(() => (loading = false));
	});

	// "Random" shuffles the name-ordered list client-side with the session seed, so
	// the order holds across re-renders and reshuffles only on reroll/new session.
	const displayed = $derived(sort === 'random' ? seededShuffle(tags, shuffleSeed.value) : tags);
</script>

<section class="space-y-4">
	<div class="flex items-center justify-between">
		<h1 class="skin-title text-2xl font-semibold text-ink">Tags</h1>
		<div class="flex items-center gap-2">
			{#if sort === 'random'}
				<SortReroll onreroll={() => shuffleSeed.reroll()} />
			{/if}
			<SortToggle bind:sort />
		</div>
	</div>

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if tags.length === 0}
		<p class="py-16 text-center text-sm text-muted">No tags indexed yet.</p>
	{:else}
		<div class="flex flex-wrap gap-2">
			{#each displayed as t (t.id)}
				<a href={`/tags/${t.id}`} class="rounded-full border border-rule bg-surface px-3 py-1.5 text-sm text-ink hover:border-accent">
					{t.name} <span class="text-xs text-muted">{t.video_count}</span>
				</a>
			{/each}
		</div>
	{/if}
</section>
