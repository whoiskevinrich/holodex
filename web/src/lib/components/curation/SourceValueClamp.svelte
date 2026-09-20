<script lang="ts">
	// A source's paragraph-length value inside SourceEditModal, clamped to four lines with a
	// Show more / Show less toggle (HOLODEX-417). With every candidate rendered in full, a
	// 2000-character overview across two or three namespaces ran to several viewport heights,
	// so the sources being compared were never on screen together — and neither were the
	// buttons. Not ExpandableText: that component *is* the one look for displayed prose
	// (muted, no styling props by design, HOLODEX-365), whereas this is the evidence under a
	// radio and keeps the row's ink tone. Measurement mirrors ExpandableText: against the
	// clamp budget, not clientHeight, so the toggle survives its own expansion; no toggle at
	// all when the text fits (HOLODEX-361).
	const LINES = 4;

	let { text, label }: { text: string; label: string } = $props();

	let expanded = $state(false);
	let clamps = $state(false);
	let el = $state<HTMLElement>();
	let width = $state(0);
	const textId = $props.id();

	function measure() {
		if (!el) return;
		const lineHeight = parseFloat(getComputedStyle(el).lineHeight);
		clamps = el.scrollHeight > lineHeight * LINES + 1;
	}

	$effect(() => {
		void [width, text];
		measure();
	});

	$effect(() => {
		document.fonts?.ready.then(measure);
	});
</script>

<span
	bind:this={el}
	bind:clientWidth={width}
	id={textId}
	class="wrap-anywhere leading-relaxed {expanded ? 'block' : 'line-clamp-4'}"
>
	{text}
</span>
{#if clamps}
	<button
		type="button"
		onclick={() => (expanded = !expanded)}
		aria-expanded={expanded}
		aria-controls={textId}
		aria-label={expanded ? `Show less of the ${label} value` : `Show the full ${label} value`}
		class="btn-quiet mt-1 inline-flex items-center gap-1 text-xs"
	>
		{expanded ? 'Show less' : 'Show more'}
		<svg
			class="h-3 w-3 transition-transform duration-200 motion-reduce:transition-none"
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
