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

class NavSearch {
	query = $state('');
	activeTab = $state<SearchTab>('all');
	results = $state<SearchResponse | null>(null);
	loading = $state(false);
	error = $state('');

	private timer: ReturnType<typeof setTimeout> | undefined;
	private reqId = 0;

	// Called on every keystroke. Clears results immediately for an empty query (no
	// debounce needed — there's nothing to show), otherwise debounces the fetch.
	setQuery(q: string) {
		this.query = q;
		clearTimeout(this.timer);
		const term = q.trim();
		if (!term) {
			this.reqId++; // invalidate any in-flight request
			this.results = null;
			this.loading = false;
			this.error = '';
			return;
		}
		this.timer = setTimeout(() => this.run(term), DEBOUNCE_MS);
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

	// Full reset — the × control and a submitted/picked search both return to a clean slate.
	clear() {
		clearTimeout(this.timer);
		this.reqId++;
		this.query = '';
		this.activeTab = 'all';
		this.results = null;
		this.loading = false;
		this.error = '';
	}
}

export const navSearch = new NavSearch();
