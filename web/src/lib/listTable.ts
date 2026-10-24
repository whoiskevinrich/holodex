// The shared Table view's rules (F82, docs/design/list-views-handoff.md), kept pure so they
// test without a DOM (testing-strategy §33.1, V4–V6). ListTable renders from these; a page
// supplies only its column list.
import type { SortOption } from './listState';

export type SortDir = 'ascending' | 'descending';

// A sort a header can set. The first is what a click on an inactive header picks; a second
// is the opposite direction, which a click on the active header flips to.
export interface ColumnSort<S extends string> {
	value: S;
	dir: SortDir;
}

export interface ColumnSpec<S extends string> {
	id: string;
	header: string;
	align?: 'left' | 'right';
	// Desktop width (any CSS length); omitted on the column that takes the rest.
	width?: string;
	// The narrowest breakpoint the column still shows at; omitted = always shown. The first
	// column never sets this — the name or title is always kept (spec P0-6).
	keepBelow?: 'sm' | 'lg';
	ownerOnly?: boolean;
	sorts?: readonly ColumnSort<S>[];
}

// The columns this viewer sees: owner-only ones never reach a visitor (spec P0-4).
export function visibleColumns<C extends ColumnSpec<string>>(columns: readonly C[], owner: boolean): C[] {
	return columns.filter((c) => owner || !c.ownerOnly);
}

// The header's sorts that the page's Sort actually offers this viewer. A header is
// clickable only when this is non-empty (spec P0-5), so a header can never set a sort the
// select doesn't show.
export function columnSorts<S extends string>(
	column: ColumnSpec<S>,
	offered: readonly SortOption<S>[],
	owner: boolean
): ColumnSort<S>[] {
	return (column.sorts ?? []).filter((s) =>
		offered.some((o) => o.value === s.value && (owner || !o.ownerOnly))
	);
}

// What clicking the header sets, or null for no change: an inactive header picks its first
// sort; the active one flips to its other direction when it has one.
export function headerClick<S extends string>(
	column: ColumnSpec<S>,
	current: S,
	offered: readonly SortOption<S>[],
	owner: boolean
): S | null {
	const sorts = columnSorts(column, offered, owner);
	if (sorts.length === 0) return null;
	const i = sorts.findIndex((s) => s.value === current);
	if (i < 0) return sorts[0].value;
	return sorts.length > 1 ? sorts[1 - i].value : null;
}

// aria-sort for the header: the direction on the active column, 'none' on another sortable
// one, and absent (undefined) on a plain header.
export function ariaSort<S extends string>(
	column: ColumnSpec<S>,
	current: S,
	offered: readonly SortOption<S>[],
	owner: boolean
): SortDir | 'none' | undefined {
	const sorts = columnSorts(column, offered, owner);
	if (sorts.length === 0) return undefined;
	return sorts.find((s) => s.value === current)?.dir ?? 'none';
}

// The class that drops a column below its breakpoint; the table never scrolls sideways
// (spec P0-6), so narrow widths shed columns instead.
export function keepClass(column: ColumnSpec<string>): string {
	if (column.keepBelow === 'sm') return 'hidden sm:table-cell';
	if (column.keepBelow === 'lg') return 'hidden lg:table-cell';
	return '';
}
