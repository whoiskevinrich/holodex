<script lang="ts">
	// The Filters slot of ListToolbar (F73): the trigger plus one panel shown two ways.
	// From `sm` up it's a non-modal popover under the button; below `sm` it's a modal bottom
	// sheet with a header, a footer and a focus trap. Fields apply live — there is no Apply
	// button — so closing never discards anything. Design handoff: list-toolbar-handoff.md.
	import type { Snippet } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fly } from 'svelte/transition';
	import { tick } from 'svelte';
	import { dismissable } from '$lib/actions/dismissable';
	import { firstFocusable, trapTab } from '$lib/focusTrap';
	import FiltersButton from './FiltersButton.svelte';

	let {
		count,
		resultLabel = '',
		onclear,
		children
	}: {
		/** Active filters, shown on the button. */
		count: number;
		/** The live result count for the sheet footer, e.g. "38 videos". */
		resultLabel?: string;
		onclear: () => void;
		children: Snippet;
	} = $props();

	const uid = $props.id();
	const panelId = `filters-${uid}`;
	// Tailwind's `sm` edge: the popover/sheet switch has to be a behaviour, not only a style,
	// because the sheet is modal (focus trap, aria-modal) and the popover is not.
	const wide = new MediaQuery('min-width: 640px');

	let open = $state(false);
	let trigger = $state<HTMLButtonElement | null>(null);
	let panel = $state<HTMLElement | null>(null);

	async function show() {
		open = true;
		await tick();
		firstFocusable(panel)?.focus();
	}

	function close(returnFocus = true) {
		open = false;
		if (returnFocus) trigger?.focus();
	}

	// Popover only: tabbing past its last field (or out of it any other way) closes it,
	// so a non-modal panel never lingers behind the focus.
	function onfocusout(e: FocusEvent) {
		if (!wide.current) return;
		const next = e.relatedTarget;
		if (next instanceof Node && !panel?.parentElement?.contains(next)) close(false);
	}
</script>

<div
	class="relative"
	data-filters={uid}
	use:dismissable={{ enabled: open, inside: `[data-filters="${uid}"]`, onclose: (viaEscape) => close(viaEscape) }}
>
	<FiltersButton bind:el={trigger} {count} expanded={open} controls={panelId} onclick={() => (open ? close() : show())} />

	{#if open}
		{#if !wide.current}
			<div class="fixed inset-0 z-40 bg-bg/70" role="presentation" onclick={() => close()}></div>
		{/if}
		<div
			bind:this={panel}
			id={panelId}
			role="dialog"
			aria-label="Filters"
			aria-modal={wide.current ? undefined : 'true'}
			tabindex="-1"
			onkeydown={(e) => !wide.current && trapTab(e, panel)}
			{onfocusout}
			transition:fly={{ y: 400, duration: wide.current || prefersReducedMotion.current ? 0 : 150 }}
			class={wide.current
				? 'absolute left-0 top-full z-20 mt-1 w-80 rounded-theme border border-rule bg-surface p-4 shadow-lg'
				: 'fixed inset-x-0 bottom-0 z-50 max-h-[80vh] overflow-y-auto border-t border-rule bg-surface p-4'}
		>
			{#if !wide.current}
				<div class="mb-4 flex items-center justify-between">
					<h2 class="font-display text-lg font-semibold text-ink">Filters</h2>
					<button type="button" class="btn-accent px-3 py-1 text-sm" onclick={() => close()}>Done</button>
				</div>
			{/if}
			<div class="space-y-4">{@render children()}</div>
			{#if !wide.current}
				<div class="mt-4 flex items-center justify-between text-sm">
					<button type="button" class="btn-quiet" onclick={onclear}>Clear all</button>
					<span class="text-muted">{resultLabel}</span>
				</div>
			{/if}
		</div>
	{/if}
</div>
