<script lang="ts" generics="T extends string = SortOrder">
	import type { SortOrder } from '$lib/types';
	import { MEDIA_SORTS } from '$lib/filters';

	// owner gates the ownerOnly entries (F55.5 Completeness sorts) — the server
	// 401s a non-owner request using them, so they must not even render as options.
	// extra prepends page-specific entries (the playlist page's `manual`, F69) so
	// one dropdown reads one MEDIA_SORTS source instead of being forked per page.
	let {
		sort = $bindable(),
		owner = false,
		extra = []
	}: { sort: T; owner?: boolean; extra?: { value: T; label: string }[] } = $props();

	// Options + order come from the single source of truth in filters.ts (F12.1).
	const OPTIONS = $derived([
		...extra,
		...(MEDIA_SORTS.filter((o) => owner || !o.ownerOnly) as { value: T; label: string }[])
	]);
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
