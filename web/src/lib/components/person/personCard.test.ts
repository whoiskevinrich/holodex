import { describe, expect, it } from 'vitest';
import { CARD_GAP_PX, placeCard, VIEWPORT_GUTTER_PX } from './personCard.svelte';

// F68 R3 / RD10, pinned from the OQ1 probe (2026-09-20): one measure decides
// placement — flip above only when the card cannot fit below, and clamp
// horizontally by the overshoot rather than end-aligning.
const rect = (left: number, top: number, w = 34, h = 17) =>
	({ left, top, right: left + w, bottom: top + h, width: w, height: h, x: left, y: top } as DOMRect);

describe('placeCard', () => {
	it('opens below-start when there is room', () => {
		expect(placeCard(rect(120, 468), 288, 126, 1280, 900)).toEqual({ above: false, shiftX: 0 });
	});

	it('flips above when the card would not fit below and there is more room above', () => {
		const p = placeCard(rect(120, 800), 288, 126, 1280, 900);
		expect(p.above).toBe(true);
	});

	it('stays below when neither side fits and below has more room', () => {
		// 30px above, 40px below: nothing fits; stay below (the page scrolls) rather than clip upward.
		expect(placeCard(rect(120, 30), 288, 126, 1280, 30 + 17 + 40).above).toBe(false);
	});

	it('stays below when neither side fits even though above has more room (HOLODEX-444)', () => {
		// 100px above, 40px below, card 126: above would sit at a negative y that nothing can
		// scroll to; below at least extends the page.
		expect(placeCard(rect(120, 100), 288, 126, 1280, 100 + 17 + 40).above).toBe(false);
	});

	it('clamps by the overshoot at a narrow viewport — the probe case: 8px past a 400px page', () => {
		// trigger at x=120, card 288 wide → right edge 408; gutter 16 → allowed 384 → shift −24.
		const p = placeCard(rect(120, 468), 288, 126, 400, 700);
		expect(p.shiftX).toBe(-(408 - (400 - VIEWPORT_GUTTER_PX)));
	});

	it('never shifts past the left gutter', () => {
		// trigger at x=20: the clamp would want −24 but only 4px of shift exist before the gutter.
		expect(placeCard(rect(20, 468), 288, 126, 300, 700).shiftX).toBe(-(20 - VIEWPORT_GUTTER_PX));
	});

	it('uses the 6px gap in the fit test', () => {
		// Exactly cardH of room below is not enough once the gap is counted.
		const top = 900 - 17 - 126;
		expect(placeCard(rect(120, top), 288, 126, 1280, 900).above).toBe(true);
		expect(placeCard(rect(120, top - CARD_GAP_PX), 288, 126, 1280, 900).above).toBe(false);
	});
});
