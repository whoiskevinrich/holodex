// The data half of the whole-list scroll on People and Studios (F80 EL11–EL13, HOLODEX-601;
// docs/architecture/stack.md "Long lists"). The list knows its total from the first read
// and holds only the 50-entry pages that have been asked for, so any index can be shown
// before the pages above it arrive. It reads through PagedList's fetcher, so the server is
// unchanged. A sort, search or reshuffle starts a new generation: replies from an older
// one are dropped, the same guard as PagedList's sequence number.
import { toMessage } from '#lib/format.ts';
import type { PageFetcher } from '#lib/pagedList.svelte.ts';

export type PageState<T> =
	| { status: 'loading' }
	| { status: 'loaded'; items: T[] }
	| { status: 'failed'; error: string };

export interface SparseSnapshot<T> {
	pages: Map<number, PageState<T>>;
	total: number;
}

/** The cache keeps at most this many pages, evicting the one farthest from view. */
const MAX_PAGES = 20;
/** During a continuous scroll, fetch at least this often if the view has barely moved. */
const CEILING_MS = 300;

export class SparseList<T> {
	total = $state(0);
	letters = $state<Record<string, number>>({});
	/** The first read of a generation is in flight. */
	loading = $state(true);
	/** The first read of a generation failed: there is no list to show. */
	error = $state('');
	pages = $state.raw(new Map<number, PageState<T>>());

	#gen = 0;
	/** A restore hasn't landed yet: the view's asks are recorded, not fetched. */
	#held = false;
	/** The last range the view asked for, as [first, last] entry indexes. */
	#want: [number, number] = [0, 0];
	#ceiling: ReturnType<typeof setTimeout> | undefined;
	readonly #fetch: PageFetcher<T>;
	readonly pageSize: number;

	constructor(fetch: PageFetcher<T>, pageSize = 50) {
		this.#fetch = fetch;
		this.pageSize = pageSize;
	}

	pageOf(index: number): number {
		return Math.floor(index / this.pageSize);
	}

	/** The entry at `index`, or undefined while its page is arriving, failed or not asked for. */
	at(index: number): T | undefined {
		const p = this.pages.get(this.pageOf(index));
		return p?.status === 'loaded' ? p.items[index % this.pageSize] : undefined;
	}

	/** The state of the page holding `index` (undefined: not asked for yet). */
	stateAt(index: number): PageState<T> | undefined {
		return this.pages.get(this.pageOf(index));
	}

	/** Start over: forget every page and read the first one, with letters. */
	load() {
		this.#held = false; // a restart overtakes a restore still landing
		return this.#start(0);
	}

	/**
	 * Start over at a saved position (EL7): its page first, which also brings the total.
	 * Asks from the view are held until `land()`: until the page has scrolled to the anchor,
	 * the view still shows the top, and fetching that would waste two reads. `from` is pages
	 * the list held when the page was left (the Media grid, HOLODEX-604): they show at once
	 * and stay on screen while the anchor's page is re-read.
	 */
	restore(anchor: number, from?: SparseSnapshot<T>) {
		this.#held = true;
		return this.#start(Math.max(anchor, 0), from);
	}

	/** The loaded pages and total, for `restore(anchor, from)` after a Back. */
	snapshot(): SparseSnapshot<T> {
		const pages = new Map<number, PageState<T>>();
		for (const [p, s] of this.pages) if (s.status === 'loaded') pages.set(p, s);
		return { pages, total: this.total };
	}

	/** The restore has landed: fetch what the view now shows. */
	land() {
		this.#held = false;
		return this.#ensure(...this.#want);
	}

