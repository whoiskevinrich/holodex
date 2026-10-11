import { describe, expect, it } from 'vitest';
import {
	ROW_PX,
	centredScrollTop,
	matchIndices,
	parseJumpQuery,
	splitMatch,
	viewportPx,
	windowRange
} from './runJump';

describe('parseJumpQuery', () => {
	it('reads digits as a 1-based position in range', () => {
		expect(parseJumpQuery(' 142 ', 389)).toEqual({ kind: 'number', index: 141 });
		expect(parseJumpQuery('1', 389)).toEqual({ kind: 'number', index: 0 });
		expect(parseJumpQuery('389', 389)).toEqual({ kind: 'number', index: 388 });
	});
	it('refuses a number outside 1 to N', () => {
		expect(parseJumpQuery('0', 389)).toEqual({ kind: 'out-of-range' });
		expect(parseJumpQuery('390', 389)).toEqual({ kind: 'out-of-range' });
	});
	it('treats anything else as title words, lowercased', () => {
		expect(parseJumpQuery('Harbor', 389)).toEqual({ kind: 'words', text: 'harbor' });
		expect(parseJumpQuery('part 2', 389)).toEqual({ kind: 'words', text: 'part 2' });
		expect(parseJumpQuery('   ', 389)).toEqual({ kind: 'empty' });
	});
});

describe('matchIndices', () => {
	const titles = new Map([
		[10, 'Harbor Session'],
		[11, 'Quiet Encore'],
		[12, 'Old harbor'],
		[13, undefined as unknown as string]
	]);
	it('keeps run order and run indices, case-insensitively', () => {
		// Shuffled order: position, not id, decides the order.
		expect(matchIndices([12, 11, 10, 13], (id) => titles.get(id), 'harbor')).toEqual([0, 2]);
	});
	it('skips an item whose details are missing', () => {
		expect(matchIndices([13], (id) => titles.get(id), 'a')).toEqual([]);
	});
});

describe('splitMatch', () => {
	it('splits around the first match, keeping the title casing', () => {
		expect(splitMatch('Old Harbor', 'harbor')).toEqual(['Old ', 'Harbor', '']);
		expect(splitMatch('Quiet', 'harbor')).toEqual(['Quiet', '', '']);
	});
});

describe('virtual window', () => {
	it('draws the visible rows plus a buffer, never past the ends', () => {
		const vp = viewportPx(389);
		expect(vp).toBe(8.5 * ROW_PX);
		expect(windowRange(0, vp, 389)).toEqual([0, 15]);
		expect(windowRange(100 * ROW_PX, vp, 389)).toEqual([94, 115]);
		expect(windowRange(389 * ROW_PX, vp, 389)).toEqual([383, 389]);
	});
	it('sizes a short run to fit', () => {
		expect(viewportPx(3)).toBe(3 * ROW_PX);
	});
	it('centres a row, clamped to the top and bottom', () => {
		const vp = viewportPx(389);
		expect(centredScrollTop(0, vp, 389)).toBe(0);
		expect(centredScrollTop(56, vp, 389)).toBe(56 * ROW_PX + ROW_PX / 2 - vp / 2);
		expect(centredScrollTop(388, vp, 389)).toBe(389 * ROW_PX - vp);
	});
});
