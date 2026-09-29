# ADR-118: List scroll snapshots survive a full reload (sessionStorage)

**Status:** Proposed
**Date:** 2026-09-28
**Deciders:** Project owner

**Supersedes:** [ADR-032](ADR-032-browse-state-preservation.md)'s consequence "Cache is
session-scoped, not durable", **for the scroll-only `listScroll` registry only**. The browse
grid's `browseCache` keeps it.
**Extends:** [ADR-114](ADR-114-list-state-model.md) D5 — the snapshot key stays the canonical
query string.
**Relates to:** HOLODEX-477 (the bug), HOLODEX-248 (`listScroll`), HOLODEX-472 (F73).

## Context

Going ← Back from a Person to the People list sometimes lands at the top. The saved offset
lived only in JS memory, so anything that reloads the document between leaving the list and
coming back wiped it:

1. a ForwardAuth (Authentik) re-auth — `triggerReauth()` calls `window.location.assign`, silently
   and periodically;
2. a new image deployed — changed chunk hashes make SvelteKit fall back to a full navigation;
3. a discarded tab (Brave/Chrome Memory Saver).

After F73 the list's filters and sort survive a reload (they're in the URL), so the scroll
reset is the only thing left that makes Back feel like a fresh page.

## Decision

**D1.** `createNavSnapshotRegistry(namespace)` mirrors each slot to `sessionStorage` under
`${namespace}:${id}` (`holodex:listScroll:people`, `holodex:listScroll:person:42`, …).
`save` writes both the memory slot and storage; `take` reads memory first, then storage, and
clears both.

**D2.** Semantics are unchanged: one-shot (`take` always clears), stale-on-mismatch (the D5 key
must match), per-id isolation. A reload is just a take from an empty memory Map.

**D3.** Storage is best-effort. Every access is in `try/catch`; when storage is missing, full,
or throws, the memory slot still covers in-app navigation exactly as before. A corrupt stored
value restores nothing.

**D4.** Covers every `listScroll` user: People, Studios, Tags, Films, Search and an entity's
videos (`EntityVideos`). No call site changes.

## Rationale

- `sessionStorage` is per tab, like the history stack it restores against. Two tabs on the same
  list don't cross-wire, and closing the tab drops the data.
- Snapshots are a few bytes (`{key, scrollY}`), so size is not a concern.
- `localStorage` was rejected: it's shared across tabs and outlives the history it describes, so
  a snapshot could restore into an unrelated visit.

## Consequences

- **Accepted gap: the Media browse grid still resets on reload.** `browseCache` carries the
  whole loaded page set, which is too heavy for `sessionStorage`. ADR-032's session-scoped
  consequence stands for it.
- **Reloading the list itself restores its scroll too.** `beforeNavigate` also fires on unload,
  so F5 on People saves the offset and the reload takes it. This matches what browsers do for
  static pages.
- **Unvisited slots can linger for the tab's lifetime.** A slot saved on leaving a list you
  never come back to stays in `sessionStorage` until the tab closes. There's at most one per list
  identity, and each is tiny.
- **Testable:** `navSnapshot.test.ts` simulates a reload with a fresh registry over the same
  storage, and covers the one-shot, mismatch, isolation, corrupt-value and no-storage paths.
