import { afterEach, describe, expect, it, vi } from 'vitest';
import { SparseList, letterAt, retryRow } from './sparseList.svelte.ts';

// A 1,512-entry "server" in a fixed order: entry i is `e${i}`. Letters B at 100, M at 700.
const TOTAL = 1512;
function server() {
	return vi.fn(async (offset: number, limit: number, fresh: boolean) => ({
		items: Array.from({ length: Math.max(Math.min(limit, TOTAL - offset), 0) }, (_, k) => `e${offset + k}`),
		total: TOTAL,
		letters: fresh ? { A: 0, B: 100, M: 700 } : undefined
	}));
}
const offsets = (f: ReturnType<typeof server>) => f.mock.calls.map((c) => c[0]).sort((a, b) => a - b);
const settle = () => new Promise((r) => setTimeout(r, 0));

afterEach(() => vi.useRealTimers());

// HOLODEX-601, testing-strategy §32.1: the sparse page cache behind the whole-list scroll.
describe('SparseList', () => {
	it('reads the first page with letters, then only the missing pages of a range plus one either side', async () => {
		const fetch = server();
		const list = new SparseList(fetch);
		await list.load();
		expect(list.total).toBe(TOTAL);
		expect(list.letters).toEqual({ A: 0, B: 100, M: 700 });
		expect(fetch.mock.calls[0]).toEqual([0, 50, true]);
		list.want(700, 740, false); // pages 14 (700–749): read 13, 14, 15
		await settle();
		expect(offsets(fetch)).toEqual([0, 650, 700, 750]);
		expect(fetch.mock.calls.slice(1).every((c) => c[1] === 50 && c[2] === false)).toBe(true);
		expect(list.at(712)).toBe('e712');
		expect(list.at(300)).toBeUndefined();
	});

	it('slots pages read in any order into the same places as a read in sequence', async () => {
		const fetch = server();
		const list = new SparseList(fetch);
		await list.load();
		const order = [30, 4, 17, 0, 22, 9, 11, 26, 2];
		for (const p of order) {
			list.want(p * 50, p * 50 + 1, false);
			await settle();
			for (const i of [p * 50, p * 50 + 49]) if (i < TOTAL) expect(list.at(i)).toBe(`e${i}`);
		}
	});

	it('drops a reply from an older generation', async () => {
		let release: () => void = () => {};
		const fetch = vi.fn(async (offset: number, limit: number) => {
			if (offset === 0 && !fetch.mock.calls.slice(1).length) await new Promise<void>((r) => (release = r));
			return { items: [`gen-${fetch.mock.calls.length}-${offset}`], total: 10 };
		});
		const list = new SparseList(fetch, 1);
		const old = list.load();
		await list.load();
		release();
		await old;
		expect(list.at(0)).toBe('gen-2-0');
	});

	it('fetches only where a fling lands, and at the ceiling during a slow scroll', async () => {
		vi.useFakeTimers();
		const fetch = server();
		const list = new SparseList(fetch);
		const first = list.load();
		await vi.runAllTimersAsync();
		await first;
		fetch.mockClear();
		// A fling: 100 scroll events from the top to the bottom, 5ms apart.
		for (let k = 0; k < 100; k++) {
			const at = Math.round((k / 99) * (TOTAL - 30));
			list.want(at, at + 30, true);
			await vi.advanceTimersByTimeAsync(5);
		}
		list.want(TOTAL - 31, TOTAL - 1, false);
		await vi.runAllTimersAsync();
		expect(offsets(fetch)).toEqual([1400, 1450, 1500]);
		// A slow scroll: one entry every 20ms for a second, never settling.
		fetch.mockClear();
		for (let k = 0; k < 50; k++) {
			list.want(600 + k, 630 + k, true);
			await vi.advanceTimersByTimeAsync(20);
		}
		expect(fetch).toHaveBeenCalled();
		expect(offsets(fetch)).toContain(600);
	});

	it('keeps at most 20 pages, evicting the farthest, and reads an evicted page again', async () => {
		const fetch = server();
		const list = new SparseList(fetch);
		await list.load();
		for (let p = 0; p < 30; p += 3) {
			list.want(p * 50, p * 50, false);
			await settle();
		}
		expect(list.pages.size).toBeLessThanOrEqual(20);
		expect(list.pages.has(0)).toBe(false); // farthest from the last view (page 27)
		fetch.mockClear();
		list.want(0, 10, false);
		await settle();
		expect(offsets(fetch)).toContain(0);
	});

	it('marks a failed page, and a retry reads only that page', async () => {
		const fetch = server();
		const list = new SparseList(fetch);
		await list.load();
		fetch.mockImplementationOnce(async () => {
			throw new Error('offline');
		});
		list.want(500, 510, false); // pages 9, 10, 11; the first read (page 9) fails
		await settle();
		expect(list.stateAt(450)).toEqual({ status: 'failed', error: 'offline' });
		expect(list.at(500)).toBe('e500');
		fetch.mockClear();
		list.want(500, 510, false); // a failed page isn't re-read by scrolling
		await settle();
		expect(fetch).not.toHaveBeenCalled();
		await list.retry(9);
		expect(offsets(fetch)).toEqual([450]);
		expect(list.at(450)).toBe('e450');
	});

	it('a failed first read leaves no list and says why', async () => {
		const list = new SparseList(
			vi.fn(async () => {
				throw new Error('down');
			})
		);
		await list.load();
		expect(list.error).toBe('down');
		expect(list.total).toBe(0);
		expect(list.loading).toBe(false);
	});

	it('refreshes in place: the visible pages stay shown, the rest go, a new total is adopted', async () => {
		const fetch = server();
		const list = new SparseList(fetch);
		await list.load();
		list.want(1000, 1020, false);
		await settle();
		fetch.mockClear();
		let release: () => void = () => {};
		fetch.mockImplementation(async (offset: number, limit: number) => {
			await new Promise<void>((r) => (release = r));
			return { items: [`new${offset}`], total: TOTAL - 1, letters: undefined };
		});
		const done = list.refresh();
		expect(list.at(1000)).toBe('e1000'); // still on screen while re-read
		expect(list.pages.has(0)).toBe(false);
		expect(offsets(fetch)).toEqual([950, 1000, 1050]);
		release();
		await vi.waitFor(() => expect(list.total).toBe(TOTAL - 1));
		void done;
	});

	it('restores at an anchor: that page first, with the total and letters', async () => {
		const fetch = server();
		const list = new SparseList(fetch);
		await list.restore(1234);
		expect(fetch.mock.calls[0]).toEqual([1200, 50, true]);
		expect(list.total).toBe(TOTAL);
		expect(list.at(1234)).toBe('e1234');
	});
});

