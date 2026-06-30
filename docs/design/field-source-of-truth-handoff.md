# Design Handoff: Per-field source-of-truth decisions (F36)

**Spec**: [Per-field source-of-truth (F36)](../specs/field-source-of-truth.md) · **ADR**: [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md)
**Builds on**: [Metadata Curation (F30)](metadata-curation-handoff.md) (chips/provenance), [Refresh Metadata (F31)](metadata-refresh-handoff.md) (header cluster, refetch idiom).
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**
**Stack**: SvelteKit (Svelte 5 runes) + Tailwind v4 CSS-first (ADR-025).

---

## Overview

On the media detail page (`/media/[id]`) Metadata section, each **replace (scalar) field** gains an
owner-only **source selector**: `Keep file` / one `Adopt {provider}` per *matched* provider / `Custom`.
It names which source is *true* for that field on this item — a **standing** choice that overrides
global mapping precedence and drives both the displayed value and what writeback commits. The default
(no decision) is the **file** value (file-first, RD4); a provider is shown as a *candidate*, never the
silent winner. This is the user-facing fix for the F31 refresh-masking bug.

**Merge (set) fields are unchanged** (RD1): they keep today's `CurationFieldRow` chips (union, per-value
include/exclude, manual-add). The selector is **replace-only**.

**Non-negotiable for devs (RD5):** changing a decision is a **DB write only — no file I/O, no spinner**.
The file is written solely by the explicit **Write decisions to file** action, which batches *all*
decided/out-of-sync fields into **one** atomic `WriteBatch` per file via the existing F30 queue. Never
write a file per toggle.

### Design-system fit (the `/design-system` check)

**One new primitive** — a segmented single-select **`SourceSelect`** control — plus reuse of everything
else:

- **Chips + provenance** — reuse `CurationChip` (`·source` suffix; accent for provider, muted for
  file/manual) and `ProvenanceBadge` verbatim for the resolved value + candidates.
- **Custom input** — reuse the inline-edit input idiom from `CurationChip`/`CurationFieldRow`
  (`rounded-theme border border-rule bg-bg px-2 py-0.5 text-xs … focus:ring-accent`; Enter commits,
  Escape cancels, blur commits).
- **Write button** — reuse the existing **Write to file** ghost button (relabel + count).
- **Owner gating + refetch-after-mutate** — `activity.effectiveOwner`; reuse `applyMediaDetail`
  (`+page.svelte`) so `resolved[]` reflects the new decision.

The only thing genuinely new is `SourceSelect` (a themed segmented radiogroup). It uses **no new
tokens** — `border-rule` / `bg-surface-2` / `text-muted` / `bg-accent`+`text-accent` for the selected
segment, `rounded-theme` container.

---

## Layout

Lives in the existing `Metadata` `<dl>` grid (`grid-cols-1 sm:grid-cols-2`,
`rounded-theme border border-rule bg-surface p-4`). For a **replace** field the `dd` becomes a small
two-row stack (owner view):

```
Metadata                         [Enrich ▾] [Clear …] [⤓ Write decisions to file · 2 out of sync]
┌────────────────────────────────────────────────────────────────────────────┐
│ Genres:  [ Drama ·file ✎ ✕ ] [ Thriller ·tmdb ✎ ✕ ] [ + Add ]               │ ← merge field: UNCHANGED (CurationFieldRow)
│                                                                              │
│ Title:   [ Blade Runner ·file ]                       ( file out of sync ⚠ ) │ ← row 1: resolved chip + per-field warn pill
│          ( Keep file | Adopt TMDB | Custom )                                 │ ← row 2: SourceSelect (owner-only)
│          candidates · file “Blade Runner”  tmdb “Blade Runner: Final Cut”    │ ← row 3: muted candidates (only if ≥1 provider)
│                                                       providers differ ·      │   …with a muted "providers differ" hint (P1-1)
└────────────────────────────────────────────────────────────────────────────┘
```

- **Row 1** — the resolved value as a `CurationChip` (`·source`), and, at the cell's end, the
  **out-of-sync** pill *only when* decided ≠ in-file (RD2; the single `text-warn` signal on the field).
