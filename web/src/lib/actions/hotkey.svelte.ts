import type { Action } from 'svelte/action';

// Page-scoped hotkeys (F62, HOLODEX-405). A hotkey is registered BY the button it
// drives — `<button use:hotkey={'e'}>` — so the button's existence is the gate: owner
// view, Admin mode, `canWriteback`, "≥1 provider configured" are all decided where the
// button renders, and nothing here re-derives them. Pressing the key focuses the button
// (ring + scroll-into-view = visible confirmation) and clicks it; a `disabled` button
// makes `click()` a native no-op, so a running action debounces key-mashing for free.
//
// The single window listener lives in `routes/+layout.svelte` and calls `guardKeydown`
// + `fire`. The `?` sheet (`HotkeySheet.svelte`) renders `hotkeys.entries` — derived,
// never hand-maintained.

export interface HotkeyEntry {
	key: string;
	label: string;
	node: HTMLElement;
}

export interface HotkeyParam {
	key: string;
	/** Row label in the `?` sheet; defaults to the node's trimmed text content. */
	label?: string;
}

/** Sentinel returned by `guardKeydown` for the sheet toggle — never a registered key. */
export const SHEET_KEY = '?';

export class HotkeyRegistry {
	entries = $state<HotkeyEntry[]>([]);

	register(entry: HotkeyEntry) {
		if (import.meta.env.DEV && this.entries.some((e) => e.key === entry.key)) {
			console.warn(`[hotkey] "${entry.key}" is already bound on this page; first registration wins`);
		}
		this.entries.push(entry);
	}

	unregister(node: HTMLElement) {
		this.entries = this.entries.filter((e) => e.node !== node);
	}

	/** First registration for `key` (RD7: first wins), or undefined. */
	find(key: string): HotkeyEntry | undefined {
		return this.entries.find((e) => e.key === key);
	}

	/** One row per key, sorted by key — what the `?` sheet lists. */
	get rows(): HotkeyEntry[] {
		const seen = new Set<string>();
		return this.entries
			.filter((e) => !seen.has(e.key) && seen.add(e.key))
			.sort((a, b) => a.key.localeCompare(b.key));
	}
}

export const hotkeys = new HotkeyRegistry();

// The part of a KeyboardEvent the guard reads — narrow so tests run without a DOM.
export interface KeyLike {
	key: string;
	defaultPrevented: boolean;
	ctrlKey: boolean;
	altKey: boolean;
	metaKey: boolean;
	target: object | null;
}

const TYPING = new Set(['INPUT', 'TEXTAREA', 'SELECT', 'VIDEO']);

/**
 * RD6, in order. Returns the key to act on (`SHEET_KEY` for the sheet toggle, or a
 * registered key), or null when nothing should fire.
 * - `defaultPrevented`: a closer handler already claimed the key (HOLODEX-249).
 * - modifiers: Ctrl/Alt/Meta combos belong to the browser (Shift is allowed so `?` works).
 * - typing targets: text fields, `<select>`, `[contenteditable]`, and `<video>` — the native
 *   player owns `f` (fullscreen) while focused.
 * - `?` toggles the sheet unless another dialog is open (the sheet never stacks), and can
 *   always close its own sheet.
 * - any open dialog — including the sheet — silences page keys (no second EnrichPicker
 *   over the first).
 * `dialogOpen` is "a dialog other than the sheet is open"; `sheetOpen` is the sheet itself.
 */
export function guardKeydown(e: KeyLike, dialogOpen: boolean, sheetOpen = false): string | null {
	if (e.defaultPrevented) return null;
	if (e.ctrlKey || e.altKey || e.metaKey) return null;
	const t = e.target as { tagName?: string; isContentEditable?: boolean } | null;
	if (t && (TYPING.has(t.tagName ?? '') || t.isContentEditable)) return null;
	if (e.key === SHEET_KEY) return dialogOpen ? null : SHEET_KEY;
	if (dialogOpen || sheetOpen) return null;
	return hotkeys.find(e.key) ? e.key : null;
}

/** RD2: focus (ring + scroll into view), then click. */
export function fire(entry: HotkeyEntry) {
	entry.node.focus();
	entry.node.click();
}

/**
 * `use:hotkey={'e'}` / `use:hotkey={{ key: 'e', label: '…' }}` — registers the node for
 * the page's lifetime, sets `aria-keyshortcuts`, and appends ` (e)` to an existing title
 * (RD8). Re-registers when the parameter changes.
 */
export const hotkey: Action<HTMLElement, string | HotkeyParam> = (node, param) => {
	const baseTitle = node.title;

	function apply(p: string | HotkeyParam) {
		const key = typeof p === 'string' ? p : p.key;
		const label =
			(typeof p === 'string' ? undefined : p.label) ?? node.textContent?.trim() ?? key;
		node.setAttribute('aria-keyshortcuts', key);
		if (baseTitle) node.title = `${baseTitle} (${key})`;
		hotkeys.register({ key, label, node });
	}

	apply(param);

	return {
		update(next) {
			hotkeys.unregister(node);
			apply(next);
		},
		destroy() {
			hotkeys.unregister(node);
		}
	};
};