describe('letterAt', () => {
	const letters = { '#': 0, A: 3, B: 10, M: 40 };
	it.each([
		[0, '#'],
		[2, '#'],
		[3, 'A'],
		[39, 'B'],
		[40, 'M'],
		[999, 'M']
	])('entry %i is under %s', (i, L) => expect(letterAt(i, letters)).toBe(L));
	it('is empty with no letters', () => expect(letterAt(5, {})).toBe(''));
});

describe('retryRow', () => {
	it.each([
		// [page, lanes, total, row]
		[0, 1, 1512, 0],
		[1, 1, 1512, 50],
		[1, 3, 1512, 17], // page 1 starts at 50, mid-row 16 (48–50): the next row, 17 (51–53)
		[2, 3, 1512, 34], // 100 is mid-row 33 (99–101): row 34
		[3, 3, 1512, 50], // 150 starts row 50
		[1, 2, 1512, 25],
		[1, 12, 1512, 5], // 50 is mid-row 4 (48–59): row 5 (60–71)
		[30, 3, 1512, 500], // the last page, 1500–1511: 1500 starts row 500
		[0, 60, 1512, -1] // a row wider than the page never fits inside it
	])('page %i at %i lanes of %i → row %i', (page, lanes, total, row) =>
		expect(retryRow(page, 50, lanes, total)).toBe(row)
	);
});
