<script lang="ts">
	// Destructive confirm dialog (F24, ADR-037): a warn-styled variant of the modal
	// idiom used by PersonPicker/EnrichPicker — role=dialog, focus trap, Esc/backdrop
	// cancel, focus returned to the trigger. Initial focus lands on Cancel so an
	// accidental Enter never deletes. Tokens only; QA 3 skins.
	import { onMount, type Snippet } from 'svelte';

	let {
		title,
		confirmLabel,
		body,
		busy = false,
		error = '',
		onconfirm,
		oncancel
	}: {
		title: string;
		confirmLabel: string;
		body: Snippet;
		busy?: boolean;
		error?: string;
		onconfirm: () => void;
		oncancel: () => void;
	} = $props();

	let dialogEl = $state<HTMLElement | null>(null);
	let cancelBtn = $state<HTMLButtonElement | null>(null);
	let trigger: HTMLElement | null = null;

	onMount(() => {
		trigger = document.activeElement as HTMLElement | null;
		cancelBtn?.focus(); // safe default for a destructive dialog
		return () => trigger?.focus?.();
	});

	function trapTab(e: KeyboardEvent) {
		if (e.key !== 'Tab' || !dialogEl) return;
		const f = [...dialogEl.querySelectorAll<HTMLElement>('button')].filter(
			(el) => !(el as HTMLButtonElement).disabled && el.offsetParent !== null
		);
		if (f.length === 0) return;
		const first = f[0];
		const last = f[f.length - 1];
		if (e.shiftKey && document.activeElement === first) {
			e.preventDefault();
			last.focus();
		} else if (!e.shiftKey && document.activeElement === last) {
			e.preventDefault();
			first.focus();
		}
	}

	function cancel() {
		if (!busy) oncancel();
	}
</script>

<div
	class="fixed inset-0 z-50 flex items-start justify-center bg-bg/70 px-4 py-[12vh]"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget) cancel();
	}}
>
	<div
		bind:this={dialogEl}
		onkeydown={trapTab}
		tabindex="-1"
		class="confirm-pop w-full max-w-md rounded-theme border border-rule bg-surface p-4 shadow-xl"
		role="dialog"
		aria-modal="true"
		aria-labelledby="confirm-title"
		aria-describedby="confirm-body"
	>
		<h2 id="confirm-title" class="skin-title text-lg font-semibold text-ink">{title}</h2>
		<div id="confirm-body" class="mt-2 space-y-2 text-sm text-ink">
			{@render body()}
		</div>
		{#if error}
			<p class="mt-3 text-sm text-warn" aria-live="polite">{error}</p>
		{/if}
		<div class="mt-4 flex flex-wrap items-center justify-end gap-2">
			<button
				bind:this={cancelBtn}
				onclick={cancel}
				disabled={busy}
				class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface-2 disabled:opacity-60"
			>
				Cancel
			</button>
			<button
				onclick={onconfirm}
				disabled={busy}
				class="rounded-theme bg-warn px-3 py-1.5 text-sm font-semibold text-warn-ink disabled:opacity-60"
			>
				{busy ? 'Working…' : confirmLabel}
			</button>
		</div>
	</div>
</div>

<svelte:window onkeydown={(e) => e.key === 'Escape' && cancel()} />

<style>
	@media (prefers-reduced-motion: no-preference) {
		.confirm-pop {
			animation: confirm-rise 0.15s cubic-bezier(0.2, 0.7, 0.2, 1) both;
		}
	}
	@keyframes confirm-rise {
		from {
			opacity: 0;
			transform: scale(0.98);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}
</style>
