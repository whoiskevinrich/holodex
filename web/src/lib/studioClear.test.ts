import { describe, it, expect } from 'vitest';
import { isCleared, resolveSelection, sourceChips } from './f36';
import { isBlankCustom, needsDecision, rowClass, willWrite, writeEntry, writesClear } from './writebackCockpit';
import type { FieldDecision, ResolvedField } from './types';

// Clearing a studio (F74, ADR-120) on the SPA side: recognising a cleared field, and the
// writeback cockpit writing it as a tag delete. Testing-strategy §22.5.

// A studio row the owner cleared while the file still says Acme — the server omits an empty
// manual_value, so the decision has none.
function cleared(over: Partial<ResolvedField> = {}): ResolvedField {
	return {
		canonical: 'studio',
		label: 'Studio',
		values: [],
		write_target: 'Publisher',
		in_sync: false,
		decision: { source: 'manual', standing: true },
		candidates: [{ source: 'file', value: 'Acme' }],
		...over
	} as ResolvedField;
}

describe('isCleared', () => {
	it('is a standing manual decision with no value — absent or blank', () => {
		expect(isCleared({ source: 'manual', standing: true })).toBe(true);
		expect(isCleared({ source: 'manual', standing: true, manual_value: '  ' })).toBe(true);
	});
	it('is not a manual literal, a file blank pin, an undecided field, or no decision', () => {
		expect(isCleared({ source: 'manual', standing: true, manual_value: 'Acme' })).toBe(false);
		expect(isCleared({ source: 'file', standing: true } as FieldDecision)).toBe(false);
		expect(isCleared({ source: 'manual', standing: false })).toBe(false);
		expect(isCleared(undefined)).toBe(false);
	});
});

describe('a cleared row in the chip model', () => {
	it('keeps the ·file chip (the undo path, R6) and adds no chip carrying a value of its own', () => {
		const chips = sourceChips(cleared());
		expect(chips.find((c) => c.key === 'file')?.value).toBe('Acme');
		// Only the standard Custom entry chip is blank — no "None" or extra empty chip.
		expect(chips.filter((c) => c.value.trim() === '').map((c) => c.key)).toEqual(['custom']);
	});
	it('seeds its staged pick onto the blank Custom it was decided as', () => {
		const f = cleared();
		expect(resolveSelection(f, sourceChips(f)).key).toBe('custom');
	});
});

describe('the cockpit writes a clear', () => {
	const seeded = { key: 'custom', custom: '' };
	it('writes a cleared row while the file still carries the studio, untouched', () => {
		const f = cleared();
		expect(writesClear(f, seeded)).toBe(true);
		expect(rowClass(f, '')).toBe('write');
		expect(willWrite(f, '', { staged: seeded, touched: false })).toBe(true);
	});
	it('needs no new decision — the blank IS the standing decision', () => {
		const f = cleared();
		expect(needsDecision(f, sourceChips(f), seeded)).toBe(false);
	});
	it('reads "=" once the file has no studio either', () => {
		const f = cleared({ candidates: [{ source: 'file', value: '' }], in_sync: true });
		expect(rowClass(f, '')).toBe('matches');
		expect(willWrite(f, '', { staged: seeded, touched: false })).toBe(false);
	});
	it('is still a blank Custom, so the HOLODEX-400 "never send manual:\'\'" guard is untouched for others', () => {
		expect(isBlankCustom(seeded)).toBe(true);
		const decided = cleared({ decision: { source: 'provider:tmdb', standing: true } });
		expect(writesClear(decided, seeded)).toBe(false);
		expect(willWrite(decided, '', { staged: seeded, touched: true })).toBe(false);
	});
	it('stops being a clear the moment the owner stages another chip', () => {
		expect(writesClear(cleared(), { key: 'file', custom: '' })).toBe(false);
	});
});

describe('writeEntry — what the dialog sends', () => {
	it('sends a cleared row as {field, clear: true}, never values: []', () => {
		expect(writeEntry(cleared(), '', { key: 'custom', custom: '' })).toEqual({ field: 'studio', clear: true });
	});
	it('sends any other row its staged value with its winning source', () => {
		const f = cleared({ decision: { source: 'provider:tmdb', standing: true }, winning_source: 'tmdb:studio' });
		expect(writeEntry(f, 'Legendary', { key: 'provider:tmdb', custom: '' })).toEqual({
			field: 'studio',
			values: ['Legendary'],
			source: 'tmdb:studio'
		});
	});
});
