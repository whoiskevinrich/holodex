# Design handoff: person & studio link picker + role badge (F40)

**Spec:** [person-media-linking.md](../specs/person-media-linking.md) · **ADR:** [ADR-059](../architecture/ADR-059-person-link-resolved-derivation.md) · **Jira:** [HOLODEX-114](https://whoiskevinrich.atlassian.net/browse/HOLODEX-114)
**Date:** 2026-07-04 · **Status:** Draft handoff

Two surfaces: **(1) the link picker** — an owner-only entity-search combobox that replaces the bare
"+ Add" text input on person-typed and studio fields; **(2) the role badge** — the person page grouping
its videos by the derived `video_people.role`. Everything reuses existing components and **semantic
tokens only** (ADR-021); QA all three skins.

---

## Overview & context

Today the F30 "+ Add" affordance ([`CurationFieldRow.svelte:139`](../../web/src/lib/components/CurationFieldRow.svelte:139))
is a bare `<input>`: the owner types a free-text string and presses Enter. For **linking a person or
studio** that's lossy — a typo mints a near-duplicate entity and the owner can't see who already exists.
The picker replaces that input with a **search-backed combobox**: type → see matching existing entities
(with disambiguation) → pick one, or **inline-create** a name-only entity (RD10). Either way the picker
submits the chosen entity's **canonical name** as a curation `add`; `RelinkVideoEntity` then derives the
`video_people` / `video_studios` link. The picker never writes the link table directly — it writes a
field value, exactly like the current "+ Add".

## Layout & anchoring — a non-modal combobox popover (not a modal)

`EnrichPicker` ([`EnrichPicker.svelte`](../../web/src/lib/components/EnrichPicker.svelte)) is the closest
sibling and the **source of the keyboard model**, but it is a full-screen `role="dialog"` modal. For a
per-field "add one credit" gesture a modal is too heavy. The picker is an **anchored popover** attached
to the "+ Add" button, using the WAI-ARIA **combobox + listbox** pattern:

- Trigger: the existing "+ Add" pill. Activating it opens the popover in place (replacing the bare input).
- Popover: `bg-surface`, `border border-rule`, `rounded-theme`, `shadow-xl`, `max-w` ~20rem, anchored
  below the trigger (flips above if it would clip the viewport). Same `enrich-rise` 0.15s scale/opacity
  entrance, gated by `prefers-reduced-motion` (copy from EnrichPicker).
- Contents (top→bottom): search `<input role="combobox">` · `aria-live` status line · results
  `<ul role="listbox">` · a persistent **Create "<query>"** row.
- **Not** a trapped modal: no backdrop, Tab is *not* trapped (it moves to the create row then out of the
  popover naturally), Esc closes and returns focus to "+ Add", click-outside closes.

## Design tokens (semantic only — never hex in components)

| Token | Usage |
|---|---|
| `bg-surface` | popover background, input background |
| `bg-surface-2` | active/hovered result row fill, chip fill |
| `border-rule` | popover border, input border, chip border, create-row divider |
| `border-accent` / `text-accent` | focused input border, active-row left border (`border-l-2`), create-row icon+text, provenance on provider values |
| `text-ink` | result name, input text, chip value |
| `text-muted` | disambiguation subline, status hint, `·record`/`·manual` provenance, unset role tag |
| `text-warn` / `border-warn` | error status line only (never accent for errors) |
| `rounded-theme` | popover, input, result rows (radius flips per skin: 2px Cinémathèque, 0 Broadcast/Brutalist) |
| `rounded-full` | chips and the "+ Add" pill (intentional pill shape, per theming rules) |
| `font-display` (`skin-title`) | field label; person-page name header |
| `font-ui` | input, result rows, hints |

The three skins resolve these to: Cinémathèque gold `#e8a33d` / serif+sans / 2px; Broadcast cyan
`#36e0d0` / mono / 0px; Brutalist lime `#d6ff3f` / mono / 0px. Components must reference the tokens, so
all three fall out for free — the mockup shows all three rendered from the same markup.

## Components

| Component | Reuse / change | Notes |
|---|---|---|
| `LinkPicker.svelte` (**new**) | models on `EnrichPicker` | Combobox popover; props `{ kind: 'person' \| 'studio', query?, search, onpick, oncreate, onclose }`. `search(q)` → `GET /api/v1/people?q=` or `/studios?q=`; `onpick(name)` / `oncreate(name)` both emit a **canonical name** the parent adds via the curation endpoint. |
| `CurationFieldRow.svelte` | **modify** | On person-typed/studio fields (owner), the "+ Add" opens `LinkPicker` instead of the inline text input. Gate on the field's `entity` marker; non-entity fields keep the plain input. |
| `CurationChip.svelte` | **reuse as-is** | Already links person values to `/people/{id}` and renders `·record`/`·provider`/`·manual` provenance. Studio values gain the same link treatment to `/studios/{id}`. |
| result row | new (inside `LinkPicker`) | `role="option"`, roving `tabindex`, avatar (headshot for person / building glyph for studio; silhouette placeholder when no image), name (`text-ink`), disambiguation subline (`text-muted`). |
| role badge | new, on person page | Small tag; see below. |

## States & interactions

| Element | State | Behavior |
|---|---|---|
| "+ Add" pill | rest | `border-rule text-muted`; hover/focus → `text-accent border-accent` (current F30 behavior) |
| "+ Add" pill | activated | Opens the popover; input autofocused, empty (or seeded if invoked from a chip's value) |
| Input | typing | Debounced 300ms; < 2 chars → no call, hint "Type at least two characters"; ≥ 2 → search |
| Result row | active (roving) | `border-l-2 border-accent bg-surface-2`; the lone `tabindex="0"` in the list |
| Result row | hover / focus | Becomes active (mouse-enter and focus both set active), mirroring EnrichPicker |
| Result row | activate (Enter/Space/click) | Calls `onpick(canonicalName)`; parent adds curation → reload; popover closes; focus returns to "+ Add" |
| Create row | always present | Label `Create "<trimmed query>"`; disabled/hidden only when query < 2 chars; activate → `oncreate(query)` |
| Popover | loading | `aria-live` hint "Searching people…" + shimmer rows |
| Popover | empty | Hint "No people match "<q>"." and the Create row is the active option |
| Popover | error | `aria-live` hint in `text-warn` "Couldn't reach search. Retry"; Create row still available |
| Popover | Esc / click-outside | Close, no change, focus returns to trigger |

## Keyboard & accessibility (mirror EnrichPicker, minus the trap)

- Input is `role="combobox"`, `aria-expanded`, `aria-controls={listId}`; list is `role="listbox"`; rows
  are `role="option"` with `aria-selected` on the active one.
- **Roving tabindex** ([`EnrichPicker.svelte:141`](../../web/src/lib/components/EnrichPicker.svelte:141)):
  the active row is the sole `tabindex="0"`; ↓ from the input enters the list; ↑ from the first row
  returns to the input; ↓ past the last result lands on the Create row; Enter/Space activate.
- **Focus return** on close (Esc, pick, create, click-outside) → the "+ Add" trigger (EnrichPicker's
  `onMount` return pattern).
- Not modal → no focus trap and no `aria-modal`; the page behind stays inert only visually.
- Every icon-only control has an `aria-label`; the status line is `aria-live="polite"`.
- Selection/active state reads via the accent **left border + `aria-selected`**, never color alone.

## Role badge (person page)

The person page groups the person's videos by the derived `video_people.role` and tags each:

- Tag chrome: `rounded-theme border px-1.5 text-[0.6rem] uppercase tracking-wide`. A **set** role
  (`Director`, `Actor`) uses `border-accent text-accent`; an **unset** role uses
  `border-rule text-muted` with the label **"Appears in"**.
- One video with two roles for the same person → two tags (the PK allows it, RD3).
- Media detail needs no per-chip role tag — role there is already the field/section (Actors vs Director).
  A person linked via a role-less field surfaces in a generic "People" area with the "Appears in" tag.

## Edge cases

- **Long names** — result name and disambiguation each `truncate` (single line, ellipsis); full value in
  `title`. Chips already truncate (`max-w-[14rem]`).
- **No headshot** — silhouette placeholder avatar (person) / building glyph (studio); never a broken img.
- **Diacritics / homonyms** — search matches by name + alias (`resolveOrCreatePerson` routing); two real
  people who share a name are **not** auto-merged — both appear as distinct rows (disambiguation subline
  is what separates them; F23 rule).
- **Already linked** — an entity already on the field is de-emphasized (or omitted) in results so the
  owner doesn't double-add; the set-merge curation is idempotent regardless.
- **Slow / offline search** — loading shimmer, then the error hint; the Create row stays usable so the
  owner is never blocked from adding.
- **Empty query** — hint only; no results, Create row inert until ≥ 2 chars.

## Responsive

| Breakpoint | Behavior |
|---|---|
| Desktop | Anchored popover below the "+ Add" pill, ~20rem wide, flips above near the viewport bottom |
| Mobile (< 640px) | Popover goes full-width within the field container; owner controls on chips are always visible (touch), consistent with F30 |

## Motion

| Element | Trigger | Animation | Duration | Easing |
|---|---|---|---|---|
| Popover | open | scale 0.98→1 + opacity (the `enrich-rise` keyframe) | 150ms | `cubic-bezier(0.2,0.7,0.2,1)` |
| — | — | all gated by `prefers-reduced-motion: no-preference` | — | — |

## Implementation notes

- Svelte 5 runes, tokens only (`rg 'zinc-|sky-|#'` over the new component must be clean).
- The picker's `onpick`/`oncreate` both funnel to the existing curation `add` transport
  (`CurationRequest { field, value: canonicalName, action: 'add' }`) — no new endpoint; the reconcile
  runs post-commit server-side.
- `GET /api/v1/people?q=` exists; add `GET /api/v1/studios?q=` (or reuse global search scoped to studios)
  for the studio kind.
- **QA all three skins** (Cinémathèque / Broadcast / Brutalist): confirm the accent left-border reads on
  each surface, the mono skins' 0-radius popover looks intentional, and the role tags don't collide with
  the accent used for active/selected state.
