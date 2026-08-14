import { describe, it, expect } from 'vitest';
import {
	baselineCandidateValue,
	decidedSource,
	fileCandidateValue,
	isPendingSelection,
	isReplaceField,
	isWritable,
	needsWriteback,
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

// HOLODEX-245 — the RD6 implicit-winner chip (empty baseline, provider wins by precedence, no
// standing decision) gets a distinct "pending" treatment in SourceSelect. isPendingSelection is
// presentational-only: it must never fire when a real decision exists or when the baseline
// itself has a value, and it must agree with selectedChipKey on which chip is actually selected.
describe('isPendingSelection', () => {
	it('is true for an undecided field with an empty file baseline and a provider winner (poster_url)', () => {
		const poster = field({
			canonical: 'poster_url',
			display: 'image_url',
			values: ['https://example.test/p.jpg'],
			winning_source: 'tmdb:poster_url',
			candidates: [{ source: 'provider:tmdb', provider: 'tmdb', value: 'https://example.test/p.jpg' }]
		});
		expect(isPendingSelection(poster, sourceChips(poster))).toBe(true);
	});

	it('is false when the file baseline has a value (the ordinary undecided case)', () => {
		const f = field(); // file "Blade Runner" wins by precedence — never pending
		expect(isPendingSelection(f, sourceChips(f))).toBe(false);
	});

	it('is false once a standing decision exists, even with an empty baseline', () => {
		const poster = field({
			canonical: 'poster_url',
			candidates: [{ source: 'provider:tmdb', provider: 'tmdb', value: 'https://example.test/p.jpg' }],
			decision: { source: 'provider:tmdb', standing: true }
		});
		expect(isPendingSelection(poster, sourceChips(poster))).toBe(false);
	});

	it('is false when the empty baseline has no provider candidate either (nothing to select)', () => {
		const empty = field({ candidates: [{ source: 'file', value: '' }] });
		expect(isPendingSelection(empty, sourceChips(empty))).toBe(false);
	});

	it('is true against the REAL API shape — decision always populated, standing:false for an implicit winner', () => {
		// Regression guard (HOLODEX-245): the backend's replaceMarkers() never omits `decision` —
		// an undecided field still reports one, with `standing: false`. A field fixture that
		// merely omits `decision` (as the other cases in this file do) is a simplification the
		// real API never produces; this case pins the actual payload shape so `!field.decision`
		// (which is never true against production data) can't silently regress back in.
		const poster = field({
			canonical: 'poster_url',
			display: 'image_url',
			values: ['https://example.test/p.jpg'],
			winning_source: 'tmdb:poster_url',
			candidates: [{ source: 'provider:tmdb', provider: 'tmdb', value: 'https://example.test/p.jpg' }],
			decision: { source: 'provider:tmdb', standing: false }
		});
		expect(isPendingSelection(poster, sourceChips(poster))).toBe(true);
		expect(selectedChipKey(poster, sourceChips(poster))).toBe('provider:tmdb');
	});

	it('applies to any baselineKey (record, F37) the same way as file', () => {
		const bio: ResolvedField = {
			canonical: 'bio',
			label: 'Bio',
			values: ['An American actress.'],
			candidates: [
				{ source: 'record', value: '' },
				{ source: 'provider:tmdb', provider: 'tmdb', value: 'An American actress.' }
			]
		};
		expect(isPendingSelection(bio, sourceChips(bio, 'record'), 'record')).toBe(true);
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
			field({ canonical: 'title', in_sync: false, write_target: 'Title' }),
			field({ canonical: 'studio', in_sync: true, write_target: 'Publisher' }),
			field({ canonical: 'director', in_sync: false, write_target: 'Artist' }),
			field({ canonical: 'year' }), // undecided ⇒ undefined ⇒ not counted
			field({ canonical: 'genres', multi: true, in_sync: false }) // merge ⇒ not counted
		];
		expect(outOfSyncCount(fields)).toBe(2);
	});
});

// HOLODEX-213: the header count and the writeback dialog's initial checked state must be the
// same predicate, so the dialog can never open pre-checking more than the header reported.
describe('needsWriteback', () => {
	it('is true for an out-of-sync replace field (an explicit decision the file lags)', () => {
		expect(
			needsWriteback(
				field({ decision: { source: 'provider:tmdb', standing: true }, in_sync: false, write_target: 'Title' })
			)
		).toBe(true);
	});

	it('is false for an out-of-sync field with no file-tag mapping for the container (HOLODEX-216)', () => {
		// A decided value the file lags, but the container has no destination tag for it — writing
		// would only silently drop it, so this must never auto-check into the batch.
		expect(
			needsWriteback(field({ decision: { source: 'manual', standing: true }, in_sync: false }))
		).toBe(false);
	});

	it('is false for a provider value winning by mapping precedence (undecided ⇒ in sync)', () => {
		// The regression: winning_source is a provider, but nothing was decided — the old
		// dialog seeded `checked` from winning_source alone and pre-checked this row.
		expect(needsWriteback(field({ winning_source: 'tmdb:title' }))).toBe(false);
	});

	it('is false for a provider-won field with no file candidate at all (poster_url)', () => {
		// poster_url has no file value to compare against, so the old `!alreadyMatches` guard
		// never fired and it arrived checked — arming a download + cover-art embed on submit.
		const poster = field({
			canonical: 'poster_url',
			display: 'image_url',
			values: ['https://example.test/p.jpg'],
			winning_source: 'tmdb:poster_url',
			candidates: [{ source: 'provider:tmdb', provider: 'tmdb', value: 'https://example.test/p.jpg' }]
		});
		expect(needsWriteback(poster)).toBe(false);
	});

	it('is false for a merge field even when the backend marks it out of sync (RD1)', () => {
		expect(needsWriteback(field({ multi: true, in_sync: false }))).toBe(false);
	});

	it('agrees with outOfSyncCount field-for-field (the two surfaces cannot disagree)', () => {
		const fields = [
			field({ canonical: 'title', in_sync: false, write_target: 'Title' }),
			field({ canonical: 'overview', winning_source: 'tmdb:overview' }),
			field({ canonical: 'poster_url', winning_source: 'tmdb:poster_url' }),
			field({ canonical: 'director', in_sync: false, write_target: 'Artist' }),
			field({ canonical: 'genres', multi: true, in_sync: false })
		];
		expect(fields.filter(needsWriteback).length).toBe(outOfSyncCount(fields));
		expect(outOfSyncCount(fields)).toBe(2);
	});
});

describe('isWritable', () => {
	it('is true when the backend stamped a destination tag', () => {
		expect(isWritable(field({ write_target: 'Title' }))).toBe(true);
	});
	it('is false when write_target is absent (no mapping for the container)', () => {
		expect(isWritable(field())).toBe(false);
	});
	it('is false for an empty write_target', () => {
		expect(isWritable(field({ write_target: '' }))).toBe(false);
	});
});
