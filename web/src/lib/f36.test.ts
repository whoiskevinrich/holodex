import { describe, it, expect } from 'vitest';
import {
	baselineCandidateValue,
	decidedSource,
	fileCandidateValue,
	isReplaceField,
	outOfSync,
	outOfSyncCount,
	providerCandidates,
	providerOf,
	selectedChipKey,
	sourceChips
} from './f36';
import type { ResolvedField } from './types';

// A minimal replace field with the F36 payload filled in. Defaults model a Title field with a
// file value and one matched provider whose value differs (the classic masking case).
function field(over: Partial<ResolvedField> = {}): ResolvedField {
	return {
		canonical: 'title',
		label: 'Title',
		values: ['Blade Runner'],
		candidates: [
			{ source: 'file', value: 'Blade Runner' },
			{ source: 'provider:tmdb', provider: 'tmdb', value: 'Blade Runner: Final Cut' }
		],
		...over
	};
}

describe('providerOf', () => {
	it('extracts the provider name', () => {
		expect(providerOf('provider:tmdb')).toBe('tmdb');
	});
	it('is empty for file/manual', () => {
		expect(providerOf('file')).toBe('');
		expect(providerOf('manual')).toBe('');
	});
});

describe('isReplaceField', () => {
	it('is true for a scalar field', () => {
		expect(isReplaceField(field())).toBe(true);
	});
	it('is false for a merge (multi) field', () => {
		expect(isReplaceField(field({ multi: true }))).toBe(false);
	});
});

describe('decidedSource', () => {
	it('defaults to file when undecided (file-first, RD4)', () => {
		expect(decidedSource(field())).toBe('file');
	});
	it('reflects an explicit provider decision', () => {
		expect(decidedSource(field({ decision: { source: 'provider:tmdb', standing: true } }))).toBe(
			'provider:tmdb'
		);
	});
	it('reflects a manual decision', () => {
		expect(decidedSource(field({ decision: { source: 'manual', standing: true } }))).toBe('manual');
	});
});

describe('fileCandidateValue', () => {
	it('returns the file baseline value', () => {
		expect(fileCandidateValue(field())).toBe('Blade Runner');
	});
	it('is empty when the file has no value', () => {
		expect(fileCandidateValue(field({ candidates: [{ source: 'provider:tmdb', value: 'X' }] }))).toBe(
			''
		);
	});
});

describe('providerCandidates', () => {
	it('lists matched providers that supply a value', () => {
		expect(providerCandidates(field()).map((c) => c.source)).toEqual(['provider:tmdb']);
	});
	it('omits a provider with an empty value (no Adopt segment)', () => {
		const f = field({
			candidates: [
				{ source: 'file', value: 'Blade Runner' },
				{ source: 'provider:tmdb', provider: 'tmdb', value: '   ' }
			]
		});
		expect(providerCandidates(f)).toEqual([]);
	});
	it('is empty for a file-only field', () => {
		expect(providerCandidates(field({ candidates: [{ source: 'file', value: 'Only' }] }))).toEqual(
			[]
		);
	});
});

describe('sourceChips', () => {
	it('anchors the file baseline first, tagged file', () => {
		const [first] = sourceChips(field());
		expect(first).toMatchObject({ key: 'file', value: 'Blade Runner', sources: ['file'] });
	});
	it('appends a Custom opener chip last', () => {
		const chips = sourceChips(field());
		expect(chips.at(-1)).toMatchObject({ key: 'custom', value: '', manual: true });
	});
	it('gives a diverging provider its own value chip', () => {
		const chips = sourceChips(field()); // file "Blade Runner" vs tmdb "Blade Runner: Final Cut"
		expect(chips.map((c) => c.key)).toEqual(['file', 'provider:tmdb', 'custom']);
		expect(chips[1]).toMatchObject({ value: 'Blade Runner: Final Cut', sources: ['tmdb'] });
	});
	it('folds a provider that agrees with the file into the file chip (no repeated value)', () => {
		const f = field({
			values: ['Legendary Pictures'],
			candidates: [
				{ source: 'file', value: 'Legendary Pictures' },
				{ source: 'provider:tmdb', provider: 'tmdb', value: 'Legendary Pictures' }
			]
		});
		const chips = sourceChips(f);
		expect(chips.map((c) => c.key)).toEqual(['file', 'custom']);
		expect(chips[0]).toMatchObject({ value: 'Legendary Pictures', sources: ['file', 'tmdb'] });
	});
	it('folds two providers that agree with each other into one chip', () => {
		const f = field({
			candidates: [
				{ source: 'file', value: 'A' },
				{ source: 'provider:tmdb', provider: 'tmdb', value: 'B' },
				{ source: 'provider:imdb', provider: 'imdb', value: 'B' }
			]
		});
		const chips = sourceChips(f);
		expect(chips.map((c) => c.key)).toEqual(['file', 'provider:tmdb', 'custom']);
		expect(chips[1].sources).toEqual(['tmdb', 'imdb']);
	});
	it('carries the frozen literal on the Custom chip when manual is decided', () => {
		const f = field({ decision: { source: 'manual', standing: true, manual_value: 'My Title' } });
		expect(sourceChips(f).at(-1)).toMatchObject({ key: 'custom', value: 'My Title', manual: true });
	});
});

