import { describe, it, expect } from 'vitest';
import { seededShuffle } from './shuffle';

describe('seededShuffle', () => {
	const items = Array.from({ length: 50 }, (_, i) => i);

	it('is a permutation — every element preserved, none added or lost', () => {
		const out = seededShuffle(items, 123);
		expect(out).toHaveLength(items.length);
		expect([...out].sort((a, b) => a - b)).toEqual(items);
	});

	it('does not mutate the input', () => {
		const copy = [...items];
		seededShuffle(items, 7);
		expect(items).toEqual(copy);
	});

	it('is deterministic for a given seed', () => {
		expect(seededShuffle(items, 999)).toEqual(seededShuffle(items, 999));
	});

	it('reorders differently for different seeds', () => {
		expect(seededShuffle(items, 1)).not.toEqual(seededShuffle(items, 2));
	});

	it('actually shuffles (not the identity order) for a typical seed', () => {
		expect(seededShuffle(items, 42)).not.toEqual(items);
	});

	it('handles empty and single-element inputs', () => {
		expect(seededShuffle([], 5)).toEqual([]);
		expect(seededShuffle(['x'], 5)).toEqual(['x']);
	});
});
