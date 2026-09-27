<script lang="ts">
	// Reusable tag display (HOLODEX-292, design handoff tag-link-chip-handoff.md):
	// linked name + optional remove control, one chip per tag.
	// Owner vs. read-only is decided by whether the caller passes `onremove` — no
	// separate isOwner boolean to keep in sync with it, since both known call sites
	// (Media, Film) only ever need those two states.
	//
	// The owner chip leads with an on-file glyph (HOLODEX-401, tag-set-writeback-handoff.md §2):
	// whether the tag is written to the file and whether the file has it now. It replaces the
	// old ·source suffix — provenance is noise to the owner (ADR-090: tags have no layer 2).
	// No glyph while the file's tags are unknown (`on_file` absent) or off the media page
	// (`written` absent).
	import type { Tag } from '$lib/types';

	let {
		tag,
		busy = false,
		onremove
	}: {
		tag: Tag;
		busy?: boolean;
		onremove?: (tagId: number) => void;
	} = $props();

	type Glyph = { mark: string; tone: string; label: string };
	let glyph = $derived.by((): Glyph | null => {
		if (tag.on_file === undefined || tag.written === undefined) return null;
		if (tag.written && tag.on_file) return { mark: 'M5.5 10l2 2 3-4', tone: 'text-muted', label: 'On the file' };
		if (tag.written)
			return {
				mark: 'M8 7.5v5M5.5 10h5',
				tone: 'text-accent',
				label: 'Not on the file yet — added on the next writeback'
			};
		if (tag.on_file)
			return {
				mark: 'M5.5 10h5',
				tone: 'text-warn',
				label: 'On the file — writeback is off, so the next writeback removes it'
			};
		return { mark: 'M1 1l14 14', tone: 'text-muted', label: 'Kept in Holodex only — writeback is off for this tag' };
	});
</script>

{#if onremove}
	<span
		class="curation-chip group relative inline-flex items-center gap-1 rounded-full border border-rule bg-surface-2 px-2.5 py-1 text-sm text-ink"
	>
		{#if glyph}
			<span class="inline-flex {glyph.tone}" title={glyph.label}>
				<svg
					class="h-3.5 w-3.5"
					viewBox="0 0 16 16"
					fill="none"
					stroke="currentColor"
					stroke-width="1.6"
					stroke-linecap="round"
					stroke-linejoin="round"
					aria-hidden="true"
				>
					<path d="M9 1H3.5A1.5 1.5 0 0 0 2 2.5v11A1.5 1.5 0 0 0 3.5 15h9a1.5 1.5 0 0 0 1.5-1.5V6z" />
					<path d={glyph.mark} />
				</svg>
			</span>
		{/if}
		<a href={`/tags/${tag.id}`} class="hover:text-accent focus-visible:text-accent"
			>{tag.name}{#if glyph}<span class="sr-only">, {glyph.label}</span>{/if}</a
		>
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
