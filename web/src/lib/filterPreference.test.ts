import { beforeEach, describe, expect, it } from 'vitest';
import {
	readEntityFilters,
	readFilters,
	validateEntityFilters,
	validateOneOf,
	validateString,
	writeFilters
} from './filterPreference';

// Node has no DOM: the same localStorage stub the theme/adminMode tests use.
function fakeStorage(): Storage {
	const m = new Map<string, string>();
	return {
		getItem: (k) => m.get(k) ?? null,
		setItem: (k, v) => void m.set(k, v),
		removeItem: (k) => void m.delete(k),
		clear: () => m.clear(),
		key: () => null,
		length: 0
	};
}

beforeEach(() => {
	(globalThis as { localStorage: Storage }).localStorage = fakeStorage();
});

describe('filterPreference (SP5 sticky filters)', () => {
	it('round-trips a value under a per-page key', () => {
		writeFilters('tags', 'categories');
		expect(readFilters('tags', validateString)).toBe('categories');
		expect(localStorage.getItem('holodex:filters:tags')).toBe('"categories"');
		// Pages don't share keys.
		expect(readFilters('media', validateString)).toBeUndefined();
	});

	it('returns undefined for a missing, malformed, or rejected value', () => {
		expect(readFilters('media', validateString)).toBeUndefined();
		localStorage.setItem('holodex:filters:media', '{not json');
		expect(readFilters('media', validateString)).toBeUndefined();
		localStorage.setItem('holodex:filters:media', '42');
		expect(readFilters('media', validateString)).toBeUndefined();
	});

	it('never throws when storage is unavailable', () => {
		(globalThis as { localStorage?: Storage }).localStorage = undefined;
		expect(readFilters('media', validateString)).toBeUndefined();
		expect(() => writeFilters('media', 'x')).not.toThrow();
		(globalThis as { localStorage: Storage }).localStorage = {
			...fakeStorage(),
			getItem: () => {
				throw new Error('blocked');
			},
			setItem: () => {
				throw new Error('full');
			}
		};
		expect(readFilters('media', validateString)).toBeUndefined();
		expect(() => writeFilters('media', 'x')).not.toThrow();
	});

	it('validateOneOf accepts only the listed values', () => {
		const v = validateOneOf(['all', 'tags', 'categories'] as const);
		expect(v('tags')).toBe('tags');
		expect(v('people')).toBeUndefined();
		expect(v(1)).toBeUndefined();
	});

	it('validateEntityFilters accepts the People/Studios shape and rejects drift', () => {
		expect(validateEntityFilters({ completeness: 'desc', missing_facet: ['birthdate'] })).toEqual({
			completeness: 'desc',
			missing_facet: ['birthdate']
		});
		expect(validateEntityFilters({ completeness: '', missing_facet: [] })).toEqual({
			completeness: '',
			missing_facet: []
		});
		// A direction that no longer exists, a non-array facet list, or a foreign shape
		// all fall back rather than wedging the control.
		expect(validateEntityFilters({ completeness: 'sideways', missing_facet: [] })).toBeUndefined();
		expect(validateEntityFilters({ completeness: 'asc', missing_facet: 'birthdate' })).toBeUndefined();
		expect(validateEntityFilters({ completeness: 'asc', missing_facet: [1] })).toBeUndefined();
		expect(validateEntityFilters(null)).toBeUndefined();
		expect(validateEntityFilters('asc')).toBeUndefined();
	});

	it('readEntityFilters falls back to the off/empty default', () => {
		expect(readEntityFilters('people')).toEqual({ completeness: '', missing_facet: [] });
		writeFilters('people', { completeness: 'asc', missing_facet: ['nationality'] });
		expect(readEntityFilters('people')).toEqual({ completeness: 'asc', missing_facet: ['nationality'] });
		expect(readEntityFilters('studios')).toEqual({ completeness: '', missing_facet: [] });
	});
});
