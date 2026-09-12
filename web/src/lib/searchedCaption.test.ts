import { describe, expect, it } from 'vitest';
import { moreLabel, searchedCaption } from './searchedCaption';

// The state table from docs/design/structured-resolve-hints-searched-caption-handoff.md
// ("States and interactions") and the QA checklist's §2 smoke items. Each case is named
// for the row it covers so a failure points at the design doc.
const shown = { loading: false, error: '', query: 'Acme Pictures Ada Lovelace' };
const two = ['[Acme Pictures] Ada Lovelace (2023-08-01) 1080p.mp4', 'Acme Pictures Ada Lovelace'];

describe('searchedCaption — visibility', () => {
	it('2.1 · no searched key ⇒ nothing (not an empty slot)', () => {
		expect(searchedCaption(undefined, shown)).toBeNull();
	});

	it('2.1 · empty array ⇒ nothing', () => {
		expect(searchedCaption([], shown)).toBeNull();
	});

	it('2.7 · hidden while loading — the previous response is stale', () => {
		expect(searchedCaption(two, { ...shown, loading: true })).toBeNull();
	});

	it('2.7 · hidden on error', () => {
		expect(searchedCaption(two, { ...shown, error: 'provider lookup failed' })).toBeNull();
	});

	it('hidden under two characters — the picker never searched', () => {
		expect(searchedCaption(two, { ...shown, query: ' a ' })).toBeNull();
	});

	it('2.6 · shown on the no-results state too — the caption rides the response, not the candidate count', () => {
		expect(searchedCaption(two, shown)).not.toBeNull();
	});
});

describe('searchedCaption — content', () => {
	it('2.2 · one entry: the query, no toggle (more = 0)', () => {
		expect(searchedCaption(['one'], shown)).toEqual({ first: 'one', more: 0, entries: ['one'] });
	});

	it('2.3 · two entries: first inline, one more, the list is the complete record in issue order', () => {
		expect(searchedCaption(two, shown)).toEqual({ first: two[0], more: 1, entries: two });
	});

	it('2.4 · the expanded list repeats searched[0] on purpose', () => {
		expect(searchedCaption(two, shown)?.entries[0]).toBe(two[0]);
	});

	it('stressed: ten entries ⇒ +9 more, all ten listed, the 600-char one intact (truncation is CSS)', () => {
		const long = 'x'.repeat(600);
		const ten = ['first', long, ...Array.from({ length: 8 }, (_, i) => `q${i + 3}`)];
		const c = searchedCaption(ten, shown);
		expect(c?.more).toBe(9);
		expect(c?.entries).toHaveLength(10);
		expect(c?.entries[1]).toHaveLength(600);
	});

	it('returns a copy, not the response array', () => {
		const c = searchedCaption(two, shown);
		expect(c?.entries).not.toBe(two);
	});
});

describe('moreLabel', () => {
	it('collapsed: +N more', () => {
		expect(moreLabel(1, false)).toBe('+1 more');
		expect(moreLabel(9, false)).toBe('+9 more');
	});
	it('expanded: show less', () => {
		expect(moreLabel(9, true)).toBe('show less');
	});
});
