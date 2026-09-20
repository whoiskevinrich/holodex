<script lang="ts">
	import { monogram } from '$lib/format';
	import type { Studio } from '$lib/types';
	import { imageAlt, isWordmark, type Wordmark } from './studioLogo';

	// The studio image box shared by StudioLinkCard (Film/Media detail) and the /studios list
	// rows (HOLODEX-432): `logo_url` → `icon_url` → monogram. A logo draws bare — no plate
	// (HOLODEX-411) — in a box that follows its own aspect (fixed 48px height, width clamped
	// to [48, 192]px, never a cover-crop; see entity/CLAUDE.md "Frame follows source aspect").
	// Icon and monogram keep the plate; dashed border only when there is no image at all.
	//
	// `wordmark` is bound OUT: the caller decides whether to render the name beside the box
	// (studioLogo.ts `showName`). It is read from the loaded image's natural size, so it stays
	// null — and the caller shows nothing beside a bare logo — until the logo has loaded.
	let {
		studio,
		wordmark = $bindable(null),
		eager = true
	}: { studio: Studio; wordmark?: Wordmark; eager?: boolean } = $props();

	const image = $derived(studio.logo_url || studio.icon_url);
	const bare = $derived(Boolean(studio.logo_url));

	let img = $state<HTMLImageElement | undefined>();

	// Only a bare logo can be a wordmark; an icon on the plate always keeps its caption.
	function decide() {
		if (!img || !bare) return;
		wordmark = isWordmark(img.naturalWidth, img.naturalHeight);
	}
	// Reset on a studio/image change, and decide immediately for an already-cached image
	// (its `load` event has already fired by the time we bind).
	$effect(() => {
		wordmark = null;
		if (bare && image && img?.complete && img.naturalWidth > 0) decide();
	});
</script>

<span
	class="flex h-12 max-w-48 min-w-12 shrink-0 items-center justify-center overflow-hidden rounded-theme {bare
		? ''
		: 'border border-rule bg-logo-plate'} {image ? '' : 'w-12 border-dashed'}"
>
	{#if image}
		<!-- `loading` before `src`: a fetch that has already started is not deferred by a
		     later `loading="lazy"`. -->
		<img
			bind:this={img}
			loading={eager ? 'eager' : 'lazy'}
			src={image}
			alt={imageAlt(studio.name, bare, wordmark)}
			class="logo-halo h-full w-auto max-w-full object-contain p-1"
			onload={decide}
			onerror={() => (wordmark = false)}
		/>
	{:else}
		<span class="font-display text-sm font-semibold text-logo-plate-ink" aria-hidden="true">
			{monogram(studio.name)}
		</span>
	{/if}
</span>
