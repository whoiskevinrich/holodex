<script lang="ts">
	// Clamp long text to a fixed line count with a chevron toggle to reveal the
	// rest — the same expand/collapse idiom as CompletenessPanel's facet list,
	// applied to prose instead of a facet breakdown. Prose that fits inside the
	// clamp gets no chevron at all (HOLODEX-361): a control that cannot change
	// what you see is one the reader learns to distrust, and its aria-expanded
	// announced a collapsed region that was never there.
	const CLAMP = { 4: 'line-clamp-4', 5: 'line-clamp-5' } as const;
	const TONE = { ink: 'text-ink', muted: 'text-muted' } as const;

	let {
		text,
		lines = 5,
		tone = 'ink',
		chevronLabel
	}: { text: string; lines?: 4 | 5; tone?: 'ink' | 'muted'; chevronLabel: string } = $props();

	let expanded = $state(false);
	let clamps = $state(false);
	let prose = $state<HTMLParagraphElement>();
	let proseWidth = $state(0);
	const textId = $props.id();

	function measure() {
		if (!prose) return;
		// Against the clamp *budget*, not clientHeight: `scrollHeight > clientHeight`
		// only holds while collapsed, so it would drop the chevron the moment the reader
		// expanded and strand them with no way back. lineHeight resolves to px because
		// leading-relaxed is set on the element below.
		const lineHeight = parseFloat(getComputedStyle(prose).lineHeight);
		clamps = prose.scrollHeight > lineHeight * lines + 1;
	}

	$effect(() => {
		// proseWidth (bound below) is the re-wrap signal; text and lines move the budget.
		void [proseWidth, text, lines];
		measure();
	});

	$effect(() => {
		// font-display: swap paints fallback metrics first, and those wrap differently
		// from the real face — the clamp can gain or lose a line when the webfont lands.
		document.fonts?.ready.then(measure);
	});
</script>

<div>
	<p
		bind:this={prose}
		bind:clientWidth={proseWidth}
		id={textId}
		class="text-sm leading-relaxed {TONE[tone]} {expanded ? '' : CLAMP[lines]}"
	>
		{text}
	</p>
	{#if clamps}
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
	{/if}
</div>
