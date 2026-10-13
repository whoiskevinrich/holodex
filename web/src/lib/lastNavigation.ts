// How the current page was reached: SvelteKit's navigation type, recorded by the root layout's
// onNavigate before the new page mounts, so a page's own init and first `$effect` already see
// this arrival's type (afterNavigate can run after them and would still hold the previous one).
// The initial load fires no onNavigate, so the default is `enter`; a reload starts there too.
// Readers: navSnapshot's `take` restores only on Back/Forward or a reload (HOLODEX-573), and
// EntityVideos picks its #videos / #films anchor over a Back restore (HOLODEX-459).
export const lastNavigation = { type: 'enter' as string };

/** True when this arrival returns to where the reader was: Back/Forward, or a reload. */
export function returningArrival(): boolean {
	return lastNavigation.type === 'popstate' || lastNavigation.type === 'enter';
}
