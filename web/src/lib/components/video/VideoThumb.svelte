<script lang="ts">
	// The Media Table's small thumbnail (F82, list-views handoff "Columns"): 48×27, 40×23
	// below `sm`. It retries a still-generating thumbnail exactly as VideoCard does
	// (thumbnailLoad.svelte.ts); until it loads, or once it gives up, the frame stays
	// empty — never a broken-image glyph. Hidden with opacity, as VideoCard does.
	import type { Video } from '#lib/types.ts';
	import { api } from '#lib/api.ts';
	import { ThumbnailLoad } from './thumbnailLoad.svelte.ts';

	let { video }: { video: Video } = $props();

	const thumb = new ThumbnailLoad(() => video.thumbnail_url || api.thumbnailURL(video.id));
</script>

<span
	class="block h-[23px] w-10 shrink-0 overflow-hidden rounded-theme border border-rule bg-surface-2 sm:h-[27px] sm:w-12"
>
	{#if !thumb.gaveUp}
		<img
			src={thumb.src}
			alt=""
			loading="lazy"
			onload={thumb.onload}
			onerror={thumb.onerror}
			class="h-full w-full object-cover {thumb.loaded ? '' : 'opacity-0'}"
		/>
	{/if}
</span>
