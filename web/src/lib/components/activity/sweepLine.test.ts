import { describe, expect, it } from 'vitest';
import { doneLineFor, skippedText, sweepFinished } from './sweepLine';
import type { SweepStatus, SweepSummary } from '$lib/types';

// SweepStatusLine's pure pieces (F66 RD10 / P0-6): the done line is this kind's
// last_run minus the dismissed batch, the running→idle edge fires once and only
// for a sweep the page saw running, and the skipped-provider text groups by reason.

const zero = { linked: 0, needs_review: 0, no_candidates: 0, failed: 0, skipped: 0, stale_skipped: 0 };
const idle = (last_run: SweepSummary | null = null): SweepStatus => ({
	state: 'idle',
	total: 0,
	done: 0,
	last_run,
	...zero
});
const running = (kind: 'person' | 'studio'): SweepStatus => ({
	state: 'running',
	kind,
	total: 21,
	done: 3,
	last_run: null,
	...zero
});
const summary = (kind: 'person' | 'studio', batch_id = 'b1'): SweepSummary => ({
	kind,
	finished_at: '2026-09-19T12:00:00Z',
	duration_ms: 400000,
	batch_id,
	total: 212,
	skipped_providers: [],
	...zero,
	linked: 23
});

describe('doneLineFor', () => {
	it("shows this kind's last run and nothing for the other kind, idle, or dismissed", () => {
		expect(doneLineFor(idle(summary('person')), 'person', '')?.linked).toBe(23);
		expect(doneLineFor(idle(summary('studio')), 'person', '')).toBeNull();
		expect(doneLineFor(idle(), 'person', '')).toBeNull();
		expect(doneLineFor(running('person'), 'person', '')).toBeNull();
		expect(doneLineFor(idle(summary('person', 'b1')), 'person', 'b1')).toBeNull();
		expect(doneLineFor(idle(summary('person', 'b2')), 'person', 'b1')?.batch_id).toBe('b2');
	});
});

describe('sweepFinished', () => {
	it('fires once on the running→idle edge of this kind only', () => {
		const edge = { sawRunning: false };
		expect(sweepFinished(edge, idle(), 'person')).toBe(false); // mounted idle: nothing
		expect(sweepFinished(edge, running('studio'), 'person')).toBe(false); // other kind
		expect(sweepFinished(edge, idle(summary('studio')), 'person')).toBe(false);
		expect(sweepFinished(edge, running('person'), 'person')).toBe(false);
		expect(sweepFinished(edge, running('person'), 'person')).toBe(false); // still running
		expect(sweepFinished(edge, idle(summary('person')), 'person')).toBe(true); // the edge
		expect(sweepFinished(edge, idle(summary('person')), 'person')).toBe(false); // once
	});

	it('never fires for a page that mounted after the sweep finished', () => {
		const edge = { sawRunning: false };
		expect(sweepFinished(edge, idle(summary('person')), 'person')).toBe(false);
		expect(sweepFinished(edge, undefined, 'person')).toBe(false);
	});
});

describe('skippedText', () => {
	it('groups by reason and truncates to three names', () => {
		expect(skippedText([{ provider: 'tmdb', reason: 'stopped responding' }])).toBe(
			'tmdb stopped responding'
		);
		expect(
			skippedText([
				{ provider: 'a', reason: 'rate-limited' },
				{ provider: 'tmdb', reason: 'stopped responding' },
				{ provider: 'b', reason: 'rate-limited' },
				{ provider: 'c', reason: 'rate-limited' },
				{ provider: 'd', reason: 'rate-limited' }
			])
		).toBe('a, b, c and 1 more rate-limited · tmdb stopped responding');
	});
});
