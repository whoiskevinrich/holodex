<script lang="ts">
	import type { Studio } from '$lib/types';
	import CompletenessRing from '$lib/components/completeness/CompletenessRing.svelte';
	import StudioLogoBox from './StudioLogoBox.svelte';
	import { showName as showNameFor, type Wordmark } from './studioLogo';

	// One /studios index row (HOLODEX-432, docs/design/studio-list-logo-handoff.md): the
	// StudioLinkCard image box (logo → icon → monogram, bare + halo, aspect-following) in front
	// of the name, the owner-only completeness ring and the video count. Same caption rule as
	// the card: a wordmark logo already says the name, so the text is dropped and the name moves
	// to the image alt + link title — announced once either way. A component rather than inline
	// `{#each}` markup only because the wordmark decision is per-row state.
	//
	// Row shape (F65.8): a positioned wrapper carries the border, the <a> is stretched over it
	// via ::after so the whole row still navigates, and the ring — a <button> since F65.8 —
	// sits as a sibling above the stretch rather than inside the link.
	let { studio, eager = true }: { studio: Studio; eager?: boolean } = $props();

	let wordmark = $state<Wordmark>(null);
	const showName = $derived(showNameFor(Boolean(studio.logo_url), wordmark));
</script>

<div
	class="relative flex items-center gap-3 rounded-theme border border-rule bg-surface px-4 py-2.5 text-ink hover:border-accent has-[a:focus-visible]:border-accent"
>
	<a
		href={`/studios/${studio.id}`}
		class="flex min-w-0 flex-1 items-center gap-3 after:absolute after:inset-0 after:content-['']"
		title={showName ? undefined : studio.name}
	>
		<StudioLogoBox {studio} {eager} bind:wordmark />
		<span class="min-w-0 flex-1 truncate">{#if showName}{studio.name}{/if}</span>
	</a>
	{#if studio.completeness}
		<!-- Owner-only by payload (F65.4/F65.5): trailing, before the count. -->
		<span class="relative z-[1] inline-flex">
			<CompletenessRing
				required={studio.completeness.required}
				extras={studio.completeness.extras}
				size="row"
				entity={{ kind: 'studio', id: studio.id }}
			/>
		</span>
	{/if}
	<span class="text-xs text-muted">{studio.video_count}</span>
</div>
