import { describe, expect, it } from 'vitest';
import type { Tag } from '#lib/types.ts';
import { SUGGESTION_CAP, namesTag, sortTagsByStatus, suggestTags, tagStatusRank } from './tagInput';

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

	// HOLODEX-612: /search stamps an alias-only hit with its aliases.
	const sf = tag(6, 'science fiction', { aliases: [{ id: 1, alias: 'scifi' }] });

	it('suggests a tag by its alias', () => {
		expect(suggestTags([sf], 'scifi', new Set()).map((t) => t.name)).toEqual(['science fiction']);
		expect(suggestTags([sf], 'SCI', new Set()).map((t) => t.name)).toEqual(['science fiction']);
	});

	it('ranks alias hits after name prefixes and before name substrings', () => {
		const rows = [tag(7, 'scifi western'), tag(8, 'retro scifi'), sf];
		expect(suggestTags(rows, 'scifi', new Set()).map((t) => t.name)).toEqual([
			'scifi western',
			'science fiction',
			'retro scifi'
		]);
	});

	it('drops an alias hit that no longer matches the text', () => {
		expect(suggestTags([sf], 'scifx', new Set())).toEqual([]);
	});

	it('keeps a name hit whose dashed alias holds the text', () => {
		const dashed = tag(9, 'science fiction', { aliases: [{ id: 2, alias: 'science-fiction' }] });
		expect(suggestTags([dashed], 'science-fic', new Set()).map((t) => t.name)).toEqual(['science fiction']);
	});

	it('caps the list', () => {
		const many = Array.from({ length: 20 }, (_, i) => tag(i, `tag ${String(i).padStart(2, '0')}`));
		expect(suggestTags(many, 'tag', new Set())).toHaveLength(SUGGESTION_CAP);
	});
});

describe('namesTag (HOLODEX-612)', () => {
	const sf = tag(1, 'science fiction', { aliases: [{ id: 1, alias: 'scifi' }] });

	it('is true for the exact name or an exact alias, ignoring case and spaces', () => {
		expect(namesTag(sf, ' Science Fiction ')).toBe(true);
		expect(namesTag(sf, 'SCIFI')).toBe(true);
	});

	it('is false for a partial name or alias', () => {
		expect(namesTag(sf, 'science')).toBe(false);
		expect(namesTag(sf, 'sci')).toBe(false);
	});
});
