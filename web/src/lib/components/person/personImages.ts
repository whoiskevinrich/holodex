// The person image-set read, cached for the session, plus the compare strip's slice
// rule. Sibling to `personCard.svelte.ts` and deliberately the same contract: F68
// gave `/people/{id}/card` one request per person per session, but
// `api.getPersonImages` had no cache of its own — so the F70 compare panel re-fetched
// the image set on *every* reopen while the card was fetched once, and P0-5 held for
// only half the payload. The browser's network log is what caught it.
//
// Module-level, so every panel on the page shares one entry per person.
import { api } from '$lib/api';
import type { PersonImage, PersonImageSet } from '$lib/types';

const cache = new Map<number, Promise<PersonImageSet>>();

/** One request per person per session. A REJECTED promise is evicted, so a failed
 *  side's Retry re-requests rather than replaying the failure (the `loadPersonCard`
 *  rule — a failure is never cached). */
export function loadPersonImages(id: number): Promise<PersonImageSet> {
	let p = cache.get(id);
	if (!p) {
		p = api.getPersonImages(id).catch((err) => {
			if (cache.get(id) === p) cache.delete(id);
			throw err;
		});
		cache.set(id, p);
	}
	return p;
}

/** Slots 2–5 of the F70 compare strip (P0-3). */
export const STRIP_GALLERY_SLOTS = 4;

/** The compare strip's gallery slots: everything but the headshot — slot 1 already
 *  serves that role, and `role` is the only thing that tells the two apart — capped,
 *  in the `sort_order` the API returned. **There is no `+N`**: the strip is a sample,
 *  not an index (OQ2), so a person with 50 images and a person with 5 look the same
 *  here on purpose. */
export function stripGallery(set: PersonImageSet): PersonImage[] {
	return set.gallery.filter((img) => img.role !== 'headshot').slice(0, STRIP_GALLERY_SLOTS);
}
