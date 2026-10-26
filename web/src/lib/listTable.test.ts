// The shared Table view's rules (testing-strategy §33.1, cases V4–V6).
import { describe, it, expect } from 'vitest';
import {
	ariaSort,
	columnSorts,
	headerClick,
	keepClass,
	visibleColumns,
	type ColumnSpec
} from './listTable';
import { ENTITY_SORTS, TAG_SORTS } from './listState';
import { MEDIA_SORTS } from './filters';

const duration: ColumnSpec<string> = {
	id: 'duration',
	header: 'Duration',
	sorts: [
		{ value: 'duration_desc', dir: 'descending' },
		{ value: 'duration_asc', dir: 'ascending' }
	]
};
const videos: ColumnSpec<string> = { id: 'videos', header: 'Videos', sorts: [{ value: 'count', dir: 'descending' }] };
const done: ColumnSpec<string> = {
	id: 'done',
	header: 'Done',
	ownerOnly: true,
	sorts: [
		{ value: 'completeness_asc', dir: 'ascending' },
		{ value: 'completeness_desc', dir: 'descending' }
	]
};
const size: ColumnSpec<string> = { id: 'size', header: 'Size', ownerOnly: true, keepBelow: 'lg' };
const title: ColumnSpec<string> = { id: 'title', header: 'Title' };

describe('V4 a header click maps to the one toolbar sort', () => {
	it('an inactive header picks its first sort', () => {
		expect(headerClick(duration, 'added_desc', MEDIA_SORTS, false)).toBe('duration_desc');
	});
	it('the active header flips to its other direction', () => {
		expect(headerClick(duration, 'duration_desc', MEDIA_SORTS, false)).toBe('duration_asc');
		expect(headerClick(duration, 'duration_asc', MEDIA_SORTS, false)).toBe('duration_desc');
	});
	it('the active header with one direction does nothing', () => {
		expect(headerClick(videos, 'count', ENTITY_SORTS, false)).toBeNull();
	});
	it('a header with no sort does nothing', () => {
		expect(headerClick(title, 'added_desc', MEDIA_SORTS, false)).toBeNull();
	});
	it('a header never sets a sort the page does not offer', () => {
		const ghost: ColumnSpec<string> = { id: 'g', header: 'G', sorts: [{ value: 'nope', dir: 'ascending' }] };
		expect(columnSorts(ghost, MEDIA_SORTS, true)).toEqual([]);
		expect(headerClick(ghost, 'added_desc', MEDIA_SORTS, true)).toBeNull();
	});
	it('an owner-only sort is clickable for the owner only', () => {
		expect(headerClick(done, 'name', ENTITY_SORTS, true)).toBe('completeness_asc');
		expect(headerClick(done, 'name', ENTITY_SORTS, false)).toBeNull();
	});
	it("Tags' list has no completeness, so the same column is plain there", () => {
		expect(columnSorts(done, TAG_SORTS, true)).toEqual([]);
	});
});

describe('V5 aria-sort', () => {
	it('the active column carries its direction', () => {
		expect(ariaSort(duration, 'duration_asc', MEDIA_SORTS, false)).toBe('ascending');
		expect(ariaSort(duration, 'duration_desc', MEDIA_SORTS, false)).toBe('descending');
		expect(ariaSort(videos, 'count', ENTITY_SORTS, false)).toBe('descending');
	});
	it("another sortable column is 'none'", () => {
		expect(ariaSort(duration, 'added_desc', MEDIA_SORTS, false)).toBe('none');
	});
	it('a plain header has no aria-sort at all', () => {
		expect(ariaSort(title, 'added_desc', MEDIA_SORTS, false)).toBeUndefined();
		expect(ariaSort(done, 'name', ENTITY_SORTS, false)).toBeUndefined();
	});
});

describe('V6 which columns show', () => {
	const cols = [title, duration, size, done];
	it('owner-only columns never reach a visitor', () => {
		expect(visibleColumns(cols, false).map((c) => c.id)).toEqual(['title', 'duration']);
	});
	it('the owner sees every column, in order', () => {
		expect(visibleColumns(cols, true).map((c) => c.id)).toEqual(['title', 'duration', 'size', 'done']);
	});
	it('a column drops below its breakpoint and otherwise always shows', () => {
		expect(keepClass(title)).toBe('');
		expect(keepClass(size)).toBe('hidden lg:table-cell');
		expect(keepClass({ id: 'res', header: 'Res', keepBelow: 'sm' })).toBe('hidden sm:table-cell');
	});
});
