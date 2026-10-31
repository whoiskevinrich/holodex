<script lang="ts" generics="T, S extends string">
	// The shared Table view (F82, docs/design/list-views-handoff.md): one <table> for every
	// list page, which supplies only its columns. Rules live in lib/listTable.ts. Clicking a
	// header picks the matching Sort entry through `onsort` — the page's one sort, never a
	// second state (HOLODEX-473). The first cell holds the item's real link; a click anywhere
	// else on the row follows it too. A table row as a positioning context is uneven across
	// browsers, so the row doesn't stretch the link over itself. Renders nothing with no items:
	// the page shows its own empty state.
	//
	// `window` mode (HOLODEX-601) draws a whole-list scroll instead of `items`: the rows near
	// the view between two spacer rows (a real <table> can't take absolutely placed rows),
	// placeholders for entries still arriving, a retry row for a failed page, and roving
	// focus on the row. `aria-rowcount` counts the header, so a body row's index is its
	// entry index + 2. `shared/VirtualRows` is the grid twin.
	import { untrack, type Snippet } from 'svelte';
	import { goto } from '$app/navigation';
	import type { SortOption } from '#lib/listState.ts';
	import type { SparseList } from '#lib/sparseList.svelte.ts';
	import {
		pinnedEntry,
		retryRows,
		windowList,
		type RovingList,
		type WindowVirtualizer
	} from '#lib/windowVirtualizer.svelte.ts';
	import {
		ariaSort,
		headerClick,
		keepClass,
		visibleColumns,
		type ColumnSpec
	} from '#lib/listTable.ts';

	type Column = ColumnSpec<S> & { cell: Snippet<[T]> };

	let {
		columns,
		items = [],
		key,
		href,
		sort,
		sorts,
		owner,
		onsort,
		window: win,
		root = $bindable()
	}: {
		columns: readonly Column[];
		items?: readonly T[];
		key: (item: T) => string | number;
		href: (item: T) => string;
		sort: S;
		sorts: readonly SortOption<S>[];
		owner: boolean;
		onsort: (s: S) => void;
		window?: {
			list: SparseList<T>;
			virt: WindowVirtualizer;
			roving: RovingList;
			/** The table's accessible name and the retry row's noun ("people"). */
			label: string;
			noun: string;
		};
		/** The <table>, bound out in window mode (the page's A–Z jump focuses inside it). */
		root?: HTMLElement;
	} = $props();

	// The window prop is set once per mount (the page swaps the table out, not the prop).
	if (untrack(() => win))
		windowList({
			get list() {
				return win!.list;
			},
			get virt() {
				return win!.virt;
			},
			get roving() {
				return win!.roving;
			},
			lanes: () => 1,
			root: () => root
		});
	const total = $derived(win?.list.total ?? 0);
	const winRows = $derived(win ? Array.from({ length: Math.max(win.virt.end - win.virt.start, 0) }, (_, k) => win.virt.start + k) : []);
	const retries = $derived(win ? retryRows(win.list, 1) : new Map<number, number>());
	const pinned = $derived(win ? pinnedEntry(win.roving, win.virt, total) : -1);

	const shown = $derived(visibleColumns(columns, owner));

	const alignClass = (c: Column) => (c.align === 'right' ? 'text-right tabular-nums' : 'text-left');

	function clickHeader(c: Column) {
		const next = headerClick(c, sort, sorts, owner);
		if (next !== null) onsort(next);
	}

	// The active sort's label, read out with its header: "Added, sorted by Date added — newest".
	const sortLabel = $derived(sorts.find((o) => o.value === sort)?.label ?? '');

	function clickRow(e: MouseEvent, item: T) {
		// The link and any control in a cell handle their own clicks. A modified or non-primary
		// click, or one that ends a text selection, is left alone rather than navigating here.
		if ((e.target as Element).closest('a, button, input, select, [role="menu"]')) return;
		if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
		if (window.getSelection()?.toString()) return;
		goto(href(item));
	}
</script>

