import { describe, it, expect } from 'vitest';
import { sourceChips } from './f36';
import { CHIP_VALUE_MAX_CHARS, isCockpitRow, isImageRow, isUnverifiable, needsDecision, rowClass, savesDecisionOnly, stacksCandidates, stagedValue, willWrite } from './writebackCockpit';
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
	it('is true for every replace field — text and image_url (HOLODEX-403) — and false for merge fields', () => {
		expect(isCockpitRow(field())).toBe(true);
		expect(isCockpitRow(field({ display: 'long_text' }))).toBe(true);
		expect(isCockpitRow(field({ display: 'image_url' }))).toBe(true);
		expect(isCockpitRow(field({ multi: true }))).toBe(false);
		expect(isImageRow(field({ display: 'image_url' }))).toBe(true);
		expect(isImageRow(field())).toBe(false);
		expect(isImageRow(field({ display: 'image_url', multi: true }))).toBe(false);
	});
});

describe('stacksCandidates', () => {
	it('stacks long_text always, and any row with a candidate a chip would truncate (HOLODEX-434)', () => {
		const short = field();
		expect(stacksCandidates(short, sourceChips(short))).toBe(false);
		expect(stacksCandidates(field({ display: 'long_text' }), sourceChips(short))).toBe(true);
		const long = 'https://www.sonypictures.com/movies/blackdynamite';
		expect(long.length).toBeGreaterThan(CHIP_VALUE_MAX_CHARS);
		const url = field({ candidates: [{ source: 'file', value: 'x' }, { source: 'provider:tmdb', provider: 'tmdb', value: long }] });
		expect(stacksCandidates(url, sourceChips(url))).toBe(true);
	});
	it('counts a standing manual literal (it renders as a value chip), never image rows or merge fields', () => {
		const long = 'a'.repeat(CHIP_VALUE_MAX_CHARS + 1);
		const manual = field({ decision: { source: 'manual', manual_value: long, standing: true } } as Partial<ResolvedField>);
		expect(stacksCandidates(manual, sourceChips(manual))).toBe(true);
		expect(stacksCandidates(field(), sourceChips(field()))).toBe(false);
		const img = field({ display: 'image_url', candidates: [{ source: 'provider:tmdb', provider: 'tmdb', value: 'https://image.tmdb.org/t/p/original/abcdef.jpg' }] });
		expect(stacksCandidates(img, sourceChips(img))).toBe(false);
		expect(stacksCandidates(field({ multi: true }), [])).toBe(false);
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
	it('lets the ledger witness stand in for the file value on an image row (ADR-101)', () => {
		const url = 'https://x/p.jpg';
		const base = {
			display: 'image_url' as const,
			write_target: 'cover.jpg',
			values: [url],
			candidates: [{ source: 'file' as const, value: '' }, { source: 'provider:tmdb' as const, provider: 'tmdb', value: url }],
			decision: { source: 'provider:tmdb' as const, standing: true }
		};
		expect(rowClass(field({ ...base, in_sync: true }), url)).toBe('matches');
		expect(rowClass(field({ ...base, in_sync: false }), url)).toBe('write');
		expect(rowClass(field({ ...base, in_sync: undefined }), url)).toBe('write');
		// witnessed, but re-pointed at another value → differs again
		expect(rowClass(field({ ...base, in_sync: true }), 'https://x/other.jpg')).toBe('write');
		// the verdict is keyed on in_sync, not display: a provider poster the allowlist
		// degraded to text is still witnessed by the ledger
		expect(rowClass(field({ ...base, display: undefined, in_sync: true }), url)).toBe('matches');
	});
	it('ignores the in_sync verdict on an undecided row — it is true by construction (HOLODEX-433)', () => {
		const url = 'https://x/p.jpg';
		const undecided = {
			display: 'image_url' as const,
			write_target: 'cover.jpg',
			values: [url],
			candidates: [{ source: 'file' as const, value: '' }, { source: 'provider:tmdb' as const, provider: 'tmdb', value: url }],
			in_sync: true
		};
		expect(rowClass(field({ ...undecided, decision: { source: 'provider:tmdb', standing: false } }), url)).toBe('write');
		expect(rowClass(field({ ...undecided, decision: undefined }), url)).toBe('write');
		// same for an undecided provider-winning text row
		expect(
			rowClass(
				field({ values: ['Blade Runner: Final Cut'], decision: { source: 'provider:tmdb', standing: false } }),
				'Blade Runner: Final Cut'
			)
		).toBe('write');
	});
});

describe('isUnverifiable', () => {
	it('is true only for a writable cockpit row whose in_sync is absent (ADR-093)', () => {
		expect(isUnverifiable(field({ in_sync: undefined }))).toBe(true);
		expect(isUnverifiable(field({ in_sync: false }))).toBe(false);
		expect(isUnverifiable(field({ in_sync: true }))).toBe(false);
		expect(isUnverifiable(field({ in_sync: undefined, write_target: undefined }))).toBe(false);
		expect(isUnverifiable(field({ in_sync: undefined, display: 'image_url' }))).toBe(true);
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
	it('is false for merge rows and for a null staged key; an image row decides like any other', () => {
		expect(needsDecision(field({ display: 'image_url' }), [], { key: 'file', custom: '' })).toBe(true);
		expect(needsDecision(field({ multi: true }), [], { key: 'file', custom: '' })).toBe(false);
		expect(needsDecision(field(), sourceChips(field()), { key: null, custom: '' })).toBe(false);
	});
});

describe('willWrite', () => {
	const tmdb = { key: 'provider:tmdb', custom: '' };
	const off = { touched: false };
	it('never writes a row that is unwritable or already matches the file', () => {
		expect(willWrite(field({ write_target: undefined }), 'X', { staged: tmdb, ...off, touched: true })).toBe(false);
		expect(willWrite(field(), 'Blade Runner', { staged: { key: 'file', custom: '' }, ...off, touched: true })).toBe(false);
	});
	it('never writes an undecided cockpit row the owner did not touch (the RD6 pending winner)', () => {
		expect(willWrite(field(), 'Blade Runner: Final Cut', { staged: tmdb, ...off })).toBe(false);
		const pending = field({ decision: { source: 'provider:tmdb', standing: false } });
		expect(willWrite(pending, 'Blade Runner: Final Cut', { staged: tmdb, ...off })).toBe(false);
	});
	it('writes an undecided cockpit row once the owner picked a chip — picking is the confirm', () => {
		expect(willWrite(field(), 'Blade Runner: Final Cut', { staged: tmdb, ...off, touched: true })).toBe(true);
	});
	it('always writes a standing-decided cockpit row that differs from the file, touched or not', () => {
		const decided = field({ decision: { source: 'provider:tmdb', standing: true } });
		expect(willWrite(decided, 'Blade Runner: Final Cut', { staged: tmdb, ...off })).toBe(true);
	});
	it('treats a blank Custom pick as nothing chosen', () => {
		const decided = field({ decision: { source: 'provider:tmdb', standing: true } });
		expect(willWrite(decided, '', { staged: { key: 'custom', custom: '  ' }, ...off, touched: true })).toBe(false);
	});
	it('never writes a merge row — nothing there can be decided from the dialog', () => {
		const genres = field({ multi: true, candidates: [{ source: 'file', value: 'drama' }] });
		expect(willWrite(genres, 'drama, action', { staged: { key: null, custom: '' }, touched: true })).toBe(false);
	});
	it('writes an image row exactly like a text row: picked provider tile → write; file tile → nothing', () => {
		const poster = field({
			display: 'image_url',
			write_target: 'cover.jpg',
			candidates: [{ source: 'file', value: '' }, { source: 'provider:tmdb', provider: 'tmdb', value: 'https://x/p.jpg' }]
		});
		expect(willWrite(poster, 'https://x/p.jpg', { staged: { key: 'provider:tmdb', custom: '' }, touched: true })).toBe(true);
		expect(willWrite(poster, 'https://x/p.jpg', { staged: { key: 'provider:tmdb', custom: '' }, touched: false })).toBe(false);
		expect(willWrite(poster, '', { staged: { key: 'file', custom: '' }, touched: true })).toBe(false);
		expect(savesDecisionOnly(poster, sourceChips(poster), '', { staged: { key: 'file', custom: '' }, touched: true })).toBe(true);
	});
});

describe('savesDecisionOnly', () => {
	const tmdb = { key: 'provider:tmdb', custom: '' };
	it('is false for an untouched row, whatever its state', () => {
		const unmapped = field({ write_target: undefined });
		expect(savesDecisionOnly(unmapped, sourceChips(unmapped), 'Blade Runner: Final Cut', { staged: tmdb, touched: false })).toBe(false);
	});
	it('is true for an unmapped field the owner decided here — the decision lands in Holodex only', () => {
		const unmapped = field({ write_target: undefined });
		expect(savesDecisionOnly(unmapped, sourceChips(unmapped), 'Blade Runner: Final Cut', { staged: tmdb, touched: true })).toBe(true);
	});
	it('is true for a decided, mapped row re-pointed at the file value (nothing to write)', () => {
		const decided = field({ decision: { source: 'provider:tmdb', standing: true } });
		expect(savesDecisionOnly(decided, sourceChips(decided), 'Blade Runner', { staged: { key: 'file', custom: '' }, touched: true })).toBe(true);
	});
	it('is false when the row writes to the file instead (the decision rides with the write)', () => {
		const f = field();
		expect(savesDecisionOnly(f, sourceChips(f), 'Blade Runner: Final Cut', { staged: tmdb, touched: true })).toBe(false);
	});
	it('is false when the staged pick equals the standing decision, or the Custom literal is blank', () => {
		const decided = field({ write_target: undefined, decision: { source: 'provider:tmdb', standing: true } });
		expect(savesDecisionOnly(decided, sourceChips(decided), 'Blade Runner: Final Cut', { staged: tmdb, touched: true })).toBe(false);
		const unmapped = field({ write_target: undefined });
		expect(savesDecisionOnly(unmapped, sourceChips(unmapped), '', { staged: { key: 'custom', custom: ' ' }, touched: true })).toBe(false);
	});
});
