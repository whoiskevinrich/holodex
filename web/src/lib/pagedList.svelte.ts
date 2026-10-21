// A server-paged list (F80): People, Studios, Tags and Films load one page, then append
// pages on Load more. `start` is the offset the loaded run begins at — 0, or an A–Z
// letter's offset (EL6) — so the run is always [start, start + items.length) of the
// server's order. Every fetch carries a sequence number, and a reply that a newer load
// has overtaken is dropped, so a fast sort/search change never shows the older answer.
import { tick, untrack } from 'svelte';
import { beforeNavigate } from '$app/navigation';
import { toMessage } from '#lib/format.ts';
import { listScroll } from '#lib/listScroll.svelte.ts';

export interface PageResult<T> {
	items: T[];
	total: number;
	letters?: Record<string, number>;
}

/**
 * Reads one page. Called at fetch time, so it sees the page's current sort/search.
 * `fresh` is true for a load (not a Load more): the one read that asks for letters.
 */
export type PageFetcher<T> = (
	offset: number,
	limit: number,
	fresh: boolean
) => Promise<PageResult<T>>;

/**
 * Move focus to the index-th entry's link inside `root` once the last page has landed
 * and the Load more button that had focus is gone (handoff, Accessibility).
 */
function focusEntry(root: HTMLElement | undefined, index: number) {
	root?.querySelectorAll<HTMLElement>('a[href]')[index]?.focus();
}

/**
 * The nav box's in-place search, settled a short pause after typing (EL5: each search
 * now asks the server). Call during component init.
 */
export function settledQuery(raw: () => string, delayMs = 250) {
	const s = $state({ value: untrack(() => raw().trim()) });
	$effect(() => {
		const next = raw().trim();
		const t = setTimeout(() => (s.value = next), next ? delayMs : 0);
		return () => clearTimeout(t);
	});
	return s;
}

/**
 * Wire a list page to its PagedList (call during component init):
 * - reading `restartOn` inside an effect, any change starts the list over at the top;
 * - the first load after a Back restores the saved run, then the scroll (EL7);
 * - leaving the page saves the run and scroll under listScroll[id], keyed by `key()`.
 * Returns the A–Z jump and Load more handlers, both with the handoff's focus moves;
 * `root()` is the element holding the entries' links.
 */
export function pagedListPage<T>(o: {
	id: string;
	list: PagedList<T>;
	key: () => string;
	restartOn: () => void;
	root: () => HTMLElement | undefined;
}) {
	let firstLoad = true;
	$effect(() => {
		o.restartOn();
		untrack(async () => {
			const snap = firstLoad ? listScroll.take(o.id, o.key()) : null;
			firstLoad = false;
			if (!snap) return o.list.load();
			await o.list.restore(snap);
			await tick();
			window.scrollTo(0, snap.scrollY);
		});
	});

	beforeNavigate(() => {
		listScroll.save(o.id, {
			key: o.key(),
			scrollY: window.scrollY,
			...o.list.snapshot()
		});
	});

	return {
		// Reload from the letter, bring the list top under the sticky bar, focus its first entry.
		async jumpTo(letter: string) {
			await o.list.jump(letter);
			await tick();
			const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
			o.root()?.scrollIntoView({
				behavior: reduce ? 'auto' : 'smooth',
				block: 'start'
			});
			focusEntry(o.root(), 0);
		},
		async loadMore() {
			const first = await o.list.loadMore();
			if (first >= 0 && !o.list.hasMore) {
				await tick();
				focusEntry(o.root(), first);
			}
		}
	};
}

// The server clamps a page at 1000 (repo.MaxListLimit); a Back restore never asks for more.
const MAX_LIMIT = 1000;

export class PagedList<T> {
	items = $state<T[]>([]);
	total = $state(0);
	letters = $state<Record<string, number>>({});
	start = $state(0);
	loading = $state(true);
	loadingMore = $state(false);
	error = $state('');
	/** The polite live-region text: what the last Load more added (handoff, Accessibility). */
	announce = $state('');
	/** The A–Z letter the run was restarted at (EL6), or '' for the start of the list. */
	letter = $state('');

