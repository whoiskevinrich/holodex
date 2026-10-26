<script lang="ts" generics="T, S extends string">
	// The shared Table view (F82, docs/design/list-views-handoff.md): one <table> for every
	// list page, which supplies only its columns. Rules live in lib/listTable.ts. Clicking a
	// header picks the matching Sort entry through `onsort` — the page's one sort, never a
	// second state (HOLODEX-473). The first cell holds the item's real link; a click anywhere
	// else on the row follows it too. A table row as a positioning context is uneven across
	// browsers, so the row doesn't stretch the link over itself. Renders nothing with no items:
	// the page shows its own empty state.
	import type { Snippet } from 'svelte';
	import { goto } from '$app/navigation';
	import type { SortOption } from '#lib/listState.ts';
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
		items,
		key,
		href,
		sort,
		sorts,
		owner,
		onsort
	}: {
		columns: readonly Column[];
		items: readonly T[];
		key: (item: T) => string | number;
		href: (item: T) => string;
		sort: S;
		sorts: readonly SortOption<S>[];
		owner: boolean;
		onsort: (s: S) => void;
	} = $props();

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

{#if items.length > 0}
	<table class="w-full table-fixed text-left text-sm">
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
		<tbody>
			{#each items as item (key(item))}
				<tr
					onclick={(e) => clickRow(e, item)}
					class="h-11 cursor-pointer border-b border-rule text-ink hover:bg-surface-2 has-[a:focus-visible]:outline has-[a:focus-visible]:-outline-offset-1 has-[a:focus-visible]:outline-accent"
				>
					{#each shown as c, i (c.id)}
						<td class="px-2 {c.noClip ? 'relative overflow-visible' : 'truncate'} {alignClass(c)} {keepClass(c)} {c.align === 'right' ? 'text-muted' : ''}">
							{#if i === 0}
								<a href={href(item)} class="block truncate focus-visible:outline-none">{@render c.cell(item)}</a>
							{:else}
								{@render c.cell(item)}
							{/if}
						</td>
					{/each}
				</tr>
			{/each}
		</tbody>
	</table>
{/if}
