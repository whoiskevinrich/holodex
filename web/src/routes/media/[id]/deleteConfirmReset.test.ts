import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// HOLODEX-529: this route component is reused across /media/A -> /media/B, and in a run
// `ended` advances while the delete confirm is still open behind playback. The per-id load
// effect must close it, or the dialog names — and Confirm deletes — the next item. Pins the
// reset structurally, as playerElement.test.ts does for the <video> invariant.
const source = readFileSync(fileURLToPath(new URL('./+page.svelte', import.meta.url)), 'utf8');

describe('media page delete confirm', () => {
	it('is reset by the per-id load effect', () => {
		const load = source.indexOf('.getMedia(current)');
		expect(load).toBeGreaterThan(-1);
		const effectStart = source.lastIndexOf('$effect(', load);
		const effect = source.slice(effectStart, load);
		expect(effect).toContain('const current = id;');
		expect(effect).toMatch(/confirmMode = null;/);
		expect(effect).toMatch(/deleteMenuOpen = false;/);
		expect(effect).toMatch(/deleteBusy = false;/);
	});
});
