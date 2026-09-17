<script lang="ts">
	// Reusable studio display (HOLODEX-290, design handoff studio-link-card-handoff.md):
	// image + linked name + video count, one card per studio. Read-only — the caller
	// composes its own StudioPicker/cascade pencil beside this, unchanged.
	import { monogram, videoCount } from '$lib/format';
	import type { Studio } from '$lib/types';

	let { studio }: { studio: Studio } = $props();

	// Logo first (HOLODEX-397, studio-logo-link-card-handoff.md): it is the role enrichment
	// fills; the icon is the /studios list well's square. The plate follows the image's own
	// aspect — fixed height, width clamped to [48px, 192px] — never a cover-crop.
	const image = $derived(studio.logo_url || studio.icon_url);
</script>

<a href={`/studios/${studio.id}`} class="flex items-center gap-3 hover:text-accent">
	<span
		class="flex h-12 max-w-48 min-w-12 shrink-0 items-center justify-center overflow-hidden rounded-theme border border-rule bg-logo-plate {image
			? ''
			: 'w-12 border-dashed'}"
	>
		{#if image}
			<img src={image} alt="" class="h-full w-auto max-w-full object-contain p-1" />
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
