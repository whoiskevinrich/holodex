// Nav search box live-query state (NS1, HOLODEX-249): debounced fetch against the
// existing GET /search aggregation endpoint, shared by the nav dropdown's
// SearchResultsPanel instance. A singleton — there is exactly one nav search box.
import { api } from './api';
import { toMessage } from './format';
import type { SearchResponse } from './types';

export type SearchTab = 'all' | 'people' | 'videos' | 'studios' | 'tags';

export const SEARCH_TABS: { key: SearchTab; label: string }[] = [
	{ key: 'all', label: 'All' },
	{ key: 'people', label: 'People' },
	{ key: 'videos', label: 'Videos' },
	{ key: 'studios', label: 'Studios' },
	{ key: 'tags', label: 'Tags' }
];

const DEBOUNCE_MS = 200;

// pageScopeFor maps a route's pathname to the entity type it's the in-place scope
// for (NS2) — the one list a page shows, and therefore the one tab that drives it
// in place instead of opening the overlay panel. Only exact top-level list routes
// declare a scope; a detail page (e.g. `/people/[id]`) intentionally falls through
// to null (panel-only) until NS6 gives it one.
export function pageScopeFor(pathname: string): SearchTab | null {
	switch (pathname) {
		case '/':
			return 'videos';
		case '/people':
			return 'people';
		case '/studios':
			return 'studios';
		case '/tags':
			return 'tags';
		default:
			return null;
	}
}

class NavSearch {
	query = $state('');
	activeTab = $state<SearchTab>('all');
	results = $state<SearchResponse | null>(null);
	loading = $state(false);
	error = $state('');
	// True when the active tab matches the current page's own scope (NS2) — that
	// page's grid is filtering in place, so the nav dropdown shows only the tab row
	// and there's nothing for the /search aggregation fetch below to do. Driven by
	// +layout.svelte via setInPlace, since only it knows the current route.
	inPlace = $state(false);

	private timer: ReturnType<typeof setTimeout> | undefined;
	private reqId = 0;

	// Called on every keystroke. Clears results immediately for an empty query or
	// while in-place (no debounce needed — there's nothing to show), otherwise
	// debounces the fetch.
	setQuery(q: string) {
		this.query = q;
		const term = q.trim();
		if (!term || this.inPlace) {
			this.discardPending();
			return;
		}
		clearTimeout(this.timer);
		this.timer = setTimeout(() => this.run(term), DEBOUNCE_MS);
	}

	// Called by +layout.svelte whenever the active tab starts or stops matching the
	// current page's scope. Entering in-place discards any pending/stale panel fetch
	// (NS3: typing on /people or /studios must not hit the network beyond the page's
	// own list load). Leaving in-place with text already typed fetches immediately —
	// not debounced, since a tab tap is a discrete action, not a keystroke — so the
	// overlay panel that tap just opened isn't left waiting for the next keystroke.
	setInPlace(active: boolean) {
		if (this.inPlace === active) return;
		this.inPlace = active;
		if (active) {
			this.discardPending();
		} else if (this.query.trim()) {
			clearTimeout(this.timer);
			this.run(this.query.trim());
		}
	}

	// Cancels any pending debounce/in-flight fetch and clears whatever it would have
	// shown — shared by setQuery's empty/in-place branch and setInPlace's entry branch.
	private discardPending() {
		clearTimeout(this.timer);
		this.reqId++; // invalidate any in-flight request
		this.results = null;
		this.loading = false;
		this.error = '';
	}

	private async run(term: string) {
		const id = ++this.reqId;
		this.loading = true;
		this.error = '';
		try {
			const res = await api.search(term);
			if (id !== this.reqId) return; // superseded by a newer keystroke
			this.results = res;
		} catch (e) {
			if (id !== this.reqId) return;
			this.error = toMessage(e);
			this.results = null;
		} finally {
			if (id === this.reqId) this.loading = false;
		}
	}

	// Full reset — the × control and a submitted/picked search both return to a clean
	// slate. Deliberately leaves `activeTab` untouched: on a scoped page (NS2),
	// snapping it to 'all' here would silently drop out of in-place mode without a
	// navigation to re-trigger the "default tab on load" effect that would otherwise
	// restore it. +layout.svelte's clearSearch() sets the right tab for its context
	// (the page's own scope, or 'all' off one) right after calling this.
	clear() {
		clearTimeout(this.timer);
		this.reqId++;
		this.query = '';
		this.results = null;
		this.loading = false;
		this.error = '';
	}
}

export const navSearch = new NavSearch();