	#seq = 0;
	/** The last replace was count(): entries come later, via loadMore(). */
	#counted = false;
	readonly #fetch: PageFetcher<T>;
	readonly pageSize: number;

	constructor(fetch: PageFetcher<T>, pageSize = 50) {
		this.#fetch = fetch;
		this.pageSize = pageSize;
	}

	get left(): number {
		return Math.max(this.total - this.start - this.items.length, 0);
	}

	get hasMore(): boolean {
		return this.left > 0;
	}

	/**
	 * Replace the list with `count` entries from `start` (a fresh page, a letter restart,
	 * or a Back restore of everything that was loaded). A failure keeps nothing stale on
	 * screen: the error replaces the list.
	 */
	load({
		start = 0,
		count = this.pageSize,
		letter = ''
	}: { start?: number; count?: number; letter?: string } = {}) {
		return this.#replace(start, Math.min(Math.max(count, this.pageSize), MAX_LIMIT), letter);
	}

	/**
	 * Read only the total and leave the list empty, for entries that are shown later but
	 * already count toward "N left" (the /tags All tab's categories, EL8). The first
	 * loadMore() then reads their first page.
	 */
	count() {
		return this.#replace(0, 1, '', true);
	}

	async #replace(start: number, limit: number, letter: string, countOnly = false) {
		const mine = ++this.#seq;
		this.#counted = countOnly;
		this.loading = true;
		this.error = '';
		this.announce = '';
		this.letter = letter;
		try {
			const res = await this.#fetch(start, limit, !countOnly);
			if (mine !== this.#seq) return;
			this.items = countOnly ? [] : (res.items ?? []);
			this.total = res.total ?? 0;
			this.letters = res.letters ?? {};
			this.start = start;
		} catch (e) {
			if (mine !== this.#seq) return;
			this.items = [];
			this.total = 0;
			this.error = toMessage(e);
		} finally {
			if (mine === this.#seq) this.loading = false;
		}
	}

	/**
	 * Re-read the same run (same start, same count) after a mutation, keeping its size.
	 * A list that was only counted stays only counted.
	 */
	refresh() {
		if (this.#counted && this.items.length === 0) return this.count();
		return this.load({
			start: this.start,
			count: this.items.length,
			letter: this.letter
		});
	}

	/** Restart the run at letter L's first entry; '' goes back to the start (EL6). */
	jump(L: string) {
		return this.load({ start: L ? (this.letters[L] ?? 0) : 0, letter: L });
	}

	/** What Back needs to rebuild this run (listScroll's `loaded` and `start`). */
	snapshot(): { loaded: number; start: number } {
		return { loaded: this.items.length, start: this.start };
	}

	/** Rebuild a run saved by snapshot(): the same entries from the same offset (EL7). */
	async restore(s: { loaded?: number; start?: number }) {
		const start = s.start ?? 0;
		const seq = this.#seq + 1;
		await this.load({ start, count: s.loaded ?? this.pageSize });
		// Only when this restore's load is still the latest (a sort change may overtake it).
		if (start > 0 && seq === this.#seq)
			this.letter = Object.keys(this.letters).find((L) => this.letters[L] === start) ?? '';
	}

	/**
	 * Append the next page. A failure keeps the loaded entries and sets `error` (shown
	 * above the list); the button comes back for a retry. Returns the index of the first
	 * appended entry, or -1 when nothing was appended.
	 */
	async loadMore(): Promise<number> {
		if (this.loadingMore || !this.hasMore) return -1;
		const mine = this.#seq;
		const firstNew = this.items.length;
		this.loadingMore = true;
		this.error = '';
		try {
			const res = await this.#fetch(this.start + firstNew, this.pageSize, false);
			if (mine !== this.#seq) return -1;
			const added = res.items ?? [];
			this.items = [...this.items, ...added];
			this.total = res.total ?? this.total;
			this.announce = `Loaded ${added.length} more. ${this.left.toLocaleString()} left.`;
			return added.length ? firstNew : -1;
		} catch (e) {
			if (mine === this.#seq) this.error = toMessage(e);
			return -1;
		} finally {
			this.loadingMore = false;
		}
	}
}
