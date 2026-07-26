<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { Video } from '$lib/types';
	import VideoGrid from '../video/VideoGrid.svelte';
	import { videoCount } from '$lib/format';

	// Shared body for the person/[id] and tag/[id] detail pages: back-link, title,
	// video count, and the reused grid. The optional `detail` snippet renders an
	// entity-specific panel (e.g. People enrichment, F22) between the count and grid;
	// the tag page omits it, keeping this component shared.
	//
	// The optional `hero` snippet REPLACES the default title+count block — the person
	// page uses it to render its banner with the name beside the portrait (so the name
	// reads as one unit with the face), supplying its own title and count within. When
	// `hero` is omitted (the tag page) the plain title+count is rendered as before.
	let {
		backHref,
		backLabel,
		name,
		videos,
		empty,
		hero,
		detail
	}: {
		backHref: string;
		backLabel: string;
		name: string;
		videos: Video[];
		empty: string;
		hero?: Snippet;
		detail?: Snippet;
	} = $props();
</script>

<section class="space-y-4">
	<a href={backHref} class="text-sm text-muted hover:text-ink">← {backLabel}</a>
	{#if hero}
		{@render hero()}
	{:else}
		<h1 class="skin-title text-2xl font-semibold text-ink">{name}</h1>
		<p class="text-sm text-muted">{videoCount(videos.length)}</p>
	{/if}
	{#if detail}{@render detail()}{/if}
	<VideoGrid {videos} {empty} />
</section>
