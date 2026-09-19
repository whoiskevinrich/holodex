// Per-candidate thumbnail slot state for the Enrich picker (F64, HOLODEX-406).
// Pure, like candidateDetail.ts: the component keeps a per-row `failed` map (an
// <img> that fired `error`) and asks this module which branch the slot renders, so
// the rule is unit-testable without a DOM (this repo has no component-test harness).
import { isHttpUrl } from '$lib/format';
import type { EnrichEntityKind } from '$lib/types';

/** The slice of a candidate the slot reads. */
export interface ImageCandidate {
	image_url?: string;
}

/**
 * Whether the slot shows the provider's image (true) or the label monogram (false).
 * The server already dropped anything off the allowlist, but the client still gates
 * through isHttpUrl() — format.ts's standing convention for any provider-supplied URL,
 * the same belt-and-suspenders profile_url gets. A row whose image errored (404,
 * refused decode, offline) falls back to the monogram and never shows the browser's
 * broken-image glyph.
 */
export function showThumb(c: ImageCandidate, failed: boolean): boolean {
	return !failed && !!c.image_url && isHttpUrl(c.image_url);
}

/** The slot's box shape — one per entity kind the picker can be opened for (HOLODEX-414). */
export type SlotShape = 'portrait' | 'landscape' | 'logo';

/**
 * Which box a picker's candidate slot draws, by the entity being matched: a person or
 * film row compares against a headshot / poster (2:3), a video row against the file's
 * own landscape thumbnail (so the provider sends a backdrop), a studio row against a
 * logo. Adding a kind means adding a shape here — never an `{#if}` in the template.
 */
export function slotShape(entityType: EnrichEntityKind): SlotShape {
	switch (entityType) {
		case 'video':
			return 'landscape';
		case 'studio':
			return 'logo';
		default:
			return 'portrait';
	}
}

/**
 * Explicit width AND height per shape, always 60 px tall from `sm` up so every collapsed
 * row is 76 px in every picker. Deliberately not `aspect-*`: an aspect box with a `w-full`
 * `<img>` inside can borrow the image's natural width (a w300 backdrop is 300 px), and
 * the geometry harness asserts the slot's px exactly. 108 (not 106.67) for 16:9 keeps
 * that assertion integer; the image `object-contain`s with a sub-pixel plate sliver.
 *
 * Below `sm` (a phone, or a 315 px dialog in a narrow pane) the two wide boxes drop to
 * 80 wide at the same aspect — 80 × 45 landscape, 80 × 40 logo — because at that width a
 * 120 px box left the name ~30 px beside "Strong match" (QA §4.4, 2026-09-18). The
 * portrait box is already the narrowest and keeps 40 × 60 everywhere. The picker pairs
 * this with stacking the match strength under the name below `sm`.
 */
export const SLOT_CLASS: Record<SlotShape, string> = {
	portrait: 'w-10 h-15',
	landscape: 'w-20 h-11.25 sm:w-27 sm:h-15',
	logo: 'w-20 h-10 sm:w-30 sm:h-15'
};
