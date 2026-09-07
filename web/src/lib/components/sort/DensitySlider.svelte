<script lang="ts">
	// Grid-density slider, shared by the media list and the People poster index (HOLODEX-331).
	// Both drive the same site-wide `mediaDensity`, and both previously carried a byte-identical
	// copy of this markup — the cap-aware inversion below is subtle enough that two copies would
	// drift, so it lives here instead.
	//
	// The range spans DENSITY_MIN..capForWidth(viewport) rather than ..DENSITY_MAX. With a fixed
	// DENSITY_MAX a 1920px window would show eight inert positions at the dense end, since the
	// grids render min(density, cap) — the same dead-stop defect the ladder extension removed.
	import { mediaDensity, viewportTierCap, DENSITY_MIN, invertDensity, effectiveDensity } from '$lib/density.svelte';

	// `label` also opts into the caption's reserved width: the media list wants a captioned
	// control sized to match its sibling filters, while the People index deliberately runs an
	// uncaptioned one inline with a segmented toggle, where a caption row would misalign it.
	// Both the caption and that width live inside the component because it can hide itself
	// below — a caller-owned wrapper would strand an empty gap, or a caption with no control.
	let { label }: { label?: string } = $props();

	const cap = $derived(viewportTierCap.value);
	// What is actually on screen. The stored preference is deliberately left alone, so returning
	// to a wider display restores it.
	const effective = $derived(effectiveDensity());
</script>

<!-- Nothing to choose below three columns: at a cap of 2 this would be a one-position slider,
     and below 480px the cap of 1 would make an invalid `min > max` range. -->
{#if cap > DENSITY_MIN}
	<div class={label ? 'min-w-[160px]' : ''}>
		{#if label}
			<span class="mb-1 block text-xs text-muted">{label}</span>
		{/if}
		<div class="flex items-center gap-2">
			<svg class="h-4 w-4 shrink-0 text-muted" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
				<rect x="3" y="3" width="7" height="7" rx="1" />
				<rect x="14" y="3" width="7" height="7" rx="1" />
				<rect x="3" y="14" width="7" height="7" rx="1" />
				<rect x="14" y="14" width="7" height="7" rx="1" />
			</svg>
			<input
				type="range"
				min={DENSITY_MIN}
				max={cap}
				step="1"
				aria-label="Grid density"
				value={invertDensity(effective, cap)}
				oninput={(e) => (mediaDensity.value = invertDensity(Number(e.currentTarget.value), cap))}
				class="accent-accent"
			/>
			<svg class="h-4 w-4 shrink-0 text-muted" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
				<rect x="4" y="4" width="16" height="16" rx="2" />
			</svg>
		</div>
	</div>
{/if}
