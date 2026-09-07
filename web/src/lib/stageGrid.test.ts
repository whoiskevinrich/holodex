import { describe, it, expect } from 'vitest';
import { stageGridTracks, stageGridWidth } from './stageGrid';

// Real geometry from the 5120x1440 display this was designed against: 5120 viewport
// minus the page's px-6 (48px) leaves 5072 of content, the density ladder's top rung
// gives 16 columns, and `gap-4` is 16px. Measured in the browser as 302px cards.
const AVAIL = 5072;
const COLS = 16;
const GAP = 16;
const STAGE = 2600;

describe('stageGridTracks', () => {
	it('sizes cards from the column count, not the card count', () => {
		// The whole point: two scenes must not become two half-container cards.
		for (const n of [1, 2, 8, 16, 40]) {
			expect(stageGridTracks(COLS, n, AVAIL, GAP).trackPx).toBe(302);
		}
	});

	it('declares no more tracks than there are cards', () => {
		expect(stageGridTracks(COLS, 2, AVAIL, GAP).trackCount).toBe(2);
		expect(stageGridTracks(COLS, 9, AVAIL, GAP).trackCount).toBe(9);
		// Beyond a full row the count saturates and the extras wrap.
		expect(stageGridTracks(COLS, 16, AVAIL, GAP).trackCount).toBe(16);
		expect(stageGridTracks(COLS, 40, AVAIL, GAP).trackCount).toBe(16);
	});

	it('reports no track width before the container has been measured', () => {
		// First paint, before bind:clientWidth resolves — the caller falls back to 1fr.
		expect(stageGridTracks(COLS, 5, 0, GAP).trackPx).toBe(0);
	});

	it('survives degenerate inputs', () => {
		expect(stageGridTracks(COLS, 0, AVAIL, GAP).trackCount).toBe(0);
		expect(stageGridTracks(0, 5, AVAIL, GAP).trackPx).toBe(0);
	});
});

describe('the stage-hold threshold', () => {
	// This is the behaviour that cannot be reached in the dev fixture, whose only film
	// has two scenes. The CSS holds the grid at `min-width: stage` until its content is
	// wider than the stage, so the flip happens where stageGridWidth crosses 2600.
	const widthFor = (n: number) => stageGridWidth(stageGridTracks(COLS, n, AVAIL, GAP), GAP);

	it('stays within the stage up to 8 cards', () => {
		for (const n of [1, 2, 4, 8]) {
			expect(widthFor(n)).toBeLessThanOrEqual(STAGE);
		}
		expect(widthFor(8)).toBe(2528); // the last count that fits
	});

	it('outgrows the stage from 9 cards on', () => {
		expect(widthFor(9)).toBe(2846);
		expect(widthFor(9)).toBeGreaterThan(STAGE);
		for (const n of [9, 10, 12, 16]) {
			expect(widthFor(n)).toBeGreaterThan(STAGE);
		}
	});

	it('reaches exactly the available width at a full row', () => {
		expect(widthFor(16)).toBe(AVAIL);
	});

	it('never exceeds the available width', () => {
		for (let n = 1; n <= 60; n++) {
			expect(widthFor(n)).toBeLessThanOrEqual(AVAIL);
		}
	});

	// The threshold is emergent from card size and stage width, not a constant, so it
	// must move on its own when either changes rather than needing a matching edit.
	it('moves with the stage width instead of being pinned to a number', () => {
		const narrowStage = 1600;
		const firstOver = (limit: number) => {
			for (let n = 1; n <= COLS; n++) if (widthFor(n) > limit) return n;
			return -1;
		};
		expect(firstOver(STAGE)).toBe(9);
		expect(firstOver(narrowStage)).toBe(6); // a smaller stage flips sooner, no code change
	});
});

describe('below the stage width the mode is inert', () => {
	// At a 1920 viewport the content is 1872 — under the stage — so `min-width: min(stage,
	// 100%)` resolves to 100% and the grid fills its container for any card count, exactly
	// as it did before this mode existed. Verified in the browser; pinned here.
	const NARROW = 1872;
	const NARROW_COLS = 8;

	it('fills the container at a full row', () => {
		const t = stageGridTracks(NARROW_COLS, 8, NARROW, GAP);
		expect(t.trackPx).toBe(220);
		expect(stageGridWidth(t, GAP)).toBe(NARROW);
	});

	it('keeps card size constant for short rows', () => {
		expect(stageGridTracks(NARROW_COLS, 2, NARROW, GAP).trackPx).toBe(220);
	});
});
