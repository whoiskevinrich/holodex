import { describe, it, expect } from 'vitest';
import { select, describe as describeEntry, entries, load } from './manifest.mjs';

/** A manifest shaped like the seeder's, small enough to reason about. */
const m = {
	entities: {
		902: {
			id: 902,
			entity: 'video',
			dimension: 'enrich',
			variant: '05',
			url: '/media/902',
			name: 'STRESS …',
			axes: { video: { people: 2, tags: 3, studios: 1, text: 'plain', namespaces: 5 } }
		},
		104: {
			id: 104,
			entity: 'video',
			dimension: 'people',
			variant: '25',
			url: '/media/104',
			name: 'STRESS …',
			axes: { video: { people: 25, tags: 3, studios: 1, text: 'plain', namespaces: 0 } }
		},
		604: {
			id: 604,
			entity: 'film',
			dimension: 'filmcast',
			variant: '25',
			url: '/films/604',
			name: 'STRESS …',
			axes: { film: { cast: 25, scenes: 2 } }
		},
		20001: {
			id: 20001,
			entity: 'person',
			dimension: 'persontext',
			variant: 'unbroken',
			url: '/people/20001',
			name: 'STRESS …',
			axes: { name: { text: 'unbroken', value: 'Aaaa…' } }
		}
	},
	by_dimension: {},
	dimensions: []
};

describe('select', () => {
	// The AC's "generalise by dimension, not by single page": one predicate over the
	// coordinate picks up every rung that satisfies it, across dimensions.
	it('selects by axis threshold across kinds', () => {
		const hit = select(m, (e) => (e.axes.video?.people ?? e.axes.film?.cast ?? 0) >= 10);
		expect(hit.map((e) => e.id)).toEqual([104, 604]);
	});

	// axes is kind-keyed — only one half is ever populated — so a predicate written
	// for videos must not blow up on the film and person entries beside them.
	it('treats a predicate that throws on another kind as not applicable', () => {
		const hit = select(m, (e) => e.axes.video.namespaces >= 5);
		expect(hit.map((e) => e.id)).toEqual([902]);
	});

	it('returns entries in id order so a report is stable between runs', () => {
		expect(entries(m).map((e) => e.id)).toEqual([104, 604, 902, 20001]);
	});

	it('selects nothing rather than everything when the coordinate is absent', () => {
		expect(select(m, (e) => e.dimension === 'gone')).toEqual([]);
	});
});

describe('describe', () => {
	it('names the dimension, the rung and the address to open', () => {
		expect(describeEntry(m.entities[104])).toBe('people/25 (video 104, /media/104)');
	});
});

describe('load', () => {
	it('says how to seed when there is no manifest', () => {
		expect(() => load('./does-not-exist.json')).toThrow(/go run \.\/testdata\/stressseed/);
	});
});
