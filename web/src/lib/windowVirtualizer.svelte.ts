// The range half of the whole-list scroll (HOLODEX-601; docs/architecture/stack.md "Long
// lists"): a runes wrapper over @tanstack/virtual-core in window mode, so the page keeps
// its own scrollbar and its sticky A–Z bar. It virtualizes GRID ROWS of `lanes` entries,
// not entries, so entry i is always in row ⌊i / lanes⌋: the retry row, A–Z and Back all
// lean on that. Every row has one height (`pitch`, the row plus its gap), which the list
// measures from a rendered row; nothing else is measured.
import { tick, untrack } from 'svelte';
import { beforeNavigate } from '$app/navigation';
import { listScroll } from '#lib/listScroll.svelte.ts';
import { retryRow, type SparseList } from '#lib/sparseList.svelte.ts';
import {
	Virtualizer,
	observeWindowOffset,
	observeWindowRect,
	windowScroll,
	type VirtualizerOptions
} from '@tanstack/virtual-core';

type Options = VirtualizerOptions<Window, Element>;

export class WindowVirtualizer {
	/** The rendered rows, overscan included: the first and the one past the last. */
	start = $state(0);
	end = $state(0);
	/** The first and last row actually in view. */
	firstVisible = $state(0);
	lastVisible = $state(0);
	scrolling = $state(false);
	rowCount = $state(0);
	pitch = $state(0);
	lanes = $state(1);
	/** The list's top in page coordinates (the core's scrollMargin). */
	margin = 0;

	#v: Virtualizer<Window, Element>;
	#opts: Options;

	constructor(estimate: number) {
		this.pitch = estimate;
		this.#opts = {
			count: 0,
			getScrollElement: () => window,
			estimateSize: () => this.pitch,
			scrollToFn: windowScroll,
			observeElementRect: observeWindowRect,
			observeElementOffset: observeWindowOffset,
			initialOffset: () => window.scrollY,
			overscan: 3,
			onChange: () => this.#sync()
		};
		this.#v = new Virtualizer(this.#opts);
	}

	/** Start observing the window; returns the cleanup. Call from an effect. */
	mount(): () => void {
		const off = this.#v._didMount();
		this.#v._willUpdate();
		this.#sync();
		return off;
	}

	get stickyTop(): number {
		return this.#opts.scrollPaddingStart ?? 0;
	}

	/** The sticky bar's height: a jump lands its row just below it. */
	setStickyTop(px: number) {
		if (px === this.#opts.scrollPaddingStart) return;
		this.#opts = { ...this.#opts, scrollPaddingStart: px };
		this.#v.setOptions(this.#opts);
		this.#sync();
	}

	/** Update what the core lays out: entry total and lanes, the measured row pitch, the list's page offset. */
	configure(c: { total: number; lanes: number; pitch?: number; margin?: number }) {
		const lanes = Math.max(c.lanes, 1);
		const rowCount = Math.ceil(c.total / lanes);
		const pitch = c.pitch && c.pitch > 0 ? c.pitch : this.pitch;
		const margin = c.margin ?? this.margin;
		if (rowCount === this.rowCount && lanes === this.lanes && pitch === this.pitch && margin === this.margin) return;
		const resize = pitch !== this.pitch || lanes !== this.lanes;
		// A view switch, resize or density step keeps the first entry in view where it was.
		const scrolledIn = resize && this.rowCount > 0 && window.scrollY > this.margin;
		const anchor = this.firstVisible * this.lanes;
		const within = window.scrollY - this.entryTop(anchor);
		this.rowCount = rowCount;
		this.lanes = lanes;
		this.pitch = pitch;
		this.margin = margin;
		this.#opts = { ...this.#opts, count: rowCount, scrollMargin: margin };
		this.#v.setOptions(this.#opts);
		if (resize) this.#v.measure();
		this.#v._willUpdate();
		if (scrolledIn) window.scrollTo(0, this.entryTop(anchor) + Math.min(within, this.pitch - 1));
		this.#sync();
	}

	/** Scroll so the row holding entry `index` sits under the sticky bars. */
	scrollToEntry(index: number, smooth = false) {
		this.#v.scrollToIndex(Math.floor(index / this.lanes), {
			align: 'start',
			behavior: smooth ? 'smooth' : 'auto'
		});
	}

	/** Bring the row holding entry `index` wholly into view, scrolling only if it isn't. */
	revealEntry(index: number) {
		this.#v.scrollToIndex(Math.floor(index / this.lanes), { align: 'auto' });
	}

	/**
	 * Scroll to entry `index`, `within` px past its row's top (by default, just below the
	 * sticky bar), once the list now rendering has measured its rows (a few frames: Back, a
	 * view switch).
	 */
	async landOn(index: number, within = -this.stickyTop) {
		for (let k = 0; k < 3; k++) await new Promise(requestAnimationFrame);
		window.scrollTo(0, this.entryTop(index) + within);
	}

	/** Where entry `index` starts, in page coordinates (Back restores against it). */
	entryTop(index: number): number {
		return this.margin + Math.floor(index / this.lanes) * this.pitch;
	}

	#sync() {
		const items = this.#v.getVirtualItems();
		this.start = items[0]?.index ?? 0;
		this.end = items.length ? items[items.length - 1].index + 1 : 0;
		// The first row at least half out from under the sticky bar: the chip and A–Z name what's
		// readable, and a row a jump landed exactly under the bar counts.
		const below = (this.#v.scrollOffset ?? 0) + this.stickyTop - this.margin;
		this.firstVisible = Math.min(Math.max(Math.round(below / this.pitch), 0), Math.max(this.rowCount - 1, 0));
		this.lastVisible = this.#v.range?.endIndex ?? 0;
		this.scrolling = this.#v.isScrolling;
	}
}

