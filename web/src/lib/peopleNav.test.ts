import { describe, it, expect } from 'vitest';
import { firstLetter, letterAnchors } from './peopleNav';

describe('firstLetter', () => {
	it('upper-cases and trims leading space', () => {
		expect(firstLetter('  alice')).toBe('A');
		expect(firstLetter('Bob')).toBe('B');
	});
	it('buckets non-ASCII-letter initials under #', () => {
		expect(firstLetter('Édith')).toBe('#'); // accented initial is not A–Z
		expect(firstLetter('宮崎駿')).toBe('#');
		expect(firstLetter('3 Doors')).toBe('#');
		expect(firstLetter('')).toBe('#');
	});
});

describe('letterAnchors', () => {
	it('maps each letter to the first index under it (first wins)', () => {
		expect(letterAnchors(['Ann', 'Amy', 'Bob', '9x'])).toEqual({ A: 0, B: 2, '#': 3 });
	});
	it('is empty for an empty list', () => {
		expect(letterAnchors([])).toEqual({});
	});
});