- **Row 2** — `SourceSelect` (owner-only). Segments: `Keep file`, one `Adopt {provider}` per matched
  provider, `Custom`. `flex flex-wrap` so it wraps under the chip on the `sm` single-column and on
  narrow cells.
- **Row 3** — muted candidates, rendered **only when there is ≥1 provider candidate** (a file-only
  field shows nothing extra — the value is already in the chip). When ≥2 providers supply *different*
  values, append a muted **"providers differ"** hint here (Open-Q3 resolution below).
- **Visitor / non-owner** — rows 2–3 are absent; the field renders exactly as today (resolved chip +
  provenance). `displayTitle` and all read-only rendering are unchanged.

### Open-Q3 resolution — the two "warn-ish" signals don't collide

There is exactly **one** `text-warn` signal per field: the **out-of-sync** pill (row 1). The
**"providers differ"** hint is **not** warn — it is **muted/informational** on the candidates line
(row 3), because disagreement is informational, not an error (token rule: `--warn` is reserved for
error/attention). The two are on different rows and different weights, so they never read as one alarm
even when both are present.

---

## The `SourceSelect` control

A themed segmented **single-select**. One row of segments inside a `rounded-theme border border-rule`
container; the **selected** segment carries `bg-accent text-accent-ink` (or `bg-surface-2 text-accent`
if a filled accent reads too heavy in Brutalist — pick one in QA, keep it a token pair). Idle segments
are `text-muted`; hover/focus → `text-accent`.

| Segment | When present | Selecting it |
|---|---|---|
| `Keep file` | always | decision → `file` (default; clears the row, source-pinned to the live file value) |
| `Adopt {provider}` | one per **matched** provider | decision → `provider:{name}` (source-pinned to that provider's live value) |
| `Custom` | always | opens the inline input; on commit, decision → `manual` + literal |

- **No-provider-matched** → just `Keep file | Custom` (two segments).
- Selecting a segment issues `PUT …/decision` (or `DELETE` for `Keep file` when reverting to default);
  on success, **refetch** the detail so the chip + provenance + sync recompute. **No file write, no
  file-write spinner** (RD5). A brief inline busy/disabled on the control during the DB round-trip is
  fine; it must not look like a file operation.
- **Custom commit** keeps the input's existing affordance: Enter commits, Escape cancels, blur commits;
  empty cancels.

---

## Writeback affordance

- Rename the header **Write to file** → **Write decisions to file**.
- Append a **count** when any field is out of sync: `Write decisions to file · {n} out of sync`. The
  count is `text-warn` (compact); when `n = 0`, show no count (button stays enabled — a re-write is
  allowed but is a no-op).
- The button keeps its existing ghost treatment + the **download-arrow** icon and the busy spinner
  (this is the **only** place a write spinner appears).
- Behavior note (dev): clicking enqueues **one** durable job → **one** `WriteBatch` per file (RD5/P0-4).

---

## Design Tokens Used

| Token | Usage |
|---|---|
| `bg-surface` | metadata card background (unchanged) |
| `bg-surface-2` | chip background; idle/segment background; file-baseline pill |
| `border-rule` | card, chip, `SourceSelect` container, input borders |
| `text-ink` | chip value text, input text |
| `text-muted` | field label, candidates line, "providers differ" hint, idle segments |
| `text-accent` / `border-accent` | provider provenance (via `ProvenanceBadge`), segment hover, focus ring |
| `bg-accent` / `text-accent-ink` | **selected** `SourceSelect` segment; existing primary buttons |
| `text-warn` / `border-warn` | **out-of-sync** pill + the header out-of-sync count — error/attention only; never the "providers differ" hint |
| `rounded-theme` | `SourceSelect` container, inputs, cards |
| `rounded-full` | chips, pills (intentional shape) |
| `font-ui` / `font-display` | inherited; no per-component font |

No `zinc-/sky-/emerald-/amber-`, no hex, no fixed `rounded-lg/md/sm/xl`, no named fonts.

---

## States and Interactions

| Element | State | Behavior |
|---|---|---|
| `SourceSelect` | undecided (default) | `Keep file` selected; chip shows file value `·file`; rows 2–3 present for owner |
| `SourceSelect` | decided keep-file | same as default (an explicit `file` decision and the default render identically) |
| `SourceSelect` | decided adopt-provider | `Adopt {provider}` selected; chip shows provider value `·{provider}` (accent) |
| `SourceSelect` | decided custom | `Custom` selected; chip shows literal `·manual` (muted) |
| `SourceSelect` | busy (DB round-trip) | control briefly disabled/`opacity-60`; **no** file-write spinner |
| `SourceSelect` | no provider matched | only `Keep file | Custom` segments |
| Custom input | open | inline input focused; Enter commit / Esc cancel / blur commit / empty cancel |
| Out-of-sync pill (row 1) | decided ≠ in-file | `text-warn`/`border-warn` pill "file out of sync"; hidden when in sync |
| "providers differ" hint (row 3) | ≥2 providers disagree | muted hint on candidates line; never `text-warn` |
| Write button | n>0 out of sync | shows `· {n} out of sync` (warn count); spinner while the job is submitted |
| Write button | n=0 | no count; enabled (no-op re-write allowed) |
| Whole control | visitor / non-owner | absent; field renders read-only resolved value as today |

---

## Responsive Behavior

| Breakpoint | Changes |
|---|---|
| ≥ `sm` (two-column `dl`) | each field cell stacks rows 1–3; `SourceSelect` `flex flex-wrap` may wrap segments within the cell |
| < `sm` (single column) | unchanged stacking; `SourceSelect` wraps under the chip; candidates line wraps freely |

The `dl` grid itself is unchanged. Nothing in `SourceSelect` may set a fixed pixel width — segments size to content; the container wraps.

---

## Edge Cases

- **Long candidate values** (e.g. a long title) — candidates line truncates per-value with `line-clamp`/ellipsis; the chip itself wraps as today. Never force the cell wider than its grid column (`minmax(0,1fr)` semantics).
- **Provider matched but no value for this field** — that provider's `Adopt` segment is **omitted** (you can't adopt an empty value); it reappears if a re-enrich populates the field.
- **Custom equals the file value** — allowed; it still records a `manual` decision (the owner may want it frozen). Provenance reads `·manual`.
- **All sources empty** — the field doesn't render today (resolver returns nothing); `SourceSelect` does not appear for a field with no value and no candidates.
- **Decision references a provider later un-matched/cleared** — the field falls back to default (file) for display; surface nothing alarming (the stored decision is harmless; clearing the match is the existing F22/F31 path).
- **Out-of-sync after an external edit** — Refresh (F31) re-reads the file; sync recomputes on refetch.

