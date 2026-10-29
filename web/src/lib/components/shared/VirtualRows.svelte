<script lang="ts" generics="T">
	// The whole-list scroll's grid body (HOLODEX-601, docs/design/entity-list-virtual-scroll-handoff.md):
	// rows of `lanes` entries between two spacers, so the page is as tall as the whole list
	// while only the rows near the view are drawn. An entry whose page hasn't arrived shows
	// the caller's `placeholder` in the entry's own box; a failed page gets one retry row on
	// its first whole grid row and its other rows dimmed. The `[data-index]` cell around each
	// entry is the roving Tab stop and carries its list position. Table view's twin is
	// `sort/ListTable`'s `window` mode; both are wired by `windowList()`.
	import type { Snippet } from 'svelte';
	import type { SparseList } from '#lib/sparseList.svelte.ts';
	import {
		pinnedEntry,
		retryRows,
		windowList,
		type RovingList,
		type WindowVirtualizer
	} from '#lib/windowVirtualizer.svelte.ts';

	let {
		list,
		virt,
		roving,
		lanes,
		label,
		noun,
		rowClass,
		entry,
		placeholder,
		root = $bindable()
	}: {
		list: SparseList<T>;
		virt: WindowVirtualizer;
		roving: RovingList;
		lanes: number;
		/** The list's accessible name ("People"). */
		label: string;
		/** The retry row's noun ("people", "studios"). */
		noun: string;
		/** One row's classes: its gap between entries and, as bottom padding, the gap below it. */
		rowClass: string;
		entry: Snippet<[T, number]>;
		placeholder: Snippet<[number]>;
		root?: HTMLElement;
	} = $props();

	windowList({
		get list() {
			return list;
		},
		get virt() {
			return virt;
		},
		get roving() {
			return roving;
		},
		lanes: () => lanes,
		root: () => root
	});

	const total = $derived(list.total);
	const rows = $derived(Array.from({ length: Math.max(virt.end - virt.start, 0) }, (_, k) => virt.start + k));
	const retries = $derived(retryRows(list, lanes));
	const pinned = $derived(pinnedEntry(roving, virt, total));

	function cells(row: number): number[] {
		const first = row * lanes;
		return Array.from({ length: Math.max(Math.min(lanes, total - first), 0) }, (_, k) => first + k);
	}
</script>

{#snippet cell(i: number)}
	{@const item = list.at(i)}
	{@const failed = list.stateAt(i)?.status === 'failed'}
	<!-- The entry is the roving Tab stop (EL13): one stop for the list, arrows inside it. -->
	<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
	<div
		role="listitem"
		data-index={i}
		tabindex={i === roving.focus ? 0 : -1}
		aria-setsize={total}
		aria-posinset={i + 1}
		aria-busy={item === undefined && !failed ? 'true' : undefined}
		class="group min-w-0 outline-none {failed ? 'opacity-55' : ''}"
	>
		{#if item !== undefined}{@render entry(item, i)}{:else}{@render placeholder(i)}{/if}
	</div>
{/snippet}

<!-- The list hears its entries' arrow keys (delegated, so one handler serves them all). -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	bind:this={root}
	role="list"
	aria-label={label}
	data-roving
	onkeydown={roving.keydown}
	onfocusin={roving.focusin}
>
	<div aria-hidden="true" style:height="{virt.start * virt.pitch}px"></div>
	{#each rows as r (r)}
		{#if retries.has(r)}
			<!-- Exactly one row's pitch (border-box, so the row's bottom gap stays inside it). -->
			<div class={rowClass} style:height="{virt.pitch}px">
				<div class="flex h-full items-center gap-4 rounded-theme border border-warn bg-surface px-4" style:grid-column="1 / -1">
					<span class="flex-1 text-sm text-warn">Couldn’t load these {noun}.</span>
					<button type="button" class="btn-ghost px-3 py-1.5 text-sm" onclick={() => list.retry(retries.get(r)!)}>
						Retry
					</button>
				</div>
			</div>
		{:else}
			<div data-row class={rowClass} style:grid-template-columns="repeat({lanes}, minmax(0, 1fr))">
				{#each cells(r) as i (i)}{@render cell(i)}{/each}
			</div>
		{/if}
	{/each}
	<div aria-hidden="true" style:height="{Math.max(virt.rowCount - virt.end, 0) * virt.pitch}px"></div>
	{#if pinned >= 0}
		<div class="sr-only">{@render cell(pinned)}</div>
	{/if}
</div>
