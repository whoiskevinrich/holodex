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
		onRemove,
		stageAligned = false
	}: {
		videos: Video[];
		empty?: string;
		sceneNumbers?: (video: Video) => number | null | undefined;
		onEditScene?: (video: Video) => void;
		/** Owner-only per-tile remove (the playlist page, F69 P0-6): an `×` in the tile's
		 *  top-right corner — the poster-upload button's treatment — as a sibling of the
		 *  card, never inside its link. Top-right is free here because the scene badge that
		 *  owns it is Films-only and film videos are out of a playlist's scope. */
		onRemove?: (video: Video) => void;
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

	// In stageAligned mode the gap is emitted inline from the same GAP the track maths uses,
	// so the two provably agree. `gap-4` stays on the element for the default path; the
	// inline value simply wins here. Without this, retuning `gap-4` would silently desync
	// the fixed tracks from the real gaps — and unlike a `1fr` grid, nothing self-corrects.
	const gridStyle = $derived(
		stageAligned && tracks.trackPx > 0
			? `gap: ${GAP}px; grid-template-columns: repeat(${tracks.trackCount}, ${tracks.trackPx}px)`
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
			{#if onRemove}
				<div class="group relative">
					<VideoCard {video} sceneNumber={sceneNumbers?.(video)} {onEditScene} />
					<button
						type="button"
						onclick={() => onRemove(video)}
						title="Remove from playlist"
						aria-label="Remove from playlist"
						class="absolute right-2 top-2 z-10 rounded-full bg-black/60 p-1.5 text-muted opacity-0 transition hover:text-ink focus-visible:opacity-100 group-hover:opacity-100"
					>
						<svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24" aria-hidden="true">
							<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
						</svg>
					</button>
				</div>
			{:else}
				<VideoCard {video} sceneNumber={sceneNumbers?.(video)} {onEditScene} />
			{/if}
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
