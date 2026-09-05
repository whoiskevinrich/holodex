# Design Handoff: Filename metadata extraction — Extraction tab, preview, revert (F48)

**Status**: Design decided (developer handoff)
**Date**: 2026-07-14
**Spec**: [metadata-extraction.md](../specs/metadata-extraction.md) · **ADR**: [ADR-067](../architecture/ADR-067-filename-extraction-confidence-and-rollback.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**
**Stack**: SvelteKit (Svelte 5 runes) + Tailwind v4 CSS-first (ADR-025).
**QA checklist**: [metadata-extraction-qa-checklist.md](metadata-extraction-qa-checklist.md)
**Extended by**: [media-page-extraction-handoff.md](media-page-extraction-handoff.md) (2026-09-04,
[HOLODEX-194](https://whoiskevinrich.atlassian.net/browse/HOLODEX-194)) — this handoff scoped the F48
UI to the `/owner` hub, which left F48.5a's single-video trigger without a surface. That handoff adds
the media-detail-page trigger and its inline review panel.

This handoff adds one new surface — an **Extraction tab** in the `/owner` hub (F35), the third
sibling of Duplicates (F43) and Enrichment (F47), whose dense-row idiom it deliberately reuses —
plus two additive pieces: a **pre-write diff preview dialog** (extends `WritebackFormDialog.svelte`,
F28) and a **Revert action** attached to a completed batch in System Activity (ADR-028). Nothing
here introduces a new token, font, or radius.

---

## Overview

Three changes, in the spec's own build order (Phasing §5–6):

1. **Extraction tab** (`/owner/extraction`, new) — a queue of pending `metadata_extraction_review`
   rows, **grouped by video** (not by entity type like Enrichment — see Resolved: grouping below),
   each row showing one field's filename value, tag value, and (for entity fields) an advisory
   suggested match.
2. **Preview-before-write dialog** — a new mode of the existing writeback confirmation surface,
   showing an **old → new diff** per field instead of an editable value, used both for a manual
   batch of resolved rows and (contextually, skippable) for auto-applied batches per F48.7b.
3. **Revert** — one button on a completed extraction (or merge-writeback) batch's activity-history
   entry, per F48.9d — no new page, no hunting for a batch id.

All three are **owner-gated** (`activity.effectiveOwner`, ADR-030).

---

## Design-system fit (the `/design-system` check)

Almost nothing new visually — this composes three components that already ship:

- **Tab shell** — add one entry to `owner/+layout.svelte`'s `tabs` array (`web/src/routes/owner/+layout.svelte:13-19`). No new chrome.
- **Grouped dense rows** — `DuplicatePairRow`/`EnrichQueueRow`'s exact rhythm (`flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-rule px-3 py-2.5 text-sm`, `role="group"`). The new `ExtractionQueueRow` is a sibling, not a reinvention — see the mockup rendered in this session (`extraction_queue_row_options`).
- **Preview dialog** — `WritebackFormDialog.svelte`'s checklist chrome (backdrop, `role="dialog"`, focus trap, per-row checkbox + status icon, footer submit) is reused wholesale; only the row body changes from an editable input to an old→new diff line (mockup: `extraction_preview_and_revert`).
- **Entity picker (for "Edit…"/"pick suggested entity" on People/Studio fields)** — reuses `EnrichPicker.svelte`'s dialog chrome, roving-tabindex candidate list, and focus-trap/Esc/return-focus wiring. Not a new picker family.
- **Revert control** — one `<button>` on an existing activity-history row (System Activity, ADR-028's list surface); no new list, no new page.

Net new component files: `ExtractionQueueRow.svelte`, and a diff-mode addition to
`WritebackFormDialog.svelte` (or a thin sibling `ExtractionPreviewDialog.svelte` if the prop shape
diverges enough — implementer's call, see Open Questions). New route: `owner/extraction/+page.svelte`.
**Introduce no new tokens.**

---

## Reuse map (don't build from scratch)

| Need | Reuse from | Notes |
|------|-----------|-------|
| Owner gating | `activity.effectiveOwner` (`activity.svelte.ts`) | Same predicate every other owner surface uses. |
| Tab shell | `owner/+layout.svelte:13-19` | Add `{ href: '/owner/extraction', label: 'Extraction' }` to `tabs`. Active-tab styling (`bg-surface-2 text-ink`) unchanged. |
| Grouped queue layout | `owner/duplicates/+page.svelte`, `owner/enrichment/+page.svelte` | Same shape: `$state` rows, `$derived` grouping, `$effect` load, resolve-in-place on action (no refetch). |
| Dense row rhythm | `DuplicatePairRow.svelte` / `EnrichQueueRow.svelte` | `flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-rule px-3 py-2.5 text-sm`, `role="group" aria-label=...`. `ExtractionQueueRow` follows this per-field, grouped under a video header row (new, but same rhythm as `EnrichQueueRow`'s per-provider chip line). |
| Confidence display | `EnrichPicker.svelte`'s `matchLabel()` idiom | Reuse the same tier-label convention (not a raw percentage — avoids false precision) for the row's confidence indicator. |
| Suggested-entity picker | `EnrichPicker.svelte` (whole file) | Dialog chrome, roving-tabindex candidate list, focus-trap/Esc/return-focus — unchanged. The extraction row's "Edit…" action for People/Studio fields opens this exact dialog, pre-seeded with the Jaro-Winkler suggestion (if any) as the top/highlighted candidate — never pre-selected/applied. |
| Preview/diff dialog | `WritebackFormDialog.svelte` (whole file) | Dialog chrome, focus trap, per-row checkbox + status icon (`idle/writing/done/error`), footer submit-count button — unchanged. New: replace the editable `<input>`/`<textarea>` row body with a two-value diff line (old, struck through in `text-muted`/`text-warn` → new, in `text-accent`) when the dialog is opened in **preview mode** (i.e. showing extraction/merge writes rather than an operator-edited manual writeback). |
| Revert control | System Activity list row (ADR-028) | One `<button>` added to a completed batch's row, gated on the row having a `batch_id` from `file_writeback_snapshots`. No new list component. |
| Warn-vs-neutral token discipline | F43/F47's regression guard | "Conflict" and "no suggestion" states in the queue are **neutral** (`text-muted`), not `text-warn` — mirrors the F47 handoff's "needs review ≠ error" rule verbatim. Only an actual write/revert failure gets `text-warn`. |

---

## 1. `/owner/extraction` — the review queue tab

### Resolved: grouping — by video, not by field type

Unlike Enrichment (grouped by entity type — People/Studios/Media, since a provider match is
independent per entity), extraction review rows are grouped **by video**: the owner is looking at
one file's filename and deciding what's true about *that file*, and a single file commonly has
2–4 pending fields at once (e.g. People conflicts alongside a Release Date single-source case).
Presenting all of a video's pending fields together lets the owner resolve a file in one pass
instead of hunting across three field-type sections for the same filename. Within a video group,
fields render in a fixed order: **People → Studio → Title → Release Date → other**, matching the
tier ordering in the spec (entity fields first, since they're the higher-stakes ones).

Video groups themselves sort **most-fields-pending first**, ties broken by filename — this puts
the videos that will clear the most backlog per click at the top, mirroring Enrichment's
actionable-first sort principle.

### Row anatomy (`ExtractionQueueRow.svelte`, one row per pending field)

```
non-entity: [field label]  filename: <value>   tag: <value>      Accept filename · Edit… · Accept tag · Dismiss
entity:     [field label]  [Alice · exists] [Bob · new] [Carol · new]     Accept cast (2 new) · Accept tag · Dismiss
```

> **HOLODEX-196 refinement (ADR-068 D2).** The anatomy below describes the original
> single-suggestion row; it still holds for **non-entity** fields (Title, Release date). **Entity**
> fields (People/Studio) now render one **chip per parsed name** instead — each chip shows the name
> and an *exists* (`bg-surface-2`/`text-ink`) or *new* (`border-accent`/`bg-accent/10`/`text-accent`)
> badge; clicking a chip opens the entity picker (`EntityPickerDialog`) seeded on that name to swap
> it to an existing entity or a corrected new name, and a small `×` removes it — none of which
> disturbs the other chips. A single **"Accept cast"** action stages the whole (possibly edited)
> list as one `manual` write (`Accept cast (N new)` when any chip is new); the per-suggestion "Pick
> suggested" and single-value "Edit…" actions are dropped for entity fields (the chips subsume
> them). Studio is the one-chip case, which doubles as a one-click fix for a mistyped studio.
> Editing a chip clears any staged accept so a stale value can't be committed.

- Video group header (once per group): `[ti-video] filename.mkv` — `text-ink font-medium`,
  truncates with full path in `title`, matches `EnrichQueueRow`'s leading-icon idiom.
- Field label: `text-muted text-xs uppercase tracking-wide`, fixed-width column so values align
  down the group (People / Studio / Title / Release date / Comment / Genre / Movie / Scene number).
- `filename:` and `tag:` values render side by side, **not** as a diff yet (this is the review
  row, not the preview) — whichever the owner hasn't picked stays neutral `text-ink`/`text-muted`.
  If one side is empty, render `— (empty)` in `text-muted italic` rather than blank space (empty
  vs. present must be visually distinguishable, per the resolver's existing "absent means no data"
  convention referenced in [[project-holodex-field-source-of-truth]]).
- **Suggested entity** (People/Studio rows only, when a Jaro-Winkler advisory match exists):
  render on its **own line below** the value row, `text-muted text-xs`, prefixed with a small
  sparkle/hint icon and the words **"suggested match — not applied"** — never as an inline chip
  that could be mistaken for an already-applied value. This was mocked both ways in this session
  (`extraction_queue_row_options`, Option A inline chip vs. Option B separate advisory line); **B
  is the resolved choice** — it's unambiguous that nothing has been applied yet, which matters
  given the spec's hard rule that fuzzy matches must never look "picked." When no suggestion
  exists (no exact and no fuzzy match), render the same line as plain "no match — will create new
  {Person/Studio}" so the owner knows what accepting the filename value actually does.
- Row actions, right-aligned, `text-accent hover:underline` (matches `EnrichQueueRow`'s "Review"
  link styling) — only the actions actually available for that field render:
  - **Accept filename** — always shown if a filename value exists.
  - **Accept tag** — always shown if a tag value exists.
  - **Pick suggested: {Name}** — only when an entity suggestion exists; opens `EnrichPicker`-style
    dialog pre-seeded on that candidate (owner still confirms — click applies, doesn't pre-select).
  - **Edit…** — always shown; opens a manual-entry affordance (plain text input for non-entity
    fields, the entity-search dialog for People/Studio).
  - **Dismiss** — always shown; durable per F48.4d, mirrors F47 RD4.

### Confidence indicator

A small tier label (not a raw percentage — same reasoning as `EnrichPicker.matchLabel()`) sits
next to the field label: **"Strong"** / **"Weak"** / **"Conflict"**, styled `text-muted text-xs`.
This is informational only — it doesn't change which actions render (those depend on which sides
have data, not on the score itself).

### Zero-cost load, resolve-on-click

`GET /owner/extraction-queue` returns grouped rows with **no** write happening on load (mirrors
Duplicates/Enrichment's zero-cost list pattern). Clicking any row action fires the resolve
endpoint for that one field, updates the row in place (no refetch per F48.4c), and the video group
shrinks/reorders as fields clear.

### Empty / loading / error

Identical wording pattern to Duplicates/Enrichment:

- Loading: `py-16 text-center text-sm text-muted` "Loading…"
- Empty: "Nothing left to review."
- Error: `text-warn` inline, `role="alert"`

Intro paragraph above the groups (see mockup): "Fields the filename and tags disagree on, or that
scored below the auto-apply threshold. Resolving a row writes it the same way an auto-applied
field would."

### Triggers surfaced in this tab

- **"Extract all"** (F48.5b) — one button in the tab's header area (next to the intro paragraph,
  right-aligned), `rounded-theme border border-rule px-2.5 py-1.5 text-sm text-accent
  hover:bg-surface-2` (same shell as Enrichment's "Refresh all"). Kicks off a batch job tracked via
  System Activity (`kind=extraction`); the tab doesn't block on it — the owner can navigate away
  and the queue simply grows as results land, same as any other background job on this app.
  **HOLODEX-196 #2:** since the 202 returns immediately with no completion signal, the button then
  reads "Extracting…" and a `text-muted` `role="status"` notice appears ("Extraction is running in
  the background — new rows appear below as files are processed…"); the queue **auto-refreshes** a
  bounded number of times, and a sibling **"Refresh"** button (`text-ink`, same shell) lets the
  owner reload in place at any time — the refresh reloads rows without the full-screen "Loading…"
  state so the list doesn't blank.
- **"Extract from filename"** (F48.5a, single-video) — lives on the video detail page, not this
  tab, alongside the existing writeback/refresh actions — out of scope for this handoff's tab
  spec, noted here only so the two entry points aren't confused. Same resolve pipeline, same
  routing outcome (auto-apply or queue row) either way.

---

## 2. Preview-before-write dialog

### Trigger

- **Owner-resolved batch** (F48.7a): after resolving N extraction-queue rows in a working
  session, a "Review N changes" button appears (contextual — only when ≥1 unresolved-and-picked
  change is pending write), opening the dialog in **preview mode**.
- **Auto-applied batch, contextual preview** (F48.7b): the first few times auto-apply writes a
  batch, show the same dialog before committing; once the owner has dismissed/confirmed several
  without editing anything, an "always skip preview for high-confidence auto-apply" affordance
  becomes available (a checkbox in the dialog footer, unchecked by default) — **this is a
  trust-building progressive disclosure, not a settings-page toggle**; keep it simple, a single
  persisted boolean is enough (no new settings surface).

### Row body (the only real visual delta from `WritebackFormDialog`)

Replace the editable `<input>`/`<textarea>` with a static two-value line:

```html
<span class="text-muted line-through decoration-warn">{oldValue || '(empty)'}</span>
<i class="ti ti-arrow-right text-muted" aria-hidden="true"></i>
<span class="text-accent font-medium">{newValue}</span>
```

- Old value: `text-muted`, struck through with `decoration-warn` (not `text-warn` on the text
  itself — the strike-through communicates "being replaced," full warn-colored text would
  over-signal this as an error state, which it isn't).
- New value: `text-accent font-medium` — this is the value about to be written, same visual
  weight as a confirmed choice elsewhere in the app (matches "Auto-applied ✓" styling in
  `EnrichQueueRow`).
- Checkbox retained exactly as `WritebackFormDialog` has it today — the owner can still uncheck
  any row to skip it at write time, same mechanism, no new state machine.
- Per-row status icon (idle/writing/done/error) — unchanged, reused verbatim.

### Footer

Unchanged from `WritebackFormDialog`: "Cancel" / "Write N fields to file" (accent button),
busy → "Writing…", error → inline `text-warn` retry hint. No new footer affordance beyond the
optional "skip preview next time" checkbox described above (F48.7b only, not shown for a
manually-resolved batch since the owner explicitly asked to review those).

---

## 3. Revert (System Activity)

A completed extraction (or merge-writeback) batch's row in the System Activity list gets one
additional control, gated on the presence of a `batch_id`:

```
rounded-theme border border-rule px-2.5 py-1.5 text-xs text-muted hover:text-ink
[ti-arrow-back-up] Revert
```

- Placement: trailing edge of the activity row, same slot pattern other row-level actions use in
  that list (implementer confirms exact slot against the current `ActivityRow`-equivalent
  component — not read in this session, verify before implementing).
- Click behavior: no confirm dialog for the *click* itself (the batch's own original write already
  went through a confirm — either the preview dialog above, or the merge's own informed-confirm
  per F48.8c) — but the button **does** show a brief inline "Reverting…" busy state and, on
  success, the row updates to reflect "Reverted" (new `text-muted` status line under the original
  entry, not a whole new row) so the owner can see it happened without leaving the activity list.
- Revert failure: `text-warn` inline error on the same row, same convention as every other
  activity-row failure state in this app.
- A reverted batch's own revert action disappears (nothing to re-revert from *that* button) but
  the **new** revert-job's own activity row gets its own Revert button (F48.9c's "revert is itself
  revertible" — no special-cased UI, it's just another activity row with a `batch_id`).

---

## Layout

| Region | Layout |
|---|---|
| Extraction tab | Same `max-w-5xl` shell as the rest of `/owner` (inherited from `+layout.svelte`); intro paragraph + "Extract all" button on one line (wraps below on mobile), then grouped sections, `space-y-5` outer / `space-y-0` per group — identical spacing to Duplicates/Enrichment. |
| Video group | Header row (`text-ink font-medium`, `border-b border-rule`) then stacked field rows, `border-t border-rule` between fields (`DuplicatePairRow` rhythm). |
| Preview dialog | `max-w-xl` (matches `WritebackFormDialog` exactly), `max-h-[60vh]` scrollable row list, sticky header/footer. |
| Suggested-entity picker | `max-w-lg` (matches `EnrichPicker` exactly) — no new dialog width introduced. |
| Revert control | Inline addition to the existing activity row's flex layout — no new breakpoint. |

No new breakpoints anywhere in this feature.

---

## Design Tokens Used

All inherited from [theming.md](theming.md) — **no new tokens, no new radius, no new font**:

| Token | Usage here |
|---|---|
| `bg-surface` / `bg-surface-2` | Queue group card bg, dialog bg, active tab |
| `text-ink` / `text-muted` | Video filenames, field labels / secondary values, "not yet reviewed"-equivalent hints, suggested-entity line |
| `text-accent` / `bg-accent` / `text-accent-ink` | New value in the diff preview, "Accept filename"/"Accept tag"/"Edit…" row actions, "Extract all" button, dialog submit button |
| `border-rule` | Row/group/dialog borders (unchanged) |
| `text-warn` / `border-warn` / `decoration-warn` | **Only** for actual write/revert failures, and the strike-through decoration on a superseded old value (decoration only — not text color, see §2 above) |
| `rounded-theme` | Rows-as-cards, buttons, chips |

**Load-bearing distinction (mirrors the F43/F47 regression risk):** "Conflict" and "no
suggestion" in the queue row are **not** warn states — `text-muted`, same as F47's
"needs review"/"not matched." Only an actual resolve/write/revert failure gets `text-warn`.

**Token guard**: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'`
stays empty for the new/changed files.

---

## States and Interactions

| Element | State | Behavior |
|---|---|---|
| Extraction tab | loading | `py-16 text-center text-sm text-muted` "Loading…" |
| Extraction tab | empty | "Nothing left to review." |
| Extraction tab | error | `text-warn` inline, `role="alert"` |
| Queue row | click "Accept filename" / "Accept tag" | Enqueues the write (F48.4c), row's field disappears from the group in place (no refetch); video group shrinks/reorders |
| Queue row | click "Edit…" (non-entity field) | Inline text input replaces the value row, Save/Cancel |
| Queue row | click "Edit…" / "Pick suggested" (entity field) | Opens `EnrichPicker`-style dialog, pre-seeded on the suggestion if present; owner must click to confirm — nothing pre-applied |
| Queue row | click "Dismiss" | Row's field marked dismissed, durable, disappears until "Extract from filename" is re-run for that video (F48.4d) |
| "Extract all" | click | Kicks off a background batch (`kind=extraction`); tab remains usable, queue grows as results land |
| Preview dialog | row checked, click submit | Sequential writes, per-row status icon updates live, matches `WritebackFormDialog`'s existing submit behavior |
| Preview dialog | row unchecked | Skipped at write time, no diff shown as struck-through (same as `WritebackFormDialog`'s existing "unchecked = skip" semantics) |
| Preview dialog (auto-apply context) | "skip preview next time" checked + submit | Future high-confidence auto-apply batches commit without opening this dialog (F48.7b) |
| Activity row | click "Revert" | Busy → "Reverting…"; success → inline "Reverted" status line; failure → `text-warn` inline error |
| Any owner control | not owner | Absent from the DOM, not merely hidden (existing convention) |

---

## Responsive Behavior

| Breakpoint | Changes |
|---|---|
| Desktop / tablet (≥640) | Queue field rows single-line where they fit; row actions right-aligned inline. Video group header and "Extract all" sit on one line. |
| Mobile (<640) | Field row actions wrap below the value line (`flex-wrap`, matching `DuplicatePairRow`); "Extract all" wraps to its own line below the intro paragraph; preview dialog's diff line wraps old→new onto two lines if needed (arrow icon stays inline with whichever side it's adjacent to). |

No new breakpoints — this is a worklist like Duplicates/Enrichment, not a grid.

---

## Edge Cases

- **Video with zero pending fields** — doesn't appear in the queue (membership rule: at least one
  `pending` row for that video).
- **Field with no tag value and no filename value** — can't occur; a `metadata_extraction_review`
  row only exists when at least one side has data (per the confidence model's "only one has data"
  scoring tier) — no row renders both sides empty.
- **Suggested entity became stale** (the owner merged/renamed it between extraction and now) —
  resolve-in-place must revalidate on click, same "stale row → treat as already-handled, drop it,
  no error toast" pattern as Duplicates/Enrichment.
- **Very long filename** — group header truncates with `title` full text (existing convention
  across every entity list in the app).
- **International text (CJK, diacritics) in filename vs. tag values** — both render as-is,
  truncate/wrap the same as any other text field; no special-casing.
- **Preview dialog with 0 checked rows** — submit button disabled, mirrors `WritebackFormDialog`'s
  existing `checkedCount === 0` guard.
- **Revert when the file has since been deleted/moved (soft-delete, F24)** — the revert write
  fails against a missing path; surfaces the standard `text-warn` error line, same as any writeback
  attempt against a gone file. Not a new failure mode.
- **Revert-of-a-revert** — no special UI; the new revert's own activity row is a normal
  extraction-writeback-shaped row with its own Revert button (F48.9c).
- **Large queue at library-scale** — same as Duplicates/Enrichment: single scroll, grouped, no
  pagination expected at personal-library scale.

---

## Animation / Motion

No new motion. Reuses exactly what ships today, gated behind
`@media (prefers-reduced-motion: no-preference)`:

| Element | Trigger | Animation | Duration | Easing |
|---|---|---|---|---|
| Preview dialog / suggested-entity picker | open | Existing dialog-rise animation (same as `EnrichPicker`'s `enrich-rise`) | 150 ms | `cubic-bezier(0.2,0.7,0.2,1)` |
| Queue row | field resolved/dismissed | In-place text/action swap, no transform — matches Enrichment's "rows update in place" (not Duplicates' "row fades on removal," since a video group persists until its *last* field clears) |
| Video group | last field resolved | Group row itself is what fades out (`DuplicatePairRow`'s existing removal fade), since the group has nothing left |

---

## Accessibility Notes

- **Queue field rows** are `role="group" aria-label="{video filename}: {field label}"`, mirrors
  `DuplicatePairRow`/`EnrichQueueRow`'s pattern — one row reads as one unit before its actions.
- **Video group headers** use a real heading level (`<h3>` or equivalent, not just styled text) so
  screen-reader users can jump between videos in the queue.
- **Suggested-entity line** must read as advisory in the accessibility tree too — "suggested
  match, not applied: {name}" as the full text content, not conveyed by icon/color alone.
- **"Pick suggested" / "Edit…" dialogs** — full focus trap + Esc + return-focus, identical to
  `EnrichPicker`'s existing implementation (roving tabindex on the candidate list, `tabindex={i
  === active ? 0 : -1}`).
- **Preview dialog** — inherits `WritebackFormDialog`'s existing focus trap/Escape/return-focus;
  per-row status changes during submission are `aria-live="polite"` (existing pattern, unchanged).
- **Revert button** busy state — `aria-live="polite"` region or `aria-busy`, matching the picker's
  existing "Refreshing…" convention.
- **Tab addition** — `owner/+layout.svelte`'s nav is already `<nav aria-label="Owner tools">`, no
  change needed beyond adding the entry.
- **Owner-only controls** absent from the DOM for non-owners (existing convention).

---

## Open questions for the build (non-blocking)

1. **Preview dialog: extend `WritebackFormDialog` in place vs. a sibling component.** The diff-mode
   row body is different enough from the editable-input row body that a `mode: 'edit' | 'preview'`
   prop on the existing component may get awkward. Recommended: try the prop-mode extension first
   (less duplication of the dialog chrome/focus-trap/footer machinery); split into a sibling only if
   the conditional rendering inside the row gets hard to follow.
2. **"Skip preview next time" persistence** — a single boolean is enough per this handoff (not a
   settings page entry), but confirm where it lives: `localStorage` (client-only, resets per
   browser) vs. a lightweight owner-settings flag (F41/ADR-060, persists across devices).
   Recommended: `localStorage` — this is a UX trust-building convenience, not a durable policy
   decision worth a DB round-trip.
3. **Revert button's exact slot in the System Activity row** — this handoff didn't read
   `ActivityRow`'s current markup; confirm the trailing-action slot pattern against the live
   component before implementing (referenced generically above as "same slot pattern other
   row-level actions use").
4. **Entity-field "Edit…" dialog: reuse `EnrichPicker` verbatim vs. a lighter search-only variant.**
   `EnrichPicker` is built around provider `resolve`/`apply`/`dismiss` calls; extraction's picker
   only needs local entity search + confirm (no provider round-trip). Recommended: a thin sibling
   that borrows the dialog chrome/roving-tabindex/focus-trap code but swaps the data-fetching for a
   local entity-search endpoint — confirm with whoever implements F48.4c's resolution paths whether
   that's cheap enough to justify, versus just calling `EnrichPicker` with a stubbed provider.
