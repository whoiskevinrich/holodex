import { describe, it, expect } from 'vitest';
import { DENSITY_MIN, DENSITY_MAX, invertDensity, capForWidth } from './density.svelte';

describe('column count at max density', () => {
	// HOLODEX-331. Asserting DENSITY_MAX alone would not catch the bug this guards:
	// VideoGrid renders Math.min(density, viewportTierCap), so a top TIERS rung below
	// DENSITY_MAX silently clamps the slider's last stops while every constant still
	// reads correctly. Mirror VideoGrid's own computation so both halves are covered.
	it('reaches 8 columns on a wide viewport', () => {
		expect(Math.min(DENSITY_MAX, capForWidth(1920))).toBe(8);
	});

	it('reaches 8 columns as soon as the widest tier starts', () => {
		expect(Math.min(DENSITY_MAX, capForWidth(1536))).toBe(8);
	});
});

describe('capForWidth tiers', () => {
	// Boundary pairs — `TIERS.find(t => width >= t.min)` is an inclusive lower bound,
	// so each rung's first pixel belongs to that rung.
	it('steps at each tier boundary', () => {
		expect(capForWidth(1536)).toBe(DENSITY_MAX);
		expect(capForWidth(1535)).toBe(4);
		expect(capForWidth(1280)).toBe(4);
		expect(capForWidth(1279)).toBe(3);
		expect(capForWidth(1024)).toBe(3);
		expect(capForWidth(1023)).toBe(2);
		expect(capForWidth(480)).toBe(2);
	});

	it('falls back to a single column below the narrowest tier', () => {
		expect(capForWidth(479)).toBe(1);
		expect(capForWidth(0)).toBe(1);
	});
});

describe('invertDensity', () => {
	// The native range input increases left-to-right, but the UI wants dragging right
	// to mean "bigger cards" (fewer columns). Pinning one endpoint fixes the constant;
	// the map is linear, so the opposite endpoint and range containment follow.
	it('maps the leftmost slider position to the most columns', () => {
		expect(invertDensity(DENSITY_MIN)).toBe(DENSITY_MAX);
	});
});
