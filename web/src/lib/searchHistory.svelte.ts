// Recent search history (QW1). Client-only: a most-recent-first list of past search
// queries kept in localStorage so the owner can re-run a search without retyping its
// syntax. No backend, no sync — purely a local convenience. Mirrors the defensive
// localStorage pattern in theme.svelte.ts.
const KEY = 'holodex-search-history';
const CAP = 10;

function read(): string[] {
	if (typeof localStorage === 'undefined') return [];
	try {
		const raw = localStorage.getItem(KEY);
		if (!raw) return [];
		const parsed = JSON.parse(raw);
		// Defensive: only accept an array of strings; anything else → empty.
		if (!Array.isArray(parsed)) return [];
		return parsed.filter((q): q is string => typeof q === 'string').slice(0, CAP);
	} catch {
		// Malformed JSON or storage error — treat as empty, never throw into the UI.
		return [];
	}
}

class SearchHistory {
	items = $state<string[]>([]);

	// Load the saved list; call once on mount.
	init() {
		this.items = read();
	}

	// Record a submitted query: trim, skip empties, case-insensitive dedupe with
	// move-to-top, cap at CAP (evicting the oldest).
	record(query: string) {
		const q = query.trim();
		if (!q) return;
		const lower = q.toLowerCase();
		const next = [q, ...this.items.filter((e) => e.toLowerCase() !== lower)].slice(0, CAP);
		this.items = next;
		this.persist();
	}

	remove(query: string) {
		this.items = this.items.filter((e) => e !== query);
		this.persist();
	}

	clear() {
		this.items = [];
		if (typeof localStorage === 'undefined') return;
		try {
			localStorage.removeItem(KEY);
		} catch {
			// Storage unavailable — history is best-effort, so swallow.
		}
	}

	private persist() {
		if (typeof localStorage === 'undefined') return;
		try {
			localStorage.setItem(KEY, JSON.stringify(this.items));
		} catch {
			// Storage full/unavailable — history is best-effort, so swallow.
		}
	}
}

export const searchHistory = new SearchHistory();
