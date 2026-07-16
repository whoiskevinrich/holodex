<script lang="ts">
	// One dense field row in the Extraction review queue (F48.6, ADR-067), grouped
	// under its video by the parent page. Mirrors EnrichQueueRow/DuplicatePairRow's
	// rhythm. Two action classes: "Accept tag" and "Dismiss" never touch the file, so
	// they fire immediately (resolve-on-click, row drops out in place); "Accept
	// filename"/"Pick suggested"/"Edit…" stage a pending write instead of writing
	// right away — the owner reviews and commits staged picks together via the
	// preview-before-write dialog (F48.7) the parent page owns. Tokens only; QA 3 skins.
	import { toMessage } from '$lib/format';
	import EntityPickerDialog from './EntityPickerDialog.svelte';
	import type { ExtractionQueueRow, ExtractionResolveAction } from '$lib/types';

	let {
		row,
		fieldLabel,
		isEntityField,
		staged,
		onstage,
		onunstage,
		resolveTag,
		dismiss,
		onhandled
	}: {
		row: ExtractionQueueRow;
		fieldLabel: string;
		isEntityField: boolean;
		staged: { action: ExtractionResolveAction; value: string } | undefined;
		/** Stage a pending write (not yet sent) — the row shows "picked", the parent
		 *  page's "Review N changes" button appears. */
		onstage: (action: ExtractionResolveAction, value: string) => void;
		onunstage: () => void;
		/** Accept the tag value — no write, resolves immediately. */
		resolveTag: () => Promise<unknown>;
		dismiss: () => Promise<unknown>;
		/** The row is fully handled (tag accepted or dismissed) — parent drops it. */
		onhandled: () => void;
	} = $props();

	let busy = $state(false);
	let error = $state('');
	let editing = $state(false); // non-entity field: inline text edit
	// svelte-ignore state_referenced_locally — seeds the initial edit value only;
	// row is stable for this row instance's lifetime (keyed by row.id in the parent).
	let editValue = $state(row.filename_value || row.tag_value || '');

	const entityKind = $derived(row.field_key === 'people' ? 'person' : 'studio');
	const entityLabel = $derived(entityKind === 'person' ? 'Person' : 'Studio');

	// Same tier-label idiom as EnrichPicker.matchLabel() — informational only, never
	// gates which actions render. Both sides present and differing is a real
	// disagreement (Conflict); otherwise the raw score decides Strong vs. Weak.
	const tier = $derived.by(() => {
		if (
			row.filename_value &&
			row.tag_value &&
			row.filename_value.trim().toLowerCase() !== row.tag_value.trim().toLowerCase()
		) {
			return 'Conflict';
		}
		return row.confidence >= 0.7 ? 'Strong' : 'Weak';
	});

	async function acceptTag() {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await resolveTag();
			onhandled();
		} catch (e) {
			error = toMessage(e);
			busy = false;
		}
	}

	async function doDismiss() {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await dismiss();
			onhandled();
		} catch (e) {
			error = toMessage(e);
			busy = false;
		}
	}

	function saveEdit() {
		const v = editValue.trim();
		if (!v) return;
		onstage('manual', v);
		editing = false;
	}
</script>

<div
	class="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-rule px-3 py-2.5 text-sm"
	role="group"
	aria-label={`${row.video_title}: ${fieldLabel}`}
>
	<div class="flex min-w-0 flex-1 flex-col gap-1">
		<div class="flex flex-wrap items-center gap-x-2 gap-y-1">
			<span class="w-28 shrink-0 text-xs uppercase tracking-wide text-muted">{fieldLabel}</span>
			<span class="shrink-0 text-xs text-muted">{tier}</span>
		</div>

		{#if editing && !isEntityField}
			<div class="flex flex-wrap items-center gap-2">
				<input
					type="text"
					bind:value={editValue}
					class="min-w-0 flex-1 rounded-theme border border-rule bg-bg px-2 py-1 text-sm text-ink focus:outline-none focus:ring-1 focus:ring-accent"
				/>
				<button onclick={saveEdit} class="text-accent hover:underline">Save</button>
				<button onclick={() => (editing = false)} class="text-muted hover:text-ink">Cancel</button>
			</div>
		{:else}
			<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
				<span class="text-xs text-muted">filename:</span>
				{#if row.filename_value}
					<span class="text-ink">{row.filename_value}</span>
				{:else}
					<span class="italic text-muted">— (empty)</span>
				{/if}
				<span class="text-xs text-muted">tag:</span>
				{#if row.tag_value}
					<span class="text-muted">{row.tag_value}</span>
				{:else}
					<span class="italic text-muted">— (empty)</span>
				{/if}
			</div>
		{/if}

		{#if isEntityField}
			<p class="text-xs text-muted">
				{#if row.suggested_entity_name}
					suggested match — not applied: {row.suggested_entity_name}
				{:else}
					no match — will create new {entityLabel}
				{/if}
			</p>
		{/if}

		{#if staged}
			<p class="text-xs text-accent">
				Picked: <span class="font-medium">{staged.value}</span> — pending write
				<button onclick={onunstage} class="ml-1 text-muted underline hover:text-ink">Undo</button>
			</p>
		{/if}

		{#if error}
			<span class="text-warn" role="alert">{error}</span>
		{/if}
	</div>

	<div class="flex shrink-0 flex-wrap items-center gap-2">
		{#if row.filename_value}
			<button
				onclick={() => onstage('filename', row.filename_value)}
				disabled={busy}
				class="text-accent hover:underline disabled:opacity-60"
			>
				Accept filename
			</button>
		{/if}
		{#if row.tag_value}
			<button onclick={acceptTag} disabled={busy} class="text-accent hover:underline disabled:opacity-60">
				Accept tag
			</button>
		{/if}
		{#if isEntityField && row.suggested_entity_name}
			{@const suggested = row.suggested_entity_name}
			<button
				onclick={() => onstage('manual', suggested)}
				disabled={busy}
				class="text-accent hover:underline disabled:opacity-60"
			>
				Pick suggested: {suggested}
			</button>
		{/if}
		{#if !editing}
			<button
				onclick={() => (editing = true)}
				disabled={busy}
				class="text-accent hover:underline disabled:opacity-60"
			>
				Edit…
			</button>
		{/if}
		<button onclick={doDismiss} disabled={busy} class="text-muted hover:text-ink disabled:opacity-60">
			Dismiss
		</button>
	</div>
</div>

{#if editing && isEntityField}
	<EntityPickerDialog
		kind={entityKind}
		seedQuery={row.suggested_entity_name || row.filename_value}
		onclose={() => (editing = false)}
		onselect={(name) => onstage('manual', name)}
	/>
{/if}
