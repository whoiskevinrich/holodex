# Design Handoff: Unified name-edit mechanism (HOLODEX-269)

Spec: [`docs/specs/unified-name-edit.md`](../specs/unified-name-edit.md) · Epic: HOLODEX-267

## Overview

Two new entity-generic components replace four divergent rename UIs with one interaction an
owner learns once:

- **`NameEditControl`** — a docked pencil beside an entity's displayed name. At rest it is
  invisible to the DOM's visual output for anyone not both owner *and* hovering/focused — a
  non-owner and an owner who hasn't interacted see byte-for-byte the same header. Click → inline
  edit → commit → either the name updates in place, or a collision hands off to `MergeOfferCard`.
- **`MergeOfferCard`** — extracted from the collision-card markup already living inside
  `AliasPanel.svelte` (`web/src/lib/components/person/AliasPanel.svelte:303-326`) into its own
  component, so `NameEditControl` can show it without an `AliasPanel` instance nearby.
  `AliasPanel` switches to using the extracted component for its own alias-add collision case —
  no behavior change there, just a shared implementation.

Four mounting contexts, all real routes:

| Entity | File | What it replaces |
|---|---|---|
| Person | `web/src/routes/people/[id]/+page.svelte` | `SourceSelect`'s `onadopt` intercept on the `name` field (F37 name-chip rename) |
| Studio | `web/src/routes/studios/[id]/+page.svelte` (via `AliasPanel allowRename`) | The inline Rename form built into `AliasPanel.svelte:204-257` |
| Tag | `web/src/routes/tags/[id]/+page.svelte` | Nothing — this page (live since Phase 1, expanded HOLODEX-259) has no rename affordance today; `NameEditControl` is purely additive beside its existing `<h1>` (`:303`) |
| Video Title | `web/src/routes/media/[id]/+page.svelte:622` | A plain non-editable `<h1>`; commits via the existing `PUT /media/{id}/fields/title/decision` |

## Component contract (resolves the spec's open question)

```ts
// NameEditControl.svelte — web/src/lib/components/entity/
let {
  name,              // string — current display name
  isOwner,           // boolean — gates the whole control; non-owner renders nothing but the name
  onCommit,          // (value: string) => Promise<{ ok: true } | { conflict: EntityRef }>
  label = 'name',    // string — for aria-label interpolation ("Rename this {label}")
  verdict            // optional snippet(conflict: EntityRef, resolve: () => void) — see below
}: {
  name: string;
  isOwner: boolean;
  onCommit: (value: string) => Promise<{ ok: true } | { conflict: EntityRef }>;
  label?: string;
  verdict?: Snippet<[EntityRef, () => void]>;
} = $props();
```

`verdict` is a **snippet prop, not a separate conditionally-rendered sibling**. Reasoning: three
of the four callers (Person, Studio, Tag) need the *exact* `MergeOfferCard` behavior (merge /
keep-separate against the identity spine); only Video Title omits it entirely (no identity
spine, no conflict state reachable — its `onCommit` never returns `{conflict}}` since the
seam is a no-op checker per the spec's Non-Goals). A snippet prop lets `NameEditControl` own the
open/close/positioning of the conflict state while the caller supplies what's *inside* it,
without `NameEditControl` importing `MergeOfferCard` directly (keeps the dependency direction
caller → control → nothing, not control → verdict-component). Default `undefined` — Video Title
simply omits the prop, and `NameEditControl`'s internal state machine skips the conflict state if
`onCommit` never resolves with `{conflict}`.

```ts
// MergeOfferCard.svelte — web/src/lib/components/entity/
let {
  entityType,    // EntityKind — 'person' | 'studio' | 'tag'
  current,       // EntityRef — the entity being renamed (id, name, video_count)
  conflict,      // EntityRef — the colliding entity
  onmerge,       // () => Promise<void> — calls api.mergeEntities, current absorbs conflict
  onkeepseparate // () => Promise<void> — calls api.dismissDuplicate
}: {
  entityType: EntityKind;
  current: EntityRef;
  conflict: EntityRef;
  onmerge: () => Promise<void>;
  onkeepseparate: () => Promise<void>;
} = $props();
```

Matches `AliasPanel`'s existing `mergeConflict`/`conflict = null` split exactly (same two verbs,
same async shape) — the extraction changes where this markup lives, not its behavior.

## Design Tokens Used

All from the existing token set (`.claude/rules/frontend-theming.md`) — no new tokens.

