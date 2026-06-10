<script lang="ts">
	import { api } from '$lib/api';
	import type { Tag } from '$lib/types';
	import SortToggle from '$lib/components/SortToggle.svelte';

	let tags = $state<Tag[]>([]);
	let sort = $state<'name' | 'count'>('name');
	let loading = $state(true);

	$effect(() => {
		const by = sort;
		loading = true;
		api
			.listTags(by)
			.then((res) => (tags = res.items ?? []))
			.finally(() => (loading = false));
	});
</script>

<section class="space-y-4">
	<div class="flex items-center justify-between">
		<h1 class="skin-title text-2xl font-semibold text-ink">Tags</h1>
		<SortToggle bind:sort />
	</div>

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if tags.length === 0}
		<p class="py-16 text-center text-sm text-muted">No tags indexed yet.</p>
	{:else}
		<div class="flex flex-wrap gap-2">
			{#each tags as t (t.id)}
				<a href={`/tags/${t.id}`} class="rounded-full border border-rule bg-surface px-3 py-1.5 text-sm text-ink hover:border-accent">
					{t.name} <span class="text-xs text-muted">{t.video_count}</span>
				</a>
			{/each}
		</div>
	{/if}
</section>
