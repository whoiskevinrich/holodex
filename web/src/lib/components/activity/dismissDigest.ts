// Local digest mutation after a dismiss (HOLODEX-416, handoff D1/D2/D5). The row
// leaves instantly and the per-kind counts follow without a refetch; the next
// loadDigest() (scan idle, tab switch) agrees with the server because the server
// applied the same rules (ADR-100 D3). Pure so the arithmetic is unit-testable
// without a component harness.
import type { JobDigest, JobKindDigest, JobRun } from '$lib/types';

// The true failure count is the sum of each kind's error count, which is always
// on the wire — the inline `failures` list is capped (digestFailureCap), so its
// length under-reports after a bad batch.
export function totalFailures(digest: JobDigest): number {
	return digest.kinds.reduce((n, k) => n + k.errors, 0);
}

// One row dismissed: it leaves the failure list, its kind's error count drops by
// one, and — if it was that kind's newest run — the kind's badge mutes (D5). The
// kinds table is keyed by kind, not by run id, so the match is on `run.kind`.
export function withoutRun(digest: JobDigest, run: JobRun): JobDigest {
	return {
		failures: digest.failures.filter((f) => f.id !== run.id),
		kinds: digest.kinds.map((k) =>
			k.kind === run.kind
				? {
						...k,
						errors: Math.max(0, k.errors - 1),
						last_dismissed:
							k.last_dismissed || (k.last_status === 'error' && k.last_run === run.started_at)
					}
				: k
		)
	};
}

// Dismiss all: every undismissed error in the window is gone, so every kind's
// error count is zero and any kind whose newest run failed now reads as
// dismissed. The failure list empties, which unmounts the callout.
export function withoutFailures(digest: JobDigest): JobDigest {
	return {
		failures: [],
		kinds: digest.kinds.map(
			(k): JobKindDigest => ({ ...k, errors: 0, last_dismissed: k.last_status === 'error' })
		)
	};
}
