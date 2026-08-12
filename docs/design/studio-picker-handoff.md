# Design Handoff: Studio relationship-edit popover (HOLODEX-271)

Spec: [`docs/specs/studio-relationship-popover.md`](../specs/studio-relationship-popover.md) ·
Epic: HOLODEX-267

## Overview

Replaces `SourceSelect`'s Studio radiogroup on the video detail page with a popover that does
three things the radiogroup can't: search the full studio library, create a studio inline, and
(via the shared collision check, HOLODEX-270) warn before committing a reassignment that would
make the video an exact composite-key duplicate of another active video.

New component: **`StudioPicker.svelte`** (`web/src/lib/components/entity/`, alongside
`EntityPickerDialog.svelte` and `NameEditControl.svelte` it's built from). It is *not* a new
picker mechanism — it composes three things that already exist:

- **The docked-pencil open/close/busy/error/conflict state machine** from `NameEditControl.svelte`
  (:42-101) — same shape, but the "editing" surface is a popover instead of an inline text field.
- **The debounced search + inline create-fallback body** from `EntityPickerDialog.svelte`
  (:67-96, :216-225), called with `kind="studio"`.
- **The `PickerShell` dialog chrome** (backdrop, focus trap, Escape, rise-in animation) that
  `EntityPickerDialog` currently hand-rolls instead of using — this story has `StudioPicker` use
  `PickerShell` directly (the shared chrome `EntityPicker`/`CategoryPicker` already use), rather
  than copying `EntityPickerDialog`'s duplicate backdrop/trap markup a third time.
- **The `CollisionOfferCard` verdict slot** from HOLODEX-270 — a hit renders inline in the page,
  in the same conflict slot `NameEditControl` already uses for Title, not inside the popover.

## Layout

Trigger: same visual language as `NameEditControl`'s pencil — an icon-only button, invisible at
rest to a non-owner or a not-yet-interacting owner, revealed on owner hover/focus via the existing
`.name-edit-row`/`.name-edit-pencil` hover-reveal CSS (`app.css`). Placed where `SourceSelect`
currently renders for the Studio field (`media/[id]/+page.svelte:690`), replacing it outright for
this field only — other resolved fields keep `SourceSelect` unchanged.

Popover: `PickerShell`'s standard dialog frame (`max-w-lg`, `rounded-theme border border-rule
bg-surface p-4 shadow-xl`, centered top-anchored at `py-[10vh]`), containing, top to bottom:

1. **Header** (`PickerShell`'s `header` snippet): `<h2>Change studio</h2>` + the shared `✕` close
   button `PickerShell` already renders.
2. **Known-candidates row** — one chip per distinct value in `field.values` (today's
   `SourceSelect` candidate set: file-declared baseline + provider-declared values), each showing
   its source tag exactly as `SourceSelect`'s chips do today (reuse that chip's visual treatment,
   not a new one). Omitted entirely if there is only one candidate (the currently-resolved value)
   — no point showing a single redundant chip above the search box.
3. **Search input** — `EntityPickerDialog`'s exact debounced-search body (`kind="studio"`,
   300ms debounce, `GET /search` via `api.search(q)`), including its `role="combobox"` + `listbox`
   roving-tabindex result list and its "type at least two characters" / "N matches" / "no matches"
   status line.
4. **Create-new fallback row** — `EntityPickerDialog`'s existing `Use "{query}" as a new studio`
   row (:216-225), verbatim behavior: shown only when the query is 2+ characters and doesn't
   exactly match a search result.

## Design Tokens Used

| Token | Usage |
|---|---|
| `bg-surface` / `border-rule` / `rounded-theme` | Popover frame (via `PickerShell`, already tokenized — no new values needed) |
| `text-ink` / `text-muted` | Chip/result label text, status line |
| `border-accent` / `bg-surface-2` | Active/focused chip and roving-tabindex result row (matches `EntityPickerDialog`'s existing active-option treatment) |
| `.btn-accent` | N/A inside the popover — every action here is a single-click select, not a two-step confirm, so no `.btn-accent`/`.btn-ghost` pair is needed in the popover body itself |
| `.btn-accent` / `.btn-ghost` | Reused as-is by `CollisionOfferCard` (HOLODEX-270) for the verdict panel, unchanged by this story |

No new colors or spacing values — every surface here already has a token-backed precedent in
`EntityPickerDialog`/`PickerShell`/`SourceSelect`.

## Components

| Component | Variant | Props | Notes |
|---|---|---|---|
| `StudioPicker` (new) | — | `field: ResolvedField`, `isOwner: boolean`, `decide: (source: string, manualValue?: string) => Promise<{ok:true}\|{conflict:VideoCollisionRef}>` | Owns pencil/open/busy/error/conflict state; renders `PickerShell` when open, `CollisionOfferCard` (via the same mechanism `NameEditControl` uses) when a commit 409s |
| `PickerShell` (existing, reused) | — | `titleId`, `onclose`, `header`, `children` | No changes — `StudioPicker` is simply a third consumer alongside `EntityPicker`/`CategoryPicker` |
| Candidate chip (new, inline in `StudioPicker`) | source-tagged | `label`, `sourceTag`, `onclick` | Visually mirrors `SourceSelect`'s existing chip (same classes), not a new visual language |
| Search body (ported from `EntityPickerDialog`) | `kind="studio"` | — | Behavior identical to today's Extraction-tab picker; `StudioPicker` inlines this logic rather than nesting `EntityPickerDialog` as a child component, since the two need to share one `PickerShell` frame instead of stacking two dialogs |
| `CollisionOfferCard` (existing, unchanged) | — | `collision: VideoCollisionRef`, `onViewExisting`, `onSaveAnyway`, `busy` | Same component HOLODEX-270 built for Title; Studio reuses it verbatim |

## States and Interactions

| Element | State | Behavior |
|---|---|---|
| Pencil trigger | At rest, non-owner or non-hovering owner | Invisible (opacity 0, same as `NameEditControl`'s pencil) |
| Pencil trigger | Owner hover/focus | Brightens; `aria-label="Change this video's studio"` |
| Pencil trigger | Click | Opens `PickerShell` popover, focuses the search input |
| Candidate chip | Click | Commits immediately (`decide(sourceTag)`) — same one-click speed as today's `SourceSelect` |
| Search result row | Click / Enter (roving tabindex, arrow keys) | Commits `decide('manual', result.name)` |
| Create-new row | Click / Enter with no exact match | Commits `decide('manual', query.trim())` — same call shape as picking a search result; the studio row is created implicitly by the existing `ReconcileVideoStudios` relink path, no separate create call |
| Any commit | Busy | Popover shows a disabled/loading state on the clicked option only (mirrors `NameEditControl`'s `busy` — the rest of the popover stays interactive-looking but the in-flight click is visually pending) |
| Any commit | Success | Popover closes, focus returns to the pencil, studio links on the page update from the new resolved value |
| Any commit | 409 conflict | Popover closes; `CollisionOfferCard` renders inline in the page's conflict slot (same slot Title uses), showing the colliding video; pencil stays focusable so the owner can retry after resolving |
| Any commit | Network/validation error (non-conflict) | Popover stays open, shows the same inline `text-warn` error line `EntityPickerDialog` already uses for search errors |
| "Save anyway" (on `CollisionOfferCard`) | Click | Re-submits `decide('manual', pendingValue, {override: true})`, commits normally, dismisses the card |
| "View existing video" (on `CollisionOfferCard`) | Click | Navigates to `/media/{collision.id}`; the pending studio change is discarded (no commit) |

## Responsive Behavior

No distinct breakpoints — `PickerShell`'s existing `max-w-lg` + `px-4` frame is already
responsive (matches `EntityPicker`/`CategoryPicker`, which ship with no dedicated mobile variant
today). No new responsive work needed.

## Edge Cases

- **Zero candidates beyond the current value**: candidate row is omitted; popover opens straight
  to the search field (see Layout §2).
- **Studio already resolved to a value the owner re-selects**: `decide` is still called (matches
  `SourceSelect`'s existing behavior — no client-side short-circuit on "same value").
  Server-side, `setFieldDecision` no-ops safely on an unchanged value (existing behavior, not
  changed by this story).
- **Search query matching an existing studio exactly**: create-new row is suppressed (existing
  `EntityPickerDialog` behavior, :216, case-insensitive exact match).
- **Very long studio name in a candidate chip or result row**: `truncate` (existing pattern in
  `EntityPickerDialog`'s result rows, :207) — no new truncation logic needed.
- **Collision on a create-new commit** (new studio name happens to complete a composite-key match
  with another video): same 409/verdict flow as any other commit path — no special case.

## Accessibility Notes

- Focus order: pencil → (on open) search input, focus-trapped inside `PickerShell` exactly as
  `EntityPicker`/`CategoryPicker` already trap it.
- Candidate chips and search results are both single-row, single-select lists — search results
  keep `EntityPickerDialog`'s existing roving-tabindex (`role="listbox"`/`role="option"`,
  arrow-key navigation); candidate chips are a small enough set (typically 1-3) to use plain
  `Tab` order rather than a second roving-tabindex group, avoiding two different keyboard models
  in one popover.
- `Escape` closes the popover (via `PickerShell`, unchanged).
- On close (any path — success, cancel, Escape), focus returns to the pencil trigger, matching
  `NameEditControl`'s existing `focusPencil()` behavior.
- `CollisionOfferCard`'s existing accessibility treatment (HOLODEX-270) is unchanged — this story
  is just a second caller.

## QA

Tokens-only, three-skin QA (Cinémathèque, Broadcast, Brutalist) per
`.claude/rules/frontend-theming.md` — candidate chips and search/create rows are new markup and
need the same contrast/hover-state check `SourceSelect` and `EntityPickerDialog` already pass.
