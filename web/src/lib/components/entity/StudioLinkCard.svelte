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
	// A logo sits bare on the page background (HOLODEX-411): logos are transparent marks, and
	// the light plate read as a cream box floating on the dark skins. The icon and monogram
	// fallbacks keep the plate — it exists so an arbitrary square mark has something to sit on.
	const bare = $derived(Boolean(studio.logo_url));

	// A wide logo (≥ 2:1) is a wordmark, so the name beside it would say the name twice
	// (HOLODEX-411 aspect rule): the caption is decided from the loaded image's natural
	// aspect and hidden for wordmarks; a squarer logo (a symbol) keeps it, as do the icon and
	// monogram states. Until a logo has loaded nothing renders beside it, so a wordmark row
	// never flashes a caption; a failed load falls back to showing the name.
	let wordmark = $state<boolean | null>(null);
	let img = $state<HTMLImageElement | undefined>();
	const showName = $derived(!bare || wordmark === false);

	function decide() {
		if (!img) return;
		wordmark = img.naturalHeight > 0 && img.naturalWidth >= 2 * img.naturalHeight;
	}
	$effect(() => {
		// Re-decide per source; a cached image can be complete before `onload` attaches.
		wordmark = null;
		if (bare && image && img?.complete && img.naturalWidth > 0) decide();
	});
</script>

<a
	href={`/studios/${studio.id}`}
	class="flex items-center gap-3 hover:text-accent"
	title={showName ? undefined : `${studio.name} · ${videoCount(studio.video_count ?? 0)}`}
>
	<span
		class="flex h-12 max-w-48 min-w-12 shrink-0 items-center justify-center overflow-hidden rounded-theme {bare
			? ''
			: 'border border-rule bg-logo-plate'} {image ? '' : 'w-12 border-dashed'}"
	>
		{#if image}
			<img
				bind:this={img}
				src={image}
				alt={showName ? '' : studio.name}
				class="h-full w-auto max-w-full object-contain p-1"
				onload={decide}
				onerror={() => (wordmark = false)}
			/>
		{:else}
			<span class="font-display text-sm font-semibold text-logo-plate-ink" aria-hidden="true">
				{monogram(studio.name)}
			</span>
		{/if}
	</span>
	{#if showName}
		<span class="min-w-0">
			<span class="block truncate text-ink group-hover:text-accent">{studio.name}</span>
			<span class="block text-xs text-muted">{videoCount(studio.video_count ?? 0)}</span>
		</span>
	{/if}
</a>
