import { describe, expect, it } from 'vitest';
import { arc, bandsAfterRefresh, ringButtonLabel, ringReading, RING_BUSY_ARC, RING_CIRCUMFERENCE } from './ring';

// The ring's reading follows the spec's § Worked examples and the design
// handoff's state table (docs/design/completeness-ring-badge-handoff.md): the
// accent arc is `required`, the ink overfill exists only once required is 100,
// a null required band hands the ring to extras, and both-null mounts nothing.
describe('ringReading', () => {
	it('fills the ring with required and never overfills below 100 (Video B)', () => {
		const r = ringReading({ required: 75, extras: 100 });
		expect(r.ring).toBe(75);
		expect(r.overfill).toBe(0);
		expect(r.label).toBe('Completeness: required 75%, extras 100%');
		expect(r.empty).toBe(false);
	});

	it('draws extras as a second lap once required is 100 (Video A)', () => {
		const r = ringReading({ required: 100, extras: 67 });
		expect(r.ring).toBe(100);
		expect(r.overfill).toBe(67);
	});

	it('draws a full accent ring with no ink when extras is 0 or null (Video C)', () => {
		expect(ringReading({ required: 100, extras: 0 }).overfill).toBe(0);
		expect(ringReading({ required: 100, extras: null }).overfill).toBe(0);
		expect(ringReading({ required: 100, extras: null }).label).toBe('Completeness: required 100%');
	});

	it('lets extras drive the ring when the type has no required band (studio)', () => {
		const r = ringReading({ required: null, extras: 100 });
		expect(r.ring).toBe(100);
		expect(r.overfill).toBe(0);
		expect(r.label).toBe('Completeness: extras 100%');
		expect(ringReading({ required: null, extras: 0 }).ring).toBe(0);
	});

	it('is empty when neither band applies', () => {
		const r = ringReading({ required: null, extras: null });
		expect(r.empty).toBe(true);
		expect(r.ring).toBe(0);
	});
});

describe('arc', () => {
	it('maps 0–100 onto the 44-unit circumference and keeps a 1–4 % tick visible', () => {
		expect(arc(100)).toBe(RING_CIRCUMFERENCE);
		expect(arc(75)).toBe(33);
		expect(arc(1)).toBeGreaterThan(0);
	});
});

// F65.8 (HOLODEX-435): the ring is a button. Its name says what a press does,
// its busy arc is a fixed quarter (never a score), and a failed re-read after
// the refresh leaves the list's bands in place rather than blanking the ring.
describe('ring button (F65.8)', () => {
	it('names the action before the reading, and says so while busy', () => {
		const r = ringReading({ required: 75, extras: 100 });
		expect(ringButtonLabel(r, false)).toBe('Refresh enrichment — Completeness: required 75%, extras 100%');
		expect(ringButtonLabel(r, true)).toBe('Refreshing enrichment — Completeness: required 75%, extras 100%');
	});

	it('spins a quarter lap that no score maps to', () => {
		expect(RING_BUSY_ARC).toBe(RING_CIRCUMFERENCE / 4);
		expect(arc(25)).toBe(RING_BUSY_ARC); // same length as 25 % — the animation, not the length, marks it busy
	});

	it('keeps the list bands when the re-read fails, takes the fresh ones when it lands', () => {
		const list = { required: 75, extras: 100 };
		expect(bandsAfterRefresh(list, null)).toBe(list);
		expect(bandsAfterRefresh(list, { required: 100, extras: 100 })).toEqual({ required: 100, extras: 100 });
	});
});
