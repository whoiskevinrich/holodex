# Design Handoff: Person Aliases ("Also known as") (F23)

**Spec**: [Person Aliases (F23)](../specs/person-aliases.md) · **ADR**: [ADR-036](../architecture/ADR-036-person-alias-search-indexing.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**
**Stack**: SvelteKit (Svelte 5 runes) + Tailwind v4 CSS-first (ADR-025).

---

## Overview

Surfaces, all owner-gated except the read-only chip display:

1. **"Also known as" panel** on the **Person detail page** (`/people/[id]`) — the person's aliases as
   chips; owner gets an add field + per-chip ✕.
2. **Merge from the person page** — a **"Merge a person in…"** button opens a `PersonPicker` modal
   (search a person → informed confirm showing both video counts → merge). The page's person is canonical.
3. **Alias-collision prompt** — when an added alias names a *different existing person*, an inline
   prompt asks the owner to merge them in or keep them separate (never a silent merge — homonyms exist).
4. **Merge from the People list** (`/people`) — an owner **"Merge people…"** mode: multi-select 2+,
   then a **"Keep which name?"** dialog chooses the canonical and folds the rest in.

Aliases are also honored at global search *and* scan time (ADR-036), but neither has a UI change: an
alias-matched person appears in the existing search palette under its canonical name.

### Design-system fit (the `/design-system` check)

No new tokens, no new primitives. This feature reuses patterns already in the system:

- **Chips** — same `rounded-full` pill shape as `ProvenanceBadge` / the alias chips already sketched in
  the F22 handoff. The "remove" affordance is a ✕ glyph button *inside* the owner variant of the chip.
- **Add field** — the same input + solid-accent button pairing the EnrichPicker and `/status` unlock
  form use (`bg-surface` input, `focus:border-accent`, `bg-accent text-accent-ink` button).
- **Panel** — same `rounded-theme border border-rule bg-surface p-4` card the Enrichment panel uses,
  rendered in the same `EntityVideos` `detail` snippet. Aliases render **above** Enrichment (they are
  core person data; enrichment is provider shadow data).
- **Owner gating** — `activity.isOwner`, identical to the Enrichment controls right beside it.

Because every piece already exists, the "audit for inconsistency" output is: **use the existing chip,
input, button, and panel idioms verbatim; introduce nothing new.**

---

## Reuse map (don't build from scratch)

| Need | Reuse from | Notes |
|------|-----------|-------|
| Owner gating | `activity.isOwner` (`activity.svelte.ts`) | Same predicate the Enrichment panel uses; controls render only when owner. |
| Panel shell | the Enrichment `<section>` in `people/[id]/+page.svelte` (lines 89–131) | Same card classes; place the Aliases panel just above it. |
| Chip shape | `ProvenanceBadge.svelte` | `rounded-full px-2 py-0.5 text-xs`; alias chips are slightly larger (`text-sm`) but same family. |
| Add input + button | EnrichPicker search input + the `/status` unlock form | `bg-surface` + `border-rule`, `focus:border-accent`; solid `bg-accent text-accent-ink` submit. |
| Inline action error | the Enrichment panel `actionError` (`text-warn` line) | Alias add/delete failures render inline the same way — never via the page-level `error`. |
| Async page load | `AsyncState.svelte` | Aliases arrive inside the existing `getPerson` payload — no separate load/spinner. |

---

## Layout

| Region | Layout |
|--------|--------|
| Aliases panel | Full-width `section` card (`rounded-theme border border-rule bg-surface p-4 space-y-3`), inside the `EntityVideos` `detail` snippet, **above** the Enrichment panel. Header row: small uppercase muted heading "Also known as" left; nothing else needed on the right. |
| Chip row | `flex flex-wrap gap-2`. Each chip a `rounded-full` pill. |
| Add row (owner) | `flex flex-wrap items-center gap-2`: text input (`flex-1 min-w-0` so it shrinks on mobile) + "Add" button. Sits below the chip row. |

No new breakpoints. Chips and the add row wrap naturally with `flex-wrap`.

---

## Design tokens used

Tokens only — every value is a semantic utility backed by a CSS variable. **No `zinc-*`/`sky-*`/hex/
named-font/fixed-radius in markup.**

| Token | Usage |
|-------|-------|
| `bg-surface` | Panel card, the add-input background |
| `bg-surface-2` | Alias chip background (quiet, recedes — same as the "from file" provenance chip) |
| `text-ink` | Alias text, input text |
| `text-muted` | "Also known as" heading, empty/help text, the ✕ remove glyph at rest |
| `border-rule` | Panel border, input border, chip border |
| `bg-accent` / `text-accent-ink` | "Add" button (solid accent CTA) |
| `text-accent` / `border-accent` | Input focus ring (`focus:border-accent`); ✕ remove glyph on hover/focus |
| `text-warn` | Add/delete **error** message only (e.g. validation, network) — never on a chip at rest |
| `rounded-theme` | Panel card, input, "Add" button |
| `rounded-full` | Alias chips (intentional pill shape, allowed by CLAUDE.md) |

### Chip treatment (decisive)

- **Read-only chip** (non-owner, or owner view of an alias): `rounded-full bg-surface-2 px-2.5 py-0.5
  text-sm text-ink` — quiet pill, legible, no border needed (matches the file-provenance chip family).
- **Owner chip** adds a trailing **✕ button**: `text-muted hover:text-accent focus:text-accent` icon
  button, `aria-label="Remove alias {alias}"`. The ✕ uses accent on hover (noteworthy, not alarming) —
  **not `--warn`**, which is reserved for errors.

> Rationale: aliases at rest are neutral metadata, so the chip is muted-surface, not accent. Only the
> *act* of removing is highlighted (accent on the ✕), and only *failures* use `--warn`.

---

## Components

| Component | Variant / Props | Notes |
|-----------|-----------------|-------|
| Aliases panel (inline in `people/[id]/+page.svelte`) | renders from `person.aliases` + `isOwner` | No new component file required — it's a small block, like the Enrichment panel beside it. Extract to `AliasPanel.svelte` only if the page gets noisy. |
| Alias chip | `owner: boolean` → with/without ✕ | Read-only pill vs. pill + remove button. |
| Add field | `disabled` while a submit is in flight | Enter in the input submits (same as clicking "Add"). |

---

## States and interactions

| Element | State | Behavior |
|---------|-------|----------|
| Panel | no aliases, not owner | **Not rendered** (nothing to show, no controls). |
| Panel | no aliases, owner | Rendered with heading + add row + a muted "No aliases yet." line. |
| Alias chip | default | Static pill. |
| Alias chip (owner) | hover/focus ✕ | ✕ goes accent; click removes the alias (optimistic: drop locally on 204, restore + show error on failure). |
| Add input | typing | Local state; no network until submit. |
| Add (Enter / click) | submit | Trim; if empty, ignore (no request). `POST …/aliases`; on success replace the list with the returned aliases and **clear the input** (keep focus for quick multi-add). |
| Add | duplicate | Server is idempotent (ADR-036); the returned list just doesn't grow — clear input, no error shown. |
| Add | too long / invalid | Server `400`; show inline `text-warn` "Alias is too long." / "Enter an alias." and keep the typed text. |
| Add / delete | network error | Inline `text-warn` message; the chip list is left consistent (optimistic delete restores the chip). |
| Controls | not owner | Absent from the DOM (not visually hidden). |
| Controls | needs token | Owner-but-locked: same as the Enrichment panel — controls appear once the admin token is entered (the layout already drives `isOwner`). |

---

## Responsive behavior

| Breakpoint | Changes |
|------------|---------|
| Desktop / tablet (≥640) | Chips wrap across the panel width; add input + button on one line. |
| Mobile (<640) | Chips wrap; the add input (`flex-1 min-w-0`) shrinks and the "Add" button wraps below if needed (`flex-wrap`). |

---

## Edge cases

- **Many aliases** — chips wrap to multiple rows; no truncation needed at personal-library scale.
- **Long single alias** (e.g. a full romanized name) — chip grows; allow it to wrap within the pill
  rather than truncating (a name you can't read defeats the purpose).
- **International text / diacritics (e.g. "Beyoncé", CJK "宮崎駿")** — must render in all skins (mono
  display faces on Broadcast/Brutalist); body text uses `font-ui`. Verify no tofu. Search folds
  diacritics (ADR-036 tokenizer), so this is also a search-QA point.
- **Duplicate add** — idempotent, silent (see states).
- **Alias equal to the canonical name** — allowed (harmless); not specially handled. Search would match
  the person via name anyway; the dedup keeps it single.
- **Deleting the last alias** — owner view falls back to "No aliases yet."; non-owner view: the panel
  disappears on next load.

---

## Animation / motion

Minimal. No modal here. Chip add/remove can use a subtle background/opacity transition only, gated
behind `@media (prefers-reduced-motion: no-preference)` — consistent with the rest of `app.css`. No
transforms required. Skin flourishes (if any) belong in `app.css` gated by `[data-theme]`, not in markup.

---

## Accessibility notes

- **Remove buttons** are real `<button>`s with `aria-label="Remove alias {alias}"` (the ✕ glyph alone
  is not a label). Keyboard-focusable; Enter/Space activate.
- **Add field** — the input has an associated label (visible "Also known as" heading + `aria-label` or a
  `<label>`); Enter submits; the "Add" button is a real submit button.
- **Error text** uses `text-warn` *and* words ("Alias is too long.") — never color alone — and is
  associated with the input via `aria-describedby` so screen readers announce it.
- **Live region** — after add/remove, the chip list change should be announced; wrap the chip
  list (or a status line) in `aria-live="polite"` so "alias added/removed" is conveyed without focus loss.
- **Owner controls** absent from the DOM for non-owners — nothing misleading in the a11y tree.
- **Focus management** — after a successful add, focus stays in the input (multi-add friendly); after a
  delete, focus moves to the next chip's remove button or back to the add input if none remain (avoid
  focus landing on `<body>` — the lesson from the F22 picker focus-return fix).

---

## Merge surfaces (F23.9–F23.11)

### `PersonPicker.svelte` (new) — person-page merge

Mirrors `EnrichPicker`'s modal a11y (role=dialog/aria-modal, combobox+listbox, **roving tabindex**,
focus trap, Esc, focus-return). Two steps in one modal:

