<script lang="ts">
	// Batch writeback form (F28 UX revision). Opens as a modal showing all
	// writable resolved fields pre-filled with their winning values. The operator
	// can edit any value and uncheck fields to skip them before submitting.
	// Writes are sequential (one exiftool pass per field) with per-row progress.
	// Focus is trapped + returned; Escape closes when idle. Tokens only; QA 3 skins.
	import { onMount } from 'svelte';
	import { toMessage, providerFromWinningSource } from '$lib/format';
	import { fileCandidateValue, isReplaceField, isWritable, needsWriteback } from '$lib/f36';
	import { waitForWritebackJob, type WritebackJobState } from '$lib/writebackJob';
	import type { DecisionSource, ResolvedField, WritebackRequest } from '$lib/types';

	let {
		fields,
		videoId,
		filePath,
		onclose,
		onapplied,
		writeback,
		jobStatus,
		decide
	}: {
		fields: ResolvedField[];
		videoId: number;
		filePath: string;
		onclose: () => void;
		onapplied: (written: string[]) => void;
		writeback: (id: number, req: WritebackRequest) => Promise<unknown>;
		// Reads one queued job's state, for polling it to completion.
		jobStatus: (jobId: number) => Promise<WritebackJobState>;
		// Creates a standing decision for one field (HOLODEX-273) — DB-only, no reload.
		decide: (canonical: string, source: DecisionSource, manualValue?: string) => Promise<unknown>;
	} = $props();

	type RowStatus = 'idle' | 'writing' | 'done' | 'error';

	interface Row {
		field: ResolvedField;
		value: string;
		// Pre-edit seed value, so submit() can tell an untouched row from a manual
		// override (HOLODEX-273) — same expression `value` was seeded with below.
		originalValue: string;
		checked: boolean;
		status: RowStatus;
		error: string;
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
			return {
				field: f,
				value: seed,
				originalValue: seed,
				checked: needsWriteback(f),
				status: 'idle' as RowStatus,
				error: ''
			};
		})
	);

	const checkedCount = $derived(rows.filter((r) => r.checked).length);
	const hasErrors = $derived(rows.some((r) => r.status === 'error'));

	// The decided rows lead; the undecided provider values collapse behind one disclosure line
	// (HOLODEX-213 option A), so the dialog's default state reads as "your decisions" without
	// hiding anything — expanding or Select all brings them back at full contrast. Splitting on
	// the same needsWriteback() that seeded `checked` means the two groups are exactly the
	// checked and unchecked sets on open.
	const decided = $derived(rows.filter((r) => needsWriteback(r.field)));
	const undecided = $derived(rows.filter((r) => !needsWriteback(r.field)));
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
	let dialogEl = $state<HTMLElement | null>(null);
	let trigger: HTMLElement | null = null;
	let unmounted = false; // stops an in-flight job poll if the dialog goes away

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
			unmounted = true;
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

		// Mark all checked rows as in-progress, build the batch payload. isWritable is
		// a defense-in-depth filter, not the primary guard — the checkbox for an
		// unwritable row is disabled below, so `checked` should never be true for one.
		const checkedRows = rows.filter((r) => r.checked && isWritable(r.field));
		for (const row of checkedRows) {
			row.status = 'writing';
			row.error = '';
		}

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
			const res = await writeback(videoId, { fields });
			// The durable queue (F30, ADR-048) answers 202 + job_id the moment the job is
			// enqueued — nothing has been written yet, so wait for it to land before
			// reporting applied (ADR-073). Every submitted field passed isWritable above,
			// so once the job succeeds it wrote all of them — the worker's own re-resolve
			// against the container can only diverge in the rare case the file's container
			// changed since this dialog opened.
			const jobId = (res as { job_id?: number } | null)?.job_id;
			if (jobId) {
				await waitForWritebackJob(jobId, jobStatus, { cancelled: () => unmounted });
				for (const row of checkedRows) row.status = 'done';
			} else {
				// Legacy synchronous path (HOLODEX-216): the response names exactly which
				// submitted fields were written vs. skipped, so a row's status reflects
				// what actually happened rather than assuming success for everything sent.
				const written = new Set((res as { written?: string[] } | null)?.written ?? []);
				for (const row of checkedRows) {
					if (written.has(row.field.canonical)) {
						row.status = 'done';
					} else {
						row.status = 'error';
						row.error = 'No tag mapping for this file — not written.';
					}
				}
			}
			onapplied(checkedRows.filter((r) => r.status === 'done').map((r) => r.field.canonical));
			if (checkedRows.every((r) => r.status === 'done')) onclose();
		} catch (e) {
			const msg = toMessage(e);
			for (const row of checkedRows) {
				row.status = 'error';
				row.error = msg;
			}
		} finally {
			busy = false;
		}
	}
