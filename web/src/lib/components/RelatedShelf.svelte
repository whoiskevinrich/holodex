<script lang="ts">
	import type { Video } from '$lib/types';
	import VideoCard from './VideoCard.svelte';

	// One "More with <name>" shelf (QW3). Rendered only when it has items — the parent
	// passes an empty array for an entity with no siblings, and we self-omit so there's
	// never an empty rail.
	let { title, href, items }: { title: string; href: string; items: Video[] } = $props();
</script>

{#if items.length > 0}
	<section class="space-y-2">
		<h2 class="skin-title text-sm font-semibold uppercase tracking-wide text-muted">
			More with <a {href} class="text-ink hover:text-accent">{title}</a>
		</h2>
		<!-- .video-grid resets the Brutalist `reel` counter (app.css) so the catalog
		     numbering restarts at 01 per shelf instead of continuing from the page. -->
		<div class="video-grid flex gap-4 overflow-x-auto pb-2">
			{#each items as video (video.id)}
				<div class="w-52 shrink-0 sm:w-56">
					<VideoCard {video} />
				</div>
			{/each}
		</div>
	</section>
{/if}
