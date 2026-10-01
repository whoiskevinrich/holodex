import { describe, expect, it } from 'vitest';
import { filterLabels } from './filterLabels';
import { mediaSchema } from './listState';

const parse = (qs: string) => mediaSchema.parseQuery(new URLSearchParams(qs));

describe('filterLabels', () => {
	it('names every active filter in chip order, q excluded', () => {
		const q = parse('q=noir&tag=3&person=9&year_min=2010&year_max=2019&resolution=HD&duration_min=5&genre=Drama');
		const labels = filterLabels(q, { 'tag:3': 'noir' }, (c) => (c === 'genre' ? 'Genre' : undefined));
		expect(labels.map((l) => l.label)).toEqual([
			'Person: …',
			'Tag: noir',
			'HD',
			'Duration ≥ 5 min',
			'2010–2019',
			'Genre: Drama'
		]);
		expect(labels.map((l) => l.kind)).toEqual(['scope', 'scope', 'filter', 'filter', 'filter', 'filter']);
	});

	it('clear drops just that filter', () => {
		const q = parse('tag=3&tag=4&year_max=2019&category_id=2');
		const byId = Object.fromEntries(filterLabels(q, {}).map((l) => [l.id, l]));
		expect(byId['tag:3'].clear).toEqual({ tag: [4] });
		expect(byId['category:2'].label).toBe('Category: …');
		expect(byId['year'].label).toBe('≤ 2019');
		expect(byId['year'].clear).toEqual({ year_min: undefined, year_max: undefined });
	});

	it('a bare query has no labels', () => {
		expect(filterLabels(parse(''), {})).toEqual([]);
	});
});