/**
 * Roving tabindex over a virtualized list (EL13, handoff "Accessibility"): the list is one
 * Tab stop, the arrow keys move entry by entry past what's drawn, and the focus target is
 * the entry's own `[data-index]` element, which keeps its node when the placeholder in it
 * turns into the entry. Enter follows the entry's link; Space ticks its checkbox (merge
 * select mode).
 */
export class RovingList {
	focus = $state(0);
	readonly #virt: WindowVirtualizer;
	readonly #total: () => number;

	constructor(virt: WindowVirtualizer, total: () => number) {
		this.#virt = virt;
		this.#total = total;
	}

	/** Track focus that arrives some other way (a click, a Tab back in). */
	focusin = (e: FocusEvent) => {
		const i = (e.target as HTMLElement).closest<HTMLElement>('[data-index]')?.dataset.index;
		if (i !== undefined) this.focus = Number(i);
	};

	keydown = (e: KeyboardEvent) => {
		const cell = (e.target as HTMLElement).closest<HTMLElement>('[data-index]');
		if (!cell || e.altKey || e.ctrlKey || e.metaKey) return;
		const v = this.#virt;
		const next = rovingMove(e.key, this.focus, v.lanes, v.lastVisible - v.firstVisible, this.#total());
		if (next !== null) {
			e.preventDefault();
			this.moveTo(cell.closest<HTMLElement>('[data-roving]')!, next);
			return;
		}
		if (e.target !== cell) return; // a control inside the entry handles its own keys
		if (e.key === 'Enter') {
			cell.querySelector<HTMLElement>('a[href]')?.click();
		} else if (e.key === ' ') {
			const box = cell.querySelector<HTMLInputElement>('input[type="checkbox"]');
			if (box) {
				e.preventDefault();
				box.click();
			}
		}
	};

	/** Move focus to entry `index`, scrolling it into view and waiting for it to be drawn. */
	moveTo(root: HTMLElement, index: number, scroll: 'reveal' | 'top' = 'reveal') {
		this.focus = index;
		if (scroll === 'top') this.#virt.scrollToEntry(index, !reducedMotion());
		else this.#virt.revealEntry(index);
		let tries = 0;
		const land = () => {
			const el = root.querySelector<HTMLElement>(`[data-index="${index}"]`);
			if (el) el.focus({ preventScroll: true });
			else if (tries++ < 30) requestAnimationFrame(land);
		};
		requestAnimationFrame(land);
	}

	/**
	 * Keep the list one Tab stop: every entry's link is out of the Tab order (the entry is
	 * the stop), and its other controls (the completeness ring) are only in it on the
	 * focused entry. Run after each render.
	 */
	syncTabStops(root: HTMLElement) {
		for (const cell of root.querySelectorAll<HTMLElement>('[data-index]')) {
			const mine = Number(cell.dataset.index) === this.focus;
			for (const c of cell.querySelectorAll<HTMLElement>('a[href], button, input')) {
				c.tabIndex = mine && c.tagName !== 'A' ? 0 : -1;
			}
		}
	}
}

/**
 * Wire a drawn list to its virtualizer (call during component init): observe the window,
 * measure the list's page offset and one row's pitch (`[data-row]`; every row has it),
 * keep the list one Tab stop, and ask the data for what's in view. The grid
 * (`shared/VirtualRows`) and the table (`sort/ListTable`'s window mode) both use it.
 */
export function windowList<T>(o: {
	list: SparseList<T>;
	virt: WindowVirtualizer;
	roving: RovingList;
	lanes: () => number;
	root: () => HTMLElement | undefined;
}) {
	const { list, virt, roving } = o;

	// The pitch is read once per layout (lanes × width), never on every render: rows are
	// equal by contract, and re-reading whichever row happens to be drawn first would let one
	// row that breaks the contract flip the pitch, and the keep-in-view scroll with it, forever.
	let measuredFor = '';
	function measure() {
		const root = o.root();
		if (!root) return;
		// A table's rows start below its header.
		const body = root instanceof HTMLTableElement ? (root.tBodies[0] ?? root) : root;
		const layout = `${o.lanes()}:${root.clientWidth}`;
		const row = layout === measuredFor ? null : root.querySelector<HTMLElement>('[data-row]');
		if (row) measuredFor = layout;
		virt.configure({
			total: list.total,
			lanes: o.lanes(),
			pitch: row?.offsetHeight,
			// Rounded: sub-pixel layout jitter would otherwise reconfigure the core every render.
			margin: Math.round(body.getBoundingClientRect().top + window.scrollY)
		});
	}

	// Untracked: mounting reads the pitch and row count, and re-mounting on each change would
	// drop the core's observers and its scrolling flag mid-drag.
	$effect(() => untrack(() => virt.mount()));

	// Re-read whenever the drawn rows, the lanes, the total or the focus change.
	$effect(() => {
		void [virt.start, virt.end, list.pages, list.total, roving.focus, o.lanes()];
		tick().then(() => {
			measure();
			const root = o.root();
			if (root) roving.syncTabStops(root);
		});
	});

	// A width change (a resize, a density step) changes a poster row's height.
	$effect(() => {
		const root = o.root();
		if (!root) return;
		const ro = new ResizeObserver(measure);
		ro.observe(root);
		return () => ro.disconnect();
	});

	// Ask for what's in view; the list waits out a fling itself.
	$effect(() => {
		const lanes = virt.lanes;
		list.want(virt.firstVisible * lanes, (virt.lastVisible + 1) * lanes - 1, virt.scrolling);
	});
}

/**
 * Wire a whole-list page to its SparseList (call during component init), the
 * `pagedListPage` of the whole-list scroll:
 * - reading `restartOn` inside an effect, any change starts the list over at the top (EL11);
 * - the first load after a Back restores the entry that was first in view, and how far into
 *   its row the page was scrolled (EL7: by index, since the row height follows the width);
 * - leaving the page saves that under listScroll[id], keyed by `key()`.
 * Returns the A–Z jump: scroll the letter's first entry under the sticky bar and focus it.
 */
export function wholeListPage<T>(o: {
	id: string;
	list: SparseList<T>;
	virt: WindowVirtualizer;
	roving: RovingList;
	key: () => string;
	restartOn: () => void;
	root: () => HTMLElement | undefined;
}) {
	const { list, virt, roving } = o;
	let firstLoad = true;
	$effect(() => {
		o.restartOn();
		untrack(async () => {
			const snap = firstLoad ? listScroll.take(o.id, o.key()) : null;
			const fresh = firstLoad;
			firstLoad = false;
			roving.focus = 0;
			if (snap?.anchor === undefined) {
				// A restart goes back to the top of the list, if the page is below it.
				if (!fresh && window.scrollY > virt.margin) window.scrollTo(0, virt.margin - virt.stickyTop);
				return list.load();
			}
			await list.restore(snap.anchor);
			await virt.landOn(snap.anchor, snap.within ?? 0);
		});
	});

	beforeNavigate(() => {
		// A list that never laid out (a failed or empty read) has no position to restore.
		if (virt.rowCount === 0) return listScroll.save(o.id, { key: o.key(), scrollY: window.scrollY });
		const anchor = virt.firstVisible * virt.lanes;
		listScroll.save(o.id, {
			key: o.key(),
			scrollY: window.scrollY,
			anchor,
			// Negative while the page is still above the list's top.
			within: window.scrollY - virt.entryTop(anchor)
		});
	});

	return {
		jumpTo(letter: string) {
			const root = o.root();
			const at = list.letters[letter];
			if (root && at !== undefined) roving.moveTo(root, at, 'top');
		}
	};
}

/** Each failed page's retry row, keyed by grid row (handoff, "A page that fails"). */
export function retryRows<T>(list: SparseList<T>, lanes: number): Map<number, number> {
	const out = new Map<number, number>();
	for (const [p, s] of list.pages) {
		if (s.status !== 'failed') continue;
		const r = retryRow(p, list.pageSize, lanes, list.total);
		if (r >= 0) out.set(r, p);
	}
	return out;
}

/** The entry kept mounted (visually hidden) because it has focus but scrolled out of range. */
export function pinnedEntry(roving: RovingList, virt: WindowVirtualizer, total: number): number {
	const row = Math.floor(roving.focus / virt.lanes);
	return roving.focus < total && (row < virt.start || row >= virt.end) ? roving.focus : -1;
}

export function reducedMotion(): boolean {
	return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

/**
 * Where a key moves the roving focus in a list of `total` entries laid out `lanes` to a row,
 * with `perPage` rows on screen. Returns the new index, or null for a key the list ignores.
 */
export function rovingMove(key: string, index: number, lanes: number, perPage: number, total: number): number | null {
	const last = total - 1;
	const clamp = (i: number) => Math.min(Math.max(i, 0), last);
	switch (key) {
		case 'ArrowDown':
			return clamp(index + lanes);
		case 'ArrowUp':
			return clamp(index - lanes);
		case 'ArrowRight':
			return lanes > 1 ? clamp(index + 1) : null;
		case 'ArrowLeft':
			return lanes > 1 ? clamp(index - 1) : null;
		case 'PageDown':
			return clamp(index + lanes * Math.max(perPage, 1));
		case 'PageUp':
			return clamp(index - lanes * Math.max(perPage, 1));
		case 'Home':
			return 0;
		case 'End':
			return last;
		default:
			return null;
	}
}
