<script module lang="ts">
	// Open modals, oldest first. Escape and backdrop dismissal belong to the topmost one only,
	// and a modal opened over another sits one layer up (HOLODEX-215).
	const stack: symbol[] = [];
</script>

<script lang="ts">
	// The one modal shell (HOLODEX-215, docs/design/shared-modal-handoff.md): backdrop, panel
	// surface + rise motion, role="dialog" wiring, focus capture/return, Tab trap, and
	// Escape/backdrop dismissal (ignored while `busy`). Callers own everything inside the
	// panel — padding, title (with `titleId`), body, buttons. Initial focus goes to the first
	// focusable control (else the panel); a caller that wants another target focuses it in its
	// own onMount, which runs after this one. Tokens only; QA Cinémathèque.
	import { onMount, type Snippet } from 'svelte';
	import { firstFocusable, trapTab } from '#lib/focusTrap.ts';

	let {
		titleId,
		onclose,
		size = 'md',
		align = 'top',
		busy = false,
		describedBy,
		returnFocus = true,
		panel = $bindable(null),
		children,
		...rest
	}: {
		titleId: string;
		onclose: () => void;
		/** sm 28rem · md 32rem · lg 48rem · media 64rem. */
		size?: 'sm' | 'md' | 'lg' | 'media';
		/** `top` (10vh) for forms, pickers and confirms; `center` for media. */
		align?: 'top' | 'center';
		/** Escape and backdrop clicks are ignored while true. */
		busy?: boolean;
		describedBy?: string;
		/** False when the caller moves focus itself on close (HotkeySheet's launcher rows). */
		returnFocus?: boolean;
		panel?: HTMLElement | null;
		children: Snippet;
		/** Pass-through `data-*` attributes for the panel (e.g. `data-hotkey-sheet`). */
		[key: `data-${string}`]: unknown;
	} = $props();

	const WIDTH = { sm: 'max-w-md', md: 'max-w-lg', lg: 'max-w-3xl', media: 'max-w-5xl' } as const;

	const id = Symbol('modal');
	let stacked = $state(false);

	function isTop() {
		return stack[stack.length - 1] === id;
	}

	function dismiss() {
		if (!busy) onclose();
	}

	onMount(() => {
		const opener = document.activeElement as HTMLElement | null;
		stacked = stack.length > 0;
		stack.push(id);
		(firstFocusable(panel) ?? panel)?.focus();

		// On `document`, bubble phase: after a control inside the panel that claims its own
		// Escape (SourceChipRow stops propagation), before any page's window-level listener. A
		// control that preventDefault()s its Escape has claimed it too (HOLODEX-249).
		function onKeydown(e: KeyboardEvent) {
			if (e.key !== 'Escape' || e.defaultPrevented || !isTop()) return;
			e.preventDefault();
			e.stopPropagation();
			dismiss();
		}
		document.addEventListener('keydown', onKeydown);

		return () => {
			document.removeEventListener('keydown', onKeydown);
			stack.splice(stack.indexOf(id), 1);
			if (returnFocus && opener?.isConnected) opener.focus();
		};
	});
</script>

<div
	class="fixed inset-0 flex justify-center px-4 {stacked ? 'z-[60] bg-bg/85' : 'z-50 bg-bg/70'} {align === 'center'
		? 'items-center py-[6vh]'
		: 'items-start py-[10vh]'}"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget && isTop()) dismiss();
	}}
>
	<div
		{...rest}
		bind:this={panel}
		onkeydown={(e) => trapTab(e, panel)}
		tabindex="-1"
		class="modal-pop flex w-full {WIDTH[size]} {size === 'media'
			? 'max-h-[85vh]'
			: 'max-h-[80vh]'} flex-col rounded-theme border border-rule bg-surface shadow-xl"
		role="dialog"
		aria-modal="true"
		aria-labelledby={titleId}
		aria-describedby={describedBy}
	>
		{@render children()}
	</div>
</div>

<style>
	@media (prefers-reduced-motion: no-preference) {
		.modal-pop {
			animation: modal-rise 0.15s cubic-bezier(0.2, 0.7, 0.2, 1) both;
		}
	}
	@keyframes modal-rise {
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
