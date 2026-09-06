import { describe, it, expect, vi } from 'vitest';
import { waitForWritebackJob, waitForWritebackBatch, waitForVideoWriteback } from './writebackJob';

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

describe('waitForWritebackBatch', () => {
	it('polls until pending+running reach zero, then resolves with the final counts', async () => {
		const answers = [
			{ pending: 3, running: 1, done: 0, failed: 0 },
			{ pending: 0, running: 2, done: 2, failed: 0 },
			{ pending: 0, running: 0, done: 4, failed: 0 }
		];
		const batchStatus = vi.fn(async () => answers.shift()!);

		const result = await waitForWritebackBatch('b1', batchStatus, fast);

		expect(batchStatus).toHaveBeenCalledTimes(3);
		expect(result).toEqual({ pending: 0, running: 0, done: 4, failed: 0 });
	});

	it('resolves (does not throw) once failed > 0 but the batch is otherwise settled', async () => {
		// Spec P0: enqueue failures are logged and non-fatal — the caller reads
		// `failed` off the resolved result, this never rejects for it.
		const batchStatus = vi.fn(async () => ({ pending: 0, running: 0, done: 3, failed: 2 }));

		await expect(waitForWritebackBatch('b1', batchStatus, fast)).resolves.toEqual({
			pending: 0,
			running: 0,
			done: 3,
			failed: 2
		});
	});

	it('keeps polling through a failed status fetch', async () => {
		let calls = 0;
		const batchStatus = vi.fn(async () => {
			calls++;
			if (calls < 3) throw new Error('network');
			return { pending: 0, running: 0, done: 1, failed: 0 };
		});

		await expect(waitForWritebackBatch('b1', batchStatus, fast)).resolves.toEqual({
			pending: 0,
			running: 0,
			done: 1,
			failed: 0
		});
		expect(calls).toBe(3);
	});

	it('surfaces a refusal that carries an HTTP status', async () => {
		const batchStatus = vi.fn(async () => {
			throw Object.assign(new Error('API failed: 401'), { status: 401 });
		});
		await expect(waitForWritebackBatch('b1', batchStatus, fast)).rejects.toThrow('401');
		expect(batchStatus).toHaveBeenCalledTimes(1);
	});

	it('gives up at the timeout and resolves with the last-known counts', async () => {
		const batchStatus = vi.fn(async () => ({ pending: 1, running: 0, done: 9, failed: 0 }));
		await expect(
			waitForWritebackBatch('b1', batchStatus, { startMs: 1, timeoutMs: 25 })
		).resolves.toEqual({ pending: 1, running: 0, done: 9, failed: 0 });
		expect(batchStatus).toHaveBeenCalled();
	});

	it('stops polling once cancelled', async () => {
		let cancelled = false;
		const batchStatus = vi.fn(async () => {
			cancelled = true;
			return { pending: 1, running: 0, done: 0, failed: 0 };
		});

		await waitForWritebackBatch('b1', batchStatus, { ...fast, cancelled: () => cancelled });
		expect(batchStatus).toHaveBeenCalledTimes(1);
	});
});

describe('waitForVideoWriteback', () => {
	it('polls until pending goes false, then resolves with the settled state', async () => {
		const answers = [
			{ pending: true, failed: false },
			{ pending: true, failed: false },
			{ pending: false, failed: false }
		];
		const fetchStatus = vi.fn(async () => answers.shift()!);

		const result = await waitForVideoWriteback(fetchStatus, fast);

		expect(fetchStatus).toHaveBeenCalledTimes(3);
		expect(result).toEqual({ pending: false, failed: false });
	});

	it('resolves (does not throw) once the write has failed', async () => {
		// Unlike waitForWritebackJob, a failure is a settled *state* for the page to
		// render as a badge, not an error to propagate — there is no dialog row left
		// to flip to "error" on this path (ADR-091).
		const fetchStatus = vi.fn(async () => ({
			pending: false,
			failed: true,
			error: "writeback rename: permission denied"
		}));

		await expect(waitForVideoWriteback(fetchStatus, fast)).resolves.toEqual({
			pending: false,
			failed: true,
			error: "writeback rename: permission denied"
		});
	});

	it('keeps polling through a failed status fetch', async () => {
		let calls = 0;
		const fetchStatus = vi.fn(async () => {
			calls++;
			if (calls < 3) throw new Error('network');
			return { pending: false, failed: false };
		});

		await expect(waitForVideoWriteback(fetchStatus, fast)).resolves.toEqual({
			pending: false,
			failed: false
		});
		expect(calls).toBe(3);
	});

	it('surfaces a refusal that carries an HTTP status', async () => {
		const fetchStatus = vi.fn(async () => {
			throw Object.assign(new Error('API failed: 401'), { status: 401 });
		});
		await expect(waitForVideoWriteback(fetchStatus, fast)).rejects.toThrow('401');
		expect(fetchStatus).toHaveBeenCalledTimes(1);
	});

	it('gives up at the timeout and resolves with the last-known state', async () => {
		const fetchStatus = vi.fn(async () => ({ pending: true, failed: false }));
		await expect(
			waitForVideoWriteback(fetchStatus, { startMs: 1, timeoutMs: 25 })
		).resolves.toEqual({ pending: true, failed: false });
		expect(fetchStatus).toHaveBeenCalled();
	});

	it('stops polling once cancelled, resolving with the last-known state', async () => {
		let cancelled = false;
		const fetchStatus = vi.fn(async () => {
			cancelled = true;
			return { pending: true, failed: false };
		});

		const result = await waitForVideoWriteback(fetchStatus, { ...fast, cancelled: () => cancelled });
		expect(fetchStatus).toHaveBeenCalledTimes(1);
		expect(result).toEqual({ pending: true, failed: false });
	});
});
