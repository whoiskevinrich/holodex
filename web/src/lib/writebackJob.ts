// Waiting on a queued writeback job (F30/ADR-048, HOLODEX-214). The writeback
// POST answers 202 the moment the job is enqueued — nothing has been written yet,
// and the file baseline `in_sync` is computed against is still the pre-write one.
// A caller that refetches on that answer gets pre-write state back and reports
// the file as still out of sync, so it has to wait for a terminal status first.
//
// No I/O and no Svelte — the status fetcher is injected — so it unit-tests in
// isolation, like the other pure modules here.

export interface WritebackJobState {
	status: string;
	error?: string;
}

// Longest we hold a caller waiting. A large MKV remux behind a busy queue can
// outrun this; on timeout we stop waiting and let the caller refetch anyway —
// what it then shows is the true current state rather than a stale one, which is
// the property that was actually broken.
export const JOB_POLL_TIMEOUT_MS = 120_000;
// Backoff rather than a fixed interval: a small MP4 tag write lands in well
// under a second, while a job queued behind a merge can take minutes (every job
// copies the whole file first, ADR-041). Starting tight answers the common case
// promptly; growing to a 5s ceiling keeps the slow case to ~20 requests instead
// of ~170.
const POLL_START_MS = 250;
const POLL_MAX_MS = 5_000;
const POLL_GROWTH = 1.5;

export interface WaitOptions {
	startMs?: number;
	timeoutMs?: number;
	// Returns true once the caller has gone away (e.g. the dialog unmounted), to
	// stop polling something nobody is waiting on.
	cancelled?: () => boolean;
}

// Shared backoff-polling loop behind waitForWritebackJob and waitForWritebackBatch
// (HOLODEX-239) — both wait on a server-side status by re-fetching with growing
// delay until `isSettled` says so, or until cancelled/timed out. A failed *fetch*
// never counts as settled (it stays out of `last` entirely) so a transient error
// can't be mistaken for a status the poller never actually saw — the caller is
// unaffected by our inability to read it, so we keep polling until the cap. An
// answer that carries an HTTP status is a real server refusal (e.g. an owner
// session that expired mid-poll) and rethrows immediately instead of waiting out
// the cap; duck-typed to keep this module free of an $lib/api import — a
// ReauthError has no status and is swallowed on purpose, since it already kicked
// off a top-level reload.
async function pollUntilSettled<T>(
	fetchStatus: () => Promise<T>,
	isSettled: (state: T) => boolean,
	opts: WaitOptions
): Promise<T | null> {
	const deadline = Date.now() + (opts.timeoutMs ?? JOB_POLL_TIMEOUT_MS);
	const cancelled = opts.cancelled ?? (() => false);
	let delay = opts.startMs ?? POLL_START_MS;
	let last: T | null = null;

	while (Date.now() < deadline) {
		if (cancelled()) return last;
		try {
			const state = await fetchStatus();
			last = state;
			if (isSettled(state)) return state;
		} catch (e) {
			if (e !== null && typeof e === 'object' && 'status' in e) throw e;
		}
		await new Promise((resolve) => setTimeout(resolve, delay));
		delay = Math.min(delay * POLL_GROWTH, POLL_MAX_MS);
	}
	return last;
}

// Resolves once the job leaves pending/running, or once we stop waiting. Throws
// on 'failed' so the caller can surface the queue's own error.
export async function waitForWritebackJob(
	jobId: number,
	jobStatus: (jobId: number) => Promise<WritebackJobState>,
	opts: WaitOptions = {}
): Promise<void> {
	const state = await pollUntilSettled(
		() => jobStatus(jobId),
		(s) => s.status !== 'pending' && s.status !== 'running',
		opts
	);
	if (state?.status === 'failed') throw new Error(state.error || 'write failed');
}

// BatchStatus is GET /writeback/batches/{batchID}/status's shape (HOLODEX-239,
// ADR-077 D3) — aggregate counts across every job sharing a batchID.
export interface BatchStatus {
	pending: number;
	running: number;
	done: number;
	failed: number;
}

// Waits on a batch the same way waitForWritebackJob waits on a single job, but
// differs where a batch must: it resolves with the final counts (the caller
// needs the summary to render) and never throws on failed > 0 — a batch's
// enqueue failures are logged and non-fatal (spec P0), so "done" is
// pending+running reaching zero, not the absence of any failure. On timeout, or
// if cancelled before any successful fetch, it resolves with the last-known
// counts (or all-zero) rather than hanging or throwing — a 50+-video batch can
// legitimately outrun the poll cap.
export async function waitForWritebackBatch(
	batchId: string,
	batchStatus: (batchId: string) => Promise<BatchStatus>,
	opts: WaitOptions = {}
): Promise<BatchStatus> {
	const state = await pollUntilSettled(
		() => batchStatus(batchId),
		(s) => s.pending === 0 && s.running === 0,
		opts
	);
	return state ?? { pending: 0, running: 0, done: 0, failed: 0 };
}
