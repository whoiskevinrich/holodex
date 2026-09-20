<script lang="ts">
	// Batch writeback form (F28 UX revision; ADR-091/HOLODEX-323 confirm-step
	// revision; HOLODEX-400 cockpit revision). Opens as a modal showing all writable
	// resolved fields. A replace field whose value differs from the file renders its
	// candidate CHOOSER — the same staged chip row / stacked rows the page's SourceBadge
	// and SourceEditModal use — and the Write button is that chooser's Confirm: picks
	// stage locally, submit() commits the decision, then enqueues the write. The write is
	// atomic over everything DECIDED: a standing decision that lags the file is always
	// written, an undecided provider value is written only once the owner picks it here
	// (picking is the confirm — there is no checkbox and no "select all"), an undecided row
	// nobody touched is never written, and a field with nothing to decide here (merge) is
	// listed read-only and never written. Image fields are cockpit rows with a tile chooser
	// (HOLODEX-403; their sync is witnessed by the write ledger, ADR-101). The dialog is the
	// cockpit for the GOLDEN
	// RECORD — Holodex's own truth for the video — of which only the mapped subset reaches the
	// file: an unmapped field (no file tag for this container) still gets the chooser, and its
	// pick is saved in Holodex alone; the gutter names which destination Write will touch
	// (file arrow / system cylinder / nothing). The write itself is fire-and-forget
	// (ADR-091): this dialog is a pre-flight confirm step that closes the instant the
	// write is *enqueued*, not once it lands — outcome (pending/failed) is a page-level
	// signal near the Metadata section, not this dialog's job to poll or display. Focus
	// is trapped + returned; Escape closes when idle (or once the single enqueue round
	// trip finishes). Tokens only; QA 3 skins. Design: docs/design/writeback-cockpit-handoff.md.
	import { onMount } from 'svelte';
	import { toMessage, providerFromWinningSource } from '$lib/format';
	import {
		fileCandidateValue,
		isWritable,
		needsWriteback,
		resolveSelection,
		sourceChips,
		type SourceChip
	} from '$lib/f36';
	import {
		isBlankCustom,
		isCockpitRow,
		isImageRow,
		isUnverifiable,
		needsDecision,
		rowClass,
		savesDecisionOnly,
		stagedValue,
		willWrite
	} from '$lib/writebackCockpit';
	import type { DecisionSource, ResolvedField, WritebackRequest } from '$lib/types';
	import SourceChipRow from '../curation/SourceChipRow.svelte';
	import SourceImageTiles from '../curation/SourceImageTiles.svelte';
	import SourceRadioList from '../curation/SourceRadioList.svelte';

	let {
		fields,
		videoId,
		filePath,
		entityImage,
		entityImageUploaded = false,
		onclose,
		onenqueued,
		writeback,
		decide
	}: {
		fields: ResolvedField[];
		videoId: number;
		filePath: string;
		// The video's own served poster (F53 route) — what the image chooser's `·file` tile
		// shows (HOLODEX-403). The file candidate's VALUE is usually '' (no `file:` source in
		// the mapping); the tile still has an image to show, this one. Omitted → placeholder.
		entityImage?: string;
		// true when that served poster is an owner upload (F52): it then is NOT the file's
		// cover art (the upload overwrote the extracted tier), so the tile is a placeholder and
		// the row carries the ADR-049 note (ADR-101 D4).
		entityImageUploaded?: boolean;
		onclose: () => void;
		// Fires once the write is accepted (the 202 ack) — before anything has
		// actually been written (ADR-091) — or, when only re-pointed-to-file decisions
		// were saved, once those land. The caller reloads to pick up the new pending
		// writeback_status row (if any) and the decisions submit() created; it does not
		// learn which fields ultimately wrote, since the job is atomic and its outcome is
		// reported on the page, not here.
		onenqueued: () => void;
		writeback: (id: number, req: WritebackRequest) => Promise<unknown>;
		// Creates or replaces the standing decision for one field (HOLODEX-273/400) — DB-only, no reload.
		decide: (canonical: string, source: DecisionSource, manualValue?: string) => Promise<unknown>;
	} = $props();

	interface Row {
		field: ResolvedField;
		// Cockpit rows (isCockpitRow): the field's SourceChip model + committed selection, and
		// the LOCAL staged pick the chooser binds to. Nothing here hits the network until submit.
		chips: SourceChip[];
		selection: { key: string; pending: boolean };
		stagedKey: string | null;
		stagedCustomValue: string;
		// Non-cockpit rows (merge) keep their seeded text `value`; for a cockpit row `value` is
		// unused and rowValue() reads the staged pick instead.
		value: string;
		// Open-time value, so the undecided group's tier sort never moves a row mid-edit.
		originalValue: string;
		// Cockpit rows: the owner picked a chip in THIS dialog. Picking is the confirm — an
		// undecided row is written (and decided) only once touched. Never reset within a dialog
		// lifetime; Cancel is the undo.
		touched: boolean;
		// A row that matches the file collapses to the "=" tier; `change` opens its chooser on
		// demand (handoff §1, M → W promotion).
		chooserOpen: boolean;
	}

	function seedRow(f: ResolvedField): Row {
		const chips = isCockpitRow(f) ? sourceChips(f) : [];
		const selection = isCockpitRow(f) ? resolveSelection(f, chips) : { key: 'file', pending: false };
		const stagedKey = isCockpitRow(f) ? selection.key : null;
		const stagedCustomValue = f.decision?.source === 'manual' ? (f.decision.manual_value ?? '') : '';
		const seed = isCockpitRow(f)
			? stagedValue(chips, { key: stagedKey, custom: stagedCustomValue })
			: f.values.join(', ');
		return {
			field: f,
			chips,
			selection,
			stagedKey,
			stagedCustomValue,
			value: seed,
			originalValue: seed,
			touched: false,
			chooserOpen: false
		};
	}

	// The rows that write on open are the standing decisions the file lags (leadRow) — the
	// header's "· {n} out of sync" set plus any decided row whose sync state cannot be read
	// back (ADR-093). Everything else is listed but inert until the owner acts: a cockpit row
	// writes once a chip is picked (willWrite) — a poster tile included, whose write triggers
	// the server-side download + cover-art embed. Merge rows have nothing to decide here (no
	// decision model, RD1; tags as a set is HOLODEX-401), so they are read-only.
	// svelte-ignore state_referenced_locally — fields prop is stable for the dialog's lifetime
	const rows = $state<Row[]>(fields.map(seedRow));

	// The value this row would write right now: the staged pick for a cockpit row, the seeded
	// text otherwise. Every "does it match the file" test below reads this, never the frozen
	// in_sync snapshot, so the gutter and submit() can't disagree.
	function rowValue(row: Row): string {
		return isCockpitRow(row.field)
			? stagedValue(row.chips, { key: row.stagedKey, custom: row.stagedCustomValue })
			: row.value;
	}

	// True when the row's LIVE value already matches the file's own tag value —
	// "matches the file, nothing to write" (the non-checkable gutter tier, R4.3).
	function rowMatchesFile(row: Row): boolean {
		return rowClass(row.field, rowValue(row)) === 'matches';
	}

	function stagedOf(row: Row) {
		return { staged: { key: row.stagedKey, custom: row.stagedCustomValue }, touched: row.touched };
	}

	// The one predicate behind the footer count and submit()'s write set (pure: willWrite).
	function rowWillWrite(row: Row): boolean {
		return willWrite(row.field, rowValue(row), stagedOf(row));
	}

	// writeCount is recomputed from the LIVE staged picks, so the footer never promises a
	// field submit() will skip (a row staged back onto the file value drops out immediately).
	const writeCount = $derived(rows.filter(rowWillWrite).length);

	// Rows whose pick is saved in Holodex but never reaches the file (pure: savesDecisionOnly):
	// an unmapped field the owner decided here, or a mapped row re-pointed at the file's own
	// value. Both are golden-record edits; dropping them would leave the old decision standing
	// (and the page reading "out of sync"). They ride along with a write, or save on their own.
	function rowDecisionOnly(row: Row): boolean {
		return savesDecisionOnly(row.field, row.chips, rowValue(row), stagedOf(row));
	}
	const decisionOnlyCount = $derived(rows.filter(rowDecisionOnly).length);

	// The decided rows lead; the undecided provider values collapse behind one disclosure line
	// (HOLODEX-213 option A), so the dialog's default state reads as "your decisions" without
	// hiding anything — expanding or Select all brings them back at full contrast. Splitting on
	// leadRow() means the first group is exactly the set that writes on open.
	//
	// Row order within undecided (R4.4): mapped-and-differing first, then unmapped rows (still
	// decidable here — the decision lands in Holodex alone), then rows that already match the
	// file, then the rows nothing can be done with here (merge) — sorted on the row's
	// ORIGINAL (open-time) value rather than the live one, so a row never jumps position while
	// the owner is mid-pick.
	function rowTier(row: Row): number {
		if (!isCockpitRow(row.field)) return 3;
		const cls = rowClass(row.field, row.originalValue);
		return cls === 'matches' ? 2 : cls === 'unwritable' ? 1 : 0;
	}
	// leadRow: writes on open — a standing decision the file lags (by the row's open-time
	// value). This is needsWriteback() plus the decided rows whose sync state is UNKNOWN
	// (ADR-093: no read-back source, so `in_sync` is absent and the header cannot count
	// them). They still lead: a decided value the file does not carry is written, and hiding
	// it behind the disclosure would let the footer promise a write the owner cannot see.
	// The header's "· {n} out of sync" is therefore a lower bound on this group, never more.
	function leadRow(row: Row): boolean {
		if (!isCockpitRow(row.field)) return needsWriteback(row.field);
		return !!row.field.decision?.standing && rowClass(row.field, row.originalValue) === 'write';
	}
	const decided = $derived(rows.filter(leadRow));
	const undecided = $derived(rows.filter((r) => !leadRow(r)).sort((a, b) => rowTier(a) - rowTier(b)));
	let showUndecided = $state(false);

	// A staged pick changed — the owner acted on this row, so it is now decided-in-dialog
	// (touched) and, if it differs from the file, will be written. Clicking the already
	// highlighted RD6 pending chip counts: that click IS the confirm. The chooser stays open
	// either way: collapsing it the instant a pick lands on the file value would unmount the
	// very radiogroup the owner is arrowing through (focus to <body>).
	function onStaged(row: Row) {
		row.chooserOpen = true;
		row.touched = true;
	}

	// Provenance tag for a row's label: the namespace before the ':' in winning_source
	// (e.g. "tmdb:title" -> "tmdb"). isProvider reuses providerFromWinningSource's baseline
	// exclusions (file/record/manual/computed) so a computed field never reads as a provider.
	function sourceTag(winningSource?: string): { name: string; isProvider: boolean } | null {
		const name = (winningSource ?? '').split(':')[0];
		return name ? { name, isProvider: !!providerFromWinningSource(winningSource) } : null;
	}

	let busy = $state(false);
	// Set on a failed enqueue (R1.3) — the write is fire-and-forget, the enqueue is not, so
	// this is the one failure mode the dialog itself must still surface. Cleared on retry.
	let enqueueError = $state('');
	let dialogEl = $state<HTMLElement | null>(null);
	let trigger: HTMLElement | null = null;

	// Focusable descendants for the trap and the initial focus. Roving-tabindex chips render
	// as <button tabindex="-1"> (CurationChip radio mode) — they must not become Tab stops, so
	// anything at tabIndex -1 is excluded here, not just disabled/hidden elements (handoff §6).
	function focusables(): HTMLElement[] {
		return [
			...(dialogEl?.querySelectorAll<HTMLElement>('input, textarea, button, [tabindex="0"]') ?? [])
		].filter(
			(el) => !(el as HTMLButtonElement).disabled && el.tabIndex !== -1 && el.offsetParent !== null
		);
	}

	onMount(() => {
		trigger = document.activeElement as HTMLElement | null;
		// Focus the first control of the first row (the decided rows lead): the checked chip of
		// a chip row, or a radio/textarea of a stacked list. Rows
		// inside the collapsed group are excluded by focusables()'s offsetParent test. With no
		// rows at all, fall back to the dialog itself (tabindex="-1").
		const first =
			focusables().find((el) => el.closest('[data-wb-rows]') !== null) ?? dialogEl;
		first?.focus();
		return () => {
			trigger?.focus?.();
		};
	});

	function trapTab(e: KeyboardEvent) {
		if (e.key !== 'Tab' || !dialogEl) return;
		const focusable = focusables();
		if (focusable.length === 0) return;
		const first = focusable[0];
		const last = focusable[focusable.length - 1];
		if (e.shiftKey && document.activeElement === first) {
			e.preventDefault();
			last.focus();
		} else if (!e.shiftKey && document.activeElement === last) {
			e.preventDefault();
			first.focus();
		}
	}

	function onKeydown(e: KeyboardEvent) {
		// An open Custom chip input stops Escape's propagation itself (SourceChipRow), so a
		// keystroke that reaches here always means "close the dialog".
		if (e.key === 'Escape' && !busy) onclose();
		trapTab(e);
	}

	// HOLODEX-273/400: the checkbox + staged pick is this dialog's commit action (its
	// equivalent of the Tier-2 badge's explicit Confirm), so submit() records the decision
	// before the write whenever one is needed — an undecided row, or a decided row whose
	// staged pick differs from the standing selection. An untouched decided row is a no-op.
	// Merge (multi) rows never carry a decision (RD1; the resolver ignores one outright, so
	// creating it would be a misleading ghost row).
	async function ensureDecision(row: Row) {
		if (isBlankCustom(stagedOf(row).staged)) return;
		if (!needsDecision(row.field, row.chips, { key: row.stagedKey, custom: row.stagedCustomValue })) return;
		const chip = row.chips.find((c) => c.key === row.stagedKey);
		if (!chip) return;
		await decide(
			row.field.canonical,
			chip.decisionSource,
			chip.key === 'custom' ? row.stagedCustomValue.trim() : undefined
		);
	}

	async function submit() {
		if (busy || (writeCount === 0 && decisionOnlyCount === 0)) return;
		busy = true;
		enqueueError = '';

		const checkedRows = rows.filter(rowWillWrite);

		// Decisions to record: every written row that needs one (ensureDecision decides), plus
		// the system-only rows (rowDecisionOnly: unmapped, or re-pointed at the file value).
		const decisionRows = rows.filter((r) => isCockpitRow(r.field) && (rowWillWrite(r) || rowDecisionOnly(r)));

		// Every written row is a cockpit row (willWrite), so each writes exactly its staged
		// value — a replace field is one value.
		const fields = checkedRows.map((r) => ({
			field: r.field.canonical,
			values: [rowValue(r)].filter((v) => v.length > 0),
			source: r.field.winning_source ?? ''
		}));

		try {
			await Promise.all(decisionRows.map(ensureDecision));
			// Nothing to write (system-only decisions): skip the enqueue — an empty writeback
			// would be a no-op job. The caller's reload still picks up the decisions.
			if (checkedRows.length > 0) await writeback(videoId, { fields });
			// Fire-and-forget (ADR-091): the 202 means the job is durably queued, not that
			// anything has been written yet. Close immediately — the caller reloads to pick
			// up the new pending writeback_status row, and the write's outcome (landed or
			// failed) is a page-level signal near Metadata, not something this dialog waits
			// on or reports. Every submitted field passed isWritable above, so the worker's
			// own re-resolve against the container can only diverge in the rare case the
			// file's container changed since this dialog opened — that shows up as a failed
			// badge on the page, not an error here.
			onenqueued();
			onclose();
		} catch (e) {
			// The write is fire-and-forget; the ENQUEUE is not (R1.3) — a rejected request
			// (expired owner session, network drop, a decide() collision) keeps the dialog
			// open with the error inline — staged picks intact — rather than vanishing silently.
			enqueueError = toMessage(e);
		} finally {
			busy = false;
		}
	}
