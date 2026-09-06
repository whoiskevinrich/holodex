<script lang="ts">
	// Batch writeback form (F28 UX revision; ADR-091/HOLODEX-323 confirm-step
	// revision). Opens as a modal showing all writable resolved fields pre-filled
	// with their winning values. The operator can edit any value and uncheck
	// fields to skip them before submitting. The write itself is fire-and-forget
	// (ADR-091): this dialog is a pre-flight confirm step that closes the instant
	// the write is *enqueued*, not once it lands — outcome (pending/failed) is a
	// page-level signal near the Metadata section, not this dialog's job to poll
	// or display. Focus is trapped + returned; Escape closes when idle (or once
	// the single enqueue round trip finishes). Tokens only; QA 3 skins.
	import { onMount } from 'svelte';
	import { toMessage, providerFromWinningSource } from '$lib/format';
	import { fileCandidateValue, isReplaceField, isWritable, needsWriteback } from '$lib/f36';
	import type { DecisionSource, ResolvedField, WritebackRequest } from '$lib/types';

	let {
		fields,
		videoId,
		filePath,
		onclose,
		onenqueued,
		writeback,
		decide
	}: {
		fields: ResolvedField[];
		videoId: number;
		filePath: string;
		onclose: () => void;
		// Fires once the write is accepted (the 202 ack) — before anything has
		// actually been written (ADR-091). The caller reloads to pick up the new
		// pending writeback_status row and any decisions ensureDecision() created;
		// it does not learn which fields ultimately wrote, since the job is
		// atomic and its outcome is reported on the page, not here.
		onenqueued: () => void;
		writeback: (id: number, req: WritebackRequest) => Promise<unknown>;
		// Creates a standing decision for one field (HOLODEX-273) — DB-only, no reload.
		decide: (canonical: string, source: DecisionSource, manualValue?: string) => Promise<unknown>;
	} = $props();

	interface Row {
		field: ResolvedField;
		value: string;
		// Pre-edit seed value, so submit() can tell an untouched row from a manual
		// override (HOLODEX-273) — same expression `value` was seeded with below.
		originalValue: string;
		checked: boolean;
	}

	// Only out-of-sync fields start checked, via the same needsWriteback() the header counts
	// with — so "· {n} out of sync" and the initial selection cannot disagree (HOLODEX-213).
	// Everything else (a provider value winning by mapping precedence, a file-won field, a
	// merge field) is listed unchecked for explicit opt-in, so submitting never writes a value
	// the owner never decided on — notably poster_url, whose write triggers a server-side
	// download + cover-art embed. image_url fields show as thumbnail + URL (read-only).
	// svelte-ignore state_referenced_locally — fields prop is stable for the dialog's lifetime
	const rows = $state<Row[]>(
		fields.map((f) => {
			const seed = f.display === 'image_url' ? (f.values[0] ?? '') : f.values.join(', ');
			return { field: f, value: seed, originalValue: seed, checked: needsWriteback(f) };
		})
	);

	// True when the row's LIVE (editable) value already matches the file's own tag value —
	// "already matches the file, nothing to write" (the non-checkable gutter tier, R4.3).
	// Shared between the gutter render and submit()'s filter so they can't disagree — same
	// reasoning as isWritable being "defense in depth, not the primary guard" below.
	function rowMatchesFile(row: Row): boolean {
		return row.field.candidates !== undefined && row.value.trim() === fileCandidateValue(row.field).trim();
	}

	// checkedCount excludes a row that now matches the file (e.g. edited back to its
	// original value) even if still toggled on internally, so the footer button's count
	// never promises to write a field submit() will actually skip.
	const checkedCount = $derived(rows.filter((r) => r.checked && !rowMatchesFile(r)).length);

	// The decided rows lead; the undecided provider values collapse behind one disclosure line
	// (HOLODEX-213 option A), so the dialog's default state reads as "your decisions" without
	// hiding anything — expanding or Select all brings them back at full contrast. Splitting on
	// the same needsWriteback() that seeded `checked` means the two groups are exactly the
	// checked and unchecked sets on open.
	//
	// Row order within undecided (R4.4): writable-and-differing first, then a field that
	// already matches the file, then a field with no tag mapping at all — sorted on the
	// row's ORIGINAL (open-time) value rather than the live one, so a row never jumps
	// position while the owner is mid-edit. decided is always uniformly tier 0 (needsWriteback
	// already implies writable-and-differing), so sorting it would be a no-op.
	function rowTier(row: Row): number {
		if (!isWritable(row.field)) return 2;
		if (row.field.candidates !== undefined && row.originalValue.trim() === fileCandidateValue(row.field).trim()) {
			return 1;
		}
		return 0;
	}
	const decided = $derived(rows.filter((r) => needsWriteback(r.field)));
	const undecided = $derived(
		rows.filter((r) => !needsWriteback(r.field)).sort((a, b) => rowTier(a) - rowTier(b))
	);
	let showUndecided = $state(false);

	function selectAllUndecided() {
		showUndecided = true;
		for (const row of undecided) row.checked = true;
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

	onMount(() => {
		trigger = document.activeElement as HTMLElement | null;
		// Focus the first interactive element of a decided row. Rows inside the collapsed group
		// still match the selector, so filter on offsetParent (as trapTab does) — otherwise
		// focus() lands on a display:none input and silently leaves focus on <body>, outside the
		// trap. With nothing to write, fall back to the dialog itself (tabindex="-1").
		const first =
			[
				...(dialogEl?.querySelectorAll<HTMLElement>(
					'input[type="checkbox"]:not(:disabled), textarea, input:not([type="checkbox"]):not(:disabled)'
				) ?? [])
			].find((el) => el.offsetParent !== null) ?? dialogEl;
		first?.focus();
		return () => {
			trigger?.focus?.();
		};
	});

	function trapTab(e: KeyboardEvent) {
		if (e.key !== 'Tab' || !dialogEl) return;
		const focusable = [
			...dialogEl.querySelectorAll<HTMLElement>(
				'input, textarea, button, [tabindex="0"]'
			)
		].filter((el) => !(el as HTMLButtonElement).disabled && el.offsetParent !== null);
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
		if (e.key === 'Escape' && !busy) onclose();
		trapTab(e);
	}

	// Auto-resize a textarea to its content, capped at 8 rows via CSS max-height.
	function autoResize(node: HTMLTextAreaElement) {
		function resize() {
			node.style.height = 'auto';
			node.style.height = node.scrollHeight + 'px';
		}
		resize();
		node.addEventListener('input', resize);
		return { destroy: () => node.removeEventListener('input', resize) };
	}

	// HOLODEX-273: an undecided row (the "provider values you haven't decided on"
	// group, or one individually checked) writes to the file today without ever
	// creating a standing field_source_decisions row — the DB still shows the field
	// undecided after the write. Checking the box is the commit action for this
	// dialog (its equivalent of the Tier-2 badge's explicit Confirm), so submit()
	// creates the decision first. A row already decided is a no-op (nothing to
	// create); image_url fields are excluded — picking a candidate there stays a
	// SourceSelect-only decision (RD5), never baked into the writeback action. Merge
	// (multi) fields are excluded too — the `fields` prop is the full resolved array
	// (Genres/Actors/Director included), but per f36.ts they keep F30 per-value
	// curation and never carry a source decision (RD1); the resolver ignores a
	// decision on a multi canonical outright, so creating one would just be a
	// misleading ghost row. An edited value (vs. the row's pre-edit seed) commits as
	// `manual` with that value, so the new decision never disagrees with what
	// actually gets written.
	async function ensureDecision(row: Row) {
		if (!isReplaceField(row.field)) return;
		if (row.field.decision?.standing) return;
		if (row.field.display === 'image_url') return;
		if (row.value.trim() !== row.originalValue.trim()) {
			await decide(row.field.canonical, 'manual', row.value);
			return;
		}
		const provider = providerFromWinningSource(row.field.winning_source);
		if (provider) await decide(row.field.canonical, `provider:${provider}`);
	}

	async function submit() {
		if (busy || checkedCount === 0) return;
		busy = true;
		enqueueError = '';

		// isWritable/rowMatchesFile are defense-in-depth filters, not the primary guard —
		// an unwritable row never renders a checkbox and a matching row's checkbox is
		// replaced by the "=" gutter glyph, so `checked` should already exclude both.
		const checkedRows = rows.filter((r) => r.checked && isWritable(r.field) && !rowMatchesFile(r));

		const fields = checkedRows.map((r) => ({
			field: r.field.canonical,
			// image_url fields: pass the URL as a single value (don't comma-split)
			values:
				r.field.display === 'image_url'
					? [r.value].filter((v) => v.length > 0)
					: r.value
							.split(/\s*,\s*/)
							.map((v) => v.trim())
							.filter((v) => v.length > 0),
			source: r.field.winning_source ?? ''
		}));

		try {
			await Promise.all(checkedRows.map(ensureDecision));
			await writeback(videoId, { fields });
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
			// open with the error inline rather than vanishing silently.
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

		<!-- Decisions: the rows the file lags. These are the ones checked on open. -->
		{#if decided.length > 0}
			<div class="space-y-3">
				{#each decided as row (row.field.canonical)}{@render fieldRow(row)}{/each}
			</div>
		{:else}
			<p class="text-xs text-muted">
				No decisions to write — nothing in this file lags a source you picked.
			</p>
		{/if}

		<!-- Undecided provider values: one line until asked for, so the default selection and the
		     dialog's visual weight both match what the header counted. -->
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
				<button onclick={selectAllUndecided} disabled={busy} class="btn-row btn-accent btn-pill shrink-0"
					>Select all</button
				>
			</div>
			<div id="wb-undecided" hidden={!showUndecided} class="mt-3 space-y-3">
				{#each undecided as row (row.field.canonical)}{@render fieldRow(row)}{/each}
			</div>
		{/if}
	</div>

	<!-- Footer -->
	<div class="flex flex-col gap-2 border-t border-rule px-4 py-3">
		{#if busy}
			<p class="text-xs text-muted" aria-live="polite">Submitting {checkedCount} field{checkedCount === 1 ? '' : 's'}…</p>
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
			<button
				onclick={submit}
				disabled={busy || checkedCount === 0}
				class="rounded-theme bg-accent px-3 py-1.5 text-sm font-medium text-accent-ink hover:opacity-90 disabled:opacity-40"
			>
				{#if busy}Writing…{:else}Write {checkedCount} field{checkedCount === 1 ? '' : 's'} to file{/if}
			</button>
		</div>
	</div>
</div>
</div>

{#snippet fieldRow(row: Row)}
				{@const writable = isWritable(row.field)}
				{@const tag = sourceTag(row.field.winning_source)}
				{@const hasFileValue = row.field.candidates !== undefined}
				{@const fileVal = fileCandidateValue(row.field)}
				<!-- matchesFile drives both the "already in file, nothing to write" line and the
				     gutter's non-checkable "=" tier (R4.3). It compares the row's LIVE (editable)
				     value against the file baseline, so it re-derives from fileVal — already
				     fetched for the "was:" line — rather than reading the frozen in_sync snapshot
				     that needsWriteback() seeds `checked` from. rowMatchesFile() below mirrors this
				     exact expression for submit()'s filter — the two must never disagree. -->
				{@const matchesFile = hasFileValue && row.value.trim() === fileVal.trim()}
				<!-- No dimming for an unchecked row: the group heading above already says these are
				     undecided, and `opacity` on a `text-muted` label lands at ~2.2:1 on every skin.
				     The checkbox carries the state; the label stays legible. -->
				<div class="flex items-start gap-3">
					<!-- Checkbox / static glyph. Three tiers (R4.3), each with its own glyph — no two
					     ever mean the same thing: a checkbox means "will be written," an equals sign
					     means "already matches, nothing to write" (not checkable), a circle-minus
					     means "no file tag for this container" (not checkable). Removing a value's
					     ability to be checked, rather than showing a disabled checkbox, is deliberate:
					     a checkbox that can never be checked reads as broken, a static glyph reads as
					     informational. -->
					<div class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center">
						{#if !writable}
							<!-- No file-tag mapping for this container (HOLODEX-216): shown, not checkable —
							     never a bare checkbox that would only silently drop the value on write. -->
							<svg
								class="h-4 w-4 text-muted"
								viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"
								role="img"
							>
								<title>No file tag for this container — can't be written</title>
								<circle cx="12" cy="12" r="9" />
								<path stroke-linecap="round" d="M7 12h10" />
							</svg>
						{:else if matchesFile}
							<svg
								class="h-4 w-4 text-muted"
								viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"
								role="img"
							>
								<title>Already matches the file — nothing to write</title>
								<path stroke-linecap="round" d="M6 9h12M6 15h12" />
							</svg>
						{:else}
							<input
								type="checkbox"
								id="wb-{row.field.canonical}"
								bind:checked={row.checked}
								disabled={busy}
								class="h-4 w-4 cursor-pointer accent-[var(--color-accent)] disabled:cursor-not-allowed"
							/>
						{/if}
					</div>

					<!-- Label + input -->
					<div class="min-w-0 flex-1">
						<div class="mb-1 flex items-center gap-1.5">
							{#if writable && !matchesFile}
								<!-- for= only when the gutter actually renders the checkbox this labels —
								     a matching row's gutter is the static "=" glyph, not an input. -->
								<label for="wb-{row.field.canonical}" class="text-xs font-medium text-muted"
									>{row.field.label}</label
								>
							{:else}
								<span class="text-xs font-medium text-muted">{row.field.label}</span>
							{/if}
							{#if tag}
								<span class="text-[0.65rem] {tag.isProvider ? 'text-accent' : 'text-muted'}"
									>·{tag.name}</span
								>
							{/if}
							{#if writable}
								<span class="text-[0.65rem] text-muted">→ {row.field.write_target}</span>
							{/if}
						</div>

						{#if !writable}
							<p class="text-xs text-muted">
								{row.value || '—'}
								<span class="block">No file tag for this container — can't be written.</span>
							</p>
						{:else if matchesFile}
							<!-- The gutter's own "=" glyph already signals this row's tier — no
							     second icon here, or the two would say the same thing twice. -->
							<p class="text-xs text-muted">
								<span class="text-ink">{row.value || '—'}</span>
								<span>— already matches the file, nothing to write</span>
							</p>
						{:else if row.field.display === 'image_url'}
							<!-- Read-only file-vs-enriched comparison (HOLODEX-245): mirrors the "was:"
							     idiom text fields get below, since an image needs a visual compare rather
							     than a value string. No selection here — picking a candidate is a
							     SourceSelect-only decision (RD5), never baked into the writeback action. -->
							<div class="flex items-start gap-3">
								<div class="flex flex-col items-start gap-1">
									<span class="text-[0.65rem] text-muted">File (current)</span>
									{#if fileVal}
										<img
											src={fileVal}
											alt="{row.field.label} — file"
											class="h-14 w-14 shrink-0 rounded-theme border border-rule object-cover"
										/>
									{:else}
										<div
											class="flex h-14 w-14 shrink-0 items-center justify-center rounded-theme border border-rule text-muted"
										>
											<svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
												<rect x="3" y="3" width="18" height="18" rx="2" />
												<circle cx="8.5" cy="8.5" r="1.5" />
												<path stroke-linecap="round" stroke-linejoin="round" d="M21 15l-5-5-9 9" />
											</svg>
										</div>
									{/if}
								</div>
								<svg class="mt-6 h-4 w-4 shrink-0 text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
									<path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14m0 0l-6-6m6 6l-6 6" />
								</svg>
								<div class="flex min-w-0 flex-col items-start gap-1">
									<span class="text-[0.65rem] text-accent">Enriched (will write)</span>
									{#if row.value}
										<img
											src={row.value}
											alt="{row.field.label} — enriched{tag ? `, from ${tag.name}` : ''}"
											class="h-14 w-14 shrink-0 rounded-theme border border-accent object-cover"
										/>
									{/if}
									<p class="max-w-[10rem] break-all text-xs text-muted">{row.value || '—'}</p>
								</div>
							</div>
						{:else}
							{#if hasFileValue}
								<p class="mb-1 text-xs text-muted">was: {fileVal || '—'}</p>
							{/if}
							{#if row.field.display === 'long_text'}
								<textarea
									bind:value={row.value}
									disabled={busy}
									rows="1"
									use:autoResize
									class="block w-full resize-none overflow-hidden rounded-theme border border-rule bg-bg px-2 py-1 text-sm text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-accent disabled:cursor-not-allowed disabled:opacity-60"
									style="max-height: 10rem"
								></textarea>
							{:else}
								<input
									type="text"
									bind:value={row.value}
									disabled={busy}
									class="block w-full rounded-theme border border-rule bg-bg px-2 py-1 text-sm text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-accent disabled:cursor-not-allowed disabled:opacity-60"
								/>
							{/if}
						{/if}
					</div>
				</div>
{/snippet}
