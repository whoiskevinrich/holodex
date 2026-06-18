# Design Handoff: Delete a Media Item ("Move to Trash" + Trash view) (F24)

**Spec**: [Delete a Media Item (F24)](../specs/delete-media.md) · **ADR**: [ADR-037](../architecture/ADR-037-soft-delete-and-purge.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**
**Stack**: SvelteKit (Svelte 5 runes) + Tailwind v4 CSS-first (ADR-025).

---

## Overview

Three owner-gated surfaces; nothing renders for a non-owner (or an owner who hasn't entered the token):

1. **Delete controls on the media detail page** (`/media/[id]`) — an owner-only **Manage** block with
   **Move to Trash** (soft-delete) and **Delete permanently** (purge-now). Each opens a **confirm
   dialog**; the destructive button is `--warn`-styled. On a soft-delete the page **navigates back to
   the library** (the item is now absent from the grid — that disappearance *is* the feedback, the
   same way enrichment uses field-population rather than a toast; there is no toast system yet).
2. **Trash view** (`/trash`, owner-only) — a list of soft-deleted items showing **deleted {when} ·
   purges {when}**, each with **Restore** and **Delete permanently**. Empty/loading/error states
   themed. Linked from the header's library-tools group, rendered only when `activity.isOwner`.
3. **Header "Trash" link** — added beside Keys/Status in the library-tools group, gated on
   `activity.isOwner` so non-owners never see it.

The actual hiding of soft-deleted items from browse/search/related/people/tags is **backend-only**
(ADR-037 §4) — no frontend change: a soft-deleted item simply stops appearing because the API no
longer returns it.

### Design-system fit (the `/design-system` check)

No new tokens and no new primitives — except one genuinely-missing pattern: a **destructive confirm
dialog**. The app has modals (`EnrichPicker`, `PersonPicker`) with the right a11y bones (role=dialog,
focus-trap, Esc, focus-return) but no reusable confirm. We add **one** small component,
`ConfirmDialog.svelte`, built from those exact idioms, with a `--warn` destructive button instead of
the accent CTA. Everything else reuses existing classes verbatim:

- **Modal shell** — same `fixed inset-0 z-50 … bg-bg/70` backdrop + `rounded-theme border border-rule
  bg-surface p-4 shadow-xl` card as `PersonPicker` (lines 129–144).
- **Destructive button** — the `--warn` token (`border-warn text-warn`, and a solid
  `bg-warn`/`text-warn-ink`-style fill for the primary destructive action — see "Warn button" below),
  the same token `/status` and the enrichment error lines already use for attention/error.
- **List rows** (Trash) — the `rounded-theme border border-rule bg-surface` card row used by
  `JobHistory` / the enrichment panel.
- **Owner gating** — `activity.isOwner`, identical to the enrichment & alias controls.

Because all but the confirm dialog already exist, the audit output is: **add `ConfirmDialog.svelte`
(warn variant of the existing modal idiom); reuse the card, row, button, and gating idioms verbatim;
introduce no new tokens.**

---

## Reuse map (don't build from scratch)

| Need | Reuse from | Notes |
|------|-----------|-------|
| Owner gating | `activity.isOwner` (`activity.svelte.ts`) | Same predicate enrichment/alias controls use; controls render only when owner. |
| Modal a11y (dialog/trap/Esc/focus-return) | `PersonPicker.svelte` (lines 61–76, 129–144, 231) | Copy the `trapTab`, backdrop-click-close, `svelte:window` Esc, and `trigger?.focus()` restore wholesale into `ConfirmDialog`. |
| Owner-gated DELETE/POST calls | `api.sendAuthed` (`api.ts` lines 53–63) | New `api.deleteMedia(id, {purge})`, `api.restoreMedia(id)`, `api.trash()` mirror `deletePersonImage`/`rescan`. |
| Inline action error | enrichment panel `actionError` (`text-warn` line) | Delete/restore failures render inline in the dialog / on the row, never via the page-level error. |
| List row + "when" formatting | `JobHistory.svelte` + `format.ts` (relative-time helper) | Trash rows mirror the job-history row; reuse/extend the existing time formatter for "deleted 2 days ago". |
| Async load state | `AsyncState.svelte` | The Trash list uses it for loading/empty/error exactly like the other list pages. |
| Solid-warn button shape | the accent CTA in `PersonPicker` (line 219–225) | Same shape (`rounded-theme px-3 py-1.5 text-sm font-semibold`), warn palette instead of accent. |

---

## Layout

| Region | Layout |
|--------|--------|
| **Manage block** (`/media/[id]`) | A `section` at the **bottom** of the detail `<article>` (below the related shelves), owner-only. Small uppercase muted heading "Manage", then a `flex flex-wrap gap-2` row: **Move to Trash** (outlined-warn) + **Delete permanently** (outlined-warn, quieter). Sits apart from the content so a destructive action is never adjacent to a navigation link. |
| **Confirm dialog** | Centered modal card (`max-w-md`), backdrop `bg-bg/70`. Title (`skin-title`), body copy, optional `text-warn` error line, then a right-aligned button row: **Cancel** (outlined neutral) + the destructive confirm (solid warn). |
| **Trash page** (`/trash`) | `mx-auto max-w-4xl space-y-4`. Page `h1` "Trash" + a muted one-liner ("Deleted items are kept for {N} days, then permanently removed."). Then the list (or empty state). |
| **Trash row** | `flex flex-wrap items-center gap-3 rounded-theme border border-rule bg-surface p-3`: title (link to nothing — the item's detail 404s now, so render as plain text + path), a muted "deleted {when} · purges {when}", spacer, then **Restore** (outlined accent) + **Delete permanently** (outlined warn) on the right. |

No new breakpoints; rows and button groups wrap with `flex-wrap`.

---

## Design tokens used

Tokens only — every value is a semantic utility backed by a CSS variable. **No `zinc-*`/`sky-*`/hex/
named-font/fixed-radius in markup.**

| Token | Usage |
|-------|-------|
| `bg-surface` | Dialog card, Trash row, Manage block (if carded) |
| `bg-surface-2` | Quiet row hover / secondary fill |
| `text-ink` | Titles, body copy |
| `text-muted` | "Manage"/"Trash" headings, "deleted {when}" timestamps, helper text, Cancel at rest |
| `border-rule` | Dialog border, row border, neutral button border |
| `text-warn` / `border-warn` | **All destructive affordances** — Move-to-Trash & Delete-permanently buttons, the purge-confirm primary button, error lines. **Deliberately distinct from `--accent`** (CLAUDE.md): destructive ≠ primary. |
| `text-accent` / `border-accent` | **Restore** only (a *recovering* action is positive, not destructive) and dialog focus rings |
| `rounded-theme` | Dialog card, rows, buttons |

### Warn button treatment (decisive)

The system has `--warn` as a color token but no solid "danger button" precedent. Define two warn
button shapes, both tokens-only:

- **Outlined-warn** (the triggers on the detail page, and Delete-permanently on a Trash row):
  `rounded-theme border border-warn px-3 py-1.5 text-sm text-warn hover:bg-warn/10`. Quiet until
  hovered; signals destructiveness without shouting.
- **Solid-warn** (the *primary confirm button inside a dialog* only): `rounded-theme bg-warn px-3
  py-1.5 text-sm font-semibold text-warn-ink` (add a `--warn-ink` readable-on-warn token alongside
  the existing `--accent-ink`, mirroring that pair; if a contrast-safe ink token is impractical for a
  skin, fall back to outlined-warn — never hardcode a hex). The solid fill appears **only** after the
  user has opened the confirm, so the loudest treatment is reserved for the final, deliberate click.

> Rationale: `--warn` carries the "this is destructive / pay attention" meaning the spec demands
> ("`--warn` for destructive affordances, never a hardcoded red"). **Restore** is the one control that
> uses `--accent`, because undoing a delete is a safe, positive action — the color split itself tells
> the user which buttons take something away and which give it back.

---

## Components

| Component | Variant / Props | Notes |
|-----------|-----------------|-------|
| `ConfirmDialog.svelte` (**new**) | `title`, `confirmLabel`, `tone: 'warn'`, `body` snippet, `busy`, `error`, `onconfirm`, `oncancel` | The only new primitive. Modal a11y copied from `PersonPicker`. Cancel = neutral outline; confirm = solid-warn (or outlined-warn fallback). Esc/backdrop = cancel. Focus starts on **Cancel** (safer default for a destructive dialog), focus-trapped, returned to trigger. |
| Manage block (inline in `/media/[id]`) | renders when `activity.isOwner` | Two trigger buttons; owns the `which dialog` state (`'soft' | 'purge' | null`). |
| `/trash/+page.svelte` (**new route**) | owner-only page | Loads `api.trash()`; rows with Restore / Delete-permanently; reuses `AsyncState`. Non-owner (or no token) → a muted "Owner only." line, no list. |
| Header Trash link (inline in `+layout.svelte`) | `{#if activity.isOwner}` | One `<a href="/trash">` in the library-tools group beside Keys/Status. |

### Confirm copy (exact)

- **Move to Trash** → title "Move to Trash?", body: "**{title}** will be hidden from your library and
  permanently deleted in {N} days. You can restore it from Trash until then." Confirm: **Move to
  Trash**.
- **Delete permanently** (from detail *or* Trash) → title "Delete permanently?", body: "**{title}**
  and its file will be **permanently removed** from disk. This cannot be undone." + the file path in a
  muted `font-mono` line (truncated, full path on `title=`). Confirm: **Delete permanently**. This is
  the "distinct, stronger confirm naming the irreversibility and the file path" (F24.10 / spec UX).

---

## States and interactions

| Element | State | Behavior |
|---------|-------|----------|
| Manage block | not owner / locked | **Not rendered** (absent from DOM). |
| Manage block | owner | Renders the two trigger buttons. |
| Move to Trash | click | Opens the soft-delete confirm. |
| Move to Trash confirm | confirm | `DELETE /media/{id}` (no `purge`); on `204` navigate to `/` (item gone from grid). On error: keep the dialog open, show inline `text-warn`. |
| Delete permanently | click (detail or Trash) | Opens the purge confirm (stronger copy + path). |
| Purge confirm | confirm | `DELETE /media/{id}?purge=true`; on `204`: detail page → navigate to `/`; Trash row → remove the row optimistically. On a disk-failure error (`409`/`500`) show the server message inline; the item stays (it's still soft-deleted, will retry). |
| Dialog | busy | Confirm button shows "…", both buttons disabled; Esc/backdrop disabled while in flight. |
| Dialog | Esc / backdrop / Cancel | Closes, no request. |
| Trash list | loading | `AsyncState` spinner. |
| Trash list | empty | Themed muted empty state: "Trash is empty." |
| Trash list | error | `text-warn` line via `AsyncState`. |
| Trash row → Restore | click | `POST /media/{id}/restore`; on success remove the row (it's live again). No confirm needed (restoring is safe/positive). On error inline `text-warn` on the row. |
| Header Trash link | not owner | Absent from DOM. |

Restore has **no** confirm dialog (it's non-destructive); both delete paths **always** confirm.

---

## Responsive behavior

| Breakpoint | Changes |
|------------|---------|
| Desktop / tablet (≥640) | Trash rows: metadata left, actions right on one line. Manage block buttons inline. |
| Mobile (<640) | Rows wrap (`flex-wrap`): actions drop below the title/metadata. Dialog is `max-w-md` with `px-4` page gutter (same as `PersonPicker`). Buttons stay reachable; nothing truncates a destructive label. |

---

## Edge cases

- **Item already purged elsewhere** (grace expired between page load and click) — the `DELETE`/
  `restore` returns `404`; show "Already removed." inline and (Trash) drop the row.
- **Restore of an item whose file vanished** — restore only clears `deleted_at`; if the file is also
  gone the scanner will mark it `active=0` separately. Restore still succeeds (the row returns); no
  special UI.
- **Disk-removal failure on purge-now** (read-only mount, permission) — server returns an error with a
  message; the dialog shows it and the item remains in Trash (will retry via the sweep). Never report
  success on a failed unlink.
- **Long title / path** — title `line-clamp`/truncate in the row; the path line truncates with the
  full value on `title=` (as the detail page's path row already does).
- **Non-owner deep-links `/trash`** — the page renders the "Owner only." line, not the list (defense
  in depth; the API also 401s).
- **`DELETE_GRACE_PERIOD_SECONDS = 0`** (auto-purge off) — purge-at copy reads "kept until you delete
  it" instead of a date; items linger in Trash until manually purged. Render conditionally on whether
  the API returns a `purge_at`.

---

## Animation / motion

Reuse `PersonPicker`'s `merge-rise` entrance (opacity + `scale(0.98)`), gated behind
`@media (prefers-reduced-motion: no-preference)` — copy it into `ConfirmDialog`. Row
removal on restore/purge can use a subtle opacity transition only; no transforms required. Skin
flourishes belong in `app.css` gated by `[data-theme]`, never per-component markup.

---

## Accessibility notes

- **Confirm dialog** is `role="dialog" aria-modal="true"` labelled by its title; focus is **trapped**,
  Esc/backdrop cancel, and focus returns to the trigger button (copy `PersonPicker`'s `trigger?.focus()`
  restore). Initial focus lands on **Cancel**, so an accidental Enter doesn't delete.
- **Destructive buttons** are real `<button>`s with words ("Move to Trash", "Delete permanently") —
  never an icon or color alone. The `--warn` color is reinforced by the verb.
- **Restore** uses `--accent` *and* the word "Restore"; color is never the only signal.
- **Error text** uses `text-warn` *and* a sentence, associated with the dialog/row via
  `aria-describedby` / `aria-live="polite"` so it's announced.
- **Owner controls absent from the DOM** for non-owners — nothing misleading in the a11y tree.
- **Live region** — after a restore/purge on the Trash page, announce the row removal via an
  `aria-live="polite"` status line so it's conveyed without focus loss; move focus to the next row's
  Restore button (or the page heading if the list is now empty) — avoid focus landing on `<body>`.

---

## Three-skin QA checklist (required before merge — CLAUDE.md)

> Numbered + verifier-tagged per the house convention. Switch skins via the header picker.

### Setup
- [ ] **0.1** `[smoke]` Set `ADMIN_TOKEN`, enter it on `/status` so `activity.isOwner` is true; have at
      least one media item indexed.

### Smoke
- [ ] **1.1** `[smoke]` `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'`
      returns empty for the changed markup (raw hex only in `app.css`; `rounded-full` pills intentional).
- [ ] **1.2** `[smoke]` `svelte-check` + `vitest` green.

### Agent / human (per skin: Cinémathèque, Broadcast, Brutalist)
- [ ] **2.1** `[human]` On `/media/[id]` as owner, the **Manage** block shows at the bottom with
      **Move to Trash** + **Delete permanently** in `--warn` (outlined), visually distinct from the
      accent used elsewhere — load-bearing on Brutalist (bright-lime accent vs warn).
- [ ] **2.2** `[human]` **Move to Trash** opens a confirm naming the {N}-day grace; confirming returns
      you to the library and the item is **gone from the grid**.
- [ ] **2.3** `[human]` **Delete permanently** opens the *stronger* confirm that names irreversibility
      **and shows the file path**; the primary button is solid-warn and reads (legible `--warn-ink`),
      or falls back to outlined-warn — never an unreadable fill.
- [ ] **2.4** `[human]` Header shows a **Trash** link **only** as owner; a non-owner (clear the token)
      sees no Trash link and no Manage block.
- [ ] **2.5** `[human]` `/trash` lists the soft-deleted item with "deleted {when} · purges {when}";
      **Restore** is `--accent`, **Delete permanently** is `--warn` — the color split reads at a glance.
- [ ] **2.6** `[human]` **Restore** (no confirm) returns the item to the library; reload `/` to confirm
      it's back. **Delete permanently** from Trash confirms, then the row disappears.
- [ ] **2.7** `[human]` Empty Trash shows a themed "Trash is empty."; loading + error states themed.
- [ ] **2.8** `[human]` Dialog radius per skin (`--radius` 2px/0/0) — `rounded-theme` card has no stray
      rounding on Broadcast/Brutalist; focus ring (`focus:border-accent`) visible on each accent.
- [ ] **2.9** `[human]` Keyboard: open a confirm, focus starts on **Cancel**, Tab is trapped, Esc
      cancels, focus returns to the trigger button. Restore/delete buttons are reachable and labelled.
- [ ] **2.10** `[agent]` A soft-deleted item is **absent** from browse, search, the person/tag page it
      belonged to, and its own `/media/{id}` (→ 404) — confirming the backend visibility seam end to end.

> Verify with `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'`
> returning empty for the changed markup.