</script>

<!-- Backdrop + centering wrapper. aria-hidden must NOT be here — the dialog
     inside is focusable. aria-modal="true" on the dialog signals to screen
     readers that content outside it is inert. -->
<div
	class="fixed inset-0 z-50 flex items-start justify-center bg-bg/70 px-4 py-[10vh]"
	onclick={(e) => { if (e.target === e.currentTarget && !busy) onclose(); }}
>

<!-- Dialog -->
<div
	bind:this={dialogEl}
	role="dialog"
	aria-modal="true"
	aria-labelledby="writeback-title"
	tabindex="-1"
	onkeydown={onKeydown}
	class="w-full max-w-xl overflow-hidden rounded-theme border border-rule bg-surface shadow-lg"
>
	<div class="flex items-center justify-between border-b border-rule px-4 py-3">
		<h2 id="writeback-title" class="text-sm font-semibold text-ink">Write metadata to file</h2>
		<button
			onclick={() => !busy && onclose()}
			disabled={busy}
			aria-label="Close"
			class="rounded-theme p-1 text-muted hover:text-ink"
		>
			<svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
				<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
			</svg>
		</button>
	</div>

	<div class="max-h-[60vh] overflow-y-auto px-4 py-3">
		<!-- File path -->
		<p class="mb-3 truncate font-mono text-xs text-muted" title={filePath}>{filePath}</p>

		<!-- Decisions: the rows the file lags. These write on open, unconditionally. -->
		{#if decided.length > 0}
			<div class="space-y-3" data-wb-rows>
				{#each decided as row (row.field.canonical)}{@render fieldRow(row)}{/each}
			</div>
		{:else}
			<p class="text-xs text-muted">
				No decisions to write — nothing in this file lags a source you picked.
			</p>
		{/if}

		<!-- Undecided provider values: one line until asked for, so the dialog's default weight
		     matches what the header counted. No "Select all": each row here is a decision the
		     owner has not made, and a chip pick — one row at a time, eyes on the value — is the
		     only way to make it. A bulk verdict belongs to the review queues (ADR-090), not here. -->
		{#if undecided.length > 0}
			<div class="mt-3 flex items-center gap-2 border-t border-rule pt-3">
				<button
					onclick={() => (showUndecided = !showUndecided)}
					aria-expanded={showUndecided}
					aria-controls="wb-undecided"
					class="btn-quiet flex min-w-0 flex-1 items-center gap-1.5 text-left text-xs"
				>
					<svg
						class="h-3 w-3 shrink-0 transition-transform {showUndecided ? 'rotate-90' : ''}"
						viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"
					>
						<path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
					</svg>
					<span class="truncate"
						>{undecided.length} provider value{undecided.length === 1 ? '' : 's'} you haven't
						decided on</span
					>
				</button>
			</div>
			<div id="wb-undecided" hidden={!showUndecided} class="mt-3 space-y-3" data-wb-rows>
				{#each undecided as row (row.field.canonical)}{@render fieldRow(row)}{/each}
			</div>
		{/if}
	</div>

	<!-- Footer -->
	<div class="flex flex-col gap-2 border-t border-rule px-4 py-3">
		{#if busy}
			<p class="text-xs text-muted" aria-live="polite">Submitting {writeCount} field{writeCount === 1 ? '' : 's'}…</p>
		{:else if enqueueError}
			<!-- R1.3: the write is fire-and-forget, the enqueue is not — this is the one
			     failure mode the dialog itself still surfaces (not the queued write's own
			     eventual success/failure, which renders on the page instead). -->
			<p class="text-xs text-warn" aria-live="polite">{enqueueError}</p>
		{/if}

		<div class="flex justify-end gap-2">
			<button
				onclick={() => onclose()}
				disabled={busy}
				class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-bg disabled:opacity-60"
			>Cancel</button>
			<!-- The button names both destinations when both are touched, and never promises
			     "Write 0 fields" when only Holodex changes. -->
			<button
				onclick={submit}
				disabled={busy || (writeCount === 0 && decisionOnlyCount === 0)}
				class="rounded-theme bg-accent px-3 py-1.5 text-sm font-medium text-accent-ink hover:opacity-90 disabled:opacity-40"
			>
				{#if busy}
					{writeCount > 0 ? 'Writing…' : 'Saving…'}
				{:else if writeCount > 0 && decisionOnlyCount > 0}
					Write {writeCount} field{writeCount === 1 ? '' : 's'}, save {decisionOnlyCount} decision{decisionOnlyCount === 1 ? '' : 's'}
				{:else if writeCount === 0 && decisionOnlyCount > 0}
					Save {decisionOnlyCount} decision{decisionOnlyCount === 1 ? '' : 's'}
				{:else}
					Write {writeCount} field{writeCount === 1 ? '' : 's'} to file
				{/if}
			</button>
		</div>
	</div>
</div>
</div>

<!-- The chooser for a cockpit row: the chip row for short fields, stacked full-width rows for
     long_text (the page's own chip-row vs. pencil+modal split, handoff §2). Both stage into the
     row; onStaged() promotes a matching row to will-write the moment the pick differs. -->
{#snippet chooser(row: Row)}
	<!-- ADR-093: a field whose tag is written but never read back has an always-empty file
	     candidate. The baseline chip says "not read back" rather than "—" so it never claims
	     the file is empty when it is merely unknown; the gutter still reads "will be written"
	     — the write happens, it just cannot be confirmed later. No warning line: this is a
	     property of the mapping (the server logs which read-back key to add), not of the row. -->
	{@const unverifiable = isUnverifiable(row.field)}
	{#if isImageRow(row.field)}
		<!-- Image tiles (HOLODEX-403, ADR-101 D3): one per candidate, no Custom opener (a pasted
		     URL cannot pass the asset-host allowlist) — though a manual literal that already
		     stands (API-made) keeps its ·manual tile, or the group would have nothing checked.
		     The ·file tile shows the video's own served poster — unless that is an owner
		     upload, which is not the file's cover art (ADR-049): placeholder + note, and the
		     upload is never a candidate. -->
		<div class="mt-1" id="wb-chooser-{row.field.canonical}">
			<SourceImageTiles
				field={row.field}
				chips={row.chips.filter((c) => c.key !== 'custom' || c.value)}
				selection={row.selection}
				bind:stagedKey={row.stagedKey}
				disabled={busy}
				baselineImage={entityImageUploaded ? undefined : entityImage}
				baselinePlaceholder={entityImageUploaded ? ['cover art', 'in file'] : ['no cover', 'art']}
				onstage={() => onStaged(row)}
			/>
			{#if entityImageUploaded}
				<p class="mt-1 text-xs text-muted">
					Your uploaded poster stays on the page; this row is the cover art inside the file.
				</p>
			{/if}
		</div>
	{:else if row.field.display === 'long_text'}
		<div class="mt-1" id="wb-chooser-{row.field.canonical}">
			<SourceRadioList
				field={row.field}
				chips={row.chips}
				bind:stagedKey={row.stagedKey}
				bind:stagedCustomValue={row.stagedCustomValue}
				disabled={busy}
				baselinePlaceholder={unverifiable ? 'Not read back from this file' : 'No value'}
				onstage={() => onStaged(row)}
			/>
		</div>
	{:else}
		<div class="mt-1" id="wb-chooser-{row.field.canonical}">
			<SourceChipRow
				field={row.field}
				chips={row.chips}
				selection={row.selection}
				bind:stagedKey={row.stagedKey}
				bind:stagedCustomValue={row.stagedCustomValue}
				disabled={busy}
				baselinePlaceholder={unverifiable ? 'not read back' : undefined}
				onstage={() => onStaged(row)}
			/>
		</div>
	{/if}
{/snippet}

{#snippet fieldRow(row: Row)}
				{@const writable = isWritable(row.field)}
				{@const tag = sourceTag(row.field.winning_source)}
				{@const cockpit = isCockpitRow(row.field)}
				{@const fileVal = fileCandidateValue(row.field)}
				<!-- matchesFile drives both the "matches the file" line and the gutter's non-checkable
				     "=" tier (R4.3). Calls the same rowMatchesFile() used by submit()'s filter, rather
				     than re-deriving the comparison here, so the two can't drift apart — it compares
				     the row's LIVE (staged) value against the file baseline, which is why it's
				     recomputed per render rather than reading the frozen in_sync snapshot that
				     needsWriteback() groups rows by. -->
				{@const matchesFile = rowMatchesFile(row)}
				{@const toFile = rowWillWrite(row)}
				{@const toSystem = rowDecisionOnly(row)}
				<!-- No dimming for an unchecked row: the group heading above already says these are
				     undecided, and `opacity` on a `text-muted` label lands at ~2.2:1 on every skin.
				     The checkbox carries the state; the label stays legible. -->
				<div class="flex items-start gap-3">
					<!-- Gutter glyph = what Write will do to this row. There is no checkbox anywhere;
					     deciding is the check action. Arrow-into-bar: written to the FILE (a standing
					     decision, or a chip the owner picked here). Cylinder: saved in HOLODEX only —
					     the field has no file tag for this container, or the pick is the file's own
					     value. = matches the file, nothing to do. ⊖ nothing to do and nothing to decide
					     here (image_url/merge), or an unmapped field already decided. Hollow circle:
					     undecided — pick a source. Static glyphs rather than disabled checkboxes on
					     purpose: a box that can never be checked reads as broken, a glyph reads as
					     information. Which data CAN reach the file is on the header line (→ tag, or
					     "no file tag for this container"), independent of the gutter. -->
					<div class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center">
						{#if !cockpit}
							<svg
								class="h-4 w-4 text-muted"
								viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"
								role="img"
							>
								<title>Nothing to decide here — not written from this dialog</title>
								<circle cx="12" cy="12" r="9" />
								<path stroke-linecap="round" d="M7 12h10" />
							</svg>
						{:else if toFile}
							<svg
								class="h-4 w-4 text-accent"
								viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"
								role="img"
							>
								<title>Will be written to the file</title>
								<path stroke-linecap="round" stroke-linejoin="round" d="M12 4v11m0 0l-4-4m4 4l4-4M5 20h14" />
							</svg>
						{:else if toSystem}
							<svg
								class="h-4 w-4 text-accent"
								viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"
								role="img"
							>
								<title>{writable ? 'Decision saved in Holodex — the file already has this value' : 'Decision saved in Holodex — no file tag for this container'}</title>
								<ellipse cx="12" cy="6" rx="7" ry="3" />
								<path d="M5 6v12c0 1.7 3.1 3 7 3s7-1.3 7-3V6" />
								<path d="M5 12c0 1.7 3.1 3 7 3s7-1.3 7-3" />
							</svg>
						{:else if !writable && row.field.decision?.standing}
							<svg
								class="h-4 w-4 text-muted"
								viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"
								role="img"
							>
								<title>Decided in Holodex — no file tag for this container, nothing to do</title>
								<circle cx="12" cy="12" r="9" />
								<path stroke-linecap="round" d="M7 12h10" />
							</svg>
						{:else if matchesFile}
							<svg
								class="h-4 w-4 text-muted"
								viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"
								role="img"
							>
								<title>Matches the file — nothing to write</title>
								<path stroke-linecap="round" d="M6 9h12M6 15h12" />
							</svg>
						{:else}
							<svg
								class="h-4 w-4 text-muted"
								viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"
								role="img"
							>
								<title>{writable ? 'Undecided — pick a source to write it' : 'Undecided — pick a source to decide it in Holodex'}</title>
								<circle cx="12" cy="12" r="6" />
							</svg>
						{/if}
					</div>

					<!-- Label + body -->
					<div class="min-w-0 flex-1">
						<div class="mb-1 flex items-center gap-1.5">
							<span class="text-xs font-medium text-muted">{row.field.label}</span>
							{#if tag}
								<span class="text-[0.65rem] {tag.isProvider ? 'text-accent' : 'text-muted'}"
									>·{tag.name}</span
								>
							{/if}
							{#if writable}
								<span class="text-[0.65rem] text-muted">→ {row.field.write_target}</span>
							{:else if cockpit}
								<!-- Which data can reach the file lives here, on every row (HOLODEX-216 made
								     the target visible; the owner asked for the inverse to be just as
								     visible): unmapped for this container, decidable here all the same. -->
								<span class="text-[0.65rem] text-muted">no file tag for this container</span>
							{/if}
							{#if matchesFile && cockpit}
								<!-- The "=" tier is collapsed, not dead (handoff §1): `change` opens the same
								     chooser a differing row shows; a non-file pick promotes the row. -->
								<button
									type="button"
									onclick={() => (row.chooserOpen = !row.chooserOpen)}
									disabled={busy}
									aria-expanded={row.chooserOpen}
									aria-controls="wb-chooser-{row.field.canonical}"
									class="btn-quiet ml-auto text-xs"
								>{row.chooserOpen ? 'close' : 'change'}</button>
							{/if}
						</div>

						{#if !writable && !cockpit}
							<p class="text-xs text-muted">
								{rowValue(row) || '—'}
								<span class="block">No file tag for this container — can't be written.</span>
							</p>
						{:else if cockpit}
							<!-- Cockpit row. The chooser has ONE mount point for both the "=" and the
							     will-write state, so staging a pick that flips the class never unmounts the
							     radiogroup the owner is arrowing through (focus would land on <body>). The
							     "matches the file" line toggles above it; the chooser shows whenever the row
							     differs OR the owner opened it. The ·file chip/row carries the on-file value,
							     so there is no separate "was:" line (handoff §2). -->
							{#if matchesFile}
								<!-- The gutter's own "=" glyph already signals this row's tier — no
								     second icon here, or the two would say the same thing twice. An image
								     row shows a thumbnail + its source instead of the URL (HOLODEX-403). -->
								<p class="flex items-center gap-2 text-xs text-muted">
									{#if isImageRow(row.field) && rowValue(row)}
										<img src={rowValue(row)} alt="" class="h-8 w-auto min-w-6 shrink-0 rounded-theme border border-rule object-contain" />
										<span class="text-ink">{tag?.name ?? 'file'} {row.field.label.toLowerCase()}</span>
									{:else}
										<span class="text-ink">{rowValue(row) || '—'}</span>
									{/if}
									<span>— matches the file</span>
								</p>
							{/if}
							{#if !matchesFile || row.chooserOpen}
								{@render chooser(row)}
							{/if}
						{:else if matchesFile}
							<!-- Merge row whose seeded text equals the file value. -->
							<p class="text-xs text-muted">
								<span class="text-ink">{rowValue(row) || '—'}</span>
								<span>— matches the file</span>
							</p>
						{:else}
							<!-- Merge (multi) row: no decision model (RD1), so nothing to decide and nothing
							     written from here — tags as a set is HOLODEX-401. Read-only. -->
							<p class="text-xs text-muted">
								<span class="text-ink">{row.value || '—'}</span>
								{#if row.field.candidates !== undefined && fileVal !== row.value}
									<span class="block">on file: {fileVal || '—'}</span>
								{/if}
								<span class="block">Nothing to decide here yet — not written from this dialog.</span>
							</p>
						{/if}
					</div>
				</div>
{/snippet}
