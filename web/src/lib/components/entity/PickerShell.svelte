<script module lang="ts">
	// Shared roving-tabindex option focus: id scheme is `${prefix}-${i}`, one per
	// consumer (EntityPicker's `merge-opt`, CategoryPicker's `category-opt`).
	export function focusOptionIn(dialogEl: HTMLElement | null, prefix: string, i: number) {
		dialogEl?.querySelector<HTMLElement>(`#${prefix}-${i}`)?.focus();
	}
</script>

<script lang="ts">
	// Shared picker chrome for the entity/tag picker family (EntityPicker's merge
	// flow, CategoryPicker's assign/remove flow, the film attach dialogs): the header
	// row with its ✕ close, over Modal (shared/Modal.svelte), which owns the shell —
	// backdrop, role="dialog", focus trap, focus return, Escape, motion. Title/subtitle
	// (`header`) and body (`children`) are step-specific, supplied by the caller.
	// `dialogEl` is bindable so callers can drive their own roving-tabindex focus (see
	// `focusOptionIn` above) against the same element.
	import type { Snippet } from 'svelte';
	import Modal from '#lib/components/shared/Modal.svelte';

	let {
		titleId,
		onclose,
		dialogEl = $bindable(null),
		header,
		children,
		size = 'md'
	}: {
		titleId: string;
		onclose: () => void;
		dialogEl?: HTMLElement | null;
		header: Snippet;
		children: Snippet;
		/** md for every picker; lg for FilmBulkAttachDialog's wider candidate list. */
		size?: 'md' | 'lg';
	} = $props();
</script>

<Modal {titleId} {onclose} {size} align="top" bind:panel={dialogEl}>
	<div class="flex min-h-0 flex-1 flex-col p-4">
		<div class="mb-3 flex items-start justify-between gap-3">
			{@render header()}
			<button onclick={onclose} aria-label="Close" class="rounded-theme px-2 py-0.5 text-muted hover:text-ink">✕</button>
		</div>

		{@render children()}
	</div>
</Modal>
