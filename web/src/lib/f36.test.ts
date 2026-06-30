import { describe, it, expect } from 'vitest';
import {
	decidedSource,
	fileCandidateValue,
	isReplaceField,
	outOfSync,
	outOfSyncCount,
	providerCandidates,
	providerOf,
	providersDiffer
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

describe('providersDiffer', () => {
	it('is false with a single provider', () => {
		expect(providersDiffer(field())).toBe(false);
	});
	it('is true when two providers disagree', () => {
		const f = field({
			candidates: [
				{ source: 'file', value: 'Blade Runner' },
				{ source: 'provider:tmdb', provider: 'tmdb', value: 'Final Cut' },
				{ source: 'provider:imdb', provider: 'imdb', value: 'The Directors Cut' }
			]
		});
		expect(providersDiffer(f)).toBe(true);
	});
	it('is false when two providers agree', () => {
		const f = field({
			candidates: [
				{ source: 'provider:tmdb', provider: 'tmdb', value: 'Same' },
				{ source: 'provider:imdb', provider: 'imdb', value: 'Same' }
			]
		});
		expect(providersDiffer(f)).toBe(false);
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
