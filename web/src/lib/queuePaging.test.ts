import { describe, expect, it } from 'vitest';
import { appendUnique, nextPage, QUEUE_PAGE, windowLimit } from './queuePaging';

describe('nextPage', () => {
	it('starts after the loaded rows that are still queued', () => {
		expect(nextPage(50)).toEqual({ offset: 50, limit: QUEUE_PAGE });
	});

	it('steps back after a removal, so the row that slid up is not skipped', () => {
		// 50 loaded, one resolved: the server's row 50 is now at index 49.
		expect(nextPage(49)).toEqual({ offset: 49, limit: QUEUE_PAGE });
	});

	it('over-fetches by the rows that may have moved later', () => {
		expect(nextPage(50, 3)).toEqual({ offset: 47, limit: QUEUE_PAGE + 3 });
	});

	it('never goes below zero', () => {
		expect(nextPage(2, 5)).toEqual({ offset: 0, limit: QUEUE_PAGE + 2 });
	});
});

describe('appendUnique', () => {
	it('appends new items and drops repeats, keeping the loaded copy', () => {
		const loaded = [{ id: 1, v: 'old' }, { id: 2, v: 'old' }];
		const next = [{ id: 2, v: 'new' }, { id: 3, v: 'new' }];
		expect(appendUnique(loaded, next, (x) => String(x.id))).toEqual([
			{ id: 1, v: 'old' },
			{ id: 2, v: 'old' },
			{ id: 3, v: 'new' }
		]);
	});
});

describe('windowLimit', () => {
	it('re-reads at least a page and at most the server cap', () => {
		expect(windowLimit(3)).toBe(QUEUE_PAGE);
		expect(windowLimit(120)).toBe(120);
		expect(windowLimit(5000)).toBe(1000);
	});
});
