<script lang="ts">
	import type { Video } from '$lib/types';
	import VideoCard from './VideoCard.svelte';

	let { videos, empty = 'No videos.' }: { videos: Video[]; empty?: string } = $props();
</script>

{#if videos.length === 0}
	<p class="py-16 text-center text-sm text-muted">{empty}</p>
{:else}
	<div class="video-grid grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
		{#each videos as video (video.id)}
			<VideoCard {video} />
		{/each}
	</div>
{/if}
