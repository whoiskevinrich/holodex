<script lang="ts">
	// One active filter or entity scope under the list toolbar (F73). Chips are the only
	// visible sign that a list is narrowed, so each one names itself and removes in one tap.
	// The retired FacetFilter chip's look; long values (a mapped-facet string) truncate at 12rem
	// with the full text in the tooltip. `negated` is an F87 NOT condition: the outlined
	// accent chip, its label starting `not ` (query-language-handoff.md); warn stays for errors.
	// F89: `ownerOnly` draws a key before the label, `problem` marks the chip a refusal names,
	// and no `onremove` is a read-only chip (a playlist's definition outside its editor).
	let {
		label,
		kind = 'filter',
		negated = false,
		ownerOnly = false,
		problem = false,
		onremove
	}: {
		label: string;
		/** 'scope' for an entity chip that arrived from an entity page (Studio: Acme). */
		kind?: 'filter' | 'scope';
		/** An F87 NOT condition: transparent fill, accent outline and text. */
		negated?: boolean;
		/** A condition only the owner may run: the key glyph, which never truncates. */
		ownerOnly?: boolean;
		/** The chip at fault in a refused definition: warn outline and text, over `negated`. */
		problem?: boolean;
		onremove?: () => void;
	} = $props();
</script>

<span
	class="inline-flex shrink-0 items-center gap-0.5 rounded-theme text-xs {onremove ? 'pl-1.5 pr-0.5' : 'px-1.5'} {problem
		? 'border border-warn py-px text-warn'
		: negated
			? 'border border-accent py-px text-accent'
			: 'bg-accent py-0.5 text-accent-ink'}"
>
	{#if ownerOnly}
		<svg aria-hidden="true" viewBox="0 0 12 12" class="mr-0.5 h-3 w-3 shrink-0" fill="none" stroke="currentColor" stroke-width="1.4">
			<circle cx="4" cy="6" r="2.6" />
			<path d="M6.6 6h4.4M9.4 6v2.2" />
		</svg>
		<span class="sr-only">Owner only:</span>
	{/if}
	<span class="max-w-[12rem] truncate" title={label}>{label}</span>
	{#if onremove}
		<button
			type="button"
			onclick={onremove}
			aria-label={`Remove ${kind}: ${label}`}
			class="rounded-theme px-1 leading-none {problem ? 'hover:bg-warn/10' : negated ? 'hover:bg-accent/10' : 'hover:bg-accent-ink/10'}"
		>
			×
		</button>
	{/if}
</span>
