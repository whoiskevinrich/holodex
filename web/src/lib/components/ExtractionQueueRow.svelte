<script lang="ts">
	// One dense field row in the Extraction review queue (F48.6, ADR-067), grouped
	// under its video by the parent page. Shares EnrichQueueRow/DuplicatePairRow's
	// container rhythm (border-t rule, flex-1 content + shrink-0 actions) but not
	// their action styling: this is the only queue row with multi-row staging and a
	// batch commit, so its controls carry that extra state. Two field shapes:
	//
	// • Entity fields (People/Studio) render one chip per parsed name (HOLODEX-196
	//   #1), each marked "exists" or "new". Click a chip to swap it to an existing
	//   entity or a corrected new name (fixes a typo, disambiguates two same-named
	//   people, corrects a studio) without disturbing the others; × removes one.
	//   The Stage control sits at the end of that same chip line so the edit→confirm
	//   loop never leaves the chip cluster (HOLODEX-199 — it used to live at the
	//   row's far right edge, ~700px away from what it commits).
	// • Non-entity fields (Title, Release date) keep the scalar filename/tag/Edit UI.
	//
	// Two commit semantics share this row, so they are kept visually unlike:
	// staging is an accent-filled pill inside the content column; "Keep tag" and
	// "Dismiss" never touch the file and resolve on click (the row drops out), so
	// they are muted ghost buttons at the row's right edge. Staged picks are written
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
		/** Stage a pending write (not yet sent) — the row's pill flips to "Staged" and
		 *  the page's commit bar counts it. */
		onstage: (action: ExtractionResolveAction, value: string) => void;
		onunstage: () => void;
		/** Keep the tag value — no write, resolves immediately. */
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
	// disagreement (conflict); otherwise the raw score decides strong vs. weak.
	// Only the two that need attention are modelled: a strong match is the
	// unremarkable case and resolves to null, so it gets no badge at all (it used to
	// render muted text identical to the field label beside it, which made a real
	// conflict invisible).
	const tier = $derived.by((): 'conflict' | 'weak' | null => {
		if (
			row.filename_value &&
			row.tag_value &&
			row.filename_value.trim().toLowerCase() !== row.tag_value.trim().toLowerCase()
		) {
			return 'conflict';
		}
		return row.confidence >= 0.7 ? null : 'weak';
	});

	function updateChip(i: number, name: string, existing: boolean) {
		chips[i] = { value: name, existing }; // $state array: index assignment is reactive
		onunstage(); // any staged pick is now stale
	}
	function removeChip(i: number) {
		chips = chips.filter((_, j) => j !== i);
		onunstage();
	}

	// The one staging control, shared by both field shapes: entity rows stage the
	// whole edited chip list, scalar rows stage the filename value (the tag value is
	// reachable via "Keep tag", a free-value edit via "Edit…"). The new-entity count
	// rides the label rather than a `title` tooltip, which touch and keyboard never see.
	const stageValue = $derived(isEntityField ? castValue : row.filename_value);
	const stageLabel = $derived(
		isEntityField ? `Stage ${chips.length}${newCount ? ` (${newCount} new)` : ''}` : 'Stage filename'
	);

	// Pill shapes, sized to sit among the rounded-full chips. Unstaged is outlined so
	// the solid accent stays reserved for a page's one primary action (the commit
	// bar) — filling in *is* the staged state, so the two never read alike.
	const PILL = 'inline-flex min-h-6 items-center rounded-full px-2.5 text-xs font-medium';
	const PILL_STAGED = `${PILL} bg-accent text-accent-ink`;
	const PILL_STAGE = `${PILL} border border-accent text-accent hover:bg-accent/10 disabled:border-rule disabled:text-muted`;

	// Immediate, row-clearing resolves — bordered so they never read as staging.
	// Disabled drops the border rather than dimming the text: a blanket
	// `opacity-60` on `text-muted` falls to ~2.9:1 in the Broadcast skin.
	const GHOST =
		'inline-flex min-h-6 items-center rounded-theme border border-rule px-2 text-xs text-muted hover:bg-surface-2 hover:text-ink disabled:border-transparent';
	// Neutral UI toggles (no side effect at all) — borderless, so GHOST keeps meaning
	// exactly one thing: this resolves the row right now.
	const TOGGLE = 'inline-flex min-h-6 items-center text-xs text-muted hover:text-ink hover:underline';

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

{#snippet stageControl()}
	{#if staged}
		<span class="{PILL_STAGED} gap-2">
			{isEntityField ? 'Staged' : `Staged: ${staged.value}`}
			<button onclick={onunstage} disabled={busy} class="underline hover:no-underline">Undo</button>
		</span>
	{:else}
		<button
			onclick={() => onstage(isEntityField ? 'manual' : 'filename', stageValue)}
			disabled={busy || !stageValue}
			class={PILL_STAGE}
		>
			{stageLabel}
		</button>
	{/if}
{/snippet}

<div
	class="flex flex-wrap items-start gap-x-3 gap-y-2 border-t border-rule px-3 py-2.5 text-sm"
	role="group"
	aria-label={`${row.video_title}: ${fieldLabel}`}
>
	<div class="flex min-w-0 flex-1 flex-col gap-1.5">
		<div class="flex flex-wrap items-center gap-x-2 gap-y-1">
			<span class="w-28 shrink-0 text-xs uppercase tracking-wide text-muted">{fieldLabel}</span>
			{#if tier === 'conflict'}
				<span class="shrink-0 rounded-theme border border-warn px-1.5 text-xs text-warn">conflict</span>
			{:else if tier === 'weak'}
				<span class="shrink-0 text-xs text-muted">weak match</span>
			{/if}
		</div>

		{#if isEntityField}
			<!-- Chip list: one per parsed name, click to edit, × to remove. The Stage
			     control trails the last chip so confirming is a short hop, not a
			     row-width traverse. -->
			{#if chips.length}
				<div class="flex flex-wrap items-center gap-1.5">
					{#each chips as chip, i (i)}
						<span
							class="inline-flex items-center gap-1 rounded-full border pl-2.5 text-sm {chip.existing
								? 'border-rule bg-surface-2 text-ink'
								: 'border-accent bg-accent/10 text-accent'}"
						>
							<button
								type="button"
								onclick={() => (editingChip = i)}
								disabled={busy}
								class="inline-flex min-h-6 items-center gap-1.5 hover:underline disabled:no-underline"
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
								class="flex h-6 w-6 items-center justify-center rounded-full text-muted hover:text-ink">✕</button
							>
						</span>
					{/each}
					{@render stageControl()}
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
				<button onclick={saveEdit} class={PILL_STAGE}>Stage edit</button>
				<button onclick={() => (editing = false)} class={TOGGLE}>Cancel</button>
			</div>
		{:else}
			<div class="flex flex-wrap items-center gap-x-3 gap-y-1.5">
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
				{@render stageControl()}
				<button onclick={() => (editing = true)} disabled={busy} class={TOGGLE}>Edit…</button>
			</div>
		{/if}

		{#if error}
			<span class="text-warn" role="alert">{error}</span>
		{/if}
	</div>

	<div class="flex shrink-0 flex-wrap items-center gap-2">
		{#if row.tag_value}
			<button onclick={acceptTag} disabled={busy} class={GHOST}>Keep tag</button>
		{/if}
		<button onclick={doDismiss} disabled={busy} class={GHOST}>Dismiss</button>
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
