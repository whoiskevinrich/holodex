// Each page's Table view columns (testing-strategy §33.1, cases V3 and V7).
import { describe, it, expect } from 'vitest';
import { MEDIA_COLUMNS, PEOPLE_COLUMNS, TAG_COLUMNS } from './listColumns';
import type { ColumnSpec } from './listTable';
import { ENTITY_SORTS, TAG_SORTS, type SortOption } from './listState';
import { MEDIA_SORTS } from './filters';

const pages: [string, readonly ColumnSpec<string>[], readonly SortOption<string>[]][] = [
	['media', MEDIA_COLUMNS, MEDIA_SORTS],
	['people', PEOPLE_COLUMNS, ENTITY_SORTS],
	['tags', TAG_COLUMNS, TAG_SORTS]
];

describe('V3 every header sort is one the page offers', () => {
	it.each(pages)('%s', (_page, columns, sorts) => {
		const offered = new Set(sorts.map((s) => s.value));
		for (const c of columns)
			for (const s of c.sorts ?? []) expect(offered, `${c.id} → ${s.value}`).toContain(s.value);
	});
});

describe('V7 the column lists are the handoff’s', () => {
	const shape = (cols: readonly ColumnSpec<string>[]) =>
		cols.map((c) => [c.header, c.align ?? 'left', c.keepBelow ?? 'always', c.ownerOnly ? 'owner' : 'all']);

	it('Media: title, duration, res, added, size and done', () => {
		expect(shape(MEDIA_COLUMNS)).toEqual([
			['Title', 'left', 'always', 'all'],
			['Duration', 'right', 'always', 'all'],
			['Res', 'left', 'sm', 'all'],
			['Added', 'left', 'sm', 'all'],
			['Size', 'right', 'lg', 'owner'],
			['Done', 'left', 'lg', 'owner']
		]);
	});
	it('People: name, videos and done', () => {
		expect(shape(PEOPLE_COLUMNS)).toEqual([
			['Name', 'left', 'always', 'all'],
			['Videos', 'right', 'always', 'all'],
			['Done', 'left', 'always', 'owner']
		]);
	});
	it('Tags: name, videos and parent', () => {
		expect(shape(TAG_COLUMNS)).toEqual([
			['Name', 'left', 'always', 'all'],
			['Videos', 'right', 'always', 'all'],
			['Parent', 'left', 'sm', 'all']
		]);
	});
	it.each(pages)('%s keeps its first column at every width, for everyone', (_page, columns) => {
		expect(columns[0].keepBelow).toBeUndefined();
		expect(columns[0].ownerOnly).toBeFalsy();
	});
});
