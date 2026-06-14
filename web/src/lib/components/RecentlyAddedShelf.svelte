<script lang="ts">
	import type { Video } from '$lib/types';
	import VideoCard from './VideoCard.svelte';

	// The 20 newest videos (F12.3), sliced from the grid's already-loaded page so
	// the shelf adds no extra request. Only rendered on the unfiltered landing
	// view, where the grid is sorted newest-first.
	let { videos }: { videos: Video[] } = $props();
	const recent = $derived(videos.slice(0, 20));
</script>

{#if recent.length > 0}
	<section class="space-y-2">
		<h2 class="skin-title text-sm font-semibold uppercase tracking-wide text-muted">Recently added</h2>
		<div class="flex gap-4 overflow-x-auto pb-2">
			{#each recent as video (video.id)}
				<div class="w-52 shrink-0 sm:w-56">
					<VideoCard {video} />
				</div>
			{/each}
		</div>
	</section>
{/if}
