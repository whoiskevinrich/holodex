import { describe, expect, it } from 'vitest';
import { thumbnailButtonLabel } from './thumbnailButton.ts';

// F85 testing-strategy §37.4 W6 (design QA 10.10).
describe('thumbnailButtonLabel', () => {
	it('reads Pick a frame, and Saving frame… while the picked frame posts', () => {
		expect(thumbnailButtonLabel({ savingFrame: false, regenerating: false })).toBe('Pick a frame');
		expect(thumbnailButtonLabel({ savingFrame: true, regenerating: false })).toBe('Saving frame…');
	});

	it('never says Regenerate thumbnail', () => {
		for (const savingFrame of [false, true]) {
			for (const regenerating of [false, true]) {
				expect(thumbnailButtonLabel({ savingFrame, regenerating })).not.toContain('Regenerate thumbnail');
			}
		}
	});
});
