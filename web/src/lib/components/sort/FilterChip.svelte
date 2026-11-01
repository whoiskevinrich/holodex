<script lang="ts">
	// One active filter or entity scope under the list toolbar (F73). Chips are the only
	// visible sign that a list is narrowed, so each one names itself and removes in one tap.
	// The retired FacetFilter chip's look; long values (a mapped-facet string) truncate at 12rem
	// with the full text in the tooltip. `negated` is an F87 NOT condition: the outlined
	// accent chip, its label starting `not ` (query-language-handoff.md); warn stays for errors.
	let {
		label,
		kind = 'filter',
		negated = false,
		onremove
	}: {
		label: string;
		/** 'scope' for an entity chip that arrived from an entity page (Studio: Acme). */
		kind?: 'filter' | 'scope';
		/** An F87 NOT condition: transparent fill, accent outline and text. */
		negated?: boolean;
		onremove: () => void;
	} = $props();
</script>

<span
	class="inline-flex shrink-0 items-center gap-0.5 rounded-theme pl-1.5 pr-0.5 text-xs {negated
		? 'border border-accent py-px text-accent'
		: 'bg-accent py-0.5 text-accent-ink'}"
>
	<span class="max-w-[12rem] truncate" title={label}>{label}</span>
	<button
		type="button"
		onclick={onremove}
		aria-label={`Remove ${kind}: ${label}`}
		class="rounded-theme px-1 leading-none {negated ? 'hover:bg-accent/10' : 'hover:bg-accent-ink/10'}"
	>
		×
	</button>
</span>
