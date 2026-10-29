<script lang="ts">
	// `VideoGrid`'s whole-list twin (F80 EL11/EL15, HOLODEX-604): the same cards in
	// `shared/VirtualRows`, so only the rows near the view are drawn. One card height is the
	// row-height contract: the card's title box is always two lines, and the placeholder is
	// the card's own frame with two title bars (handoff, "The other library lists").
	import type { Video } from '#lib/types.ts';
	import type { SparseList } from '#lib/sparseList.svelte.ts';
	import type { RovingList, WindowVirtualizer } from '#lib/windowVirtualizer.svelte.ts';
	import VirtualRows from '#lib/components/shared/VirtualRows.svelte';
	import VideoCard from './VideoCard.svelte';
	import { activity } from '#lib/activity.svelte.ts';
	import { effectiveDensity } from '#lib/density.svelte.ts';

	let {
		list,
		virt,
		roving,
		label = 'Videos',
		onRemove,
		root = $bindable()
	}: {
		list: SparseList<Video>;
		virt: WindowVirtualizer;
		roving: RovingList;
		label?: string;
		/** The playlist page's owner remove, as on `VideoGrid`. */
		onRemove?: (video: Video) => void;
		root?: HTMLElement;
	} = $props();

	const lanes = $derived(effectiveDensity());
</script>

<!-- data-layout drives the card aspect ratio via app.css, as on VideoGrid. -->
<div class="video-grid" data-layout={activity.cardLayout}>
	<VirtualRows {list} {virt} {roving} {lanes} {label} noun="videos" rowClass="grid gap-4 pb-4" bind:root>
		{#snippet entry(video)}
			{#if onRemove}
				<div class="group relative">
					<VideoCard {video} />
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
				<VideoCard {video} />
			{/if}
		{/snippet}
		{#snippet placeholder()}
			<div>
				<div class="video-frame"></div>
				<!-- The card's caption box: p-3 around the two-line (h-10) title. -->
				<div class="p-3">
					<div class="flex h-10 flex-col justify-center gap-2.5">
						<span class="block h-2.5 w-4/5 rounded-theme bg-rule"></span>
						<span class="block h-2.5 w-1/2 rounded-theme bg-rule"></span>
					</div>
				</div>
			</div>
		{/snippet}
	</VirtualRows>
</div>
