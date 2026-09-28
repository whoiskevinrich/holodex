import { describe, expect, it } from 'vitest';
import { PALETTE_MODE, haloClass } from './halo';

describe('PALETTE_MODE (ADR-115)', () => {
	it('is dark — Cinémathèque is the only look', () => {
		expect(PALETTE_MODE).toBe('dark');
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