{#snippet itemCells(item: T)}
	{#each shown as c, i (c.id)}
		<td class="px-2 {c.noClip ? 'relative overflow-visible' : 'truncate'} {alignClass(c)} {keepClass(c)} {c.align === 'right' ? 'text-muted' : ''}">
			{#if i === 0}
				<a href={href(item)} class="block truncate focus-visible:outline-none">{@render c.cell(item)}</a>
			{:else}
				{@render c.cell(item)}
			{/if}
		</td>
	{/each}
{/snippet}

<!-- A window-mode row: the entry once its page is in, else its placeholder (the 32px avatar
     box and a bar per cell, handoff "Placeholders"). -->
{#snippet windowRow(i: number, hidden = false)}
	{@const item = win!.list.at(i)}
	{@const failed = win!.list.stateAt(i)?.status === 'failed'}
	<tr
		data-row
		data-index={i}
		tabindex={i === win!.roving.focus ? 0 : -1}
		aria-rowindex={i + 2}
		aria-busy={item === undefined && !failed ? 'true' : undefined}
		onclick={(e) => item !== undefined && clickRow(e, item)}
		class="h-11 cursor-pointer border-b border-rule text-ink outline-none hover:bg-surface-2 focus-visible:outline focus-visible:-outline-offset-1 focus-visible:outline-accent {failed ? 'opacity-55' : ''} {hidden ? 'sr-only' : ''}"
	>
		{#if item !== undefined}
			{@render itemCells(item)}
		{:else}
			{#each shown as c, k (c.id)}
				<td class="px-2 {keepClass(c)}">
					<span class="flex items-center gap-3 {c.align === 'right' ? 'justify-end' : ''}">
						{#if k === 0}<span class="h-8 w-8 shrink-0 border border-rule bg-surface-2"></span>{/if}
						<span class="h-2.5 rounded-theme bg-rule {k === 0 ? 'w-2/5' : 'w-3.5'}"></span>
					</span>
				</td>
			{/each}
		{/if}
	</tr>
{/snippet}

{#if win && total > 0}
	<!-- The table hears its rows' arrow keys (delegated, so one handler serves them all). -->
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<table
		bind:this={root}
		aria-label={win.label}
		aria-rowcount={total + 1}
		data-roving
		onkeydown={win.roving.keydown}
		onfocusin={win.roving.focusin}
		class="w-full table-fixed text-left text-sm"
	>
		{@render head()}
		<tbody>
			{#if win.virt.start > 0}
				<tr aria-hidden="true"><td colspan={shown.length} class="p-0" style:height="{win.virt.start * win.virt.pitch}px"></td></tr>
			{/if}
			{#each winRows as r (r)}
				{#if retries.has(r)}
					<tr class="h-11 border-b border-warn">
						<td colspan={shown.length} class="px-2">
							<span class="flex items-center gap-4">
								<span class="flex-1 text-warn">Couldn’t load these {win.noun}.</span>
								<button type="button" class="btn-ghost px-3 py-1 text-sm" onclick={() => win.list.retry(retries.get(r)!)}>Retry</button>
							</span>
						</td>
					</tr>
				{:else}
					{@render windowRow(r)}
				{/if}
			{/each}
			{#if win.virt.rowCount > win.virt.end}
				<tr aria-hidden="true"><td colspan={shown.length} class="p-0" style:height="{(win.virt.rowCount - win.virt.end) * win.virt.pitch}px"></td></tr>
			{/if}
			{#if pinned >= 0}{@render windowRow(pinned, true)}{/if}
		</tbody>
	</table>
{:else if !win && items.length > 0}
	<table class="w-full table-fixed text-left text-sm">
		{@render head()}
		<tbody>
			{#each items as item (key(item))}
				<tr
					onclick={(e) => clickRow(e, item)}
					class="h-11 cursor-pointer border-b border-rule text-ink hover:bg-surface-2 has-[a:focus-visible]:outline has-[a:focus-visible]:-outline-offset-1 has-[a:focus-visible]:outline-accent"
				>
					{@render itemCells(item)}
				</tr>
			{/each}
		</tbody>
	</table>
{/if}

{#snippet head()}
		<thead>
			<tr class="border-b border-rule">
				{#each shown as c (c.id)}
					{@const dir = ariaSort(c, sort, sorts, owner)}
					<th
						scope="col"
						aria-sort={dir}
						style:width={c.width}
						class="px-2 py-1.5 text-xs font-medium uppercase tracking-wide {alignClass(c)} {keepClass(c)} {dir === 'ascending' || dir === 'descending' ? 'text-accent' : 'text-muted'}"
					>
						{#if c.hideHeader}
							<span class="sr-only">{c.header}</span>
						{:else if dir === undefined}
							{c.header}
						{:else}
							<button
								type="button"
								onclick={() => clickHeader(c)}
								class="inline-flex w-full items-center gap-1 uppercase tracking-wide hover:text-ink focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent {c.align === 'right' ? 'justify-end' : ''}"
							>
								{c.header}{#if dir === 'ascending'}<span aria-hidden="true">↑</span>{:else if dir === 'descending'}<span aria-hidden="true">↓</span>{/if}{#if dir !== 'none'}<span class="sr-only">, sorted by {sortLabel}</span>{/if}
							</button>
						{/if}
					</th>
				{/each}
			</tr>
		</thead>
{/snippet}