1. **Pick** — search input filters the people list client-side (excludes the canonical person); each
   row shows `name` + video count (`rounded-theme border-l-2` active row = `border-accent bg-surface-2`,
   exactly like the candidate rows).
2. **Informed confirm** — "Merge **{name}** ({n} videos) into **{canonical}**?" + a muted explanation
   that videos move, the name becomes an alias, and it can't be auto-undone. **Back** (outlined) /
   **Merge** (solid accent) buttons. Errors inline in `text-warn`.

### Collision prompt (person page, inline)

When `POST …/aliases` returns 409, render an inline card (`rounded-theme border border-rule bg-surface-2
p-3`, `aria-live="polite"`): "**{name}** ({n} videos) is already a separate person. Are they the same
as {person}?" → **Yes, merge them in** (solid accent) / **No, keep separate** (outlined). This is the
homonym safeguard — the only way an add-alias turns into a merge is the owner clicking through it.

### People-list multi-select (`/people`)

Owner-only **"Merge people…"** toggle turns rows into checkbox `<label>`s (`accent-accent` checkbox,
selected row `border-accent`). A header **"Merge N selected"** (solid accent, disabled < 2) opens the
**"Keep which name?"** dialog: radio list (`accent-accent`) of the selected people with video counts,
**Back** / **Merge**. Confirm folds every non-canonical selection into the chosen one, then reloads.

