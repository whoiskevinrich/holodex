import { describe, it, expect } from 'vitest';
import { sourceChips } from './f36';
import { isCockpitRow, isUnverifiable, needsDecision, rowClass, stagedValue } from './writebackCockpit';
import type { ResolvedField } from './types';

// Same fixture shape as f36.test.ts: a Title field with a file value and one matched provider
// whose value differs. `write_target` set so the row is writable by default.
function field(over: Partial<ResolvedField> = {}): ResolvedField {
	return {
		canonical: 'title',
		label: 'Title',
		values: ['Blade Runner'],
		write_target: 'Title',
		in_sync: true,
		candidates: [
			{ source: 'file', value: 'Blade Runner' },
			{ source: 'provider:tmdb', provider: 'tmdb', value: 'Blade Runner: Final Cut' }
		],
		...over
	} as ResolvedField;
}

describe('isCockpitRow', () => {
	it('is true for a plain replace field and false for image_url and merge fields', () => {
		expect(isCockpitRow(field())).toBe(true);
		expect(isCockpitRow(field({ display: 'long_text' }))).toBe(true);
		expect(isCockpitRow(field({ display: 'image_url' }))).toBe(false);
		expect(isCockpitRow(field({ multi: true }))).toBe(false);
	});
});

describe('stagedValue', () => {
	const chips = sourceChips(field());
	it('returns the staged chip value, the trimmed custom literal, or nothing', () => {
		expect(stagedValue(chips, { key: 'file', custom: '' })).toBe('Blade Runner');
		expect(stagedValue(chips, { key: 'provider:tmdb', custom: '' })).toBe('Blade Runner: Final Cut');
		expect(stagedValue(chips, { key: 'custom', custom: '  Director Cut ' })).toBe('Director Cut');
		expect(stagedValue(chips, { key: null, custom: 'ignored' })).toBe('');
		expect(stagedValue(chips, { key: 'provider:gone', custom: '' })).toBe('');
	});
});

describe('rowClass', () => {
	it('is unwritable when the container has no tag, regardless of value', () => {
		expect(rowClass(field({ write_target: undefined }), 'anything')).toBe('unwritable');
	});
	it('matches when the live value equals the file value (whitespace-insensitive)', () => {
		expect(rowClass(field(), 'Blade Runner')).toBe('matches');
		expect(rowClass(field(), '  Blade Runner ')).toBe('matches');
	});
	it('is write when the live value differs from the file value', () => {
		expect(rowClass(field(), 'Blade Runner: Final Cut')).toBe('write');
	});
	it('matches an empty file value only with an empty staged value', () => {
		const f = field({ candidates: [{ source: 'file', value: '' }, { source: 'provider:tmdb', provider: 'tmdb', value: 'X' }] });
		expect(rowClass(f, '')).toBe('matches');
		expect(rowClass(f, 'X')).toBe('write');
	});
	it('never matches when the field carries no candidates at all', () => {
		expect(rowClass(field({ candidates: undefined }), '')).toBe('write');
	});
});

describe('isUnverifiable', () => {
	it('is true only for a writable cockpit row whose in_sync is absent (ADR-093)', () => {
		expect(isUnverifiable(field({ in_sync: undefined }))).toBe(true);
		expect(isUnverifiable(field({ in_sync: false }))).toBe(false);
		expect(isUnverifiable(field({ in_sync: true }))).toBe(false);
		expect(isUnverifiable(field({ in_sync: undefined, write_target: undefined }))).toBe(false);
		expect(isUnverifiable(field({ in_sync: undefined, display: 'image_url' }))).toBe(false);
	});
});

describe('needsDecision', () => {
	it('is true for an undecided row whatever is staged (the checkbox is the commit)', () => {
		const f = field();
		const chips = sourceChips(f);
		expect(needsDecision(f, chips, { key: 'provider:tmdb', custom: '' })).toBe(true);
		expect(needsDecision(f, chips, { key: 'file', custom: '' })).toBe(true);
	});
	it('is true for an RD6 pending winner (decision present but not standing)', () => {
		const f = field({
			values: ['Blade Runner: Final Cut'],
			decision: { source: 'provider:tmdb', standing: false },
			candidates: [{ source: 'file', value: '' }, { source: 'provider:tmdb', provider: 'tmdb', value: 'Blade Runner: Final Cut' }]
		});
		const chips = sourceChips(f);
		expect(needsDecision(f, chips, { key: 'provider:tmdb', custom: '' })).toBe(true);
	});
	it('is false for a decided row whose staged pick is the committed selection', () => {
		const f = field({ decision: { source: 'provider:tmdb', standing: true } });
		const chips = sourceChips(f);
		expect(needsDecision(f, chips, { key: 'provider:tmdb', custom: '' })).toBe(false);
	});
	it('is true when a decided row is re-pointed at a different chip', () => {
		const f = field({ decision: { source: 'provider:tmdb', standing: true } });
		const chips = sourceChips(f);
		expect(needsDecision(f, chips, { key: 'file', custom: '' })).toBe(true);
		expect(needsDecision(f, chips, { key: 'custom', custom: 'Other' })).toBe(true);
	});
	it('compares the custom literal when both committed and staged are manual', () => {
		const f = field({ decision: { source: 'manual', standing: true, manual_value: 'Mine' } });
		const chips = sourceChips(f);
		expect(needsDecision(f, chips, { key: 'custom', custom: 'Mine' })).toBe(false);
		expect(needsDecision(f, chips, { key: 'custom', custom: ' Mine ' })).toBe(false);
		expect(needsDecision(f, chips, { key: 'custom', custom: 'Yours' })).toBe(true);
	});
	it('is false for non-cockpit rows and for a null staged key', () => {
		expect(needsDecision(field({ display: 'image_url' }), [], { key: 'file', custom: '' })).toBe(false);
		expect(needsDecision(field({ multi: true }), [], { key: 'file', custom: '' })).toBe(false);
		expect(needsDecision(field(), sourceChips(field()), { key: null, custom: '' })).toBe(false);
	});
});
