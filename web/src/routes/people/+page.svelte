<script lang="ts">
	import { api } from '$lib/api';
	import type { Person } from '$lib/types';
	import SortToggle from '$lib/components/SortToggle.svelte';

	let people = $state<Person[]>([]);
	let sort = $state<'name' | 'count'>('name');
	let loading = $state(true);

	$effect(() => {
		const by = sort;
		loading = true;
		api
			.listPeople(by)
			.then((res) => (people = res.items ?? []))
			.finally(() => (loading = false));
	});
</script>

<section class="space-y-4">
	<div class="flex items-center justify-between">
		<h1 class="skin-title text-2xl font-semibold text-ink">People</h1>
		<SortToggle bind:sort />
	</div>

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if people.length === 0}
		<p class="py-16 text-center text-sm text-muted">No people indexed yet.</p>
	{:else}
		<ul class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
			{#each people as p (p.id)}
				<li>
					<a href={`/people/${p.id}`} class="flex items-center justify-between rounded-theme border border-rule bg-surface px-4 py-2.5 text-ink hover:border-accent">
						<span>{p.name}</span>
						<span class="text-xs text-muted">{p.video_count}</span>
					</a>
				</li>
			{/each}
		</ul>
	{/if}
</section>
