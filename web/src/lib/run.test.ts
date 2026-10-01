import { describe, expect, it } from 'vitest';
import {
	loadRun,
	neighbours,
	nextPass,
	parseRunParam,
	runHref,
	saveRun,
	setMode,
	shuffle,
	startRun,
	type Run,
	type RunSource
} from './run';

// testing-strategy §23.4: the run module is pure and fully covered here, because
// web/ has no component harness for the strip that drives it.

const ids20 = Array.from({ length: 20 }, (_, i) => i + 1);
const source: RunSource = {
	kind: 'query',
	query: 'tag=3',
	label: { kind: 'Tag', name: 'noir', href: '/tags/3' }
};
const sorted = (a: number[]) => a.slice().sort((x, y) => x - y);

describe('shuffle', () => {
	it('is a permutation and leaves its input alone', () => {
		const input = ids20.slice();
		const out = shuffle(input, 1);
		expect(sorted(out)).toEqual(ids20);
		expect(input).toEqual(ids20);
	});

	it('is stable per seed and differs across seeds', () => {
		expect(shuffle(ids20, 42)).toEqual(shuffle(ids20, 42));
		expect(shuffle(ids20, 42)).not.toEqual(shuffle(ids20, 43));
	});

	it('passes empty and single lists through', () => {
		expect(shuffle([], 7)).toEqual([]);
		expect(shuffle([9], 7)).toEqual([9]);
	});

	it('is unbiased: each value lands in each position about equally often', () => {
		// Catches the classic off-by-one (j = rand·i instead of rand·(i+1)), which never
		// leaves an item where it started.
		const n = 5;
		const trials = 20000;
		const counts = Array.from({ length: n }, () => new Array(n).fill(0));
		for (let s = 0; s < trials; s++) {
			shuffle([0, 1, 2, 3, 4], s).forEach((v, pos) => counts[v][pos]++);
		}
		const expected = trials / n;
		for (const row of counts) for (const c of row) expect(Math.abs(c - expected) / expected).toBeLessThan(0.05);
	});
});

describe('startRun', () => {
	it('plays the source order in order, a permutation shuffled', () => {
		const inOrder = startRun({ id: 'a', source, ids: ids20, mode: 'in-order', seed: 5 });
		expect(inOrder.order).toEqual(ids20);
		expect(inOrder.repeat).toBe(false);
		const shuffled = startRun({ id: 'b', source, ids: ids20, mode: 'shuffled', seed: 5 });
		expect(sorted(shuffled.order)).toEqual(ids20);
		expect(shuffled.order).not.toEqual(ids20);
		expect(shuffled.ids).toEqual(ids20);
	});

	it('gives two presses two orders, each the whole set once (P0-11)', () => {
		const a = startRun({ id: 'a', source, ids: ids20, mode: 'shuffled', seed: 100 });
		const b = startRun({ id: 'b', source, ids: ids20, mode: 'shuffled', seed: 101 });
		expect(a.order).not.toEqual(b.order);
		expect(new Set(a.order).size).toBe(20);
		expect(new Set(b.order).size).toBe(20);
	});
});

describe('neighbours', () => {
	it('walks the play order and reports a non-member as -1', () => {
		expect(neighbours([10, 20, 30], 20)).toEqual({ index: 1, prev: 10, next: 30 });
		expect(neighbours([10, 20, 30], 10)).toEqual({ index: 0, prev: null, next: 20 });
		expect(neighbours([10, 20, 30], 30)).toEqual({ index: 2, prev: 20, next: null });
		expect(neighbours([10, 20, 30], 99)).toEqual({ index: -1, prev: null, next: null });
	});
});

