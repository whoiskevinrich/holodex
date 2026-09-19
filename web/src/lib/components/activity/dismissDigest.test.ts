import { describe, expect, it } from 'vitest';
import type { JobDigest, JobRun } from '$lib/types';
import { totalFailures, withoutFailures, withoutRun } from './dismissDigest';

// The digest's local mutation after a dismiss (HOLODEX-416). The server applies
// the same rules (ADR-100 D3); these pin the client copy so the row leaves, the
// count drops and the badge mutes without a refetch — and so the next refetch
// finds nothing to disagree with.
function run(id: number, kind: string, started_at: string): JobRun {
	return {
		id,
		kind,
		started_at,
		trigger: 'manual',
		status: 'error',
		finished_at: started_at,
		duration_ms: 1,
		seen: 0,
		added: 0,
		updated: 0,
		removed: 0,
		skipped: 0,
		errors: 1,
		error_message: 'boom'
	};
}

const digest = (): JobDigest => ({
	kinds: [
		{
			kind: 'writeback',
			runs: 4,
			errors: 2,
			last_run: '2026-09-18T10:00:00Z',
			last_status: 'error',
			last_dismissed: false
		},
		{
			kind: 'enrich',
			runs: 3,
			errors: 1,
			last_run: '2026-09-18T09:00:00Z',
			last_status: 'ok',
			last_dismissed: false
		}
	],
	failures: [
		run(30, 'writeback', '2026-09-18T10:00:00Z'),
		run(20, 'writeback', '2026-09-18T08:00:00Z'),
		run(10, 'enrich', '2026-09-18T07:00:00Z')
	]
});

describe('totalFailures', () => {
	it('sums kinds[].errors, never the capped failures length', () => {
		const d = digest();
		d.kinds[0].errors = 73;
		expect(totalFailures(d)).toBe(74);
		expect(d.failures.length).toBe(3);
	});
});

describe('withoutRun', () => {
	it('removes the row and decrements that kind only', () => {
		const next = withoutRun(digest(), run(20, 'writeback', '2026-09-18T08:00:00Z'));
		expect(next.failures.map((f) => f.id)).toEqual([30, 10]);
		expect(next.kinds[0].errors).toBe(1);
		expect(next.kinds[1].errors).toBe(1);
		expect(totalFailures(next)).toBe(2);
	});

	it("mutes the badge only when the dismissed run is the kind's newest (D5)", () => {
		const older = withoutRun(digest(), run(20, 'writeback', '2026-09-18T08:00:00Z'));
		expect(older.kinds[0].last_dismissed).toBe(false);

		const newest = withoutRun(digest(), run(30, 'writeback', '2026-09-18T10:00:00Z'));
		expect(newest.kinds[0].last_dismissed).toBe(true);
		expect(newest.kinds[0].last_status).toBe('error'); // still the newest run's truth
	});

	it('does not mute a kind whose newest run is ok, even when dismissing its older error', () => {
		const next = withoutRun(digest(), run(10, 'enrich', '2026-09-18T07:00:00Z'));
		expect(next.kinds[1]).toMatchObject({ errors: 0, last_status: 'ok', last_dismissed: false });
	});

	it('never goes below zero and leaves the input untouched', () => {
		const d = digest();
		d.kinds[1].errors = 0;
		const next = withoutRun(d, run(10, 'enrich', '2026-09-18T07:00:00Z'));
		expect(next.kinds[1].errors).toBe(0);
		expect(d.failures.length).toBe(3);
		expect(d.kinds[0].errors).toBe(2);
	});
});

describe('withoutFailures', () => {
	it('empties the list, zeroes every kind and mutes only the kinds whose newest run failed', () => {
		const next = withoutFailures(digest());
		expect(next.failures).toEqual([]);
		expect(totalFailures(next)).toBe(0);
		expect(next.kinds[0]).toMatchObject({ errors: 0, last_status: 'error', last_dismissed: true });
		expect(next.kinds[1]).toMatchObject({ errors: 0, last_status: 'ok', last_dismissed: false });
	});
});
