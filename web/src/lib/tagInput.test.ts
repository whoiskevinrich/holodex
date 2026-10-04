import { describe, expect, it } from 'vitest';
import type { Tag } from '$lib/types';
import { SUGGESTION_CAP, sortTagsByStatus, suggestTags, tagStatusRank } from './tagInput';

const tag = (id: number, name: string, extra: Partial<Tag> = {}): Tag => ({ id, ref: `tag:${id}`, name, ...extra });

describe('tagStatusRank (HOLODEX-519 D4)', () => {
	it('ranks pending add, pending removal, on file, Holodex only', () => {
		expect(tagStatusRank(tag(1, 'a', { written: true, on_file: false }))).toBe(0);
		expect(tagStatusRank(tag(1, 'a', { written: false, on_file: true }))).toBe(1);
		expect(tagStatusRank(tag(1, 'a', { written: true, on_file: true }))).toBe(2);
		expect(tagStatusRank(tag(1, 'a', { written: false, on_file: false }))).toBe(3);
	});

	it('ranks an unknown status last, with Holodex only', () => {
		expect(tagStatusRank(tag(1, 'a'))).toBe(3);
		expect(tagStatusRank(tag(1, 'a', { written: true }))).toBe(3);
	});
});

describe('sortTagsByStatus', () => {
	it('orders by status, then name within a status', () => {
		const tags = [
			tag(1, 'noir', { written: true, on_file: true }),
			tag(2, 'favourite', { written: false, on_file: false }),
			tag(3, 'heist', { written: true, on_file: false }),
			tag(4, '1940s', { written: true, on_file: true }),
			tag(5, 'b-movie', { written: false, on_file: true })
		];
		expect(sortTagsByStatus(tags).map((t) => t.name)).toEqual(['heist', 'b-movie', '1940s', 'noir', 'favourite']);
	});

	it('falls back to name order when the file tags are unknown', () => {
		expect(sortTagsByStatus([tag(1, 'zeta'), tag(2, 'alpha')]).map((t) => t.name)).toEqual(['alpha', 'zeta']);
	});

	it('does not mutate its input', () => {
		const tags = [tag(1, 'b'), tag(2, 'a')];
		sortTagsByStatus(tags);
		expect(tags.map((t) => t.name)).toEqual(['b', 'a']);
	});
});

describe('suggestTags', () => {
	const all = [tag(1, 'new wave'), tag(2, 'art necessity'), tag(3, 'neo-noir'), tag(4, 'noir'), tag(5, 'network')];

	it('puts prefix matches before substring matches, each in name order', () => {
		expect(suggestTags(all, 'ne', new Set()).map((t) => t.name)).toEqual([
			'neo-noir',
			'network',
			'new wave',
			'art necessity'
		]);
	});

	it('matches case-insensitively and ignores surrounding spaces', () => {
		expect(suggestTags(all, '  NOIR ', new Set()).map((t) => t.name)).toEqual(['noir', 'neo-noir']);
	});

	it('leaves out tags already on the video', () => {
		expect(suggestTags(all, 'ne', new Set([3, 5])).map((t) => t.name)).toEqual(['new wave', 'art necessity']);
	});

	it('is empty for a blank query', () => {
		expect(suggestTags(all, '   ', new Set())).toEqual([]);
	});

	it('caps the list', () => {
		const many = Array.from({ length: 20 }, (_, i) => tag(i, `tag ${String(i).padStart(2, '0')}`));
		expect(suggestTags(many, 'tag', new Set())).toHaveLength(SUGGESTION_CAP);
	});
});
