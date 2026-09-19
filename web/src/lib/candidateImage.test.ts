import { describe, expect, it } from 'vitest';
import { SLOT_CLASS, showThumb, slotShape } from './candidateImage';

// F64 slot rule (design handoff "Content spec"): image when the server let an
// http(s) image_url through and the <img> has not errored; monogram otherwise.
describe('showThumb', () => {
	it('shows the image for an http(s) image_url that has not failed', () => {
		expect(
			showThumb({ image_url: 'https://cdn.acme.example/t/w185/1.jpg' }, false),
		).toBe(true);
		expect(
			showThumb(
				{ image_url: 'http://stub:9100/p/twins/thumb/portrait-1.png' },
				false,
			),
		).toBe(true);
	});
	it('falls back to the monogram when the key is absent (pre-F64 provider or stripped by core)', () => {
		expect(showThumb({}, false)).toBe(false);
		expect(showThumb({ image_url: undefined }, false)).toBe(false);
	});
	it('falls back to the monogram once the image errored', () => {
		expect(
			showThumb({ image_url: 'https://cdn.acme.example/t/w185/404.jpg' }, true),
		).toBe(false);
	});
	it('never renders a non-http(s) value even if one slipped past the server', () => {
		expect(showThumb({ image_url: 'javascript:alert(1)' }, false)).toBe(false);
		expect(showThumb({ image_url: 'data:image/png;base64,AAAA' }, false)).toBe(
			false,
		);
		expect(showThumb({ image_url: '' }, false)).toBe(false);
	});
});

// HOLODEX-414: the slot's box follows the entity kind — always 60 tall, width by shape.
describe('slotShape', () => {
	it('gives person and film the 2:3 portrait (headshot / poster)', () => {
		expect(slotShape('person')).toBe('portrait');
		expect(slotShape('film')).toBe('portrait');
	});
	it('gives video the landscape box (the provider sends a backdrop)', () => {
		expect(slotShape('video')).toBe('landscape');
	});
	it('gives studio the logo box', () => {
		expect(slotShape('studio')).toBe('logo');
	});
	it('maps every shape to an explicit width and the shared 60px height from sm up, never an aspect class', () => {
		expect(SLOT_CLASS.portrait).toBe('w-10 h-15');
		expect(SLOT_CLASS.landscape).toBe('w-20 h-11.25 sm:w-27 sm:h-15');
		expect(SLOT_CLASS.logo).toBe('w-20 h-10 sm:w-30 sm:h-15');
		for (const cls of Object.values(SLOT_CLASS)) {
			expect(cls).not.toMatch(/aspect-/);
			expect(cls).toMatch(/(^|\s)(sm:)?h-15\b/);
		}
	});
	it('narrows only the two wide boxes below sm, keeping each aspect (QA §4.4)', () => {
		// 80 × 45 is 16:9, 80 × 40 is 2:1 — same shapes as their sm sizes, so the same
		// image letterboxes the same way; the portrait box is already the narrowest.
		expect(SLOT_CLASS.landscape).toMatch(/^w-20 h-11\.25 /);
		expect(SLOT_CLASS.logo).toMatch(/^w-20 h-10 /);
		expect(SLOT_CLASS.portrait).not.toMatch(/sm:/);
	});
});
