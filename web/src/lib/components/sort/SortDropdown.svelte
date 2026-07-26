<script lang="ts">
	import type { SortOrder } from '$lib/types';
	import { MEDIA_SORTS } from '$lib/filters';

	let { sort = $bindable() }: { sort: SortOrder } = $props();

	// Options + order come from the single source of truth in filters.ts (F12.1).
	const OPTIONS = MEDIA_SORTS;
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
