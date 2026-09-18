// Per-candidate thumbnail slot state for the Enrich picker (F64, HOLODEX-406).
// Pure, like candidateDetail.ts: the component keeps a per-row `failed` map (an
// <img> that fired `error`) and asks this module which branch the slot renders, so
// the rule is unit-testable without a DOM (this repo has no component-test harness).
import { isHttpUrl } from '$lib/format';

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
