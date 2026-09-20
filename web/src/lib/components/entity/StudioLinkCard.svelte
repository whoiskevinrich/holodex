<script lang="ts">
	import { videoCount } from '$lib/format';
	import type { Studio } from '$lib/types';
	import StudioLogoBox from './StudioLogoBox.svelte';
	import { showName as showNameFor, type Wordmark } from './studioLogo';

	let { studio }: { studio: Studio } = $props();

	// Caption rule (HOLODEX-411): a wordmark logo carries its own name, so the name + count
	// caption is dropped and moves to the image alt + link title. See studioLogo.ts.
	let wordmark = $state<Wordmark>(null);
	const showName = $derived(showNameFor(Boolean(studio.logo_url), wordmark));
</script>

<a
	href={`/studios/${studio.id}`}
	class="flex items-center gap-3 hover:text-accent"
	title={showName ? undefined : `${studio.name} · ${videoCount(studio.video_count ?? 0)}`}
>
	<StudioLogoBox {studio} bind:wordmark />
	{#if showName}
		<span class="min-w-0">
			<span class="block truncate text-ink group-hover:text-accent">{studio.name}</span>
			<span class="block text-xs text-muted">{videoCount(studio.video_count ?? 0)}</span>
		</span>
	{/if}
</a>
