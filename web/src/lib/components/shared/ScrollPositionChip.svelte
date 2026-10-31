<script lang="ts">
	// "412 of 1,512" beside the page scrollbar while a whole-list scroll moves (HOLODEX-601,
	// handoff "Position chip"): the first entry in view out of the total, in every sort. It
	// fades out a second after scrolling stops (at once under reduced motion; app.css owns
	// the fade). Decorative: each entry already announces its own position. Hidden below sm,
	// where the A–Z rail sits on that edge and touch scrollbars are overlays.
	import type { WindowVirtualizer } from '#lib/windowVirtualizer.svelte.ts';

	let { virt, total }: { virt: WindowVirtualizer; total: number } = $props();

	let shown = $state(false);
	$effect(() => {
		if (virt.scrolling) {
			shown = true;
			return;
		}
		const t = setTimeout(() => (shown = false), 1000);
		return () => clearTimeout(t);
	});

	const first = $derived(Math.min(virt.firstVisible * virt.lanes + 1, total));
	// Track the scrollbar thumb: the share of rows above the view, along the viewport's height.
	const progress = $derived(virt.rowCount > 1 ? virt.firstVisible / (virt.rowCount - 1) : 0);
</script>

{#if total > 0 && virt.rowCount > virt.lastVisible - virt.firstVisible + 1}
	<div
		aria-hidden="true"
		data-shown={shown}
		class="scroll-position-chip pointer-events-none fixed right-3 z-20 hidden rounded-theme border border-rule bg-surface px-2 py-1 text-xs text-ink sm:block"
		style:top="calc({progress} * (100vh - 3rem) + 0.5rem)"
	>
		{first.toLocaleString()} of {total.toLocaleString()}
	</div>
{/if}
