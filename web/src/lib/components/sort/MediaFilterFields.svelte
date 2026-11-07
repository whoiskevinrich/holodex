<script lang="ts">
	// The browse Filters panel's fields: Resolution, Year, Duration and the mapped facets.
	// Browse's list query and the playlist editor's draft both drive them through `set`.
	import MappedFacets from '#lib/components/curation/MappedFacets.svelte';
	import type { MediaQuery } from '#lib/listState.ts';
	import type { Facet, Resolution } from '#lib/types.ts';

	let {
		query,
		set,
		onfacets
	}: {
		query: MediaQuery;
		set: (patch: Partial<MediaQuery>) => void;
		onfacets: (facets: Facet[]) => void;
	} = $props();

	const RESOLUTIONS: Resolution[] = ['All', 'SD', 'HD', 'FHD', '4K'];
	const num = (v: string) => (v === '' ? undefined : Number(v) || undefined);
	const numberInput =
		'w-20 rounded-theme border border-rule bg-bg px-2 py-1.5 text-sm text-ink focus:border-accent focus:outline-none';
</script>

<div>
	<span class="mb-1 block text-xs text-muted">Resolution</span>
	<div class="flex overflow-hidden rounded-theme border border-rule text-sm" role="group" aria-label="Resolution">
		{#each RESOLUTIONS as r (r)}
			<button
				type="button"
				aria-pressed={(query.resolution ?? 'All') === r}
				onclick={() => set({ resolution: r })}
				class="px-3 py-1 {(query.resolution ?? 'All') === r
					? 'bg-accent text-accent-ink'
					: 'text-muted hover:text-ink'}"
			>
				{r}
			</button>
		{/each}
	</div>
</div>
<div>
	<span class="mb-1 block text-xs text-muted">Year</span>
	<div class="flex items-center gap-2">
		<input type="number" aria-label="Year from" placeholder="from" class={numberInput}
			value={query.year_min ?? ''} oninput={(e) => set({ year_min: num(e.currentTarget.value) })} />
		<span class="text-xs text-muted">to</span>
		<input type="number" aria-label="Year to" placeholder="to" class={numberInput}
			value={query.year_max ?? ''} oninput={(e) => set({ year_max: num(e.currentTarget.value) })} />
	</div>
</div>
<div>
	<span class="mb-1 block text-xs text-muted">Duration (min)</span>
	<div class="flex items-center gap-2">
		<input type="number" min="0" aria-label="Minimum duration in minutes" placeholder="min" class={numberInput}
			value={query.duration_min ?? ''} oninput={(e) => set({ duration_min: num(e.currentTarget.value) })} />
		<span class="text-xs text-muted">to</span>
		<input type="number" min="0" aria-label="Maximum duration in minutes" placeholder="max" class={numberInput}
			value={query.duration_max ?? ''} oninput={(e) => set({ duration_max: num(e.currentTarget.value) })} />
	</div>
</div>
<MappedFacets bind:mapped={() => query.mapped ?? {}, (m) => set({ mapped: m })} {onfacets} />
