<script lang="ts">
	// Clamp long text to a fixed line count with a chevron toggle to reveal the
	// rest — the same expand/collapse idiom as CompletenessPanel's facet list,
	// applied to prose instead of a facet breakdown.
	const CLAMP = { 4: 'line-clamp-4', 5: 'line-clamp-5' } as const;
	const TONE = { ink: 'text-ink', muted: 'text-muted' } as const;

	let {
		text,
		lines = 5,
		tone = 'ink',
		chevronLabel
	}: { text: string; lines?: 4 | 5; tone?: 'ink' | 'muted'; chevronLabel: string } = $props();

	let expanded = $state(false);
	const textId = $props.id();
</script>

<div>
	<p id={textId} class="text-sm leading-relaxed {TONE[tone]} {expanded ? '' : CLAMP[lines]}">
		{text}
	</p>
	<button
		type="button"
		onclick={() => (expanded = !expanded)}
		aria-expanded={expanded}
		aria-controls={textId}
		aria-label={expanded ? `Collapse ${chevronLabel}` : `Show full ${chevronLabel}`}
		title={expanded ? 'Show less' : 'Show more'}
		class="btn-quiet mt-1 flex h-7 w-7 items-center justify-center rounded-theme hover:bg-surface-2"
	>
		<svg
			class="h-4 w-4 transition-transform duration-200 motion-reduce:transition-none"
			class:rotate-180={expanded}
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			aria-hidden="true"
		>
			<path stroke-linecap="round" stroke-linejoin="round" d="M6 9l6 6 6-6" />
		</svg>
	</button>
</div>
