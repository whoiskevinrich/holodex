# Spec: Page-scoped hotkeys (F62)

**Status**: Draft
**Phase**: Story (Jira [HOLODEX-405](https://whoiskevinrich.atlassian.net/browse/HOLODEX-405))
**Owner**: Project owner
**Date**: 2026-09-16
**Feature block**: **F62** — single-key hotkeys for owner actions, built as a small system where a
future page-specific key is one attribute on one button. v1 ships two keys (`e` refresh enrichment,
`f` write decisions to file) and a `?` sheet that lists whatever is bound on the current page.

**Depends on** (all shipped):
- `EnrichProviderChips` **Refresh all** (RD8/P1-2) — `runEnrichRefreshAll` refreshes every linked
  provider and opens the picker for every unlinked one, so one button already means "enrich or
  refresh" across N providers. Mounted on media, people, films and studios detail.
- Media detail **Write decisions to file** button (F28 / ADR-041) — gated by `canWriteback`
  (`isOwner && resolved.length > 0`), opens `WritebackFormDialog`.
- Admin mode (F29) — the effective owner gate `activity.isOwner && adminMode.enabled` is already
  applied at render time by both buttons above.
- HOLODEX-249 — the `defaultPrevented` convention: a handler closer to the event target that has
  claimed a key wins; window-level listeners bail on `e.defaultPrevented`.

**Related**: `web/src/lib/actions/dismissable.ts` — the existing shared keyboard idiom this action
sits beside · `routes/+layout.svelte` (Ctrl-K) and `routes/+page.svelte` (`/`, Escape, arrow grid
nav) — the two pre-existing window listeners, which this spec does **not** migrate.

**ADR**: none — no data model, deployment or cross-cutting decision; the mechanism is a Svelte action
plus a module-level store, both frontend-local.

**Design handoff**: [hotkeys-handoff.md](../design/hotkeys-handoff.md) — the `?` sheet (layout,
empty state, dismissal) and the focus-then-click feedback. Mockup: `hotkeys-sheet.svg`.

**Test plan**: [testing-strategy.md](../testing-strategy.md) §5 — one Vitest row for the registry
and guards, one component row for the sheet.

---

## Problem Statement

Refreshing enrichment and writing decisions to the file are the two actions the owner performs most
often on a detail page, and each is a mouse trip to a mid-page button. The owner wants single-key
access to them, and wants the *next* such key to be cheap to add. Today the frontend has three
unrelated keyboard idioms (a Ctrl-K listener in the layout, a `/`/Escape/arrows listener on the
browse page, per-dialog Escape handlers plus `use:dismissable`) and no place to hang a fourth key
without writing a fourth idiom.

## Goals

1. `e` on any detail page with a Refresh-all button does what clicking Refresh all does.
2. `f` on media detail does what clicking Write decisions to file does.
3. Adding a future hotkey is one attribute on one existing button — no new gating logic.
4. `?` shows every key that works on the current page, derived from what is bound, never
   hand-maintained.
5. A hotkey never fires while the user is typing, controlling the video player, or inside a dialog.

## Non-Goals

- **User-remappable bindings** — one owner, keys live in code. Configurability would sprawl.
- **Chords / sequences (`g e`) and modifier combos (`Ctrl-E`)** — no key pressure yet; plain
  single keys suffice for v1 and modifiers collide with the browser.
- **Migrating the three existing listeners** (Ctrl-K, browse `/`+arrows, dialog Escapes) onto the
  registry — working code; migrate opportunistically, not here. The `?` sheet lists them statically.
- **Binding `f` on the films / tags batch-writeback buttons** — media detail only, as asked. The
  mechanism supports it later as one attribute.
- **A per-provider `e` chooser** — Refresh all already spans every configured provider.

## Resolved Decisions

| # | Decision | Why |
|---|---|---|
| RD1 | Hotkeys are registered **by the button**, via a Svelte action `use:hotkey={'e'}`. | The button's existence *is* the gate: owner, Admin mode, `canWriteback`, `providers.length > 0` are all already decided where the button renders. A page-level key map would re-derive every one of them and drift. |
| RD2 | Firing = `node.focus()` then `node.click()`. | Focus scrolls the button into view and shows the focus ring, so the owner sees what fired. Native `disabled` makes `click()` a no-op, so a running Refresh all debounces key-mashing for free. |
| RD3 | `e` binds to **Refresh all** in `EnrichProviderChips`. | `runEnrichRefreshAll` already refreshes linked providers and opens the picker for unlinked ones — "enrich or refresh" in one button, for any number of providers. |
| RD4 | `f` binds to **Write decisions to file** on media detail only. | Opens `WritebackFormDialog`; never writes blind. Films/tags batch buttons stay unbound (Non-Goals). |
| RD5 | `?` opens a global sheet in `+layout.svelte`: a derived **Page** group (the registry) plus a static **Navigation** group (Ctrl-K, `/`, arrows). | One listener, one sheet, every page. The static group keeps the sheet honest about keys that predate the registry without migrating them. |
| RD6 | Guards, in order: bail if `e.defaultPrevented`; bail if any modifier is held; bail if the target is `INPUT`/`TEXTAREA`/`SELECT`/`VIDEO`/`[contenteditable]`; bail if a `[role="dialog"]` is in the DOM. | `defaultPrevented` is the HOLODEX-249 convention. `VIDEO` is new: native player controls treat `f` as fullscreen when focused. The dialog check stops `e` opening a second `EnrichPicker` over the first. |
| RD7 | Duplicate registration of a key logs a `console.warn` in dev; the **first** registration wins. | The only way it happens is a page mounting two Refresh-all rows — a bug worth surfacing, not a case worth designing for. Multiple *providers* in one row are the normal case and register one key. |
| RD8 | The action sets `aria-keyshortcuts` on the node and appends the key to its `title` as `(e)`. | Assistive tech and hover both discover the key from the button itself, with no extra markup at the call site. |

## User Stories

- As the owner on a person/studio/film/media page, I want to press `e` to refresh enrichment so
  that I don't hunt for the Refresh all button after editing the record.
- As the owner on a media page with resolved decisions, I want to press `f` to open the write-to-file
  dialog so that the review step is one key away.
- As the owner typing in the search box, a field editor, or a dialog, I want `e` and `f` to type
  letters so that hotkeys never hijack input.
- As the owner with the video player focused, I want `f` to go fullscreen as the browser intends
  so that the player's own keys still work.
- As the owner, I want `?` to show which keys work here so that I don't have to remember which
  pages support what.
- As a visitor (or the owner in visitor view), pressing `e` or `f` does nothing, because the
  buttons those keys drive aren't on the page.
- As a developer adding the next page-specific action, I want to add `use:hotkey={'x'}` to the
  button and have gating, `?` listing, `aria-keyshortcuts` and the title hint all follow.

## Requirements

### Must-Have (P0)

**P0-1 — `use:hotkey` action + registry.** `web/src/lib/actions/hotkey.svelte.ts` exports the action
and a module store `hotkeys` (`$state` list of `{ key, label, node }`). The action registers on mount
with `label` = the node's trimmed text content (overridable via `{ key, label }`), unregisters on
destroy, and updates when its parameter changes.
- [ ] Mounting a button with `use:hotkey={'e'}` adds `{ key: 'e', label: 'Refresh all' }` to the store; unmounting removes it.
- [ ] The node gains `aria-keyshortcuts="e"`; an existing `title` becomes `"<title> (e)"`, a missing one is left absent.
- [ ] Registering `e` twice logs one `console.warn` naming the key in dev builds and the first node stays bound.

**P0-2 — One window listener.** Installed once from `+layout.svelte` (bubble phase). For a keydown
whose `e.key` matches a registered key it applies RD6's guards, then `preventDefault()`, `focus()`,
`click()` on the bound node.
- [ ] Given Refresh all is rendered, when `e` is pressed with body focused, then Refresh all receives focus and its click handler runs once.
- [ ] Given focus is in `#global-search-input`, a `<textarea>`, a `<select>`, a `[contenteditable]`, or the media `<video>`, when `e` or `f` is pressed, then nothing fires and the default (typing / fullscreen) proceeds.
- [ ] Given `EnrichPicker` (or any `[role="dialog"]`) is open, when `e` is pressed, then nothing fires.
- [ ] Given a closer handler called `preventDefault()` on the keydown, then the listener does nothing.
- [ ] Given Ctrl/Alt/Meta is held, then the listener does nothing (Shift is allowed so `?` works).
- [ ] Given Refresh all is `disabled` (refreshing), when `e` is pressed, then no second refresh starts.

**P0-3 — `e` → Refresh all.** `use:hotkey={'e'}` on the Refresh all button in
`EnrichProviderChips.svelte`. Because the button is inside `{#if providers.length > 0}` and the row
is owner-only at every mount site, no new condition is added.
- [ ] Media, people, films and studios detail: `e` triggers Refresh all in owner view with ≥1 provider configured.
- [ ] Visitor view / no providers: `e` is a no-op (the store has no `e`).

**P0-4 — `f` → Write decisions to file.** `use:hotkey={'f'}` on the media detail writeback button
(already inside `{#if canWriteback}`).
- [ ] Media detail, owner, `resolved.length > 0`: `f` opens `WritebackFormDialog`.
- [ ] Any other page, or `canWriteback` false: `f` is a no-op.
- [ ] With the `<video>` focused, `f` toggles fullscreen and the dialog does not open.

**P0-5 — `?` sheet.** `?` (Shift+/) toggles a `[role="dialog"]` sheet rendered from `+layout.svelte`:
a **Page** group listing `hotkeys` entries as `<kbd>key</kbd> label`, and a static **Navigation**
group (`Ctrl/⌘ K` focus search · `/` focus search · `← → ↑ ↓` move between cards · `Esc` clear
filters / close). Escape, `?` or a backdrop click closes it; focus returns to the opener. Centered modal on the `ConfirmDialog` surface (design RD: A over a corner card).
- [ ] On a page with no registered keys the Page group reads "No page shortcuts here" and the Navigation group still renders.
- [ ] The sheet is itself a `[role="dialog"]`, so while open `e`/`f` are guarded by RD6.
- [ ] The typing guard applies: `?` inside an input types a question mark.
- [ ] Tokens-only styling; passes three-skin QA.

### Nice-to-Have (P1)

**P1-1 — Sheet entries are live.** Clicking an entry in the Page group fires it (same focus-then-click
path) and closes the sheet. Cheap once P0-5 exists; makes the sheet a tiny command palette.

### Future Considerations (P2)

- **P2-1** — bind `f` on the films and tags batch-writeback buttons (one attribute each).
- **P2-2** — migrate Ctrl-K / `/` / arrows onto the registry and drop the static Navigation group.
- **P2-3** — chords or a `>`-prefixed command mode in the search box, if single keys run out.

## Behavior detail

```
keydown (window, bubble)
  ├─ e.defaultPrevented        → return          (HOLODEX-249)
  ├─ ctrl/alt/meta held        → return
  ├─ target ∈ INPUT/TEXTAREA/SELECT/VIDEO/[contenteditable] → return
  ├─ key === '?'               → toggle sheet; preventDefault; return
  ├─ document.querySelector('[role="dialog"]') → return
  ├─ entry = registry.find(key) ; none → return
  └─ preventDefault; entry.node.focus(); entry.node.click()
```

The `?` check precedes the dialog check so `?` can close its own sheet. The registry is a plain
`$state` array in a `.svelte.ts` module, so the sheet re-renders as buttons mount and unmount
(e.g. `canWriteback` flipping true after decisions load).

## UI (grounded in real components)

- **Buttons**: unchanged visually. The action only adds `aria-keyshortcuts` and the `(e)` /
  `(f)` title suffix. Focus ring on fire is the existing `focus-visible` treatment.
- **`?` sheet**: a small centered panel on the same surface/border tokens as `ConfirmDialog`,
  `<kbd>` styled with `bg-surface-2 border-rule rounded-theme px-1.5 font-mono text-xs`. Two
  labelled groups. Exact layout, empty state and stressed state (8+ entries) in the design handoff.

## Success Metrics

Personal tool, one owner — no telemetry. The feature succeeded when:
- The owner uses `e`/`f` instead of the buttons on their own instance for a week without a
  mis-fire report (a key firing while typing, in the player, or over a dialog).
- The next page-specific hotkey lands as a one-line diff on a button plus a test row.

## Open Questions

None blocking. Resolved during spec: focus-then-click (RD2), static Navigation group (RD5), global
sheet (RD5).

## Timeline / routing

Single PR on `HOLODEX-405-ux-hotkeys`, opened Draft with this spec. Gates: spec (this doc) → design
handoff (`?` sheet + committed SVG) → frontend → testing-strategy rows → three-skin QA. No ADR, no
security review (frontend-only, no new endpoint, no auth change).