---

## Animation / Motion

| Element | Trigger | Animation | Duration | Easing |
|---|---|---|---|---|
| `SourceSelect` segment | select / hover | color/background token transition | ~150ms | default `transition` |
| Custom input | open | none required (appears in place) | — | — |
| Write button icon | submitting | reuse existing `animate-spin` | — | — |

No layout-shifting animation; selecting a segment must not jump the grid.

---

## Accessibility Notes

- **`SourceSelect` = `role="radiogroup"`** with `role="radio"` segments and `aria-checked` on the
  selected one. **Roving tabindex** (cf. [[feedback-keyboard-list-roving-tabindex]] / `EnrichPicker`):
  the group is one Tab stop landing on the checked segment; **Left/Right (and Up/Down)** move and change
  selection; selection applies on arrow (or Space/Enter) — match native radio semantics.
- `aria-label` per segment names the value: `Keep file value "Blade Runner"`, `Adopt TMDB value "Blade Runner: Final Cut"`, `Custom value`.
- The group has `aria-label="Source of truth for {field label}"`.
- The out-of-sync pill uses `aria-label="{field label} is out of sync with the file"`; the "providers differ" hint is plain text (informational), not an alert.
- The candidates line is readable text, associated with the field via proximity in the `dd`; no live region (it's static).
- Focus-visible ring (`focus:ring-accent` idiom) on segments and the input; the Custom input traps nothing (inline), Escape returns focus to the `Custom` segment.
- Color is never the only signal: selected segment also reads via `aria-checked`; provenance is text (`·source`), not just hue.