describe('setMode (RD11)', () => {
	const run = startRun({ id: 'a', source, ids: ids20, mode: 'in-order', seed: 1 });
	const current = 6; // index 5: 1..5 have played

	it('shuffling on keeps the history and the current item, permutes the rest', () => {
		const on = setMode(run, current, 'shuffled', 9);
		expect(on.order.slice(0, 6)).toEqual([1, 2, 3, 4, 5, 6]);
		expect(sorted(on.order.slice(6))).toEqual(ids20.slice(6));
		expect(on.order.slice(6)).not.toEqual(ids20.slice(6));
		expect(on.mode).toBe('shuffled');
	});

	it('shuffling off restores the rest to source order', () => {
		const on = setMode(run, current, 'shuffled', 9);
		const off = setMode(on, current, 'in-order', 0);
		expect(off.order).toEqual(ids20);
		expect(off.mode).toBe('in-order');
	});

	it('off then on gives a new order', () => {
		const on1 = setMode(run, current, 'shuffled', 9);
		const on2 = setMode(setMode(on1, current, 'in-order', 0), current, 'shuffled', 10);
		expect(on2.order.slice(6)).not.toEqual(on1.order.slice(6));
	});

	it('never replays history before the pass ends, across two toggles', () => {
		const shuffled = startRun({ id: 'a', source, ids: ids20, mode: 'shuffled', seed: 3 });
		const cur = shuffled.order[7];
		const t1 = setMode(shuffled, cur, 'in-order', 0);
		const t2 = setMode(t1, cur, 'shuffled', 4);
		const history = shuffled.order.slice(0, 7);
		// Previous walks exactly what played, in the order it played.
		expect(t2.order.slice(0, 7)).toEqual(history);
		expect(t2.order[7]).toBe(cur);
		for (const id of history) expect(t2.order.indexOf(id)).toBeLessThan(7);
		expect(new Set(t2.order).size).toBe(20);
	});

	it('reorders the whole list when the current item is not in the run', () => {
		const on = setMode(run, 999, 'shuffled', 9);
		expect(sorted(on.order)).toEqual(ids20);
		expect(on.order).not.toEqual(ids20);
	});
});

describe('nextPass (RD12)', () => {
	it('reads the fresh source, so a video added mid-pass is in the next pass', () => {
		const run = startRun({ id: 'a', source, ids: [1, 2, 3], mode: 'in-order', seed: 1 });
		const next = nextPass(run, [1, 2, 3, 4], 3, 2);
		expect(next.order).toEqual([1, 2, 3, 4]);
		expect(next.ids).toEqual([1, 2, 3, 4]);
	});

	it('reshuffles a shuffled run under the new seed', () => {
		const run = startRun({ id: 'a', source, ids: ids20, mode: 'shuffled', seed: 1 });
		const next = nextPass(run, ids20, run.order[19], 2);
		expect(next.seed).toBe(2);
		expect(sorted(next.order)).toEqual(ids20);
	});

	it('never starts a pass with the item that just ended', () => {
		const run = startRun({ id: 'a', source, ids: [1, 2], mode: 'shuffled', seed: 1 });
		for (let s = 0; s < 200; s++) {
			expect(nextPass(run, [1, 2], 1, s).order[0]).toBe(2);
			expect(nextPass(run, [1, 2], 2, s).order[0]).toBe(1);
		}
		const big = startRun({ id: 'a', source, ids: ids20, mode: 'shuffled', seed: 1 });
		for (let s = 0; s < 500; s++) expect(nextPass(big, ids20, 7, s).order[0]).not.toBe(7);
	});

	it('replays a one-item run (it has nothing else to start with)', () => {
		const run = startRun({ id: 'a', source, ids: [5], mode: 'shuffled', seed: 1 });
		expect(nextPass(run, [5], 5, 2).order).toEqual([5]);
	});

	it('plays 7 items of a 3-item shuffled run with no back-to-back repeat', () => {
		let run = startRun({ id: 'a', source, ids: [1, 2, 3], mode: 'shuffled', seed: 11 });
		const played: number[] = [];
		let i = 0;
		let seed = 12;
		while (played.length < 7) {
			played.push(run.order[i]);
			if (i === run.order.length - 1) {
				run = nextPass(run, [1, 2, 3], run.order[i], seed++);
				i = 0;
			} else i++;
		}
		for (let k = 1; k < played.length; k++) expect(played[k]).not.toBe(played[k - 1]);
	});
});

