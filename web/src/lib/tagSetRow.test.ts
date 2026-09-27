import { describe, it, expect } from 'vitest';
import { isTagSetRow, needsWriteback, outOfSyncCount } from './f36';
import { rowClass, tagSetDiff, willWrite } from './writebackCockpit';
import type { ResolvedField } from './types';

// HOLODEX-401 (F72, ADR-111): the genres row is the tag set — no decision, the server's set
// comparison is its sync verdict, and it writes whenever the file lags or is unread.
function tags(over: Partial<ResolvedField> = {}): ResolvedField {
	return {
		canonical: 'genres',
		label: 'Tags',
		multi: true,
		values: ['drama', 'crime'],
		items: [
			{ value: 'drama', sources: ['tag'], on_file: true },
			{ value: 'crime', sources: ['tag'], on_file: false }
		],
		file_only: ['Noir'],
		in_sync: false,
		write_target: 'QuickTime:Genre',
		...over
	} as ResolvedField;
}
const pick = { staged: { key: null, custom: '' }, touched: false };

describe('isTagSetRow', () => {
	it('is the multi genres row only', () => {
		expect(isTagSetRow(tags())).toBe(true);
		expect(isTagSetRow(tags({ canonical: 'actors' }))).toBe(false);
		expect(isTagSetRow(tags({ multi: false }))).toBe(false);
	});
});

describe('tag set row writes', () => {
	it('writes on open when the file lags, untouched (P0-4e)', () => {
		expect(rowClass(tags(), '')).toBe('write');
		expect(willWrite(tags(), '', pick)).toBe(true);
	});
	it('writes on open when the file was never read (in_sync unknown, P0-4c)', () => {
		expect(willWrite(tags({ in_sync: undefined }), '', pick)).toBe(true);
	});
	it('matches the file when the sets agree, and never writes then (P0-4a)', () => {
		expect(rowClass(tags({ in_sync: true }), '')).toBe('matches');
		expect(willWrite(tags({ in_sync: true }), '', pick)).toBe(false);
	});
	it('is unwritable without a write target (P0-4d)', () => {
		expect(willWrite(tags({ write_target: undefined }), '', pick)).toBe(false);
	});
	it('counts in the header only when known out of sync — unknown is not "differs"', () => {
		expect(needsWriteback(tags())).toBe(true);
		expect(needsWriteback(tags({ in_sync: undefined }))).toBe(false);
		expect(outOfSyncCount([tags(), tags({ canonical: 'actors' })])).toBe(1);
	});
});

describe('tagSetDiff', () => {
	it('splits the write set by on_file and lists the drops', () => {
		expect(tagSetDiff(tags())).toEqual({
			known: true,
			applied: ['drama', 'crime'],
			onFile: ['drama'],
			willAdd: ['crime'],
			willDrop: ['Noir']
		});
	});
	it('lists every applied tag plainly while the file is unread', () => {
		const d = tagSetDiff(tags({ in_sync: undefined, file_only: undefined }));
		expect(d.known).toBe(false);
		expect(d.applied).toEqual(['drama', 'crime']);
		expect([...d.onFile, ...d.willAdd, ...d.willDrop]).toEqual([]);
	});
});
