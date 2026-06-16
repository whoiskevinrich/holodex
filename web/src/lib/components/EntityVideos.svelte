<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { Video } from '$lib/types';
	import VideoGrid from './VideoGrid.svelte';
	import { videoCount } from '$lib/format';

	// Shared body for the person/[id] and tag/[id] detail pages: back-link, title,
	// video count, and the reused grid. The optional `detail` snippet renders an
	// entity-specific panel (e.g. People enrichment, F22) between the count and grid;
	// the tag page omits it, keeping this component shared.
	let {
		backHref,
		backLabel,
		name,
		videos,
		empty,
		detail
	}: {
		backHref: string;
		backLabel: string;
		name: string;
		videos: Video[];
		empty: string;
		detail?: Snippet;
	} = $props();
</script>

<section class="space-y-4">
	<a href={backHref} class="text-sm text-muted hover:text-ink">← {backLabel}</a>
	<h1 class="skin-title text-2xl font-semibold text-ink">{name}</h1>
	<p class="text-sm text-muted">{videoCount(videos.length)}</p>
	{#if detail}{@render detail()}{/if}
	<VideoGrid {videos} {empty} />
</section>
