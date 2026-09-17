<script lang="ts">
	// `?` keyboard-shortcuts sheet (F62, HOLODEX-405). Two groups: "This page" is derived
	// from the hotkey registry (whatever `use:hotkey` buttons are mounted right now);
	// "Navigation" is the static list of keys that predate the registry (Ctrl-K in the
	// layout, `/`/arrows/Esc on the browse page) — listed so the sheet is honest, not
	// migrated (spec Non-Goals). Surface, trap, and rise motion mirror ConfirmDialog.
	import { onMount } from 'svelte';
	import { hotkeys, fire, type HotkeyEntry } from '$lib/actions/hotkey.svelte';

	let { onclose }: { onclose: () => void } = $props();

	let dialogEl = $state<HTMLDivElement | null>(null);
	let trigger: HTMLElement | null = null;

	const isMac =
		typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.userAgent);
	const NAV: { keys: string[]; label: string }[] = [
		{ keys: [isMac ? '⌘' : 'Ctrl', 'K'], label: 'Focus search' },
		{ keys: ['/'], label: 'Focus search' },
		{ keys: ['←', '→'], label: 'Move between cards' },
		{ keys: ['Esc'], label: 'Close · clear filters' }
	];

	// Past six rows a group flows into two columns so the panel never scrolls (handoff).
	const cols = (n: number) => (n > 6 ? 'sm:grid-cols-2' : '');

	onMount(() => {
		trigger = document.activeElement as HTMLElement | null;
		dialogEl?.focus();
		return () => trigger?.focus?.();
	});

	function trapTab(e: KeyboardEvent) {
		if (e.key !== 'Tab' || !dialogEl) return;
		const f = [...dialogEl.querySelectorAll<HTMLElement>('button')].filter(
			(el) => el.offsetParent !== null
		);
		if (f.length === 0) {
			e.preventDefault();
			return;
		}
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

	// P1-1: a row is a launcher — close, then run the same path a keypress takes.
	// `fire` does its own focus(), so the unmount focus-restore is suppressed: if the
	// click opened a dialog, a deferred trigger.focus() would steal focus back out of it.
	function run(entry: HotkeyEntry) {
		trigger = null;
		onclose();
		fire(entry);
	}
</script>

<div
	class="fixed inset-0 z-50 flex items-start justify-center bg-bg/70 px-4 py-[12vh]"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget) onclose();
	}}
>
	<div
		bind:this={dialogEl}
		onkeydown={trapTab}
		tabindex="-1"
		class="confirm-pop w-full max-w-md rounded-theme border border-rule bg-surface p-4 shadow-xl"
		role="dialog"
		aria-modal="true"
		aria-labelledby="hotkeys-title"
		data-hotkey-sheet
	>
		<h2 id="hotkeys-title" class="skin-title text-lg font-semibold text-ink">Keyboard shortcuts</h2>

		<h3 class="mt-3 mb-1 text-[0.65rem] tracking-wide text-muted uppercase">This page</h3>
		{#if hotkeys.rows.length === 0}
			<p class="text-sm text-muted italic">No page shortcuts here</p>
		{:else}
			<ul class="-mx-1.5 grid {cols(hotkeys.rows.length)} gap-x-4">
				{#each hotkeys.rows as entry (entry.key)}
					<li>
						<button
							type="button"
							onclick={() => run(entry)}
							class="flex w-full items-center gap-2.5 rounded-theme px-1.5 py-1 text-left text-sm text-ink hover:bg-surface-2 focus-visible:bg-surface-2"
						>
							<kbd
								class="inline-block min-w-[1.25rem] rounded-theme border border-rule bg-surface-2 px-1.5 py-px text-center font-mono text-xs text-ink"
								>{entry.key}</kbd
							>
							<span class="truncate">{entry.label}</span>
						</button>
					</li>
				{/each}
			</ul>
		{/if}

		<h3 class="mt-3 mb-1 text-[0.65rem] tracking-wide text-muted uppercase">Navigation</h3>
		<ul class="-mx-1.5 grid {cols(NAV.length)} gap-x-4">
			{#each NAV as row (row.label + row.keys.join())}
				<li class="flex items-center gap-2.5 px-1.5 py-1 text-sm text-ink">
					<span class="flex gap-1">
						{#each row.keys as k (k)}
							<kbd
								class="inline-block min-w-[1.25rem] rounded-theme border border-rule bg-surface-2 px-1.5 py-px text-center font-mono text-xs text-ink"
								>{k}</kbd
							>
						{/each}
					</span>
					<span class="truncate">{row.label}</span>
				</li>
			{/each}
		</ul>

		<p class="mt-3 flex items-center gap-2.5 text-xs text-muted">
			<kbd
				class="inline-block min-w-[1.25rem] rounded-theme border border-rule bg-surface-2 px-1.5 py-px text-center font-mono text-xs text-ink"
				>?</kbd
			>
			<span class="ml-auto">toggles this sheet</span>
		</p>
	</div>
</div>

<!-- Capture + stopPropagation (the `use:dismissable` idiom): Escape closes only this
     sheet and never reaches a page's own window listener (browse's Esc clears filters). -->
<svelte:window
	onkeydowncapture={(e) => {
		if (e.key === 'Escape') {
			e.stopPropagation();
			onclose();
		}
	}}
/>

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
