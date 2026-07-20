<script lang="ts">
	// One dense field row in the Extraction review queue (F48.6, ADR-067), grouped
	// under its video by the parent page. Mirrors EnrichQueueRow/DuplicatePairRow's
	// rhythm. Two field shapes:
	//
	// • Entity fields (People/Studio) render one chip per parsed name (HOLODEX-196
	//   #1), each marked "exists" or "new". Click a chip to swap it to an existing
	//   entity or a corrected new name (fixes a typo, disambiguates two same-named
	//   people, corrects a studio) without disturbing the others; × removes one.
	//   "Accept cast" stages the whole edited list — editing one name can no longer
	//   collapse the field to a single value.
	// • Non-entity fields (Title, Release date) keep the scalar filename/tag/Edit UI.
	//
	// "Accept tag" and "Dismiss" never touch the file, so they resolve on click (the
	// row drops out); the accept actions stage a pending write the owner commits
	// together via the preview-before-write dialog (F48.7). Tokens only; QA 3 skins.
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
		/** Stage a pending write (not yet sent) — the row shows "selected", the parent
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

	// One editable chip per parsed name. `value` is what will be written
	// (an existing entity's canonical name, or the parsed/typed new name);
	// `existing` drives the exists/new badge.
	interface Chip {
		value: string;
		existing: boolean;
	}
	// The backend always builds `candidates` for entity fields (splitting the
	// filename value), so there's no non-empty-filename case the map misses.
	function initChips(): Chip[] {
		return (row.candidates ?? []).map((c) => ({ value: c.entity_name ?? c.name, existing: !!c.entity_id }));
	}
	// svelte-ignore state_referenced_locally — seeded once; the row is keyed by id.
	let chips = $state<Chip[]>(isEntityField ? initChips() : []);
	let editingChip = $state<number | null>(null);

	const castValue = $derived(chips.map((c) => c.value).join(', '));
	const newCount = $derived(chips.filter((c) => !c.existing).length);

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

	function updateChip(i: number, name: string, existing: boolean) {
		chips[i] = { value: name, existing }; // $state array: index assignment is reactive
		onunstage(); // any staged accept is now stale
	}
	function removeChip(i: number) {
		chips = chips.filter((_, j) => j !== i);
		onunstage();
	}
	function acceptCast() {
		if (chips.length === 0) return;
		onstage('manual', castValue);
	}

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
	class="flex flex-wrap items-start gap-x-3 gap-y-2 border-t border-rule px-3 py-2.5 text-sm"
	role="group"
	aria-label={`${row.video_title}: ${fieldLabel}`}
>
	<div class="flex min-w-0 flex-1 flex-col gap-1.5">
		<div class="flex flex-wrap items-center gap-x-2 gap-y-1">
			<span class="w-28 shrink-0 text-xs uppercase tracking-wide text-muted">{fieldLabel}</span>
			<span class="shrink-0 text-xs text-muted">{tier}</span>
		</div>

		{#if isEntityField}
			<!-- Chip list: one per parsed name, click to edit, × to remove. -->
			{#if chips.length}
				<div class="flex flex-wrap items-center gap-1.5">
					{#each chips as chip, i (i)}
						<span
							class="inline-flex items-center gap-1 rounded-full border py-0.5 pl-2 pr-1 text-sm {chip.existing
								? 'border-rule bg-surface-2 text-ink'
								: 'border-accent bg-accent/10 text-accent'}"
						>
							<button
								type="button"
								onclick={() => (editingChip = i)}
								disabled={busy}
								class="inline-flex items-center gap-1.5 hover:underline disabled:opacity-60"
								title="Edit this {entityLabel.toLowerCase()}"
							>
								{chip.value}
								<span class="text-xs {chip.existing ? 'text-muted' : 'text-accent'}"
									>{chip.existing ? 'exists' : 'new'}</span
								>
							</button>
							<button
								type="button"
								onclick={() => removeChip(i)}
								disabled={busy}
								aria-label={`Remove ${chip.value}`}
								class="rounded-full px-1 text-muted hover:text-ink disabled:opacity-60">✕</button
							>
						</span>
					{/each}
				</div>
			{:else}
				<p class="text-xs italic text-muted">No {entityLabel.toLowerCase()} parsed from the filename.</p>
			{/if}

			{#if row.tag_value}
				<p class="text-xs text-muted">file tag currently: {row.tag_value}</p>
			{/if}
		{:else if editing}
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

		{#if staged}
			<p class="text-xs text-accent">
				Selected: <span class="font-medium">{staged.value}</span> — pending write
				<button onclick={onunstage} class="ml-1 text-muted underline hover:text-ink">Undo</button>
			</p>
		{/if}

		{#if error}
			<span class="text-warn" role="alert">{error}</span>
		{/if}
	</div>

	<div class="flex shrink-0 flex-wrap items-center gap-2">
		{#if isEntityField}
			{#if chips.length}
				<button
					onclick={acceptCast}
					disabled={busy}
					class="text-accent hover:underline disabled:opacity-60"
					title={newCount ? `Writes ${chips.length}, creates ${newCount}` : `Writes ${chips.length}`}
				>
					Accept cast{newCount ? ` (${newCount} new)` : ''}
				</button>
			{/if}
		{:else}
			{#if row.filename_value}
				<button
					onclick={() => onstage('filename', row.filename_value)}
					disabled={busy}
					class="text-accent hover:underline disabled:opacity-60"
				>
					Accept filename
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
		{/if}
		{#if row.tag_value}
			<button onclick={acceptTag} disabled={busy} class="text-accent hover:underline disabled:opacity-60">
				Accept tag
			</button>
		{/if}
		<button onclick={doDismiss} disabled={busy} class="text-muted hover:text-ink disabled:opacity-60">
			Dismiss
		</button>
	</div>
</div>

{#if editingChip !== null}
	<EntityPickerDialog
		kind={entityKind}
		seedQuery={chips[editingChip]?.value ?? ''}
		onclose={() => (editingChip = null)}
		onselect={(name, existing) => {
			if (editingChip !== null) updateChip(editingChip, name, existing);
		}}
	/>
{/if}
