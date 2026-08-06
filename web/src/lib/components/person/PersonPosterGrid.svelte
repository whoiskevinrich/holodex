<script lang="ts">
	// Poster-mode grid for the People index (F55) — mirrors VideoGrid's density→column
	// computation, doubled (RD8): a 2:3 poster reads fine at roughly half a 16:9 thumbnail's
	// width, so People shows ~2x the columns Videos does at the same density setting. The cap
	// is derived from viewportTierCap rather than a second hand-maintained tier table, so it
	// can't drift out of the stated 2:1 ratio if density.svelte.ts's TIERS ever change.
	import type { Person } from '$lib/types';
	import PersonPosterCard from './PersonPosterCard.svelte';
	import { mediaDensity, viewportTierCap } from '$lib/density.svelte';

	let { people, empty = 'No people.' }: { people: Person[]; empty?: string } = $props();

	const cols = $derived(Math.min(mediaDensity.value, viewportTierCap.value) * 2);
</script>

{#if people.length === 0}
	<p class="py-16 text-center text-sm text-muted">{empty}</p>
{:else}
	<div
		class="people-poster-grid grid gap-4"
		style={`grid-template-columns: repeat(${cols}, minmax(0, 1fr))`}
	>
		{#each people as person, i (person.id)}
			<PersonPosterCard {person} eager={i < 12} />
		{/each}
	</div>
{/if}
