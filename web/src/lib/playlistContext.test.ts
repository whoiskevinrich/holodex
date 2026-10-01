import { describe, expect, it } from 'vitest';
import { setPlayIntent, takePlayIntent } from './playlistContext';

// The `?playlist=` parse, href and neighbours cases moved to run.test.ts with the code
// (F75: F69's context is now a run with a playlist source), case for case.

describe('play intent', () => {
	it('is consumed once, only by the video it names', () => {
		setPlayIntent(7);
		expect(takePlayIntent(7)).toBe(true);
		expect(takePlayIntent(7)).toBe(false); // consumed
	});
	it('is cleared by any other outcome, so a stale intent never fires later', () => {
		setPlayIntent(7);
		expect(takePlayIntent(8)).toBe(false); // a different video loaded
		expect(takePlayIntent(7)).toBe(false); // and the intent is gone
	});
	it('is empty on a fresh load (a reload or shared link never autoplays)', () => {
		expect(takePlayIntent(1)).toBe(false);
	});
});