describe('selectedChipKey', () => {
	it('selects the file chip when undecided (file-first)', () => {
		const f = field();
		expect(selectedChipKey(f, sourceChips(f))).toBe('file');
	});
	it('selects the provider chip a provider decision points at', () => {
		const f = field({ decision: { source: 'provider:tmdb', standing: true } });
		expect(selectedChipKey(f, sourceChips(f))).toBe('provider:tmdb');
	});
	it('selects the file chip when the decided provider folded into it', () => {
		const f = field({
			decision: { source: 'provider:tmdb', standing: true },
			candidates: [
				{ source: 'file', value: 'Same' },
				{ source: 'provider:tmdb', provider: 'tmdb', value: 'Same' }
			]
		});
		expect(selectedChipKey(f, sourceChips(f))).toBe('file');
	});
	it('selects the Custom chip for a manual decision', () => {
		const f = field({ decision: { source: 'manual', standing: true, manual_value: 'X' } });
		expect(selectedChipKey(f, sourceChips(f))).toBe('custom');
	});
});

// F37 — the entity-generic baseline (RD4): baselineKey='record' must anchor/fold/select
// exactly like the 'file' default. Person fields carry `record` candidates and decisions.
describe("baselineKey='record' (F37 person fields)", () => {
	// A person bio: empty record baseline + one provider value (the common enrichment-only case).
	function personField(over: Partial<ResolvedField> = {}): ResolvedField {
		return {
			canonical: 'bio',
			label: 'Bio',
			values: ['An American actress.'],
			candidates: [
				{ source: 'record', value: '' },
				{ source: 'provider:tmdb', provider: 'tmdb', value: 'An American actress.' }
			],
			...over
		};
	}
	// The name field: the record baseline is the live people.name.
	function nameField(over: Partial<ResolvedField> = {}): ResolvedField {
		return personField({
			canonical: 'name',
			label: 'Name',
			values: ['Jennifer Lawrence'],
			candidates: [
				{ source: 'record', value: 'Jennifer Lawrence' },
				{ source: 'provider:tmdb', provider: 'tmdb', value: 'Jennifer Shrader Lawrence' }
			],
			...over
		});
	}

	it('baselineCandidateValue reads the record candidate', () => {
		expect(baselineCandidateValue(nameField(), 'record')).toBe('Jennifer Lawrence');
		expect(baselineCandidateValue(personField(), 'record')).toBe('');
	});

	it('decidedSource defaults to record when undecided', () => {
		expect(decidedSource(personField(), 'record')).toBe('record');
	});

	it('anchors the record baseline first, tagged record — even when empty (RD3)', () => {
		const [first] = sourceChips(personField(), 'record');
		expect(first).toMatchObject({
			key: 'record',
			value: '',
			sources: ['record'],
			decisionSource: 'record'
		});
	});

	it('gives a diverging provider its own chip and appends Custom last', () => {
		const chips = sourceChips(nameField(), 'record');
		expect(chips.map((c) => c.key)).toEqual(['record', 'provider:tmdb', 'custom']);
		expect(chips[1]).toMatchObject({ value: 'Jennifer Shrader Lawrence', sources: ['tmdb'] });
	});

	it('folds a provider that agrees with the record into the record chip (·record + tmdb)', () => {
		const f = nameField({
			candidates: [
				{ source: 'record', value: 'Jennifer Lawrence' },
				{ source: 'provider:tmdb', provider: 'tmdb', value: 'Jennifer Lawrence' }
			]
		});
		const chips = sourceChips(f, 'record');
		expect(chips.map((c) => c.key)).toEqual(['record', 'custom']);
		expect(chips[0]).toMatchObject({ value: 'Jennifer Lawrence', sources: ['record', 'tmdb'] });
	});

	it('an empty record baseline never folds providers into it', () => {
		const chips = sourceChips(personField(), 'record');
		expect(chips.map((c) => c.key)).toEqual(['record', 'provider:tmdb', 'custom']);
	});

	it('undecided + record has a value ⇒ record chip selected (record-first)', () => {
		const f = nameField();
		expect(selectedChipKey(f, sourceChips(f, 'record'), 'record')).toBe('record');
	});

	it('undecided + empty record ⇒ provider chip selected (RD6 — the raw-enrichment display)', () => {
		const f = personField();
		expect(selectedChipKey(f, sourceChips(f, 'record'), 'record')).toBe('provider:tmdb');
	});

	it('a standing record decision (blank-pin, RD3) selects the — record chip', () => {
		const f = personField({ decision: { source: 'record', standing: true } });
		expect(selectedChipKey(f, sourceChips(f, 'record'), 'record')).toBe('record');
	});

	it('a provider decision selects the provider chip; folded ⇒ the record chip', () => {
		const decided = personField({ decision: { source: 'provider:tmdb', standing: true } });
		expect(selectedChipKey(decided, sourceChips(decided, 'record'), 'record')).toBe(
			'provider:tmdb'
		);
		const folded = nameField({
			decision: { source: 'provider:tmdb', standing: true },
			candidates: [
				{ source: 'record', value: 'Same' },
				{ source: 'provider:tmdb', provider: 'tmdb', value: 'Same' }
			]
		});
		expect(selectedChipKey(folded, sourceChips(folded, 'record'), 'record')).toBe('record');
	});

	it('a manual decision selects the Custom chip carrying the frozen literal', () => {
		const f = personField({ decision: { source: 'manual', standing: true, manual_value: 'Mine' } });
		const chips = sourceChips(f, 'record');
		expect(chips.at(-1)).toMatchObject({ key: 'custom', value: 'Mine', manual: true });
		expect(selectedChipKey(f, chips, 'record')).toBe('custom');
	});

	it('an unmatched provider decision falls back to the record chip', () => {
		const f = personField({ decision: { source: 'provider:gone', standing: true } });
		expect(selectedChipKey(f, sourceChips(f, 'record'), 'record')).toBe('record');
	});

	it('person fields never read out of sync (in_sync is absent by contract)', () => {
		expect(outOfSync(personField())).toBe(false);
		expect(outOfSyncCount([personField(), nameField()])).toBe(0);
	});
});

