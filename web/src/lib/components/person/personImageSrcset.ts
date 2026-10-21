import type { PersonImageWidth } from '#lib/types.ts';

// The `?w=` tiers the person image routes serve (HOLODEX-332, images.md "Person image width
// variants"). A tier is the output width, so each `w` descriptor below is exact.
const TIERS: PersonImageWidth[] = [160, 320, 640];

// Stored originals are capped at 2000px on the longest side (PersonImageMaxDimension), so 2000w
// stands in for the original when the caller doesn't know its real width.
const ORIGINAL_MAX_WIDTH = 2000;

// A `srcset` over the tiers plus the original as the largest candidate. `url` builds the
// image's URL at a tier, or the original with no width. A tier at or above a known original
// width is dropped: the server would only serve the original under it.
export function personImageSrcset(
	url: (width?: PersonImageWidth) => string,
	originalWidth = ORIGINAL_MAX_WIDTH
): string {
	return [
		...TIERS.filter((w) => w < originalWidth).map((w) => `${url(w)} ${w}w`),
		`${url()} ${originalWidth}w`
	].join(', ');
}
