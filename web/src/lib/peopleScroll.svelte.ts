// People-list scroll preservation (mirrors browse.svelte.ts / ADR-032). Holodex is an
// SPA (ssr=false): opening a person destroys the list page, and SvelteKit resets scroll
// to the top for in-app (push) navigation — so pressing ← Back lands you at the top of a
// long A–Z list instead of where you were. This module-scoped cache survives client
// navigation (the JS module isn't torn down), stashing the window scroll offset keyed by
// the active sort so returning to the list restores your position once it re-renders.
// Session-scoped only: a full reload starts fresh.
let saved: { sort: string; scrollY: number } | null = null;

export const peopleScroll = {
	save(s: { sort: string; scrollY: number }) {
		saved = s;
	},
	// One-shot: return the saved offset iff it was captured under the same sort (a sort
	// change reorders the list, invalidating the position), then clear it so a later
	// fresh visit doesn't reuse a stale offset.
	take(sort: string): number | null {
		const s = saved;
		saved = null;
		return s && s.sort === sort ? s.scrollY : null;
	}
};
