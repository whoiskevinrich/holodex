<script lang="ts">
	import type { Video } from '$lib/types';
	import { formatDuration, resolutionBucket } from '$lib/format';

	let { video }: { video: Video } = $props();

	const bucket = $derived(resolutionBucket(video.width));
</script>

<a href={`/media/${video.id}`} class="group block">
	<!-- Cover placeholder (embedded cover art lands in Phase 2 — ADR-009).
	     `.video-frame` carries the per-skin flourishes (letterbox, scanlines,
	     index counter) from app.css. -->
	<div class="video-frame flex items-center justify-center transition group-hover:border-accent">
		<svg class="h-10 w-10 text-rule" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
			<path d="M8 5v14l11-7z" />
		</svg>
		<span
			class="absolute bottom-1.5 right-1.5 z-[2] rounded-theme bg-black/70 px-1.5 py-0.5 text-xs tabular-nums text-ink"
		>
			{formatDuration(video.duration_sec)}
		</span>
		{#if video.width > 0}
			<span
				class="absolute left-1.5 top-1.5 z-[2] rounded-theme bg-accent px-1.5 py-0.5 text-[10px] font-semibold text-accent-ink"
			>
				{bucket}
			</span>
		{/if}
	</div>

	<div class="space-y-1.5 p-3">
		<h3 class="skin-title line-clamp-2 text-sm font-medium text-ink" title={video.title}>
			{video.title}
		</h3>
		{#if video.tags?.length}
			<div class="flex flex-wrap gap-1">
				{#each video.tags.slice(0, 3) as tag (tag.id)}
					<span class="rounded-theme bg-surface-2 px-1.5 py-0.5 text-[10px] text-muted">{tag.name}</span>
				{/each}
			</div>
		{/if}
	</div>
</a>
