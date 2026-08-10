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
		title = '',
		decorative = false
	}: {
		name: string;
		iconUrl?: string;
		size?: number;
		title?: string;
		// decorative drops the alt/aria-label (HOLODEX-266, ADR-083): set when a caller's
		// own wrapping element already carries the accessible name (e.g. ProviderLinkBadge's
		// visible text label), so the icon isn't announced twice.
		decorative?: boolean;
	} = $props();

	const label = $derived(title || name);
	// A real logo keeps its own aspect: fixed HEIGHT, auto width, capped — so a wide
	// wordmark (TMDB's, HOLODEX-161) reads legibly instead of squashing into a square.
	// The monogram fallback stays a square plate (it's a single letter). Pixel sizing is
	// inline (not a themeable token); colors/radius/font are all tokens below.
	const imgStyle = $derived(`height:${size}px;width:auto;max-width:${size * 4}px`);
	const monoStyle = $derived(
		`width:${size}px;height:${size}px;font-size:${Math.round(size * 0.6)}px;line-height:1`
	);
</script>

{#if iconUrl}
	<img
		src={iconUrl}
		alt={decorative ? '' : label}
		{title}
		style={imgStyle}
		class="inline-block shrink-0 rounded-theme bg-logo-plate object-contain align-middle"
	/>
{:else}
	<span
		{title}
		aria-label={decorative ? undefined : label}
		aria-hidden={decorative ? 'true' : undefined}
		style={monoStyle}
		class="inline-flex shrink-0 items-center justify-center rounded-theme bg-logo-plate align-middle font-display font-semibold text-logo-plate-ink"
		>{monogram(name)}</span
	>
{/if}
