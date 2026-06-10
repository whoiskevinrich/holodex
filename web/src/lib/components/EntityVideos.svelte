<script lang="ts">
	import type { Video } from '$lib/types';
	import VideoGrid from './VideoGrid.svelte';
	import { videoCount } from '$lib/format';

	// Shared body for the person/[id] and tag/[id] detail pages: back-link, title,
	// video count, and the reused grid.
	let {
		backHref,
		backLabel,
		name,
		videos,
		empty
	}: { backHref: string; backLabel: string; name: string; videos: Video[]; empty: string } =
		$props();
</script>

<section class="space-y-4">
	<a href={backHref} class="text-sm text-muted hover:text-ink">← {backLabel}</a>
	<h1 class="skin-title text-2xl font-semibold text-ink">{name}</h1>
	<p class="text-sm text-muted">{videoCount(videos.length)}</p>
	<VideoGrid {videos} {empty} />
</section>
