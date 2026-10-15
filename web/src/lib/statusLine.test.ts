import { afterEach, describe, expect, it, vi } from 'vitest';
import { StatusLine } from './statusLine.svelte';
import { enrichedLine } from './enrichRefresh';

describe('StatusLine (HOLODEX-86)', () => {
	afterEach(() => vi.useRealTimers());

	it('clears itself after the delay', () => {
		vi.useFakeTimers();
		const s = new StatusLine(4000);
		s.show('Enriched from tmdb · 2 fields');
		vi.advanceTimersByTime(3999);
		expect(s.text).toBe('Enriched from tmdb · 2 fields');
		vi.advanceTimersByTime(1);
		expect(s.text).toBe('');
	});

	it('restarts the clock on a newer line, so the old timer cannot cut it short', () => {
		vi.useFakeTimers();
		const s = new StatusLine(4000);
		s.show('first');
		vi.advanceTimersByTime(3000);
		s.show('second');
		vi.advanceTimersByTime(3000);
		expect(s.text).toBe('second');
		vi.advanceTimersByTime(1000);
		expect(s.text).toBe('');
	});

	it("show('') clears at once", () => {
		const s = new StatusLine();
		s.show('x');
		s.show('');
		expect(s.text).toBe('');
	});
});

describe('enrichedLine', () => {
	it('names the provider and pluralizes the count', () => {
		expect(enrichedLine('tmdb', 1)).toBe('Enriched from tmdb · 1 field');
		expect(enrichedLine('tmdb', 7)).toBe('Enriched from tmdb · 7 fields');
	});

	it('does not claim success for an empty response', () => {
		expect(enrichedLine('tmdb', 0)).toBe('No values came back from tmdb');
	});
});
