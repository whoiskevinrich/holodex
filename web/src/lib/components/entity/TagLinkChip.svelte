<script lang="ts">
	// Reusable tag display (HOLODEX-292, design handoff tag-link-chip-handoff.md):
	// linked name + provenance suffix + optional remove control, one chip per tag.
	// Owner vs. read-only is decided by whether the caller passes `onremove` — no
	// separate isOwner boolean to keep in sync with it, since both known call sites
	// (Media, Film) only ever need those two states.
	import type { Tag } from '$lib/types';
	import { isProviderSource, providerOf } from '$lib/f36';

	let {
		tag,
		busy = false,
		onremove
	}: {
		tag: Tag;
		busy?: boolean;
		onremove?: (tagId: number) => void;
	} = $props();

	let sourceIsProvider = $derived(!!tag.source && isProviderSource(tag.source));
	let sourceLabel = $derived(sourceIsProvider ? providerOf(tag.source!) : tag.source);
</script>

{#if onremove}
	<span
		class="curation-chip group relative inline-flex items-center gap-1 rounded-full border border-rule bg-surface-2 px-2.5 py-1 text-sm text-ink"
	>
		<a href={`/tags/${tag.id}`} class="hover:text-accent focus-visible:text-accent">{tag.name}</a>
		{#if tag.source && tag.source !== 'manual'}
			<span class="{sourceIsProvider ? 'text-accent' : 'text-muted'} text-[0.65rem]">
				·{sourceLabel}
			</span>
		{/if}
		<span class="curation-actions ml-0.5 inline-flex items-center">
			<button
				type="button"
				onclick={() => onremove?.(tag.id)}
				disabled={busy}
				aria-label={`Remove tag ${tag.name}`}
				title={tag.source === 'file'
					? 'Removing a file-sourced tag may reappear on the next rescan'
					: undefined}
				class="rounded p-0.5 -m-0.5 text-muted hover:text-accent focus-visible:text-accent"
			>
				×
			</button>
		</span>
	</span>
{:else}
	<a
		href={`/tags/${tag.id}`}
		class="rounded-full border border-rule bg-surface-2 px-2.5 py-1 text-sm text-ink hover:text-accent focus-visible:text-accent"
	>
		{tag.name}
	</a>
{/if}
