import { describe, expect, it, vi } from 'vitest';

vi.mock('$app/navigation', () => ({ beforeNavigate: () => {} }));
const { PagedList } = await import('./pagedList.svelte.ts');

// A 7-entry "server": offsets into ['a'..'g'], letters B at 1 and E at 4.
const ALL = ['a', 'b', 'c', 'd', 'e', 'f', 'g'];
function server() {
	return vi.fn(async (offset: number, limit: number, fresh: boolean) => ({
		items: ALL.slice(offset, offset + limit),
		total: ALL.length,
		letters: fresh ? { A: 0, B: 1, E: 4 } : undefined
	}));
}

// F80: one page, then Load more appends; a Back restore rebuilds the run from the
// server.
describe('PagedList', () => {
	it('pages through every entry once and announces each page', async () => {
		const fetch = server();
		const list = new PagedList(fetch, 3);
		await list.load();
		expect(list.items).toEqual(['a', 'b', 'c']);
		expect(list.left).toBe(4);
		expect(await list.loadMore()).toBe(3);
		expect(list.announce).toBe('Loaded 3 more. 1 left.');
		await list.loadMore();
		expect(list.items).toEqual(ALL);
		expect(list.hasMore).toBe(false);
		expect(await list.loadMore()).toBe(-1);
		expect(fetch).toHaveBeenCalledTimes(3);
		expect(fetch.mock.calls.map((c) => c[2])).toEqual([true, false, false]);
	});

	it('counts without loading, then Load more reads the first page (HOLODEX-589)', async () => {
		const fetch = server();
		const list = new PagedList(fetch, 3);
		await list.count();
		expect(list.items).toEqual([]);
		expect(list.left).toBe(7);
		expect(fetch.mock.calls[0]).toEqual([0, 1, false]);
		await list.refresh(); // a counted list stays counted
		expect(list.items).toEqual([]);
		expect(fetch.mock.calls[1]).toEqual([0, 1, false]);
		expect(await list.loadMore()).toBe(0);
		expect(list.items).toEqual(['a', 'b', 'c']);
		expect(list.left).toBe(4);
	});

	it('restores a saved run', async () => {
		const list = new PagedList(server(), 2);
		await list.restore({ loaded: 4 });
		expect(list.items).toEqual(['a', 'b', 'c', 'd']);
		expect(list.snapshot()).toEqual({ loaded: 4 });
	});

	it('keeps the loaded entries when a Load more fails', async () => {
		const fetch = server();
		const list = new PagedList(fetch, 3);
		await list.load();
		fetch.mockRejectedValueOnce(new Error('offline'));
		expect(await list.loadMore()).toBe(-1);
		expect(list.items).toEqual(['a', 'b', 'c']);
		expect(list.error).toBe('offline');
		expect(list.hasMore).toBe(true);
	});

	it('drops an older load that answers after a newer one', async () => {
		let release: () => void = () => {};
		const fetch = vi.fn(async (offset: number, limit: number) => {
			if (limit === 2) await new Promise<void>((r) => (release = r));
			return { items: ALL.slice(offset, offset + limit), total: ALL.length };
		});
		const list = new PagedList(fetch, 2);
		const slow = list.load();
		await list.load({ count: 4 });
		release();
		await slow;
		expect(list.items).toEqual(['a', 'b', 'c', 'd']);
	});
});
