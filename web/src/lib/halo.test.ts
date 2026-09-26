import { describe, expect, it } from 'vitest';
import { haloClass, paletteMode } from './halo';

describe('paletteMode (HOLODEX-463)', () => {
	it('classifies every shipped skin background as dark', () => {
		for (const bg of ['#0c0a09', '#060814', '#0a0a0a']) expect(paletteMode(bg)).toBe('dark');
	});

	it('classifies a light custom background as light', () => {
		expect(paletteMode('#ffffff')).toBe('light');
		expect(paletteMode('#f3ece1')).toBe('light');
	});

	it('splits at the equal-contrast luminance (~0.179)', () => {
		expect(paletteMode('#737373')).toBe('dark'); // L ≈ 0.171
		expect(paletteMode('#777777')).toBe('light'); // L ≈ 0.184
	});

	it('falls back to dark on a missing or malformed value', () => {
		expect(paletteMode(undefined)).toBe('dark');
		expect(paletteMode('white')).toBe('dark');
		expect(paletteMode('#fff')).toBe('dark');
	});
});

describe('haloClass (HOLODEX-463)', () => {
	it('is empty by default — the halo is off', () => {
		expect(haloClass(undefined)).toBe('');
		expect(haloClass([])).toBe('');
	});

	it('emits one hook per saved mode', () => {
		expect(haloClass(['dark'])).toBe('halo-dark');
		expect(haloClass(['dark', 'light'])).toBe('halo-dark halo-light');
	});
});
