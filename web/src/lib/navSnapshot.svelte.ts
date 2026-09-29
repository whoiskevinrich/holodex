// One-shot, key-validated view snapshot for restoring state across SPA navigation
// (ADR-032). Holodex is an SPA (ssr=false): a list/grid is destroyed on navigating away
// and re-created on Back, losing scroll position (and, for the browse grid, every loaded
// "Load more" page). The JS module isn't torn down, so a snapshot saved on
// `beforeNavigate` survives and is restored on return — but only while the view's KEY
// still matches (the filter/sort signature unchanged); a key change invalidates it.
// One-shot: `take` clears the snapshot, so each navigate-away must re-save and a snapshot
// can't be reused across visits. The single slot is session-scoped: a full reload starts
// empty. The registry's scroll-only slots survive a reload via sessionStorage (ADR-118).
//
// Two shapes ride on the same mechanics:
//  - createNavSnapshot(): a single slot, for a view with exactly one caller (the browse
//    grid's browseCache — heavier, carries the whole loaded page set, not just scroll).
//  - createNavSnapshotRegistry(): many independent slots keyed by list identity, for
//    lightweight scroll-only snapshots shared across unrelated pages (listScroll,
//    HOLODEX-248). A single shared slot reused across pages via key-prefixing let one
//    page's mismatched take() silently clear another page's still-valid snapshot (take()
//    always clears); the registry keys each page's slot by its own identity so
//    interleaved navigation across different lists can't cross-wire.

export interface Keyed {
	/** The filter/sort signature that produced this snapshot; restore only if it matches. */
	key: string;
}

export function createNavSnapshot<T extends Keyed>() {
	let snap: T | null = null;
	return {
		save(s: T) {
			snap = s;
		},
		// Return the snapshot iff its key matches the current view, else null; clears on
		// read either way (consumed once; stale-on-mismatch).
		take(key: string): T | null {
			const s = snap;
			snap = null;
			return s && s.key === key ? s : null;
		},
		clear() {
			snap = null;
		}
	};
}

// A registry of independent one-shot slots, addressed by `id` — the list's own identity
// (e.g. 'people', 'studios', or `person:${personId}`). Each id owns its own slot, so one
// list saving/taking never disturbs another's, unlike a single createNavSnapshot()
// instance reused across pages.
//
// Each slot is mirrored to sessionStorage under `${namespace}:${id}` so it outlives a full
// reload — a ForwardAuth re-auth redirect, a new deploy's chunk-hash fallback, a discarded
// tab (ADR-118, HOLODEX-477). sessionStorage is per tab, like the history it restores
// against. The in-memory Map stays the fallback when storage is missing or throws; the
// one-shot and stale-on-mismatch semantics are the same either way.
export function createNavSnapshotRegistry<T extends Keyed>(namespace: string) {
	const slots = new Map<string, T>();
	const storageKey = (id: string) => `${namespace}:${id}`;
	function readStored(id: string): T | undefined {
		try {
			const raw = sessionStorage.getItem(storageKey(id));
			return raw == null ? undefined : (JSON.parse(raw) as T);
		} catch {
			// No storage, or a corrupt value — nothing to restore.
			return undefined;
		}
	}
	function removeStored(id: string) {
		try {
			sessionStorage.removeItem(storageKey(id));
		} catch {
			// Unavailable storage holds nothing to remove.
		}
	}
	return {
		save(id: string, s: T) {
			slots.set(id, s);
			try {
				sessionStorage.setItem(storageKey(id), JSON.stringify(s));
			} catch {
				// Storage full/unavailable — the memory slot still covers SPA navigation. Drop
				// any older stored slot so a reload can't restore it in place of this one.
				removeStored(id);
			}
		},
		take(id: string, key: string): T | null {
			const s = slots.get(id) ?? readStored(id);
			slots.delete(id);
			removeStored(id);
			return s && s.key === key ? s : null;
		},
		clear(id: string) {
			slots.delete(id);
			removeStored(id);
		}
	};
}
