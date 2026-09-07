<script lang="ts">
	import type { Video } from '$lib/types';
	import VideoCard from './VideoCard.svelte';
	import { activity } from '$lib/activity.svelte';
	import { effectiveDensity } from '$lib/density.svelte';
	import { stageGridTracks } from '$lib/stageGrid';

	let {
		videos,
		empty = 'No videos.',
		sceneNumbers,
		onEditScene,
		stageAligned = false
	}: {
		videos: Video[];
		empty?: string;
		sceneNumbers?: (video: Video) => number | null | undefined;
		onEditScene?: (video: Video) => void;
		/** Hold the stage width until there are enough cards to outgrow it, then grow and
		 *  centre (HOLODEX-331 §9.6). Only meaningful for a grid that spans the full page
		 *  width outside the stage cap — today just the film page's Scenes list. Off
		 *  everywhere else, where the container is already the thing being filled. */
		stageAligned?: boolean;
	} = $props();

	// Responsive reflow (F12.6) + user density preference: the viewport tier caps how many
	// columns fit before cards get too small; the density slider picks how many of those
	// columns to actually use, up to the tier's cap. Column count is computed in JS (not
	// Tailwind grid-cols-N utilities) because the target column count is a runtime value the
	// Tailwind scanner can't see at build time.
	const cols = $derived(effectiveDensity());

	// --- stageAligned only, below ---

	// Width available to the grid, measured rather than derived from the viewport: the
	// stage-aligned grid sizes itself to its content, so it cannot be measured directly,
	// and computing from `innerWidth` would hardcode the page's own padding here.
	let avail = $state(0);
	const GAP = 16; // matches `gap-4`

	// Card size from the full column count, track count from the card count — see
	// $lib/stageGrid for why those differ and what each one buys.
	const tracks = $derived(stageGridTracks(cols, videos.length, avail, GAP));

	const gridStyle = $derived(
		stageAligned && tracks.trackPx > 0
			? `grid-template-columns: repeat(${tracks.trackCount}, ${tracks.trackPx}px)`
			: `grid-template-columns: repeat(${cols}, minmax(0, 1fr))`
	);
</script>

{#snippet grid()}
	<!-- data-layout drives the card aspect ratio via app.css. -->
	<div
		class="video-grid grid gap-4"
		class:stage-aligned={stageAligned}
		style={gridStyle}
		data-layout={activity.cardLayout}
	>
		{#each videos as video (video.id)}
			<VideoCard {video} sceneNumber={sceneNumbers?.(video)} {onEditScene} />
		{/each}
	</div>
{/snippet}

{#if videos.length === 0}
	<p class="py-16 text-center text-sm text-muted">{empty}</p>
{:else if stageAligned}
	<!-- Full-width measuring box. Only rendered in this mode so the other grids don't each
	     carry a ResizeObserver they have no use for. -->
	<div bind:clientWidth={avail}>
		{@render grid()}
	</div>
{:else}
	{@render grid()}
{/if}
