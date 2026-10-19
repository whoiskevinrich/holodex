import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ServerSearch } from './serverSearch.svelte.ts';

// HOLODEX-586: the pickers' bounded server search — debounced, and an older query's
// answer never lands over a newer one's.
describe('ServerSearch', () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => vi.useRealTimers());

	it('debounces to one request for the last query', async () => {
		const run = vi.fn(async (q: string) => [q]);
		const s = new ServerSearch(run, 300);
		s.set('a');
		s.set('ab');
		expect(s.loading).toBe(true);
		await vi.advanceTimersByTimeAsync(300);
		expect(run).toHaveBeenCalledTimes(1);
		expect(run).toHaveBeenCalledWith('ab');
		expect(s.items).toEqual(['ab']);
		expect(s.answered).toBe('ab');
		expect(s.loading).toBe(false);
	});

	it('drops a slow answer to an older query', async () => {
		const resolvers: Record<string, (v: string[]) => void> = {};
		const s = new ServerSearch((q) => new Promise<string[]>((r) => (resolvers[q] = r)), 0);
		s.set('old');
		await vi.advanceTimersByTimeAsync(0);
		s.set('new');
		await vi.advanceTimersByTimeAsync(0);
		resolvers.new(['new']);
		await vi.advanceTimersByTimeAsync(0);
		resolvers.old(['old']);
		await vi.advanceTimersByTimeAsync(0);
		expect(s.items).toEqual(['new']);
		expect(s.answered).toBe('new');
	});

	it('clears on an empty query without a request', async () => {
		const run = vi.fn(async (q: string) => [q]);
		const s = new ServerSearch(run, 0);
		s.set('x');
		await vi.advanceTimersByTimeAsync(0);
		s.set('  ');
		await vi.advanceTimersByTimeAsync(0);
		expect(run).toHaveBeenCalledTimes(1);
		expect(s.items).toEqual([]);
		expect(s.answered).toBe('');
	});

	it('surfaces a failed request as an error with no rows', async () => {
		const s = new ServerSearch<string>(async () => {
			throw new Error('boom');
		}, 0);
		s.set('x');
		await vi.advanceTimersByTimeAsync(0);
		expect(s.items).toEqual([]);
		expect(s.error).toBe('boom');
		expect(s.loading).toBe(false);
	});
});