### Tokens for merge surfaces

Reuses the same token set as the alias panel. New utility: **`accent-accent`** on the checkbox/radio
(`accent-color: var(--accent)`) so the native control themes per skin — verified gold/cyan/lime. No
hex, no palette literals; modal cards use `rounded-theme`, chips/pills stay `rounded-full`.

### Merge a11y

- Both modals are `role="dialog" aria-modal="true"` labelled by their heading; focus trapped; Esc
  closes; focus returns to the trigger (PersonPicker restores via the captured `document.activeElement`).
- The picker list is a `role="listbox"` with roving tabindex; ↑/↓ move, Enter/Space pick, ↑ from the
  top returns to the search box (mirrors `EnrichPicker`).
- Confirm/collision buttons are real `<button>`s with words ("Yes, merge them in"), never color-only.

## Three-skin QA checklist (required before merge — CLAUDE.md)

Render `/people/[id]` with aliases present in **Cinémathèque, Broadcast, Brutalist**:

- [ ] **Alias chips legible** in each skin — muted-surface pill reads on the panel card; ✕ glyph
      visible and turns accent on hover/focus.
- [ ] **Panel radius** correct per skin (`--radius`: 2px / 0 / 0) — `rounded-theme` card has no stray
      rounded corners on Broadcast/Brutalist; only the chips stay pill (`rounded-full`, intentional).
- [ ] **Heading** "Also known as" muted/uppercase reads in each skin.
- [ ] **Add input focus ring** (`focus:border-accent`) visible on each accent (lime/cyan/gold).
- [ ] **"Add" button** solid accent + `text-accent-ink` legible in each skin.
- [ ] **Error state** (`text-warn`) distinct from accent in each skin (the warn/accent separation is
      load-bearing on Brutalist where accent is bright lime).
- [ ] **International/CJK alias** renders (no tofu) in mono-faced skins.
- [ ] **Empty (owner) / populated / read-only (non-owner)** states all themed.
- [ ] **Owner vs locked** — with `ADMIN_TOKEN` set and no token entered, no add/remove controls; chips
      still show read-only; after unlock, add/remove work.
- [ ] **Search-by-alias** — add "Ziggy" to a person, type "zig" in the global search box, the person
      appears (cross-checks ADR-036 end-to-end, in any one skin).
- [ ] **Merge picker** — dialog card radius per skin (2/0/0), active row accent left-bar, **Back**
      (outlined) vs **Merge** (solid accent) legible; the informed-confirm text readable.
- [ ] **Collision prompt** — the inline card reads on `bg-surface-2`; "Yes, merge them in" (accent) vs
      "No, keep separate" (outlined) clearly distinct.
- [ ] **People-list select mode** — checkbox/radio `accent-accent` shows the skin accent (lime/cyan/
      gold); selected-row `border-accent` visible; "Merge N selected" button legible.

> Verify with `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'`
> returning empty for the changed markup.