// Default-key regression guard: the file spelling stays byte-compatible with F36.
describe('baselineKey default (media page unchanged)', () => {
	it('baselineCandidateValue defaults to the file baseline (fileCandidateValue parity)', () => {
		const f: ResolvedField = {
			canonical: 'title',
			label: 'Title',
			values: ['Blade Runner'],
			candidates: [
				{ source: 'file', value: 'Blade Runner' },
				{ source: 'provider:tmdb', provider: 'tmdb', value: 'Blade Runner: Final Cut' }
			]
		};
		expect(baselineCandidateValue(f)).toBe(fileCandidateValue(f));
		expect(baselineCandidateValue(f)).toBe('Blade Runner');
	});
});

describe('outOfSync / outOfSyncCount', () => {
	it('an undecided field is in sync (no warn signal)', () => {
		expect(outOfSync(field())).toBe(false);
	});
	it('a field is out of sync when in_sync is explicitly false', () => {
		expect(outOfSync(field({ in_sync: false }))).toBe(true);
	});
	it('in_sync true reads in sync', () => {
		expect(outOfSync(field({ in_sync: true }))).toBe(false);
	});
	it('counts only out-of-sync replace fields (merge fields excluded, RD1)', () => {
		const fields = [
			field({ canonical: 'title', in_sync: false }),
			field({ canonical: 'studio', in_sync: true }),
			field({ canonical: 'director', in_sync: false }),
			field({ canonical: 'year' }), // undecided ⇒ undefined ⇒ not counted
			field({ canonical: 'genres', multi: true, in_sync: false }) // merge ⇒ not counted
		];
		expect(outOfSyncCount(fields)).toBe(2);
	});
});
