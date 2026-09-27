<script lang="ts">
	// One active filter or entity scope under the list toolbar (F73). Chips are the only
	// visible sign that a list is narrowed, so each one names itself and removes in one tap.
	// Same look as FacetFilter's chip; long values (a mapped-facet string) truncate at 12rem
	// with the full text in the tooltip.
	let {
		label,
		kind = 'filter',
		onremove
	}: {
		label: string;
		/** 'scope' for an entity chip that arrived from an entity page (Studio: Acme). */
		kind?: 'filter' | 'scope';
		onremove: () => void;
	} = $props();
</script>

<span
	class="inline-flex shrink-0 items-center gap-0.5 rounded-theme bg-accent py-0.5 pl-1.5 pr-0.5 text-xs text-accent-ink"
>
	<span class="max-w-[12rem] truncate" title={label}>{label}</span>
	<button
		type="button"
		onclick={onremove}
		aria-label={`Remove ${kind}: ${label}`}
		class="rounded-theme px-1 leading-none hover:bg-accent-ink/10"
	>
		×
	</button>
</span>
