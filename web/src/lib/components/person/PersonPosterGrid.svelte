<script lang="ts">
	// Poster-mode grid for the People index (F55). The 2:1 ratio against the video grid (RD8 —
	// a 2:3 poster reads fine at roughly half a 16:9 thumbnail's width) lives in
	// density.svelte.ts as `posterColumns`, next to the tier rungs it doubles, so the two are
	// tuned and tested together rather than drifting apart via a bare `* 2` here.
	//
	// Eager-load exactly the first row (`i < cols`) — it was a literal 12, which silently
	// meant "one row" only while the column ceiling was 6; raising it left the top row's
	// trailing posters lazy-loading above the fold. Deriving it keeps that true as the
	// ladder grows (HOLODEX-331).
	import type { Person } from '$lib/types';
	import PersonPosterCard from './PersonPosterCard.svelte';
	import { posterColumns } from '$lib/density.svelte';

	let { people, empty = 'No people.' }: { people: Person[]; empty?: string } = $props();

	const cols = $derived(posterColumns());
</script>

{#if people.length === 0}
	<p class="py-16 text-center text-sm text-muted">{empty}</p>
{:else}
	<div
		class="people-poster-grid grid gap-4"
		style={`grid-template-columns: repeat(${cols}, minmax(0, 1fr))`}
	>
		{#each people as person, i (person.id)}
			<PersonPosterCard {person} eager={i < cols} />
		{/each}
	</div>
{/if}
