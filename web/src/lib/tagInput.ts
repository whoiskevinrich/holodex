// Pure helpers for the media page's owner tag block (HOLODEX-519,
// docs/design/media-tag-input-handoff.md): the chip order and the add input's suggestions.
import type { Tag } from '$lib/types';

// Pending writeback changes first (D4): add, removal, on the file, Holodex only. The same
// `written` × `on_file` truth table TagLinkChip draws its glyph from; with either field
// unknown there is no glyph, so the tag ranks last and the list falls back to name order.
export function tagStatusRank(tag: Tag): number {
	if (tag.written === undefined || tag.on_file === undefined) return 3;
	if (tag.written && !tag.on_file) return 0;
	if (!tag.written && tag.on_file) return 1;
	if (tag.written && tag.on_file) return 2;
	return 3;
}

export function sortTagsByStatus(tags: readonly Tag[]): Tag[] {
	return [...tags].sort((a, b) => tagStatusRank(a) - tagStatusRank(b) || a.name.localeCompare(b.name));
}

export const SUGGESTION_CAP = 8;

// Existing tags matching the typed text: prefix matches, then substring matches, each in
// name order, leaving out tags already on the video. Empty for a blank query.
export function suggestTags(all: readonly Tag[], query: string, exclude: ReadonlySet<number>): Tag[] {
	const q = query.trim().toLowerCase();
	if (!q) return [];
	const prefix: Tag[] = [];
	const inner: Tag[] = [];
	for (const t of all) {
		if (exclude.has(t.id)) continue;
		const at = t.name.toLowerCase().indexOf(q);
		if (at === 0) prefix.push(t);
		else if (at > 0) inner.push(t);
	}
	const byName = (a: Tag, b: Tag) => a.name.localeCompare(b.name);
	return [...prefix.sort(byName), ...inner.sort(byName)].slice(0, SUGGESTION_CAP);
}
