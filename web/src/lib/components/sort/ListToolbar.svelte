<script lang="ts">
	// The one toolbar every list page renders (F73, design handoff list-toolbar-handoff.md).
	// Layout only — it owns the row, the slot order and the chips row, never list state
	// (that is $lib/listState). One row that never wraps:
	//   [Sort] [Reroll?] [Filters?]  ···ml-auto···  [View?] [⋯?]
	// An empty slot renders nothing. The right group is pushed over by one `ml-auto` wrapper,
	// not a spacer element: in a gap-2 row a spacer adds a gap the 375px budget lacks.
	import type { Snippet } from 'svelte';
	import SortReroll from './SortReroll.svelte';
	import PageActions, { type PageAction } from './PageActions.svelte';

	let {
		sort,
		reroll,
		filters,
		view,
		actions = [],
		chips,
		chipCount = 0,
		onclear,
		count,
		mode
	}: {
		/** The sort select (SortDropdown compact). */
		sort: Snippet;
		/** Set while the sort is Random. Below `sm` it moves into ⋯ when the page has one. */
		reroll?: () => void;
		/** The FilterPanel, on pages that have filters. */
		filters?: Snippet;
		/** View toggle and/or density slider. */
		view?: Snippet;
		/** Owner page actions; the ⋯ menu renders only when this is non-empty. */
		actions?: PageAction[];
		/** Active filter and scope chips. */
		chips?: Snippet;
		chipCount?: number;
		/** Clears every filter and scope; offered once two or more chips are showing. */
		onclear?: () => void;
		/** The count line ("38 videos", plus owner status links); announced politely. */
		count?: Snippet;
		/**
		 * A page mode (People's merge selection, Tags' manage mode). While set, it replaces
		 * the whole toolbar row — "2 selected · Merge · Cancel" — so a mode never stacks a
		 * second bar above the list (F73 divergences D2/D3, the owner's pick).
		 */
		mode?: Snippet;
	} = $props();

	const menu = $derived<PageAction[]>(
		reroll && actions.length ? [{ label: 'Shuffle again', onselect: reroll, compactOnly: true }, ...actions] : actions
	);
</script>

<div class="space-y-2">
	<div class="flex items-center gap-2" data-list-toolbar>
		{#if mode}
			{@render mode()}
		{:else}
		{@render sort()}
		{#if reroll}
			<span class={actions.length ? 'hidden sm:inline-flex' : 'inline-flex'}>
				<SortReroll onreroll={reroll} />
			</span>
		{/if}
		{@render filters?.()}
		{#if view || menu.length}
			<div class="ml-auto flex items-center gap-2">
				{@render view?.()}
				{#if menu.length}<PageActions items={menu} />{/if}
			</div>
		{/if}
		{/if}
	</div>

	{#if chips && chipCount > 0}
		<div class="flex items-center gap-1.5 overflow-x-auto sm:flex-wrap" data-list-chips>
			{@render chips()}
			{#if onclear && chipCount >= 2}
				<button type="button" class="btn-quiet shrink-0 text-xs" onclick={onclear}>Clear</button>
			{/if}
		</div>
	{/if}

	{#if count}
		<p class="text-sm text-muted" aria-live="polite">{@render count()}</p>
	{/if}
</div>
