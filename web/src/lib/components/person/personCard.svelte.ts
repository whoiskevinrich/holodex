// Hover-card plumbing for PersonLinkChip (F68, HOLODEX-431): the per-session
// card cache and the "one card open app-wide" latch. Module-level so every chip
// on a page shares both — twelve chips in a row swap one card, and a person
// hovered twice costs one request (spec R2/R6).
import type { PersonCard } from '$lib/types';
import { api } from '$lib/api';

/** Hover-intent delay before a card opens (RD9). Keyboard focus skips it. */
export const OPEN_DELAY_MS = 250;
/** Grace after the pointer leaves chip + card before the card closes (RD9). */
export const CLOSE_GRACE_MS = 150;
/** Gap between the trigger and the card, and the viewport gutter the clamp keeps (R3). */
export const CARD_GAP_PX = 6;
export const VIEWPORT_GUTTER_PX = 16;

// One request per person per session; a failed request is not cached so the
// next hover retries (R6). The flight is shared and reference-counted: two chips
// for the same person share one request, and a chip that leaves before it lands
// only *releases* its interest — the request is aborted when the last waiter
// goes, never out from under another chip still waiting on it.
interface Flight {
	promise: Promise<PersonCard>;
	controller: AbortController | null; // null once settled
	waiters: number;
}
const cache = new Map<number, Flight>();

export interface CardLease {
	promise: Promise<PersonCard>;
	/** Call when this chip no longer wants the result (it left before the card landed). */
	release(): void;
}

export function loadPersonCard(id: number): CardLease {
	let flight = cache.get(id);
	if (!flight) {
		const controller = new AbortController();
		const f: Flight = { promise: null as unknown as Promise<PersonCard>, controller, waiters: 0 };
		f.promise = api
			.personCard(id, controller.signal)
			.then((card) => {
				f.controller = null;
				return card;
			})
			.catch((err) => {
				if (cache.get(id) === f) cache.delete(id);
				throw err;
			});
		cache.set(id, f);
		flight = f;
	}
	const f = flight;
	f.waiters++;
	let released = false;
	return {
		promise: f.promise,
		release() {
			if (released) return;
			released = true;
			f.waiters--;
			if (f.waiters <= 0 && f.controller) f.controller.abort();
		}
	};
}

/** After the ring refreshed the person (F65.8), the next open must re-read. */
export function invalidatePersonCard(id: number): void {
	cache.delete(id);
}

/** Which chip's card is open — at most one app-wide (R2). Keyed by a per-mount token,
 * not the person id, so two chips for the same person still open one at a time. */
export const hoverCard = $state<{ openToken: symbol | null }>({ openToken: null });

/** True when the device cannot hover: the chip stays a plain link (RD9). Read once
 * per mount; a hybrid device that changes pointers mid-session is out of scope. */
export function cannotHover(): boolean {
	return typeof window !== 'undefined' && window.matchMedia('(pointer: coarse)').matches;
}

export interface Placement {
	above: boolean;
	/** Negative x shift, px, so the card's right edge stays inside the viewport gutter. */
	shiftX: number;
}

/** One measure decides placement (RD10, OQ1 probe): flip above when the card would
 * not fit below, clamp horizontally rather than end-align. `cardW`/`cardH` are
 * the card's rendered size; `trigger` is the chip's rect. */
export function placeCard(
	trigger: DOMRect,
	cardW: number,
	cardH: number,
	viewportW: number,
	viewportH: number
): Placement {
	const spaceBelow = viewportH - trigger.bottom;
	const above = spaceBelow < cardH + CARD_GAP_PX && trigger.top > spaceBelow;
	const right = trigger.left + cardW;
	const shiftX = -Math.max(0, right - (viewportW - VIEWPORT_GUTTER_PX));
	return { above, shiftX: Math.max(shiftX, -(trigger.left - VIEWPORT_GUTTER_PX)) || 0 };
}
