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

// Resolves once the job leaves pending/running, or once we stop waiting. Throws
// on 'failed' so the caller can surface the queue's own error. A failed status
// *fetch* is not fatal: the job is unaffected by our inability to read it, so we
// keep polling until the cap rather than reporting a write that may well have
// succeeded as an error.
export async function waitForWritebackJob(
	jobId: number,
	jobStatus: (jobId: number) => Promise<WritebackJobState>,
	opts: WaitOptions = {}
): Promise<void> {
	const deadline = Date.now() + (opts.timeoutMs ?? JOB_POLL_TIMEOUT_MS);
	const cancelled = opts.cancelled ?? (() => false);
	let delay = opts.startMs ?? POLL_START_MS;

	while (Date.now() < deadline) {
		if (cancelled()) return;
		let state: WritebackJobState | null = null;
		try {
			state = await jobStatus(jobId);
		} catch (e) {
			// A transport failure says nothing about the job, so keep polling. But an
			// answer from the server is a real refusal — an ApiError carries the HTTP
			// status, so an owner session that expired mid-write surfaces immediately
			// instead of waiting out the cap. Duck-typed to keep this module free of
			// an $lib/api import. A ReauthError has no status and is swallowed on
			// purpose: it has already kicked off a top-level reload.
			if (e !== null && typeof e === 'object' && 'status' in e) throw e;
		}
		if (state) {
			if (state.status === 'failed') throw new Error(state.error || 'write failed');
			// Anything that isn't still queued is success — including the 'done'
			// the server synthesizes for a row it deleted on completion.
			if (state.status !== 'pending' && state.status !== 'running') return;
		}
		await new Promise((resolve) => setTimeout(resolve, delay));
		delay = Math.min(delay * POLL_GROWTH, POLL_MAX_MS);
	}
}
