# Design Handoff: People attach/detach + relationship picker (HOLODEX-272)

Spec: [`docs/specs/people-relationship-picker.md`](../specs/people-relationship-picker.md) · Epic: HOLODEX-267

## Overview

Two changes to the video detail page's People section
(`web/src/routes/media/[id]/+page.svelte:902-923`):

1. **Grid fix** — the section guard becomes `{#if isOwner || video.people?.length}` (matching
   Tags/Studio), and each `PersonPoster` card gets a Tags-style hover-reveal remove control. An
   owner with zero people now sees the section (with a lone "Add person" tile) instead of nothing.
2. **New `PersonPicker.svelte`** — a docked "Add person" tile opens a `PickerShell` popover. Unlike
   `StudioPicker` (single-select, one field), this is **multi-select**: it shows the people already
   attached (each independently removable) alongside search/create, and every attach requires an
   explicit Actor/Director role choice (spec § Resolved Decisions #2).

Structural precedent: `StudioPicker.svelte` (299 lines) for the debounced-search/create-fallback/
collision-verdict-slot shape; `PickerShell.svelte` for dialog chrome (focus trap, Escape,
backdrop). Both are reused as-is where possible — only the body (candidate list + role UI) is new.

## Layout

The picker is a single popover, not a two-pane layout — same footprint class as `StudioPicker`'s
(`PickerShell` sizes itself to content, typically ~360px wide). Three stacked regions inside:

1. **Attached list** — chips, one per currently-linked person, each showing name + role badge +
   a remove `×`. Wraps to multiple lines; no scroll container (a video's people list is small in
   practice — dozens at most, and the spec's P1 nice-to-have is a "view all" affordance if that
   assumption breaks, not a change to this layout).
2. **Search** — the same `role="combobox"` input + debounced (300ms) `<ul role="listbox">` pattern
   as `StudioPicker`, including the create-fallback row when the query has no exact match.
3. **Commit-time role control** — see States and Interactions below; it lives inline in each result
   row, not as a separate section.

## Design Tokens Used

All values are Holodex's existing semantic tokens (`web/src/app.css`) — no new tokens introduced.
Cinémathèque (default) values shown; Broadcast and Brutalist substitute their own skin values for
the same variable names, and the component must reference the variables, never the literals below.

| Token | Cinémathèque value | Usage |
|---|---|---|
| `--surface-2` (`bg-surface-2`) | `#181310` | Popover body, attached chips, search field bg |
| `--rule` (`border-rule`) | `#2a2622` | Popover border, chip border, dashed add-tile border |
| `--ink` (`text-ink`) | `#f3ece1` | Chip text, result-row text |
| `--muted` (`text-muted`) | `#9b9082` | Placeholder text, role badge (unselected), add-tile idle state |
| `--accent` (`bg-accent`/`text-accent`) | `#e8a33d` | Selected role pill, add-tile hover, "add as..." label |
| `--accent-ink` (`text-accent-ink`) | `#1a1206` | Text on the selected role pill |
| `--warn` (`text-warn`/`border-warn`) | `#e2603f` | Collision verdict only (via `CollisionOfferCard`, unchanged) |
| `rounded-theme` | skin-dependent radius | Poster frame corners, popover corners |
| `font-display` | `'Fraunces Variable'` | Popover header ("Add person") |
| `font-ui` | `'Archivo Variable'` | Everything else (labels, input, chips) |

## Components

| Component | Variant | Props | Notes |
|---|---|---|---|
| `PersonPoster.svelte` | unchanged | `personId, name, version?, eager?` | No component change — the remove control is call-site markup around it, same as `PersonPoster` is already just a wrapper the caller composes around |
| New: hover-remove overlay | — | — | Not a component — inline markup at the `+page.svelte` call site, styled with the same `.curation-chip`/`.curation-actions`-derived hover-reveal pattern Tags uses (`web/src/routes/media/[id]/+page.svelte:821-843`), adapted from a chip shape to an absolutely-positioned corner badge on the poster frame (see mockup) |
| New: `PersonPicker.svelte` | — | `people: ResolvedPerson[], isOwner: boolean, attach: (personId, role, source, manualValue?) => Promise<{ok:true}\|{conflict: VideoCollisionRef}>, detach: (personId) => Promise<{ok:true}\|{conflict: VideoCollisionRef}>, verdict?: Snippet<[VideoCollisionRef, () => void]>` | Composes `PickerShell` for chrome; internal state (`open`, `query`, `candidates`, `busyKey`, `conflict`) mirrors `StudioPicker`'s shape 1:1, extended with `attachedRole: Map<personId, 'actor'\|'director'>` and a `pendingRole` per in-flight result row |
| Reused as-is: `PickerShell.svelte` | — | `titleId, onclose, dialogEl` (bindable) | No change — generic chrome already supports this |
| Reused as-is: verdict slot mechanism | — | `verdict?: Snippet<[VideoCollisionRef, () => void]>` | Same pattern as `StudioPicker` — `{#if conflict && verdict}{@render verdict(...)}{/if}` sits as a sibling after the `{#if open}` block, not nested inside it |

## States and Interactions

### Role choice — resolved direction

The mockup compared two shapes: **(A)** a single Actor/Director segmented toggle above the search
box that sets the mode for every subsequent pick, versus **(B)** a role choice inline on each
result row, defaulting to Actor. **Build (B).** Rationale: a video's people list routinely mixes
roles in one sitting (a few actors, one director) — (A) forces a toggle-search-toggle-search loop
to add both, while (B) lets an owner add every candidate in one pass without leaving the search
results. (B) also keeps the role decision co-located with the specific person it applies to,
which reads more directly than a global mode the owner has to remember they're in. Default every
row to "Actor" (the common case) so the one-click path stays one click; Director is one more click
away, never pre-selected.

| Element | State | Behavior |
|---|---|---|
| `PersonPoster` card (grid) | Rest | Identical to today — no visible chrome beyond the existing frame + caption |
| `PersonPoster` card (grid) | Hover/focus (owner only) | Remove `×` fades in, top-right corner of the frame, `opacity 0→1` over `120ms` |
| Remove `×` | Click | `busyKey` locks to that person id, button shows a spinner glyph in place of `×`; on success the card is removed from the grid; on failure, `×` returns and an inline error is shown below the grid (same pattern as Tags' `tagBusy`/error paragraph) |
| "Add person" tile | Rest | Dashed `border-rule` outline, `+` icon + "Add person" label in `text-muted`, sized to match the poster grid cell |
| "Add person" tile | Hover/focus | Border and label switch to `text-accent`/`border-accent` |
| "Add person" tile | Click | Opens `PersonPicker` popover via `PickerShell` |
| Attached-person chip | Rest | Name + small role badge (`text-muted`) + `×` |
| Attached-person chip | Click `×` | Same commit path as the grid's remove control (this chip is the picker's own view of the same data — detaching here or from the grid behind it produces the same call) |
| Search input | Typing | 300ms debounce, identical to `StudioPicker.onInput` |
| Search input | No results, non-empty query | Renders the create-fallback row ("Use "…" as a new person"), same as `StudioPicker` |
| Result row | Rest | Name + role-pill pair (`Actor` / `Director`), `Actor` pre-selected |
| Result row | Click a role pill | Toggles that row's selection only — does not affect other rows or previously-committed attaches |
| Result row | Click the row itself (outside the pills) | Commits attach with whichever role is currently selected on that row; `busyKey` locks that row, row shows a spinner |
| Commit (attach or detach) | Success | Attached list updates in place; search stays open so more people can be added in the same session (this is the multi-select difference from `StudioPicker`, which closes on commit) |
| Commit (attach) | 409 collision | Same handoff `StudioPicker` already uses: `conflict` is set, the picker closes, `verdict` snippet renders `CollisionOfferCard` inline where the popover was |
| Commit | Network/validation error | Inline `commitError` paragraph inside the popover, same styling as `StudioPicker`'s |
| Escape / backdrop click | — | Closes the popover, unchanged `PickerShell` behavior |

## Responsive Behavior

No new breakpoint logic — `PersonPicker` inherits `PickerShell`'s existing responsive chrome
(same popover sizing/positioning behavior as `StudioPicker` at all viewport widths). The People
grid itself already reflows via its existing `grid-cols-3 sm:grid-cols-4 md:grid-cols-6`; the new
"Add person" tile is simply one more grid item and reflows with it.

| Breakpoint | Changes |
|---|---|
| Desktop (≥768px) | Grid at `md:grid-cols-6`; popover anchored near the "Add person" tile |
| Tablet (640–767px) | Grid at `sm:grid-cols-4`; popover unchanged |
| Mobile (<640px) | Grid at `grid-cols-3`; remove `×` cannot rely on hover — always-visible at reduced opacity (`opacity-70`, same convention as Tags' touch fallback, not a new pattern) |

## Edge Cases

- **Zero people, owner viewing**: section renders with only the "Add person" tile (this is the
  bug fix — today the section doesn't render at all).
- **Zero people, non-owner viewing**: section renders nothing, same as today (guard is
  `isOwner || video.people?.length`).
- **Removing the last attached person**: no special-case confirmation — matches Tags' remove
  behavior, which also has no "are you sure."
  Attach/detach are already reversible one-click actions.
- **Adding a person who collides** (would produce a duplicate composite key with another video):
  handled entirely by the existing `verdict`/`CollisionOfferCard` mechanism — no new UI for this
  component to design.
- **Same person as both Actor and Director**: not one row with two roles — `video_people`'s PK is
  `(video_id, person_id, role)` (ADR-072), so this is two separate attach actions, each producing
  its own row in the attached list with its own role badge and its own remove `×`.
- **Search returns a person already attached**: result row shows a "already attached as {role}"
  state instead of role pills — clicking it is a no-op (use the attached-chip's `×` to change or
  remove instead of re-adding).
- **Long name overflow** (chip or grid caption): existing `line-clamp-2`/ellipsis handling
  carries over unchanged from Tags/the current People grid.

## Animation / Motion

| Element | Trigger | Animation | Duration | Easing |
|---|---|---|---|---|
| Remove `×` (grid card) | Hover/focus in | Opacity fade | 120ms | ease-out |
| Popover | Open | `PickerShell`'s existing `merge-rise` keyframe (already gated by `prefers-reduced-motion: no-preference`) | unchanged | unchanged |
| Result row commit | Busy | Row dims slightly, spinner replaces role pills | unchanged (matches `StudioPicker` busy treatment) | — |

## Accessibility Notes

- Remove `×` on each grid card: `aria-label="Remove {name}"`, always present in the DOM (not just
  on hover) so keyboard/screen-reader users can reach it via Tab — visual fade-in is cosmetic
  only, never a focus gate (same rule the Tags chips already follow).
- "Add person" tile: real `<button>`, `aria-haspopup="dialog"`.
- Search input: `role="combobox"`, `aria-expanded`, `aria-controls` pointing at the results
  `<ul role="listbox">` — identical wiring to `StudioPicker`.
- Result-row role pills: grouped with `role="group" aria-label="Role for {name}"`; each pill is a
  real `<button aria-pressed="true|false">`, not a div — keyboard-operable without relying on the
  mouse-only click-row-to-commit gesture (Enter on a focused pill toggles it; a separate "Add"
  action or Enter-on-row commits — do not overload one keystroke for both).
- Attached-person chip `×`: `aria-label="Remove {name} ({role})"`.
- Focus trap, Escape-to-close, and return-focus-to-trigger are inherited unchanged from
  `PickerShell` — no new work, just don't bypass it.
- 3-skin QA required before this gate closes (Cinémathèque, Broadcast, Brutalist) per
  `.claude/rules/frontend-theming.md` — the role-pill selected state (`bg-accent`/`text-accent-ink`)
  is the one new color pairing this component introduces; verify contrast in all three.
