<script lang="ts">
	import PageActions from '../sort/PageActions.svelte';
	import type { RunMode } from '$lib/run';

	// Play all ▾ (F75, design handoff "The split button"). The main half starts a run in
	// order; the caret opens PageActions' menu (one menu, not two) with Play all and
	// Shuffle. With `shuffleFirst` (a playlist's Always shuffle) the main half is Shuffle
	// and the menu's second item is Play in order. The main half never changes to the
	// last-used option (owner decision). `onplay` resolves false when the run couldn't
	// start; the button then says so beside itself (there is no toast system).
	let {
		variant = 'accent',
		shuffleFirst = false,
		disabled = false,
		onplay
	}: {
		variant?: 'accent' | 'primary';
		shuffleFirst?: boolean;
		disabled?: boolean;
		onplay: (mode: RunMode) => Promise<boolean>;
	} = $props();

	let busy = $state(false);
	let failed = $state(false);

	async function play(mode: RunMode) {
		if (busy || disabled) return;
		busy = true;
		failed = false;
		const ok = await onplay(mode);
		busy = false;
		failed = !ok;
	}

	const primaryMode = $derived<RunMode>(shuffleFirst ? 'shuffled' : 'in-order');
	const items = $derived(
		shuffleFirst
			? [
					{ label: 'Shuffle', icon: shuffleIcon, onselect: () => play('shuffled') },
					{ label: '▶ Play in order', onselect: () => play('in-order') }
				]
			: [
					{ label: '▶ Play all', hint: 'in order', onselect: () => play('in-order') },
					{ label: 'Shuffle', icon: shuffleIcon, onselect: () => play('shuffled') }
				]
	);

	const halves = $derived(
		variant === 'primary'
			? {
					main: 'rounded-theme rounded-r-none bg-accent px-4 py-2 text-sm font-medium text-accent-ink',
					caret: 'rounded-theme rounded-l-none border-l border-accent-ink bg-accent px-2 py-2 text-sm text-accent-ink'
				}
			: {
					main: 'btn-accent rounded-r-none px-3 py-1.5 text-sm',
					caret: 'btn-accent rounded-l-none border-l-accent px-2 py-1.5 text-sm'
				}
	);
</script>

{#snippet shuffleIcon()}
	<svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
		<path d="M2 4h3l6 8h3M2 12h3l6-8h3M12 2l2 2-2 2M12 10l2 2-2 2" />
	</svg>
{/snippet}

{#snippet caretFace()}▾{/snippet}

<span class="inline-flex flex-wrap items-center gap-2">
	{#if disabled}
		<!-- Withdrawn (F69's treatment): ghost look, full-contrast label, never opacity. -->
		<span class="inline-flex">
			<span class="btn-ghost rounded-r-none px-3 py-1.5 text-sm" aria-disabled="true">
				{shuffleFirst ? 'Shuffle' : '▶ Play all'}
			</span>
			<span class="btn-ghost rounded-l-none border-l-0 px-2 py-1.5 text-sm" aria-disabled="true">▾</span>
		</span>
	{:else}
		<span class="inline-flex">
			<button type="button" class={halves.main} aria-busy={busy} onclick={() => play(primaryMode)}>
				{#if shuffleFirst}
					<span class="inline-flex items-center gap-1.5">{@render shuffleIcon()} Shuffle</span>
				{:else}
					▶ Play all
				{/if}
			</button>
			<PageActions {items} label="More play options" face={caretFace} triggerClass={halves.caret} />
		</span>
	{/if}
	{#if failed}<span class="text-sm text-warn" role="alert">Couldn't start playback</span>{/if}
</span>