| Token | Usage |
|---|---|
| `rounded-theme` | Pencil button, inline input, card container |
| `border-rule` | Pencil button border, card border, input border at rest |
| `text-muted` | Pencil glyph at rest, helper copy |
| `text-ink` | Entity name, card body text |
| `text-warn` | Error copy (rename failure, non-conflict) |
| `bg-surface` / `bg-surface-2` | Card body / nested inputs — `bg-surface-2` for `MergeOfferCard` (matches `AliasPanel`'s `conflict` card), `bg-surface` for the inline rename input |
| `btn-accent` | Save / "Yes, merge them in" (primary action) |
| `btn-quiet` | Cancel |
| `hover:border-accent hover:text-ink` | Pencil hover treatment (exact match to `categories/[id]`'s existing pencil, `+page.svelte:201`) |
| `focus:border-accent focus:outline-none` | Inline input focus ring |
| `skin-title` | Unaffected — sits on the same `<h1>` `NameEditControl` docks beside, not inside |

## Components

| Component | Variant | Props | Notes |
|---|---|---|---|
| `NameEditControl` | at-rest, editing, committing, conflict | see contract above | New. Files under `entity/` (consumed by person/studio/tag/media routes — meets the folder's "2+ entity types" rule) |
| `MergeOfferCard` | choice, merge-confirming, keep-separate-confirming, error | see contract above | Extracted from `AliasPanel.svelte:303-326`. Files under `entity/` |
| `AliasPanel` | unchanged except: Rename form (`:204-257`) removed, `conflict`-card markup (`:303-326`) replaced by `<MergeOfferCard>` | `allowRename` prop **removed** (dead once Studio's rename moves to `NameEditControl`) | Existing component, modified |

## States and Interactions — `NameEditControl`

| State | Trigger | Behavior |
|---|---|---|
| At rest, non-owner | default | Renders `{name}` only — no pencil in the DOM at all (not `display:none`; not rendered), so a visitor's markup is identical to today's plain `<h1>{name}</h1>` |
| At rest, owner, not hovering | default | Pencil rendered but `opacity: 0` (see Animation) — present in the DOM for keyboard reachability, invisible until interaction |
| At rest, owner, hover/focus | `:hover` / `:focus-within` on the name row | Pencil fades to `opacity: 1`, border/text shift to `hover:border-accent hover:text-ink` |
| Editing | click/Enter on pencil | Name swaps for an inline `<input>` pre-filled + auto-selected (matches `categories/[id]` and `AliasPanel`'s existing rename-input pattern exactly — `bind:this` + `.select()` on open), Save (`btn-accent`) + Cancel (`btn-quiet`) buttons appear |
| Committing | Save clicked | Input + both buttons `disabled`, Save label swaps to a busy string ("Saving…", matching `AliasPanel`'s `{renameBusy ? 'Renaming…' : 'Rename'}` idiom) |
| Success | `onCommit` resolves `{ok: true}` | Reverts to at-rest with the new name; no toast (matches existing rename flows, which are silent on success) |
| Conflict | `onCommit` resolves `{conflict}` | Editing state closes; `verdict` snippet renders in its place (inline, same position the input occupied — not a modal, per spec P0) |
| Error (non-conflict) | `onCommit` rejects | Stays in editing state; error text (`text-warn`) appears below the input, same placement as `AliasPanel`'s `#rename-error` |

## States and Interactions — `MergeOfferCard`

| State | Trigger | Behavior |
|---|---|---|
| Choice | conflict received | Card shows `conflict.name` + video count + "is already a separate {noun}. Are they the same as {current.name}?" (verbatim copy from `AliasPanel:305-308`) with two actions |
| Merge-confirming | "Yes, merge them in" clicked | Button disables, label busies (no separate confirm step *within* the card — the card itself already functions as the informed confirm per RD8, since it names both entities and their video counts before any button is clickable) |
| Keep-separate | "No, keep separate" clicked | Immediate — calls `onkeepseparate`, no further prompt (same as today) |
| Busy | either action pending | Both buttons `disabled:opacity-60` (existing pattern on `bg-accent`/`border-rule` buttons — **not** `text-muted`, so this does not trip the theming rule's contrast-bug check) |
| Error | either call rejects | `text-warn` message below the actions, card stays open for retry |

## Edge Cases

- **Empty/whitespace-only input**: Save is a no-op (mirrors every existing rename form's
  `const next = value.trim(); if (!next) return;` guard) — no error shown, since this is a
  no-op, not a failure.
- **Unchanged name submitted**: no-op, closes back to at-rest without a network call (mirrors
  `AliasPanel.submitRename`'s `if (next === entityName) { cancelRename(); return; }`).
- **Long names**: no truncation inside the input (it's an edit surface, not a display surface);
  the at-rest `<h1>` keeps whatever wrapping behavior it has today — `NameEditControl` doesn't
  change the heading's own typography or overflow rules.
- **Slow network on commit**: the busy state has no timeout — stays disabled until the promise
  settles, consistent with every other async form in this codebase (no optimistic rename, since a
  409 must be caught before the name changes).
- **Video Title's missing conflict path**: if `onCommit` is ever wired without a `verdict` snippet
  and the promise still resolves `{conflict}` (a caller bug, not a real state for Video today),
  `NameEditControl` should fail loudly in dev (console error) rather than silently drop the
  conflict — cheap guard against a future HOLODEX-270 wiring mistake.

## Animation / Motion

| Element | Trigger | Animation | Duration | Easing |
|---|---|---|---|---|
| Pencil opacity | hover/focus-within on name row | `opacity 0 → 1` | 120ms | `ease` |

Reuses the exact mechanism already in `app.css` for `.curation-actions` (`opacity:0` →
`:hover`/`:focus-within` → `opacity:1`, `@media (hover:none)` forces `opacity:1` for touch) — do
**not** reuse the literal `.curation-chip`/`.curation-actions` class names on the name row,
though, since those names carry F30 chip-specific meaning elsewhere in the codebase and this is a
different element (an `<h1>` row, not a chip). Add a new interaction-only pair with the identical
three rules — e.g. `.name-edit-row` / `.name-edit-pencil` — right beside `.curation-actions` in
`app.css`, same "no colors/tokens, not skin-gated" comment.

## Accessibility Notes

- Pencil button: `aria-label="Rename this {label}"` (e.g. "Rename this person") — always present
  in the accessibility tree for an owner even at `opacity: 0`, so keyboard/AT users don't lose
  the control to a hover-only affordance (matches `.curation-actions`' existing "always in DOM,
  visually hidden until interaction" contract, not `display:none`/`visibility:hidden`).
- Focus order: pencil sits immediately after the name in DOM order (matches `categories/[id]`'s
  existing layout) so Tab reaches it right after the heading, not at the end of the page.
- Inline input: `aria-label="Rename this {label}"`, `aria-describedby` pointing at the error
  paragraph when present (exact pattern from `AliasPanel.svelte:231-232`).
- `MergeOfferCard`: `aria-live="polite"` on the card container (matches `AliasPanel:304` and its
  near-miss card at `:331`) so screen readers announce the collision without an extra interaction.
- On successful commit, focus returns to the pencil button (now showing the new name) — not lost
  to a removed input, matching this codebase's existing focus-restoration discipline
  (`ConfirmDialog`, `tags/[id]`'s reparent-confirm flow).

## Cross-context notes

- **Person**: `name` is removed from the generic resolved-fields `SourceSelect` list (Tier-1
  exclusivity per the spec) — `NameEditControl`'s `onCommit` calls `api.renameEntity('person', id,
  value)` directly, same primitive `AliasPanel`'s rename form already calls for Studio.
- **Studio**: `AliasPanel`'s `allowRename` prop and its Rename form/button (`:204-211`, `:225-257`)
  are deleted; `NameEditControl` is mounted beside the studio's own `<h1>` instead, with
  `AliasPanel`'s `conflict` prop wiring redirected through `NameEditControl`'s `verdict` snippet.
- **Tag**: mounts beside `tags/[id]/+page.svelte:303`'s existing `<h1>{tag?.name}</h1>` — nothing
  else on that page changes (Hierarchy/Categories/Details cards are untouched).
- **Video Title**: `onCommit` calls the existing `PUT /media/{id}/fields/title/decision`
  (`source: 'manual'`); no `verdict` snippet passed (see Component contract above) — `title` is
  removed from the generic Metadata `SourceSelect` list, same Tier-1-exclusivity treatment as
  Person's `name`.

## Three-skin QA checklist (before this ships)

1. Pencil `opacity:0→1` transition and `hover:border-accent` are legible in Cinémathèque,
   Broadcast, and Brutalist — the accent color differs enough per skin that a low-contrast pencil
   in one skin is a realistic regression.
2. `MergeOfferCard`'s `bg-surface-2` reads as distinct from its parent `bg-surface` card in all
   three skins (mirrors the existing `AliasPanel` conflict card, which already passes this today —
   regression check only).
3. Keyboard-only pass: Tab to pencil → Enter opens edit → Tab through input/Save/Cancel → Escape
   or Cancel returns focus to the pencil.
