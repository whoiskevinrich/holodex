// Pure helpers for the media page's owner tag block (HOLODEX-519,
// docs/design/media-tag-input-handoff.md): the chip order and the add input's suggestions.
import type { Tag } from '#lib/types.ts';

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

// The first of the tag's aliases containing the typed text, if any (HOLODEX-612). /search
// stamps every tag hit with its aliases.
export function matchedAlias(tag: Tag, query: string): string | undefined {
	const q = query.trim().toLowerCase();
	if (!q) return undefined;
	return tag.aliases?.find((a) => a.alias.toLowerCase().includes(q))?.alias;
}

// Whether the text is exactly the tag's name or one of its aliases: adding it would
// attach this tag, not create one.
export function namesTag(tag: Tag, text: string): boolean {
	const q = text.trim().toLowerCase();
	return tag.name.toLowerCase() === q || !!tag.aliases?.some((a) => a.alias.toLowerCase() === q);
}

// Existing tags matching the typed text: name prefix matches, then alias matches, then
// name substring matches, each in name order, leaving out tags already on the video.
// Empty for a blank query.
export function suggestTags(all: readonly Tag[], query: string, exclude: ReadonlySet<number>): Tag[] {
	const q = query.trim().toLowerCase();
	if (!q) return [];
	const prefix: Tag[] = [];
	const alias: Tag[] = [];
	const inner: Tag[] = [];
	for (const t of all) {
		if (exclude.has(t.id)) continue;
		const at = t.name.toLowerCase().indexOf(q);
		if (at === 0) prefix.push(t);
		else if (matchedAlias(t, q)) alias.push(t);
		else if (at > 0) inner.push(t);
	}
	const byName = (a: Tag, b: Tag) => a.name.localeCompare(b.name);
	return [...prefix.sort(byName), ...alias.sort(byName), ...inner.sort(byName)].slice(0, SUGGESTION_CAP);
}
