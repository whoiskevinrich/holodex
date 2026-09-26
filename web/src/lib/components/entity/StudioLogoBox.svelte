<script lang="ts">
	import { monogram } from '$lib/format';
	import { haloClass } from '$lib/halo';
	import type { Studio } from '$lib/types';
	import { imageAlt, isWordmark, type Wordmark } from './studioLogo';

	// The studio image box shared by StudioLinkCard (Film/Media detail) and the /studios list
	// rows (HOLODEX-432): `logo_url` → `icon_url` → monogram. Any image draws bare — no plate
	// (HOLODEX-411 for logos, HOLODEX-437 for icons) — wearing the owner's per-role, per-palette
	// halo instead (HOLODEX-463, ADR-109: default off; the shown role's choice), in a
	// box that follows its own aspect (fixed 48px height, width clamped to [48, 192]px, never
	// a cover-crop; see entity/CLAUDE.md "Frame follows source aspect"). The `p-1` inset is
	// transparent room for the halo inside the `overflow-hidden` box. Only the monogram keeps
	// the plate (dashed) — it is text and needs a ground.
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
	const halo = $derived(haloClass(studio.image_halo?.[bare ? 'logo' : 'icon']));

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
	class="flex h-12 max-w-48 min-w-12 shrink-0 items-center justify-center overflow-hidden rounded-theme {image
		? ''
		: 'w-12 border border-dashed border-rule bg-logo-plate'}"
>
	{#if image}
		<!-- `loading` before `src`: a fetch that has already started is not deferred by a
		     later `loading="lazy"`. -->
		<img
			bind:this={img}
			loading={eager ? 'eager' : 'lazy'}
			src={image}
			alt={imageAlt(studio.name, bare, wordmark)}
			class="{halo} h-full w-auto max-w-full object-contain p-1"
			onload={decide}
			onerror={() => (wordmark = false)}
		/>
	{:else}
		<span class="font-display text-sm font-semibold text-logo-plate-ink" aria-hidden="true">
			{monogram(studio.name)}
		</span>
	{/if}
</span>