describe('URL', () => {
	it('round-trips a query source through runHref and parseRunParam', () => {
		const run = startRun({
			id: 'abc123',
			source: { ...source, query: 'person=12&q=noir', sort: 'title_asc' },
			ids: [1],
			mode: 'in-order',
			seed: 1
		});
		const url = new URL('http://h' + runHref(9, run));
		expect(url.pathname).toBe('/media/9');
		expect(parseRunParam(url)).toEqual({ run: 'abc123', from: 'person=12&q=noir', sort: 'title_asc' });
	});

	it('round-trips a playlist source, seed included', () => {
		const run = startRun({
			id: 'p1',
			source: { kind: 'playlist', id: 3, seed: 42, label: { kind: '', name: 'Mix', href: '/playlists/3' } },
			ids: [1],
			mode: 'in-order',
			seed: 1
		});
		expect(parseRunParam(new URL('http://h' + runHref(9, run)))).toEqual({ run: 'p1', playlist: 3, seed: 42 });
	});

	it('still reads an F69 ?playlist= link with no run id', () => {
		expect(parseRunParam(new URL('http://h/media/5?playlist=3'))).toEqual({ playlist: 3 });
		expect(parseRunParam(new URL('http://h/media/5?playlist=3&seed=42'))).toEqual({ playlist: 3, seed: 42 });
	});

	it('is null outside a run and drops a malformed id or seed', () => {
		expect(parseRunParam(new URL('http://h/media/5'))).toBeNull();
		expect(parseRunParam(new URL('http://h/media/5?playlist=abc'))).toBeNull();
		expect(parseRunParam(new URL('http://h/media/5?run=abc'))).toBeNull();
		expect(parseRunParam(new URL('http://h/media/5?playlist=3&seed=x'))).toEqual({ playlist: 3 });
		expect(parseRunParam(new URL('http://h/media/5?from=tag%3D1&run=BAD!'))).toEqual({ from: 'tag=1' });
	});

	it('accepts an empty query (bare browse is "everything")', () => {
		expect(parseRunParam(new URL('http://h/media/5?run=a1&from='))).toEqual({ run: 'a1', from: '' });
	});
});

describe('storage', () => {
	function memory(): Storage {
		const m = new Map<string, string>();
		return {
			getItem: (k) => m.get(k) ?? null,
			setItem: (k, v) => void m.set(k, v),
			removeItem: (k) => void m.delete(k),
			clear: () => m.clear(),
			key: (i) => [...m.keys()][i] ?? null,
			get length() {
				return m.size;
			}
		};
	}

	it('round-trips a run', () => {
		const s = memory();
		const run: Run = startRun({ id: 'r1', source, ids: ids20, mode: 'shuffled', seed: 8, repeat: true });
		saveRun(s, run);
		expect(loadRun(s, 'r1')).toEqual(run);
	});

	it('is null for a missing or corrupt run, and survives a throwing storage', () => {
		const s = memory();
		expect(loadRun(s, 'nope')).toBeNull();
		s.setItem('holodex:run:bad', '{not json');
		expect(loadRun(s, 'bad')).toBeNull();
		const throwing = {
			getItem: () => {
				throw new Error('blocked');
			},
			setItem: () => {
				throw new Error('blocked');
			}
		} as unknown as Storage;
		expect(loadRun(throwing, 'r1')).toBeNull();
		expect(() => saveRun(throwing, startRun({ id: 'r1', source, ids: [1], mode: 'in-order', seed: 1 }))).not.toThrow();
		expect(loadRun(undefined, 'r1')).toBeNull();
	});
});
