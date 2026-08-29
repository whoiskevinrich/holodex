<script lang="ts">
	// Reusable studio display (HOLODEX-290, design handoff studio-link-card-handoff.md):
	// icon + linked name + video count, one card per studio. Read-only — the caller
	// composes its own StudioPicker/cascade pencil beside this, unchanged.
	import { monogram, videoCount } from '$lib/format';
	import type { Studio } from '$lib/types';

	let { studio }: { studio: Studio } = $props();
</script>

<a href={`/studios/${studio.id}`} class="flex items-center gap-3 hover:text-accent">
	<span
		class="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-theme border border-rule bg-logo-plate {studio.icon_url
			? ''
			: 'border-dashed'}"
	>
		{#if studio.icon_url}
			<img src={studio.icon_url} alt="" class="h-full w-full object-contain p-1" />
		{:else}
			<span class="font-display text-sm font-semibold text-logo-plate-ink" aria-hidden="true">
				{monogram(studio.name)}
			</span>
		{/if}
	</span>
	<span class="min-w-0">
		<span class="block truncate text-ink group-hover:text-accent">{studio.name}</span>
		<span class="block text-xs text-muted">{videoCount(studio.video_count ?? 0)}</span>
	</span>
</a>
