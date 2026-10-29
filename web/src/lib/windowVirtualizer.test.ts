import { describe, expect, it, vi } from 'vitest';

vi.mock('$app/navigation', () => ({ beforeNavigate: () => {} }));
const { rovingMove } = await import('./windowVirtualizer.svelte.ts');

// The whole-list scroll's keys (HOLODEX-601, handoff "Accessibility"): ↑/↓ move a row of
// lanes, ←/→ an entry in a multi-lane view, PageUp/PageDown a screen, Home/End the ends of
// the whole list, never past them.
describe('rovingMove', () => {
	it.each([
		// [key, index, lanes, perPage, total, next]
		['ArrowDown', 4, 3, 10, 100, 7],
		['ArrowUp', 4, 3, 10, 100, 1],
		['ArrowUp', 1, 3, 10, 100, 0],
		['ArrowDown', 98, 3, 10, 100, 99],
		['ArrowRight', 4, 3, 10, 100, 5],
		['ArrowLeft', 0, 3, 10, 100, 0],
		['PageDown', 0, 3, 10, 100, 30],
		['PageUp', 50, 2, 10, 100, 30],
		['PageDown', 0, 1, 0, 100, 1], // an unmeasured screen still moves one row
		['Home', 57, 3, 10, 100, 0],
		['End', 3, 3, 10, 1512, 1511]
	])('%s from %i at %i lanes → %i', (key, index, lanes, perPage, total, next) =>
		expect(rovingMove(key, index, lanes, perPage, total)).toBe(next)
	);

	it('leaves ←/→ to the page in a one-lane list, and ignores other keys', () => {
		expect(rovingMove('ArrowRight', 4, 1, 10, 100)).toBeNull();
		expect(rovingMove('ArrowLeft', 4, 1, 10, 100)).toBeNull();
		expect(rovingMove('a', 4, 3, 10, 100)).toBeNull();
	});
});
