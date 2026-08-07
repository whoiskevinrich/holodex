import type { Tag } from './types';

// findTagByName resolves a typed parent name against an already-loaded tag list —
// exact, case-insensitive, excluding the tag being edited (F50, ADR-075 D1 P1-2).
// Shared by the /tags list's Manage-mode parent control and tags/[id]'s Parent
// control so both surfaces reject a typo the same way instead of silently
// creating a bogus parent.
export function findTagByName(tags: Tag[], name: string, excludeId?: number): Tag | undefined {
	const trimmed = name.trim().toLowerCase();
	return tags.find((t) => t.id !== excludeId && t.name.toLowerCase() === trimmed);
}

// cycleMessage is the ADR-075 D1 server-side cycle guard's display copy — shared
// so every parent control surfaces the identical message.
export function cycleMessage(name: string): string {
	return `Can't set ${name} as its own ancestor.`;
}
