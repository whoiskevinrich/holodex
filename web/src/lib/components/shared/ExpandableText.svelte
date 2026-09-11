<script lang="ts">
	// Clamp long text to a fixed line count with a chevron toggle to reveal the
	// rest — the same expand/collapse idiom as CompletenessPanel's facet list,
	// applied to prose instead of a facet breakdown. Prose that fits inside the
	// clamp gets no chevron at all (HOLODEX-361): a control that cannot change
	// what you see is one the reader learns to distrust, and its aria-expanded
	// announced a collapsed region that was never there.
	//
	// No styling props on purpose (HOLODEX-365): long prose — bio, overview, description — is
	// muted `text-sm leading-relaxed`, and this component is where that is decided. A `tone`
	// knob used to exist, defaulted to ink, and only one of three call sites set it, so the
	// "one look for prose" rule was opt-in per page. `lines` stays: 4 vs 5 is a layout fit
	// (the photo-pinned Person hero vs the rail), not a typography choice.
	const CLAMP = { 4: 'line-clamp-4', 5: 'line-clamp-5' } as const;

	let {
		text,
		lines = 5,
		chevronLabel
	}: { text: string; lines?: 4 | 5; chevronLabel: string } = $props();

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
	<!-- `wrap-anywhere` (HOLODEX-363, same rule as SourceBadge's value span): the text is
	     arbitrary and an unbreakable token wider than the box is otherwise clipped by the
	     clamp's overflow:hidden — silently, with no scroll and nothing poking out. The
	     media page's overview now sits in the rail, whose floor is 320px, so a 60-character
	     token overflowed a 390px track by 34px at 1024 and lost its tail. -->
	<p
		bind:this={prose}
		bind:clientWidth={proseWidth}
		id={textId}
		class="wrap-anywhere text-sm leading-relaxed text-muted {expanded ? '' : CLAMP[lines]}"
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
