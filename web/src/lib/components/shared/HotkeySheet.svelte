<script lang="ts">
	// `?` keyboard-shortcuts sheet (F62, HOLODEX-405). Two groups: "This page" is derived
	// from the hotkey registry (whatever `use:hotkey` buttons are mounted right now);
	// "Navigation" is the static list of keys that predate the registry (Ctrl-K in the
	// layout, `/`/arrows/Esc on the browse page) — listed so the sheet is honest, not
	// migrated (spec Non-Goals). Modal (shared/Modal.svelte) owns the shell: surface, trap,
	// rise motion, and Escape — handled on `document` before any window listener, so it never
	// reaches a page's own (browse's Esc clears filters).
	import { onMount } from 'svelte';
	import Modal from '#lib/components/shared/Modal.svelte';
	import { hotkeys, fire, type HotkeyEntry } from '#lib/actions/hotkey.svelte.ts';

	let { onclose }: { onclose: () => void } = $props();

	let dialogEl = $state<HTMLElement | null>(null);
	let returnFocus = $state(true);

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

	// Rows are launchers, not the obvious first stop, so focus the panel itself rather than
	// Modal's first focusable. Runs after Modal's onMount, so this wins.
	onMount(() => dialogEl?.focus());

	// P1-1: a row is a launcher — close, then run the same path a keypress takes.
	// `fire` does its own focus(), so Modal's unmount focus-return is suppressed: if the
	// click opened a dialog, a deferred opener.focus() would steal focus back out of it.
	function run(entry: HotkeyEntry) {
		returnFocus = false;
		onclose();
		fire(entry);
	}
</script>

<Modal titleId="hotkeys-title" {onclose} size="sm" align="top" {returnFocus} bind:panel={dialogEl} data-hotkey-sheet>
	<div class="p-4">
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
</Modal>
