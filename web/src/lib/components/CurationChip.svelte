<script lang="ts">
	// One value chip in a curated field (F30). Shows the value, its provenance, and
	// — for the owner — inline edit, remove (suppress), and a "don't write" toggle.
	// Owner controls are revealed on hover/focus to keep dense fields calm (they stay
	// in the DOM + focusable for keyboard/SR). Tokens only; QA 3 skins. Values render
	// as plain text (Svelte auto-escapes; never {@html}) per security condition C4.
	//
	// When `person` is supplied (actor/director fields), the value is a link to the
	// person page. Faces live in the page's dedicated People section, so the chip stays
	// text-only here to avoid showing the same cast twice in two treatments.
	import type { ResolvedValue } from '$lib/types';

	let {
		item,
		isOwner,
		showRemove = true,
		person,
		onedit,
		onremove,
		ontogglewrite
	}: {
		item: ResolvedValue;
		isOwner: boolean;
		showRemove?: boolean;
		person?: { id: number; headshot_version?: number };
		// Owner-only mutation handlers — optional, so a read-only reuse (isOwner={false}, e.g.
		// the F36 resolved chip) needs no no-op props. They are only invoked inside the isOwner
		// block, so an owner caller must still supply them.
		onedit?: (oldValue: string, newValue: string) => void;
		onremove?: (value: string) => void;
		ontogglewrite?: (value: string, noWrite: boolean) => void;
	} = $props();

	let editing = $state(false);
	let draft = $state('');

	// Provenance: providers read accented; file/manual baseline read muted.
	const isProvider = $derived(item.sources.some((s) => s !== 'file' && s !== 'manual'));
	const provenance = $derived(item.manual ? 'manual' : item.sources.join(' + '));

	function startEdit() {
		draft = item.value;
		editing = true;
	}
	function commitEdit() {
		const next = draft.trim();
		editing = false;
		if (next && next !== item.value) onedit?.(item.value, next);
	}
	function onEditKey(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			commitEdit();
		} else if (e.key === 'Escape') {
			e.preventDefault();
			editing = false;
		}
	}

	// Hit-area padding keeps the tap target ~20px while the glyph stays 12px; the
	// negative margin absorbs the padding so chip layout doesn't shift.
	const ctrl = 'rounded p-1 -m-0.5 text-muted hover:text-accent focus-visible:text-accent';
</script>

{#if editing}
	<!-- svelte-ignore a11y_autofocus -->
	<input
		bind:value={draft}
		onkeydown={onEditKey}
		onblur={commitEdit}
		autofocus
		aria-label="Edit value"
		class="inline-block w-32 rounded-theme border border-rule bg-bg px-2 py-0.5 text-xs text-ink focus:outline-none focus:ring-1 focus:ring-accent"
	/>
{:else}
	<span
		class="curation-chip inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs {item.no_write
			? 'border-rule text-muted line-through opacity-70'
			: 'border-rule bg-surface-2 text-ink'}"
		aria-label={`${item.value}, from ${provenance}${item.no_write ? ', not written to file' : ''}`}
	>
		{#if person}
			<a href={`/people/${person.id}`} class="hover:text-accent focus-visible:text-accent">{item.value}</a>
		{:else}
			<span>{item.value}</span>
		{/if}
		<span class="{isProvider ? 'text-accent' : 'text-muted'} text-[0.65rem]">·{provenance}</span>

		{#if isOwner}
			<!-- Controls reveal on hover/focus (.curation-actions in app.css) to reduce
			     per-chip density (critique #1); always shown on touch. -->
			<span class="curation-actions ml-0.5 inline-flex items-center gap-0.5">
				<button type="button" onclick={startEdit} aria-label={`Edit ${item.value}`} class={ctrl}>
					<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg>
				</button>
				<!-- "Don't write" toggle: a document glyph (distinct from the section's
				     download-arrow Write button); aria-pressed + the chip's strikethrough
				     carry the state. -->
				<button
					type="button"
					onclick={() => ontogglewrite?.(item.value, !item.no_write)}
					aria-pressed={item.no_write}
					title={item.no_write ? 'Include in file write' : "Don't write to file"}
					aria-label={item.no_write ? `Include ${item.value} in file write` : `Exclude ${item.value} from file write`}
					class={ctrl}
				>
					<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M9 13h6m-6 4h6m4 4H5a2 2 0 01-2-2V5a2 2 0 012-2h8l6 6v10a2 2 0 01-2 2z"/></svg>
				</button>
				{#if showRemove}
					<button type="button" onclick={() => onremove?.(item.value)} aria-label={`Remove ${item.value}`} class={ctrl}>
						<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/></svg>
					</button>
				{/if}
			</span>
		{/if}
	</span>
{/if}
