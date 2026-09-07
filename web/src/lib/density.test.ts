import { describe, it, expect } from 'vitest';
import { DENSITY_MIN, DENSITY_MAX, invertDensity, capForWidth } from './density.svelte';

describe('capForWidth tiers', () => {
	// The requirement that started HOLODEX-331 — "at max density I should see 8 videos in each
	// row" — plus the ultrawide rungs that stopped the ladder dead-ending above 1536px. The
	// boundary pairs also pin the `min`-descending invariant `capForWidth`'s `find` relies on:
	// an out-of-order rung would shadow the ones below it and show up here.
	it('steps at each tier boundary', () => {
		expect(capForWidth(3840)).toBe(16);
		expect(capForWidth(3839)).toBe(12);
		expect(capForWidth(2560)).toBe(12);
		expect(capForWidth(2559)).toBe(8);
		expect(capForWidth(1920)).toBe(8); // the width the "8 per row" requirement was set at
		expect(capForWidth(1536)).toBe(8);
		expect(capForWidth(1535)).toBe(4);
		expect(capForWidth(1280)).toBe(4);
		expect(capForWidth(1279)).toBe(3);
		expect(capForWidth(1024)).toBe(3);
		expect(capForWidth(1023)).toBe(2);
		expect(capForWidth(480)).toBe(2);
	});

	it('falls back to a single column below the narrowest tier', () => {
		expect(capForWidth(479)).toBe(1);
		expect(capForWidth(412)).toBe(1); // Pixel 7 Pro
		expect(capForWidth(0)).toBe(1);
	});

	// The ladder is deliberately tuned only as far as 5120 (a 49" ultrawide). Above 3840 the
	// cap stays put, so cards grow again rather than columns being added. Pinned so the next
	// person on a wider panel finds a documented boundary instead of rediscovering the bug.
	it('stops adding columns above the tuned ceiling', () => {
		expect(capForWidth(5120)).toBe(DENSITY_MAX);
		expect(capForWidth(7680)).toBe(DENSITY_MAX);
	});
});

// The ladder's whole purpose: hold card size roughly constant at max density instead of letting
// it balloon. Before the ultrawide rungs, 5120px rendered four 1252px cards.
//
// GAP mirrors `gap-4` in VideoGrid.svelte / PersonPosterGrid.svelte, and PAGE_PADDING mirrors
// `px-6` on <main> in routes/+layout.svelte. Nothing links them to those files, so treat this as
// an approximate guard on the tuning rather than an exact geometry assertion — if the layout
// changes, retune the bands rather than assuming the ladder broke.
describe('card width across the ladder', () => {
	const GAP = 16;
	const PAGE_PADDING = 48;
	const widthAt = (viewport: number, cols: number) =>
		(viewport - PAGE_PADDING - (cols - 1) * GAP) / cols;

	const LADDER = [1536, 1920, 2560, 3840, 5120];

	it('keeps video cards in a readable band at every tier', () => {
		for (const viewport of LADDER) {
			const w = widthAt(viewport, capForWidth(viewport));
			expect(w).toBeGreaterThan(150);
			expect(w).toBeLessThan(350);
		}
	});

	// People posters run at 2x the columns, so their band is roughly halved. Asserted here so
	// the doubling is tuned against the same rungs rather than being an untested `* 2`.
	//
	// The floor is 75px, not half the video floor: the 1536 rung is the tightest point on the
	// whole ladder for People, landing at 78px (vs 102px at 1920 and 151px at 5120). That is
	// the cost of holding the ratio derived rather than giving People its own rungs — a
	// deliberate call (HOLODEX-331), and this bound is what makes a regression past it visible.
	it('keeps person posters in a readable band at every tier', () => {
		for (const viewport of LADDER) {
			const w = widthAt(viewport, capForWidth(viewport) * 2);
			expect(w).toBeGreaterThan(75);
			expect(w).toBeLessThan(180);
		}
	});
});

describe('invertDensity', () => {
	// The native range input increases left-to-right, but the UI wants dragging right to mean
	// "bigger cards" (fewer columns). It inverts against the *current* viewport cap, not
	// DENSITY_MAX, so the slider spans only reachable columns — inverting against a fixed 16 at
	// a 3-column cap would return 15, far outside the slider's own range.
	const CAPS = [3, 4, 8, 12, 16];

	it('maps the leftmost slider position to that viewport’s most columns', () => {
		for (const cap of CAPS) {
			expect(invertDensity(DENSITY_MIN, cap)).toBe(cap);
		}
	});

	it('is self-inverse and in range for every reachable stop', () => {
		for (const cap of CAPS) {
			for (let n = DENSITY_MIN; n <= cap; n++) {
				const out = invertDensity(n, cap);
				expect(out).toBeGreaterThanOrEqual(DENSITY_MIN);
				expect(out).toBeLessThanOrEqual(cap);
				expect(invertDensity(out, cap)).toBe(n);
			}
		}
	});
});
