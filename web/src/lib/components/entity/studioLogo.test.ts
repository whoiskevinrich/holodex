import { describe, expect, it } from 'vitest';
import { imageAlt, isWordmark, showName } from './studioLogo';

// The caption rule from docs/design/studio-logo-link-card-handoff.md §1 (HOLODEX-411),
// shared by StudioLinkCard and the /studios list rows (HOLODEX-432): a bare logo at least
// twice as wide as tall is a wordmark and renders alone; everything else keeps the name.
describe('isWordmark', () => {
	it('is true at exactly 2:1 and wider', () => {
		expect(isWordmark(200, 100)).toBe(true);
		expect(isWordmark(1200, 100)).toBe(true);
	});

	it('is false for a symbol mark, a portrait logo, and an unloaded image', () => {
		expect(isWordmark(199, 100)).toBe(false);
		expect(isWordmark(100, 150)).toBe(false);
		expect(isWordmark(0, 0)).toBe(false);
	});
});

describe('showName', () => {
	it('always shows the name for an icon or monogram (not bare)', () => {
		expect(showName(false, null)).toBe(true);
		expect(showName(false, true)).toBe(true);
		expect(showName(false, false)).toBe(true);
	});

	it('hides the name beside a bare logo until it loads, and beside a wordmark', () => {
		expect(showName(true, null)).toBe(false);
		expect(showName(true, true)).toBe(false);
	});

	it('shows the name beside a bare symbol mark or a failed load', () => {
		expect(showName(true, false)).toBe(true);
	});
});

describe('imageAlt', () => {
	it('announces the name exactly once: empty alt when the caption shows, the name when it does not', () => {
		expect(imageAlt('Aurora Pictures', false, null)).toBe('');
		expect(imageAlt('Aurora Pictures', true, false)).toBe('');
		expect(imageAlt('Aurora Pictures', true, null)).toBe('Aurora Pictures');
		expect(imageAlt('Aurora Pictures', true, true)).toBe('Aurora Pictures');
	});
});
