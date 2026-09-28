<script lang="ts">
	// The toolbar's Filters trigger (F73). From `sm` up it reads "Filters · n"; below `sm` it
	// collapses to a funnel plus the count, because the 375px toolbar budget can't fit the
	// word (design handoff, Responsive). The accessible name is the same at every width.
	let {
		count,
		expanded,
		controls,
		onclick,
		el = $bindable(null)
	}: {
		count: number;
		expanded: boolean;
		controls: string;
		onclick: () => void;
		el?: HTMLButtonElement | null;
	} = $props();
</script>

<button
	bind:this={el}
	type="button"
	{onclick}
	aria-haspopup="dialog"
	aria-expanded={expanded}
	aria-controls={controls}
	aria-label={count ? `Filters, ${count} active` : 'Filters'}
	class="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-theme border bg-surface px-3 text-sm {count
		? 'border-accent text-accent'
		: 'border-rule text-ink hover:bg-surface-2'}"
>
	<svg class="h-3.5 w-3.5 sm:hidden" viewBox="0 0 14 14" fill="none" aria-hidden="true">
		<path d="M1 1.5h12L8.5 7v4.5l-3 1.5V7z" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round" />
	</svg>
	<span class="hidden sm:inline">Filters{count ? ` · ${count}` : ''}</span>
	{#if count}<span class="sm:hidden">{count}</span>{/if}
</button>
