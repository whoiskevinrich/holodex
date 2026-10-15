<script lang="ts">
	// Destructive confirm dialog (F24, ADR-037): a warn-styled confirm on the shared
	// modal shell — Modal (shared/Modal.svelte) owns the backdrop, focus trap, Esc/backdrop
	// cancel (skipped while busy) and focus return. Initial focus lands on Cancel so an
	// accidental Enter never deletes. Tokens only; QA Cinémathèque.
	import { onMount, type Snippet } from 'svelte';
	import Modal from '#lib/components/shared/Modal.svelte';

	let {
		title,
		confirmLabel,
		cancelLabel = 'Cancel',
		body,
		busy = false,
		error = '',
		variant = 'destructive',
		onconfirm,
		oncancel
	}: {
		title: string;
		/** Omit with onconfirm for an informational state with no action (e.g. a refusal). */
		confirmLabel?: string;
		/** Ghost-button label; override when "Cancel" misreads (e.g. "Keep separate", F37). */
		cancelLabel?: string;
		body: Snippet;
		busy?: boolean;
		error?: string;
		/** 'destructive' (default) uses warn colours; 'accent' for non-destructive confirmations. */
		variant?: 'destructive' | 'accent';
		/** Omitted → no confirm button: the dialog only reports, and cancelLabel closes it. */
		onconfirm?: () => void;
		oncancel: () => void;
	} = $props();

	let dialogEl = $state<HTMLElement | null>(null);
	let cancelBtn = $state<HTMLButtonElement | null>(null);

	onMount(() => {
		// A checked radio in the body (e.g. a merge-target picker) takes initial focus;
		// otherwise Cancel is the safe default so an accidental Enter never confirms.
		// Runs after Modal's own first-focusable focus, so this wins.
		const checkedRadio = dialogEl?.querySelector<HTMLElement>('input[type="radio"]:checked');
		(checkedRadio ?? cancelBtn)?.focus();
	});

	function cancel() {
		if (!busy) oncancel();
	}
</script>

<Modal
	titleId="confirm-title"
	onclose={oncancel}
	size="sm"
	align="top"
	{busy}
	describedBy="confirm-body"
	bind:panel={dialogEl}
>
	<div class="flex min-h-0 flex-1 flex-col p-4">
		<h2 id="confirm-title" class="skin-title text-lg font-semibold text-ink">{title}</h2>
		<!-- The body is the only part allowed to scroll (HOLODEX-417): SourceEditModal renders a
		     paragraph per source, and an unbounded panel inside a fixed overlay put Save/Cancel
		     below the viewport with nothing to scroll. Same flex-column shell as EnrichPicker. -->
		<div id="confirm-body" class="mt-2 min-h-0 flex-1 space-y-2 overflow-y-auto text-sm text-ink">
			{@render body()}
		</div>
		{#if error}
			<p class="mt-3 shrink-0 text-sm text-warn" aria-live="polite">{error}</p>
		{/if}
		<div class="mt-4 flex shrink-0 flex-wrap items-center justify-end gap-2">
			<button
				bind:this={cancelBtn}
				onclick={cancel}
				disabled={busy}
				class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface-2 disabled:opacity-60"
			>
				{cancelLabel}
			</button>
			{#if onconfirm}
				<button
					onclick={onconfirm}
					disabled={busy}
					class="rounded-theme px-3 py-1.5 text-sm font-semibold disabled:opacity-60 {variant === 'accent'
						? 'bg-accent text-accent-ink'
						: 'bg-warn text-warn-ink'}"
				>
					{busy ? 'Working…' : confirmLabel}
				</button>
			{/if}
		</div>
	</div>
</Modal>