</script>

{#snippet checkIcon(cls: string)}
	<svg class={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" aria-hidden="true">
		<path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
	</svg>
{/snippet}

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
			<p class="text-xs text-muted" aria-live="polite">Writing {checkedCount} field{checkedCount === 1 ? '' : 's'} to file…</p>
		{:else if hasErrors}
			<p class="text-xs text-warn" aria-live="polite">
				Write failed — uncheck any fields you want to skip and try again.
			</p>
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
				{@const isDone = row.status === 'done'}
				{@const isWriting = row.status === 'writing'}
				{@const isError = row.status === 'error'}
				{@const writable = isWritable(row.field)}
				{@const tag = sourceTag(row.field.winning_source)}
				{@const hasFileValue = row.field.candidates !== undefined}
				{@const fileVal = fileCandidateValue(row.field)}
				<!-- matchesFile drives the "already in file, nothing to write" line. It compares
				     the row's LIVE (editable) value against the file baseline, so it re-derives
				     from fileVal — already fetched for the "was:" line — rather than reading the
				     frozen in_sync snapshot that needsWriteback() seeds `checked` from. -->
				{@const matchesFile = hasFileValue && row.value.trim() === fileVal.trim()}
				<!-- No dimming for an unchecked row: the group heading above already says these are
				     undecided, and `opacity` on a `text-muted` label lands at ~2.2:1 on every skin.
				     The checkbox carries the state; the label stays legible. -->
				<div class="flex items-start gap-3">
					<!-- Checkbox / status icon -->
					<div class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center">
						{#if isDone}
							{@render checkIcon('h-4 w-4 text-accent')}
						{:else if isWriting}
							<svg class="h-4 w-4 animate-spin text-muted" viewBox="0 0 24 24" fill="none" aria-hidden="true">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
							</svg>
						{:else if isError}
							<svg class="h-4 w-4 text-warn" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
								<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
							</svg>
						{:else if !writable}
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
							{#if writable}
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
							<p class="flex items-center gap-1.5 text-xs text-muted">
								{@render checkIcon('h-3.5 w-3.5 shrink-0')}
								<span class="text-ink">{row.value || '—'}</span>
								<span>— already in file, nothing to write</span>
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
									disabled={busy || isDone || isWriting}
									rows="1"
									use:autoResize
									class="block w-full resize-none overflow-hidden rounded-theme border border-rule bg-bg px-2 py-1 text-sm text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-accent disabled:cursor-not-allowed disabled:opacity-60"
									style="max-height: 10rem"
								></textarea>
							{:else}
								<input
									type="text"
									bind:value={row.value}
									disabled={busy || isDone || isWriting}
									class="block w-full rounded-theme border border-rule bg-bg px-2 py-1 text-sm text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-accent disabled:cursor-not-allowed disabled:opacity-60"
								/>
							{/if}
						{/if}

						{#if isError}
							<p class="mt-1 text-xs text-warn" aria-live="polite">{row.error}</p>
						{/if}
					</div>
				</div>
{/snippet}
