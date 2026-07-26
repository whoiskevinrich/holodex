import { describe, it, expect, vi } from 'vitest';
import { waitForWritebackJob } from './writebackJob';

// Tiny timings keep these on real timers without slowing the suite.
const fast = { startMs: 1, timeoutMs: 500 };

describe('waitForWritebackJob', () => {
	it('polls until the job leaves pending/running', async () => {
		// 'done' is the only success signal the poller ever sees — the queue
		// deletes the row on success and the server synthesizes 'done' for it.
		const statuses = ['pending', 'pending', 'running', 'done'];
		const jobStatus = vi.fn(async () => ({ status: statuses.shift() ?? 'done' }));

		await waitForWritebackJob(1, jobStatus, fast);

		// It must not stop on the first non-terminal answer — that is exactly the
		// premature "applied" that left the page showing pre-write state.
		expect(jobStatus).toHaveBeenCalledTimes(4);
	});

	it("throws the queue's own error when the job fails", async () => {
		const jobStatus = vi.fn(async () => ({ status: 'failed', error: 'mkvpropedit exploded' }));
		await expect(waitForWritebackJob(1, jobStatus, fast)).rejects.toThrow('mkvpropedit exploded');
	});

	it('throws a generic error when a failed job carries no message', async () => {
		const jobStatus = vi.fn(async () => ({ status: 'failed' }));
		await expect(waitForWritebackJob(1, jobStatus, fast)).rejects.toThrow('write failed');
	});

	it('keeps polling through a failed status fetch', async () => {
		// The job is unaffected by our inability to read it, so a transient fetch
		// error must not be reported as a failed write.
		let calls = 0;
		const jobStatus = vi.fn(async () => {
			calls++;
			if (calls < 3) throw new Error('network');
			return { status: 'done' };
		});

		await expect(waitForWritebackJob(1, jobStatus, fast)).resolves.toBeUndefined();
		expect(calls).toBe(3);
	});

	it('surfaces a refusal that carries an HTTP status', async () => {
		// An expired owner session answers every poll with 401. That is not a
		// transient unreadability — waiting it out to the cap would strand the
		// operator on a dialog that then claims success.
		const jobStatus = vi.fn(async () => {
			throw Object.assign(new Error('API failed: 401'), { status: 401 });
		});
		await expect(waitForWritebackJob(1, jobStatus, fast)).rejects.toThrow('401');
		expect(jobStatus).toHaveBeenCalledTimes(1);
	});

	it('gives up at the timeout instead of hanging', async () => {
		const jobStatus = vi.fn(async () => ({ status: 'running' }));
		// Resolves rather than throwing: a write still in flight is not a failure,
		// and the caller refetching now shows the true current state.
		await expect(
			waitForWritebackJob(1, jobStatus, { startMs: 1, timeoutMs: 25 })
		).resolves.toBeUndefined();
		expect(jobStatus).toHaveBeenCalled();
	});

	it('stops polling once cancelled', async () => {
		let cancelled = false;
		const jobStatus = vi.fn(async () => {
			cancelled = true; // e.g. the dialog unmounted mid-poll
			return { status: 'running' };
		});

		await waitForWritebackJob(1, jobStatus, { ...fast, cancelled: () => cancelled });
		expect(jobStatus).toHaveBeenCalledTimes(1);
	});
});
