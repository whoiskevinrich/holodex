<script lang="ts">
	import type { ExternalLink } from '$lib/types';
	import ProviderLinkBadge from '../enrichment/ProviderLinkBadge.svelte';
	import { videoCount, sortExternalLinks } from '$lib/format';

	// The muted video-count line under an entity's title, with its provider-link badges
	// (HOLODEX-266, ADR-083 DD1) appended after a `·` separator. Shared by EntityVideos'
	// own default title block (studio/tag) and the person page's `hero` snippet, which
	// renders its own title/portrait layout but wants this identical meta row beneath it.
	// `count` is null until the grid's first load reports it (HOLODEX-501): render no number
	// rather than a "0 videos" flash.
	let {
		count,
		links,
		entityName
	}: { count: number | null; links: ExternalLink[]; entityName: string } = $props();

	const sortedLinks = $derived(sortExternalLinks(links));
</script>

<div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted">
	{#if count != null}<span>{videoCount(count)}</span>{/if}
	{#if sortedLinks.length}
		{#if count != null}<span aria-hidden="true">·</span>{/if}
		{#each sortedLinks as link (link.provider)}
			<ProviderLinkBadge {link} {entityName} />
		{/each}
	{/if}
</div>
