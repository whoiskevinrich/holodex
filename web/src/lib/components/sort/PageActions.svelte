<script module lang="ts">
	export interface PageAction {
		label: string;
		onselect: () => void;
		/** Only offered below `sm`, where the toolbar has no room for it inline. */
		compactOnly?: boolean;
	}
</script>

<script lang="ts">
	// The owner's page-actions menu (⋯) at the far right of the list toolbar (F73): Merge
	// people, Manage tags, Save as playlist… Same trigger/menu markup as the ⋯ menus on
	// EnrichProviderChips and /tags; arrow keys move between items, Escape returns focus.
	import { tick } from 'svelte';
	import { dismissable } from '$lib/actions/dismissable';

	// `label` names the trigger for a ⋯ that isn't the page's own (Tags' manage mode bar).
	let { items, label = 'Page actions' }: { items: PageAction[]; label?: string } = $props();

	const uid = $props.id();
	let open = $state(false);
	let trigger = $state<HTMLButtonElement | null>(null);
	let menu = $state<HTMLElement | null>(null);

	function menuItems(): HTMLElement[] {
		return [...(menu?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])].filter(
			(el) => el.offsetParent !== null
		);
	}

	async function show() {
		open = true;
		await tick();
		menuItems()[0]?.focus();
	}

	function close(returnFocus = true) {
		open = false;
		if (returnFocus) trigger?.focus();
	}

	function onkeydown(e: KeyboardEvent) {
		const list = menuItems();
		const i = list.indexOf(document.activeElement as HTMLElement);
		const to =
			e.key === 'ArrowDown' ? (i + 1) % list.length
			: e.key === 'ArrowUp' ? (i <= 0 ? list.length - 1 : i - 1)
			: e.key === 'Home' ? 0
			: e.key === 'End' ? list.length - 1
			: -1;
		if (to < 0) return;
		e.preventDefault();
		list[to]?.focus();
	}

	// Tab (or any focus move) out of the menu closes it, as a menu should.
	function onfocusout(e: FocusEvent) {
		const next = e.relatedTarget;
		if (next instanceof Node && !menu?.parentElement?.contains(next)) close(false);
	}
</script>

<div
	class="relative"
	data-page-actions={uid}
	use:dismissable={{ enabled: open, inside: `[data-page-actions="${uid}"]`, onclose: (viaEscape) => close(viaEscape) }}
>
	<button
		bind:this={trigger}
		type="button"
		aria-haspopup="menu"
		aria-expanded={open}
		aria-label={label}
		onclick={() => (open ? close() : show())}
		class="inline-flex h-8 w-8 items-center justify-center rounded-theme border border-rule bg-surface text-muted hover:text-ink"
	>
		⋯
	</button>
	{#if open}
		<div
			bind:this={menu}
			role="menu"
			tabindex="-1"
			{onkeydown}
			{onfocusout}
			class="absolute right-0 top-full z-20 mt-1 min-w-max rounded-theme border border-rule bg-surface p-1 shadow-sm"
		>
			{#each items as item (item.label)}
				<button
					type="button"
					role="menuitem"
					onclick={() => {
						close();
						item.onselect();
					}}
					class="block w-full rounded-theme px-3 py-1.5 text-left text-sm text-ink hover:bg-surface-2 {item.compactOnly
						? 'sm:hidden'
						: ''}"
				>
					{item.label}
				</button>
			{/each}
		</div>
	{/if}
</div>
