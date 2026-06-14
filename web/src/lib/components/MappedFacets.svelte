<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import type { Facet } from '$lib/types';

	// mapped is a bindable record of canonical -> selected value ('' = Any). The
	// parent seeds it from the URL via onfacets once the canonical names are known
	// (F20.5), keeping URL→state ownership in one place.
	let { mapped = $bindable(), onfacets }: { mapped: Record<string, string>; onfacets?: (facets: Facet[]) => void } = $props();
	let facets = $state<Facet[]>([]);

	onMount(async () => {
		const r = await api.facets().catch(() => ({ facets: [] }));
		facets = r.facets ?? [];
		onfacets?.(facets);
	});

	function pick(canonical: string, value: string) {
		mapped = { ...mapped, [canonical]: value };
	}
</script>

{#each facets as facet (facet.canonical)}
	{#if facet.values.length}
		<div>
			<label class="mb-1 block text-xs text-muted" for={`facet-${facet.canonical}`}>{facet.label}</label>
			<select
				id={`facet-${facet.canonical}`}
				value={mapped[facet.canonical] ?? ''}
				onchange={(e) => pick(facet.canonical, e.currentTarget.value)}
				class="rounded-theme border border-rule bg-surface px-3 py-2 text-sm text-ink outline-none focus:border-accent"
			>
				<option value="">{facet.label}: Any</option>
				{#each facet.values as v (v.value)}
					<option value={v.value}>{v.value} ({v.count})</option>
				{/each}
			</select>
		</div>
	{/if}
{/each}
