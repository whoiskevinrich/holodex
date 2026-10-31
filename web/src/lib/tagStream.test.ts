import { describe, expect, it } from 'vitest';
import { tagStream, type TagRow } from './tagStream.ts';
import type { Category, Tag } from './types.ts';

// F80 EL8/EL14 (HOLODEX-604): the All tab's tags-then-categories stream read by offset.
function fakeStream(nTags: number, nCats: number, failCats = false) {
	const tags = Array.from({ length: nTags }, (_, i) => ({ id: i, name: `t${i}` }) as Tag);
	const cats = Array.from({ length: nCats }, (_, i) => ({ id: i, name: `c${i}` }) as Category);
	const page = <T>(all: T[]) => (offset: number, limit: number) =>
		Promise.resolve({ items: all.slice(offset, offset + limit), total: all.length });
	return tagStream({
		tags: page(tags),
		categories: failCats ? () => Promise.reject(new Error('boom')) : page(cats),
		show: () => ({ tags: true, categories: true })
	});
}

const label = (r: TagRow) => (r.kind === 'tag' ? `t${r.tag.id}` : `c${r.cat.id}`);

describe('tagStream', () => {
	for (const nTags of [0, 49, 50, 237]) {
		for (const nCats of [1, 60]) {
			it(`tiles ${nTags} tags then ${nCats} categories in any page order`, async () => {
				const want = [
					...Array.from({ length: nTags }, (_, i) => `t${i}`),
					...Array.from({ length: nCats }, (_, i) => `c${i}`)
				];
				const pages = Math.ceil(want.length / 50);
				const inOrder = fakeStream(nTags, nCats);
				const seq: string[] = [];
				for (let p = 0; p < pages; p++) {
					const r = await inOrder.fetch(p * 50, 50, p === 0);
					expect(r.total).toBe(nTags + nCats);
					seq.push(...r.items.map(label));
				}
				expect(seq).toEqual(want);

				// Shuffled: the last page first, so the categories' start is learned mid-stream.
				const shuffled = fakeStream(nTags, nCats);
				const out: string[][] = [];
				for (const p of [...Array(pages).keys()].reverse()) {
					out[p] = (await shuffled.fetch(p * 50, 50, false)).items.map(label);
				}
				expect(out.flat()).toEqual(want);
			});
		}
	}

	it('fails only a page that holds categories when they fail', async () => {
		const s = fakeStream(120, 10, true);
		await expect(s.fetch(0, 50, true)).rejects.toThrow('boom'); // the count read
		const t = fakeStream(120, 10, true);
		await t.fetch(0, 50, true).catch(() => {});
		await expect(t.fetch(100, 50, false)).rejects.toThrow('boom');
	});

	it('reads one kind on the Tags and Categories tabs', async () => {
		const tags = [{ id: 1, name: 'a' } as Tag];
		const one = <T>(all: T[]) => () => Promise.resolve({ items: all, total: all.length });
		let show = { tags: true, categories: false };
		const s = tagStream({ tags: one(tags), categories: () => Promise.reject(new Error('no')), show: () => show });
		expect((await s.fetch(0, 50, true)).items.map(label)).toEqual(['t1']);
		show = { tags: false, categories: true };
		await expect(s.fetch(0, 50, true)).rejects.toThrow('no');
	});
});
