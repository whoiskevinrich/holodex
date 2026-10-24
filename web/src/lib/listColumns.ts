// Each list page's Table view columns (F82; docs/design/list-views-handoff.md, "Columns"),
// without their cells — a page adds a `cell` snippet per id. Kept apart from the pages so
// testing-strategy §33.1 V3/V7 can check them against each page's sort list.
import type { ColumnSpec } from './listTable';
import type { EntitySort } from './listState';
import type { PeopleTagSort, SortOrder } from './types';

const completeness = [
	{ value: 'completeness_desc', dir: 'descending' },
	{ value: 'completeness_asc', dir: 'ascending' }
] as const;

export const MEDIA_COLUMNS: readonly ColumnSpec<SortOrder>[] = [
	{
		id: 'title',
		header: 'Title',
		sorts: [
			{ value: 'title_asc', dir: 'ascending' },
			{ value: 'title_desc', dir: 'descending' }
		]
	},
	{
		id: 'duration',
		header: 'Duration',
		align: 'right',
		width: '5.5rem',
		sorts: [
			{ value: 'duration_desc', dir: 'descending' },
			{ value: 'duration_asc', dir: 'ascending' }
		]
	},
	{
		id: 'res',
		header: 'Res',
		width: '3.5rem',
		keepBelow: 'sm',
		sorts: [
			{ value: 'resolution_desc', dir: 'descending' },
			{ value: 'resolution_asc', dir: 'ascending' }
		]
	},
	{
		id: 'added',
		header: 'Added',
		width: '6.5rem',
		keepBelow: 'sm',
		sorts: [
			{ value: 'added_desc', dir: 'descending' },
			{ value: 'added_asc', dir: 'ascending' }
		]
	},
	{ id: 'size', header: 'Size', align: 'right', width: '5.5rem', keepBelow: 'lg', ownerOnly: true },
	{ id: 'done', header: 'Done', width: '4rem', keepBelow: 'lg', ownerOnly: true, sorts: completeness }
];

export const PEOPLE_COLUMNS: readonly ColumnSpec<EntitySort>[] = [
	{ id: 'name', header: 'Name', sorts: [{ value: 'name', dir: 'ascending' }] },
	{ id: 'videos', header: 'Videos', align: 'right', width: '5.5rem', sorts: [{ value: 'count', dir: 'descending' }] },
	{ id: 'done', header: 'Done', width: '4rem', ownerOnly: true, sorts: completeness }
];

// Tag rows and, on the All and Categories tabs, category rows share these columns: a
// category's count is its tags, and it has no parent.
export const TAG_COLUMNS: readonly ColumnSpec<PeopleTagSort>[] = [
	{ id: 'name', header: 'Name', sorts: [{ value: 'name', dir: 'ascending' }] },
	{ id: 'videos', header: 'Videos', align: 'right', width: '5.5rem', sorts: [{ value: 'count', dir: 'descending' }] },
	{ id: 'parent', header: 'Parent', width: '30%', keepBelow: 'sm' }
];
