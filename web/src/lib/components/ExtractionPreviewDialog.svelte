<script lang="ts">
	// Preview-before-write dialog (F48.7a, ADR-067) — reuses WritebackFormDialog's
	// chrome (backdrop, focus trap, per-row checkbox + status icon, footer submit)
	// but replaces the editable row body with a static old → new diff line, since
	// these values are already decided (staged by the Extraction tab's row actions),
	// not something the owner edits here. Each checked row is its own
	// resolveExtractionReview call — sequential, so per-row status reflects real
	// progress rather than a single all-or-nothing batch result. Tokens only; QA 3 skins.
	import { onMount } from 'svelte';
	import { toMessage } from '$lib/format';
	import type { ExtractionPreviewItem, ExtractionResolveAction } from '$lib/types';

	let {
		items,
		onclose,
		onsubmitted,
		resolve
	}: {
		items: ExtractionPreviewItem[];
		onclose: () => void;
		/** Called once with every reviewId that was successfully written+resolved —
		 *  the parent drops them from the queue and clears their staged pick. */
		onsubmitted: (resolvedReviewIds: number[]) => void;
		resolve: (reviewId: number, action: ExtractionResolveAction, value: string) => Promise<unknown>;
	} = $props();

	type RowStatus = 'idle' | 'writing' | 'done' | 'error';
	interface Row extends ExtractionPreviewItem {
		checked: boolean;
		status: RowStatus;
		error: string;
	}

	// svelte-ignore state_referenced_locally — items is stable for the dialog's lifetime
	const rows = $state<Row[]>(items.map((it) => ({ ...it, checked: true, status: 'idle', error: '' })));

	const checkedCount = $derived(rows.filter((r) => r.checked).length);
	const hasErrors = $derived(rows.some((r) => r.status === 'error'));

	let busy = $state(false);
	let dialogEl = $state<HTMLElement | null>(null);
	let trigger: HTMLElement | null = null;

	onMount(() => {
		trigger = document.activeElement as HTMLElement | null;
		dialogEl?.querySelector<HTMLElement>('input[type="checkbox"]:not(:disabled)')?.focus();
		return () => trigger?.focus?.();
	});

	function trapTab(e: KeyboardEvent) {
		if (e.key !== 'Tab' || !dialogEl) return;
		const focusable = [...dialogEl.querySelectorAll<HTMLElement>('input, button, [tabindex="0"]')].filter(
			(el) => !(el as HTMLButtonElement).disabled && el.offsetParent !== null
		);
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

	async function submit() {
		if (busy || checkedCount === 0) return;
		busy = true;

		const resolvedIds: number[] = [];
		for (const row of rows) {
			if (!row.checked) continue;
			row.status = 'writing';
			row.error = '';
			try {
				await resolve(row.reviewId, row.action, row.newValue);
				row.status = 'done';
				resolvedIds.push(row.reviewId);
			} catch (e) {
				row.status = 'error';
				row.error = toMessage(e);
			}
		}

		busy = false;
		if (resolvedIds.length > 0) onsubmitted(resolvedIds);
		if (!hasErrors) onclose();
	}
</script>

<div
	class="fixed inset-0 z-50 flex items-start justify-center bg-bg/70 px-4 py-[10vh]"
	onclick={(e) => {
		if (e.target === e.currentTarget && !busy) onclose();
	}}
>
	<div
		bind:this={dialogEl}
		role="dialog"
		aria-modal="true"
		aria-labelledby="extraction-preview-title"
		tabindex="-1"
		onkeydown={onKeydown}
		class="w-full max-w-xl overflow-hidden rounded-theme border border-rule bg-surface shadow-lg"
	>
		<div class="flex items-center justify-between border-b border-rule px-4 py-3">
			<h2 id="extraction-preview-title" class="text-sm font-semibold text-ink">
				Review {rows.length} change{rows.length === 1 ? '' : 's'}
			</h2>
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
			<div class="space-y-3">
				{#each rows as row (row.reviewId)}
					{@const isDone = row.status === 'done'}
					{@const isWriting = row.status === 'writing'}
					{@const isError = row.status === 'error'}
					<div class="flex items-start gap-3" class:opacity-50={!row.checked && !isDone && !isError}>
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
									id="ext-preview-{row.reviewId}"
									bind:checked={row.checked}
									disabled={busy}
									class="h-4 w-4 cursor-pointer accent-[var(--color-accent)] disabled:cursor-not-allowed"
								/>
							{/if}
						</div>

						<div class="min-w-0 flex-1">
							<label for="ext-preview-{row.reviewId}" class="mb-1 block text-xs font-medium text-muted">
								{row.videoTitle} · {row.fieldLabel}
							</label>
							<p class="text-sm">
								<span class="text-muted line-through decoration-warn">{row.oldValue || '(empty)'}</span>
								<span class="text-muted" aria-hidden="true">→</span>
								<span class="font-medium text-accent">{row.newValue}</span>
							</p>
							{#if isError}
								<p class="mt-1 text-xs text-warn" aria-live="polite">{row.error}</p>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		</div>

		<div class="flex flex-col gap-2 border-t border-rule px-4 py-3">
			{#if busy}
				<p class="text-xs text-muted" aria-live="polite">Writing {checkedCount} field{checkedCount === 1 ? '' : 's'} to file…</p>
			{:else if hasErrors}
				<p class="text-xs text-warn" aria-live="polite">
					Some writes failed — uncheck any rows you want to skip and try again.
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
