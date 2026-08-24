<script module lang="ts">
	// Shared roving-tabindex option focus: id scheme is `${prefix}-${i}`, one per
	// consumer (EntityPicker's `merge-opt`, CategoryPicker's `category-opt`).
	export function focusOptionIn(dialogEl: HTMLElement | null, prefix: string, i: number) {
		dialogEl?.querySelector<HTMLElement>(`#${prefix}-${i}`)?.focus();
	}
</script>

<script lang="ts">
	// Shared dialog chrome for the entity/tag picker family (EntityPicker's merge
	// flow, CategoryPicker's assign/remove flow): backdrop, role="dialog" wrapper,
	// focus trap, trigger-focus save/restore, Escape-to-close, and the rise-in
	// animation. Title/subtitle (`header`) and body (`children`) are step-specific,
	// supplied by the caller. `dialogEl` is bindable so callers can drive their own
	// roving-tabindex focus (see `focusOptionIn` above) against the same element.
	import { onMount, type Snippet } from 'svelte';

	let {
		titleId,
		onclose,
		dialogEl = $bindable(null),
		header,
		children,
		widthClass = 'max-w-lg',
		paddingClass = 'py-[10vh]'
	}: {
		titleId: string;
		onclose: () => void;
		dialogEl?: HTMLElement | null;
		header: Snippet;
		children: Snippet;
		widthClass?: string;
		paddingClass?: string;
	} = $props();

	let trigger: HTMLElement | null = null;

	onMount(() => {
		trigger = document.activeElement as HTMLElement | null;
		return () => trigger?.focus?.();
	});

	function trapTab(e: KeyboardEvent) {
		if (e.key !== 'Tab' || !dialogEl) return;
		const f = [...dialogEl.querySelectorAll<HTMLElement>('input, button, [tabindex="0"]')].filter(
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
</script>

<div
	class="fixed inset-0 z-50 flex items-start justify-center bg-bg/70 px-4 {paddingClass}"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget) onclose();
	}}
>
	<div
		bind:this={dialogEl}
		onkeydown={trapTab}
		tabindex="-1"
		class="merge-pop flex max-h-[80vh] w-full {widthClass} flex-col rounded-theme border border-rule bg-surface p-4 shadow-xl"
		role="dialog"
		aria-modal="true"
		aria-labelledby={titleId}
	>
		<div class="mb-3 flex items-start justify-between gap-3">
			{@render header()}
			<button onclick={onclose} aria-label="Close" class="rounded-theme px-2 py-0.5 text-muted hover:text-ink">✕</button>
		</div>

		{@render children()}
	</div>
</div>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<style>
	@media (prefers-reduced-motion: no-preference) {
		.merge-pop {
			animation: merge-rise 0.15s cubic-bezier(0.2, 0.7, 0.2, 1) both;
		}
	}
	@keyframes merge-rise {
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