	/**
	 * Re-read what's in view after a mutation (EL9): keep the position, the total and the
	 * visible entries on screen until the replies replace them, and drop every other page.
	 */
	refresh() {
		this.#held = false; // a re-read of the view ends a restore's hold, as land() does
		const gen = ++this.#gen;
		const [from, to] = this.#span(...this.#want);
		const kept = new Map<number, PageState<T>>();
		for (const [p, s] of this.pages) if (p >= from && p <= to && s.status === 'loaded') kept.set(p, s);
		this.pages = kept;
		const reads: Promise<void>[] = [];
		for (let p = from; p <= to; p++) reads.push(this.#read(p, gen, false));
		return Promise.all(reads).then(() => {});
	}

	/**
	 * The view now shows entries first..last. While the view is still scrolling nothing is
	 * fetched, except at a ceiling when the view has barely moved (a slow continuous scroll);
	 * a fling across the list fetches only where it lands.
	 */
	want(first: number, last: number, scrolling: boolean) {
		this.#want = [first, last];
		if (this.#held) return;
		if (!scrolling) {
			clearTimeout(this.#ceiling);
			this.#ceiling = undefined;
			void this.#ensure(first, last);
			return;
		}
		if (this.#ceiling) return;
		const from = first;
		this.#ceiling = setTimeout(() => {
			this.#ceiling = undefined;
			const [f, l] = this.#want;
			if (Math.abs(f - from) <= l - f + 1) void this.#ensure(f, l);
			else this.want(f, l, true);
		}, CEILING_MS);
	}

	/** Read one failed page again (the retry row). */
	retry(page: number) {
		return this.#read(page, this.#gen, false);
	}

	async #start(anchor: number, from?: SparseSnapshot<T>) {
		const gen = ++this.#gen;
		clearTimeout(this.#ceiling);
		this.#ceiling = undefined;
		this.pages = new Map(from?.pages);
		if (from) this.total = from.total;
		this.loading = true;
		this.error = '';
		this.#want = [anchor, anchor];
		const page = this.pageOf(anchor);
		try {
			await this.#read(page, gen, true, true);
		} catch (e) {
			if (gen !== this.#gen) return;
			this.total = 0;
			this.error = toMessage(e);
		} finally {
			if (gen === this.#gen) this.loading = false;
		}
	}

	/** The pages covering first..last plus one page either side, within the list. */
	#span(first: number, last: number): [number, number] {
		const lastPage = this.pageOf(Math.max(this.total - 1, 0));
		return [Math.max(this.pageOf(first) - 1, 0), Math.min(this.pageOf(last) + 1, lastPage)];
	}

	/** Fetch the pages of the span that haven't been asked for. */
	#ensure(first: number, last: number) {
		const [from, to] = this.#span(first, last);
		const reads: Promise<void>[] = [];
		for (let p = from; p <= to; p++) {
			if (!this.pages.has(p)) reads.push(this.#read(p, this.#gen, false));
		}
		return Promise.all(reads).then(() => {});
	}

	/**
	 * Read page `p` for generation `gen`. A loaded page stays on screen while it is re-read.
	 * `first` rethrows, so a failed first read empties the list.
	 */
	async #read(p: number, gen: number, fresh: boolean, first = false) {
		if (this.pages.get(p)?.status !== 'loaded') this.#set(p, { status: 'loading' });
		try {
			const res = await this.#fetch(p * this.pageSize, this.pageSize, fresh);
			if (gen !== this.#gen) return;
			this.total = res.total ?? 0;
			if (fresh) this.letters = res.letters ?? {};
			this.#set(p, { status: 'loaded', items: res.items ?? [] });
			this.#evict();
		} catch (e) {
			if (gen !== this.#gen) return;
			if (first) {
				this.#delete(p);
				throw e;
			}
			this.#set(p, { status: 'failed', error: toMessage(e) });
		}
	}

	#set(p: number, s: PageState<T>) {
		this.pages = new Map(this.pages).set(p, s);
	}

	#delete(p: number) {
		const next = new Map(this.pages);
		next.delete(p);
		this.pages = next;
	}

	#evict() {
		if (this.pages.size <= MAX_PAGES) return;
		const mid = this.pageOf((this.#want[0] + this.#want[1]) / 2);
		const far = [...this.pages.keys()].sort((a, b) => Math.abs(b - mid) - Math.abs(a - mid));
		const next = new Map(this.pages);
		for (const p of far.slice(0, this.pages.size - MAX_PAGES)) next.delete(p);
		this.pages = next;
	}
}

/** The letter of the entry at `index` in name order: the last letter starting at or before it. */
export function letterAt(index: number, letters: Record<string, number>): string {
	let best = '';
	let at = -1;
	for (const [L, offset] of Object.entries(letters)) {
		if (offset <= index && offset > at) {
			best = L;
			at = offset;
		}
	}
	return best;
}

/**
 * The grid row that carries a failed page's retry row (handoff, "A page that fails"): the
 * first row lying wholly inside the page, so it never covers a neighbouring page's
 * entries. -1 when the page holds no whole row (only when lanes exceed the page).
 */
export function retryRow(page: number, pageSize: number, lanes: number, total: number): number {
	const first = page * pageSize;
	const end = Math.min(first + pageSize, total); // exclusive
	const row = Math.ceil(first / lanes);
	const rowEnd = Math.min((row + 1) * lanes, total);
	return row * lanes < end && rowEnd <= end ? row : -1;
}
