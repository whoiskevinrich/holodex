<script lang="ts">
	// Batch writeback form (F28 UX revision). Opens as a modal showing all
	// writable resolved fields pre-filled with their winning values. The operator
	// can edit any value and uncheck fields to skip them before submitting.
	// Writes are sequential (one exiftool pass per field) with per-row progress.
	// Focus is trapped + returned; Escape closes when idle. Tokens only; QA 3 skins.
	import { onMount } from 'svelte';
	import { toMessage } from '$lib/format';
	import type { ResolvedField, WritebackRequest } from '$lib/types';

	let {
		fields,
		videoId,
		filePath,
		onclose,
		onapplied,
		writeback
	}: {
		fields: ResolvedField[];
		videoId: number;
		filePath: string;
		onclose: () => void;
		onapplied: (written: string[]) => void;
		writeback: (id: number, req: WritebackRequest) => Promise<unknown>;
	} = $props();

	type RowStatus = 'idle' | 'writing' | 'done' | 'error';

	interface Row {
		field: ResolvedField;
		value: string;
		checked: boolean;
		status: RowStatus;
		error: string;
	}

	// Exclude image_url fields (no tag mapping). Provider-won fields start checked;
	// file-only fields start unchecked so the operator opts in explicitly.
	// svelte-ignore state_referenced_locally — fields prop is stable for the dialog's lifetime
	const rows = $state<Row[]>(
		fields
			.filter((f) => f.display !== 'image_url')
			.map((f) => {
				const providerWon =
					!!f.winning_source && !f.winning_source.startsWith('file:');
				return {
					field: f,
					value: f.values.join(', '),
					checked: providerWon,
					status: 'idle' as RowStatus,
					error: ''
				};
			})
	);

	const checkedCount = $derived(rows.filter((r) => r.checked).length);
	const hasErrors = $derived(rows.some((r) => r.status === 'error'));
	const totalWritten = $derived(rows.filter((r) => r.status === 'done').length);

	let busy = $state(false);
	let dialogEl = $state<HTMLElement | null>(null);
	let trigger: HTMLElement | null = null;

	onMount(() => {
		trigger = document.activeElement as HTMLElement | null;
		// Focus first interactive element: first checked row's input or the cancel button.
		const first = dialogEl?.querySelector<HTMLElement>(
			'input[type="checkbox"]:not(:disabled), textarea, input:not([type="checkbox"]):not(:disabled)'
		);
		first?.focus();
		return () => trigger?.focus?.();
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

	async function submit() {
		if (busy || checkedCount === 0) return;
		busy = true;
		const written: string[] = [];

		for (const row of rows) {
			if (!row.checked || row.status === 'done') continue;
			row.status = 'writing';
			row.error = '';
			try {
				const values = row.value
					.split(/\s*,\s*/)
					.map((v) => v.trim())
					.filter((v) => v.length > 0);
				await writeback(videoId, {
					field: row.field.canonical,
					values,
					source: row.field.winning_source ?? ''
				});
				row.status = 'done';
				written.push(row.field.canonical);
			} catch (e) {
				row.status = 'error';
				row.error = toMessage(e);
			}
		}

		busy = false;
		onapplied(written);
		if (!hasErrors) onclose();
	}
</script>

<!-- Backdrop + centering wrapper -->
<div
	class="fixed inset-0 z-50 flex items-start justify-center bg-bg/70 px-4 py-[10vh]"
	aria-hidden="true"
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
			class="rounded-theme p-1 text-muted hover:text-ink disabled:opacity-40"
		>
			<svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
				<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
			</svg>
		</button>
	</div>

	<div class="max-h-[60vh] overflow-y-auto px-4 py-3">
		<!-- File path -->
		<p class="mb-3 truncate font-mono text-xs text-muted" title={filePath}>{filePath}</p>

		<!-- Field rows -->
		<div class="space-y-3">
			{#each rows as row (row.field.canonical)}
				{@const isDone = row.status === 'done'}
				{@const isWriting = row.status === 'writing'}
				{@const isError = row.status === 'error'}
				<div class="flex items-start gap-3" class:opacity-50={!row.checked && !isDone && !isError}>
					<!-- Checkbox / status icon -->
					<div class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center">
						{#if isDone}
							<svg class="h-4 w-4 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" aria-hidden="true">
								<path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
							</svg>
						{:else if isWriting}
							<svg class="h-4 w-4 animate-spin text-muted" viewBox="0 0 24 24" fill="none" aria-hidden="true">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
							</svg>
						{:else if isError}
							<svg class="h-4 w-4 text-warn" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
								<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
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
						<label
							for="wb-{row.field.canonical}"
							class="mb-1 block text-xs font-medium text-muted"
						>{row.field.label}</label>

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

						{#if isError}
							<p class="mt-1 text-xs text-warn" aria-live="polite">{row.error}</p>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	</div>

	<!-- Footer -->
	<div class="flex flex-col gap-2 border-t border-rule px-4 py-3">
		{#if busy}
			<div class="flex items-center gap-2 text-xs text-muted" aria-live="polite">
				<span>Writing field {totalWritten + 1} of {checkedCount}…</span>
				<div class="h-1 flex-1 overflow-hidden rounded-full bg-bg">
					<div
						class="h-full rounded-full bg-accent transition-all"
						style="width: {checkedCount > 0 ? (totalWritten / checkedCount) * 100 : 0}%"
					></div>
				</div>
			</div>
		{:else if hasErrors}
			<p class="text-xs text-warn" aria-live="polite">
				Some fields failed — uncheck written fields and retry, or close to keep partial results.
			</p>
		{/if}

		<div class="flex justify-end gap-2">
			<button
				onclick={() => onclose()}
				disabled={busy}
				class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-bg disabled:opacity-40"
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
