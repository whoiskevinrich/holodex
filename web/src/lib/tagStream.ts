// The Tags page's Posters and Table views as one whole-list stream (F80 EL8/EL14,
// HOLODEX-604): on the All tab, the tags in the chosen sort and then the categories in
// name order, as one list of offsets, so the whole-list scroll can read any page of it.
// The tags' total says where the categories start. A page that straddles the boundary
// reads the last tags and the first categories, and a page past it reads categories
// alone. The Tags and Categories tabs read their own kind only.
import type { PageFetcher } from '#lib/pagedList.svelte.ts';
import type { Category, Tag } from '#lib/types.ts';

export type TagRow = { kind: 'tag'; tag: Tag } | { kind: 'category'; cat: Category };

type Read<T> = (offset: number, limit: number) => Promise<{ items?: T[] | null; total: number }>;

const asTag = (tag: Tag): TagRow => ({ kind: 'tag', tag });
const asCat = (cat: Category): TagRow => ({ kind: 'category', cat });

/**
 * A PageFetcher over the stream. `show` says which kinds the current tab holds. The two
 * totals are remembered between pages, so a page of categories needn't re-read the tags;
 * call `reset()` before re-reading after a write, and on any restart.
 */
export function tagStream(o: {
	tags: Read<Tag>;
	categories: Read<Category>;
	show: () => { tags: boolean; categories: boolean };
}) {
	let tagTotal: number | undefined;
	let catTotal: number | undefined;

	const fetch: PageFetcher<TagRow> = async (offset, limit) => {
		const show = o.show();
		if (!show.categories) {
			const r = await o.tags(offset, limit);
			return { items: (r.items ?? []).map(asTag), total: r.total };
		}
		if (!show.tags) {
			const r = await o.categories(offset, limit);
			return { items: (r.items ?? []).map(asCat), total: r.total };
		}
		let tags: Tag[] = [];
		if (tagTotal === undefined || offset < tagTotal) {
			const r = await o.tags(offset, limit);
			tagTotal = r.total;
			tags = r.items ?? [];
		}
		// The slots of this page that fall past the last tag.
		const catFrom = Math.max(offset - tagTotal, 0);
		const catSlots = offset + limit - Math.max(offset, tagTotal);
		let cats: Category[] = [];
		if (catSlots > 0) {
			const r = await o.categories(catFrom, catSlots);
			catTotal = r.total;
			cats = r.items ?? [];
		} else if (catTotal === undefined) {
			catTotal = (await o.categories(0, 1)).total;
		}
		return { items: [...tags.map(asTag), ...cats.map(asCat)], total: tagTotal + catTotal };
	};

	return {
		fetch,
		reset() {
			tagTotal = undefined;
			catTotal = undefined;
		}
	};
}
