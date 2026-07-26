<script lang="ts">
	import type { Video } from '$lib/types';
	import VideoCard from './VideoCard.svelte';
	import { activity } from '$lib/activity.svelte';

	let { videos, empty = 'No videos.' }: { videos: Video[]; empty?: string } = $props();
</script>

{#if videos.length === 0}
	<p class="py-16 text-center text-sm text-muted">{empty}</p>
{:else}
	<!-- Responsive reflow (F12.6): 1 col <480px, 2 cols through ≤768px, scaling up
	     on wider viewports. data-layout drives the card aspect ratio via app.css. -->
	<div
		class="video-grid grid grid-cols-1 gap-4 min-[480px]:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5"
		data-layout={activity.cardLayout}
	>
		{#each videos as video (video.id)}
			<VideoCard {video} />
		{/each}
	</div>
{/if}
