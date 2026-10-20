// Load-more bookkeeping for the owner queues (HOLODEX-587). Each queue page holds
// QUEUE_PAGE items, and the server returns `total` across every page.
//
// The rule that keeps an action from skipping items: the next page starts at the
// count of loaded items that are still in the server's queue at their loaded
// position. A row that Resolve, Restore or Delete removes locally has also left the
// server's list, so dropping it from `loaded` moves the next offset back by one
// automatically. Two cases need help, and the helpers below cover both:
// - A row that left the server's queue but stays on screen (an enrichment row
//   handled in place) is not counted toward the offset.
// - A row that moved later in the server's order (an extraction video that lost a
//   field and sorts lower) may have pulled an unloaded row in ahead of the offset.
//   The next page over-fetches by that many, and appendUnique drops the repeats.

export const QUEUE_PAGE = 50;

/** Where the next page starts. `moved` is the number of loaded items that may have
 *  moved later in the server's order since they were loaded. */
export function nextPage(stillQueued: number, moved = 0): { offset: number; limit: number } {
	const back = Math.min(moved, stillQueued);
	return { offset: stillQueued - back, limit: QUEUE_PAGE + back };
}

/** `loaded` followed by any of `next` not already loaded, compared by key. An item can
 *  arrive twice when the queue shifted under the offset; the loaded copy wins. */
export function appendUnique<T>(loaded: T[], next: T[], key: (item: T) => string): T[] {
	const seen = new Set(loaded.map(key));
	return [...loaded, ...next.filter((item) => !seen.has(key(item)))];
}

/** How many loaded rows to re-read when a window is refreshed in place: everything on
 *  screen, at least one page, and never more than the server's cap of 1000. */
export function windowLimit(loaded: number): number {
	return Math.min(Math.max(loaded, QUEUE_PAGE), 1000);
}
