<script lang="ts">
	import type { SortOrder } from '$lib/types';

	let { sort = $bindable() }: { sort: SortOrder } = $props();

	// Label map drives the <option> list and its order (F12.1).
	const OPTIONS: { value: SortOrder; label: string }[] = [
		{ value: 'added_desc', label: 'Date added — newest' },
		{ value: 'added_asc', label: 'Date added — oldest' },
		{ value: 'title_asc', label: 'Title — A→Z' },
		{ value: 'title_desc', label: 'Title — Z→A' },
		{ value: 'duration_desc', label: 'Duration — longest' },
		{ value: 'duration_asc', label: 'Duration — shortest' },
		{ value: 'resolution_desc', label: 'Resolution — highest' },
		{ value: 'resolution_asc', label: 'Resolution — lowest' }
	];
</script>

<div>
	<label class="mb-1 block text-xs text-muted" for="sort">Sort</label>
	<select
		id="sort"
		bind:value={sort}
		class="rounded-theme border border-rule bg-surface px-3 py-2 text-sm text-ink outline-none focus:border-accent"
	>
		{#each OPTIONS as o (o.value)}
			<option value={o.value}>{o.label}</option>
		{/each}
	</select>
</div>
