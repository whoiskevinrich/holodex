<script lang="ts">
	// Provider brand glyph (ADR-059, HOLODEX-134). Renders the self-hosted provider icon
	// when one is cached, else a themed monogram (the provider's initial) on the shared
	// logo plate — the same fallback the studios list uses (HOLODEX-126). This component
	// owns only the icon-or-monogram presentation; the caller supplies the served
	// icon_url (from the providers store). Tokens only, so it reads on all three skins.
	// Consumed by the provenance badge, enrich controls, and website label
	// (HOLODEX-135/136/137).
	import { monogram } from '$lib/format';

	let {
		name,
		iconUrl = '',
		size = 16,
		title = ''
	}: { name: string; iconUrl?: string; size?: number; title?: string } = $props();

	const label = $derived(title || name);
	// Dynamic pixel sizing (not a themeable token) stays inline; colors/radius/font are
	// all tokens below.
	const box = $derived(`width:${size}px;height:${size}px`);
</script>

{#if iconUrl}
	<img
		src={iconUrl}
		alt={label}
		{title}
		style={box}
		class="inline-block shrink-0 rounded-theme bg-logo-plate object-contain align-middle"
	/>
{:else}
	<span
		{title}
		aria-label={label}
		style="{box};font-size:{Math.round(size * 0.6)}px;line-height:1"
		class="inline-flex shrink-0 items-center justify-center rounded-theme bg-logo-plate align-middle font-display font-semibold text-logo-plate-ink"
		>{monogram(name)}</span
	>
{/if}
