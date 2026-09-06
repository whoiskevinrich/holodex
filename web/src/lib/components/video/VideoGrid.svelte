<script lang="ts">
	import type { Video } from '$lib/types';
	import VideoCard from './VideoCard.svelte';
	import { activity } from '$lib/activity.svelte';
	import { mediaDensity, viewportTierCap } from '$lib/density.svelte';

	let {
		videos,
		empty = 'No videos.',
		sceneNumbers,
		onEditScene
	}: {
		videos: Video[];
		empty?: string;
		sceneNumbers?: (video: Video) => number | null | undefined;
		onEditScene?: (video: Video) => void;
	} = $props();

	// Responsive reflow (F12.6) + user density preference: the viewport tier caps how many
	// columns fit before cards get too small; the density slider picks how many of those
	// columns to actually use, up to the tier's cap. Column count is computed in JS (not
	// Tailwind grid-cols-N utilities) because the target column count is a runtime value the
	// Tailwind scanner can't see at build time.
	const cols = $derived(Math.min(mediaDensity.value, viewportTierCap.value));
</script>

{#if videos.length === 0}
	<p class="py-16 text-center text-sm text-muted">{empty}</p>
{:else}
	<!-- data-layout drives the card aspect ratio via app.css. -->
	<div
		class="video-grid grid gap-4"
		style={`grid-template-columns: repeat(${cols}, minmax(0, 1fr))`}
		data-layout={activity.cardLayout}
	>
		{#each videos as video (video.id)}
			<VideoCard {video} sceneNumber={sceneNumbers?.(video)} {onEditScene} />
		{/each}
	</div>
{/if}
