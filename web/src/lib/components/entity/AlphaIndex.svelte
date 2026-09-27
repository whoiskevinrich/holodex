<script lang="ts">
	// The A–Z jump index for the People and Studios lists (F73 R9). From `sm` up it's the
	// sticky horizontal bar; below `sm` a one-column rail on the right edge (text-xs, 16px
	// pitch), so no letters sit above the first row on a phone. Only one of the two is ever
	// displayed, so assistive tech sees one "Jump to letter" nav. The page renders it only
	// while sorted by name, and gives its rows `pr-5` below `sm` so the rail never covers a
	// tappable row. Absent letters stay as decorative, aria-hidden placeholders.
	let { anchors, onjump }: { anchors: Record<string, number>; onjump: (letter: string) => void } = $props();

	const ALPHABET = '#ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('');
	const label = (L: string) => `Jump to ${L === '#' ? 'non-alphabetic names' : L}`;
</script>

<nav
	aria-label="Jump to letter"
	class="sticky top-0 z-10 -mx-1 hidden flex-wrap gap-0.5 bg-bg/85 px-1 py-1.5 backdrop-blur sm:flex"
>
	{#each ALPHABET as L (L)}
		{#if L in anchors}
			<button
				onclick={() => onjump(L)}
				aria-label={label(L)}
				class="rounded-theme px-1.5 py-0.5 text-xs font-medium text-muted hover:bg-surface-2 hover:text-accent"
			>
				{L}
			</button>
		{:else}
			<span class="px-1.5 py-0.5 text-xs text-muted opacity-30" aria-hidden="true">{L}</span>
		{/if}
	{/each}
</nav>

<nav aria-label="Jump to letter" class="fixed right-1 top-1/2 z-10 flex -translate-y-1/2 flex-col sm:hidden">
	{#each ALPHABET as L (L)}
		{#if L in anchors}
			<button
				onclick={() => onjump(L)}
				aria-label={label(L)}
				class="px-1 text-xs font-medium leading-4 text-muted hover:text-accent focus-visible:text-accent"
			>
				{L}
			</button>
		{:else}
			<span class="px-1 text-xs leading-4 text-muted opacity-30" aria-hidden="true">{L}</span>
		{/if}
	{/each}
</nav>
