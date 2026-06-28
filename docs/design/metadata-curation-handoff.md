# Design Handoff: Granular Metadata Curation (F30)

**Spec:** [docs/specs/metadata-curation.md](../specs/metadata-curation.md) ·
**ADR:** [ADR-048](../architecture/ADR-048-metadata-curation-and-write-queue.md) (curation/merge + write queue) ·
**Security:** conditional design sign-off — render curated values as **text only, never `{@html}`** (condition C4) ·
**Builds on:** the F27/F28 metadata section in [`web/src/routes/media/[id]/+page.svelte`](../../web/src/routes/media/%5Bid%5D/+page.svelte), [`ProvenanceBadge.svelte`](../../web/src/lib/components/ProvenanceBadge.svelte), [`WritebackFormDialog.svelte`](../../web/src/lib/components/WritebackFormDialog.svelte), the roving-tabindex pattern from [`EnrichPicker.svelte`](../../web/src/lib/components/EnrichPicker.svelte).

> **Stack:** SvelteKit (Svelte 5 runes) + Tailwind v4 CSS-first. **Tokens only — no `zinc-*`/hex/`rounded-(lg|md|sm|xl)`.** **QA all three skins** (Cinémathèque, Broadcast, Brutalist). These two rules are load-bearing — see [`.claude/CLAUDE.md` › Frontend theming](../../.claude/CLAUDE.md) and [theming.md](theming.md).

---

## Overview

Today the **Metadata** section of the video detail page renders each resolved field as one row — values joined with `, ` and a single field-level "from {provider}" badge — and a separate **Write to file** button opens a batch dialog of editable fields. This is *field-level*: an owner accepts or skips a whole field.

F30 makes it *value-level*. Each field becomes a row of **value chips**. A chip shows the value, its provenance, and (for the owner) controls to **edit**, **remove (suppress)**, or **exclude from the file write (don't-write)**. An **add-value** affordance lets the owner type a value no source supplied. The merged set is the **deduplicated union** of file + manual + provider(s), so a value both the file and TMDB supply appears once, badged with both sources. "Write to file" then commits the **curated set** through the durable write queue.

**Who sees what:** guests/non-owners see the read-only merged value (chips with provenance, no controls — exactly today's information, restructured). Owners see the full curation affordances. Gating is `isOwner` (client UX) over the server `requireOwner` gate (the real gate — ADR-030).

---

## Layout

Lives in the existing `Metadata` `<section>` of the media detail page. The current `<dl>` grid (`grid-cols-1 sm:grid-cols-2`, `rounded-theme border border-rule bg-surface p-4`) is **kept**; each field cell's value area changes from inline text to a **chip wrap row**.

```
Metadata                          [Enrich from TMDB] [Clear TMDB] [⤓ Write to file]
┌───────────────────────────────────────────────────────────────────────────┐
│ Genres:  [ Drama ·tmdb ✎ ✕ ]  [ Science Fiction ·tmdb+file ✎ ✕ ]           │  ← set field (merge mode): chip wrap
│          [ Sci-Fi ·manual ✎ ✕ ]  [ + Add ]                                  │
│                                                                             │
│ Title:   [ Fight Club ·tmdb ✎ ]                                  (override) │  ← scalar field: single chip, no remove
│                                                                             │
│ Overview: ┌─────────────────────────────────────────────────────────────┐ │  ← long_text: block, edit pencil top-right
│           │ A ticking-time-bomb insomniac…                       ·tmdb ✎ │ │
│           └─────────────────────────────────────────────────────────────┘ │
│                                                                             │
│ Poster:  [img]  ·tmdb   (no write/edit — image fields are display-only)     │  ← image_url: unchanged from today
└───────────────────────────────────────────────────────────────────────────┘
```

- **Set field (merge mode, `multi: true`)** → horizontal **chip wrap** (`flex flex-wrap gap-1.5`), trailing **+ Add** chip.
- **Scalar field (precedence)** → a single value chip; **no remove** (a scalar always resolves to one value), edit = override; an "(override)" hint appears when a manual edit is active.
- **`long_text`** → keep the block paragraph; controls (pencil) sit inline at the end, provenance badge after.
- **`image_url`** → unchanged from today (display-only; no chip controls — image writeback stays in the batch dialog/ADR-039 path).

---

## Design Tokens Used

| Token | Usage |
|-------|-------|
| `bg-surface` | metadata card background (unchanged) |
| `bg-surface-2` | **chip background** (matches the file-baseline `ProvenanceBadge` pill) |
| `border-rule` | card border, chip border, input border |
| `text-ink` | chip value text, input text |
| `text-muted` | field label, provenance/secondary text, idle icon buttons |
| `text-accent` / `border-accent` | provider-provenance outline (via `ProvenanceBadge`), icon hover, focus ring, queued/active state |
| `bg-accent` / `text-accent-ink` | primary "Write to file" / confirm button, "+ Add" submit |
| `text-warn` / `border-warn` | **error/failed-write state only** (never for remove/destructive affordances per token rules — remove uses muted→accent hover, confirmation carries the weight) |
| `rounded-theme` | inputs, buttons, the card |
| `rounded-full` | chips and provenance pills (intentional pill shape) |
| `font-mono` `text-xs` | file path in the write dialog |
| `accent-[var(--color-accent)]` | native checkbox accent (as in `WritebackFormDialog`) |

No new tokens. Skin-specific flourishes (if any) attach to existing hook classes in `app.css` gated by `[data-theme]` — not per-component markup.

---

## Components

| Component | New/Reuse | Props | Notes |
|-----------|-----------|-------|-------|
| `CurationChip` | **New** | `value`, `sources: string[]`, `manual: boolean`, `written: boolean`, `removable: boolean`, `editable`, `onedit`, `onremove`, `ontogglewrite` | The unit. Renders value + inline provenance + (owner) edit/remove/write-toggle. Roving-tabindex item. |
| `CurationFieldRow` | **New** | `field: ResolvedField`, `isOwner`, `onadd`, … | Wraps chips for one field; owns the roving-tabindex group + the **+ Add** affordance + the add-input. |
| `AddValueInput` | **New (inline)** | `onsubmit`, `oncancel` | Appears in place of the **+ Add** chip when activated. Text input; Enter submits, Esc cancels. |
| `ProvenanceBadge` | **Reuse** | `provider`, `label` | Already the provider/file pill. **Extend** to accept a multi-source label, e.g. `tmdb + file`. Inline (smaller) variant for use inside a chip. |
| `WritebackFormDialog` | **Evolve** | `fields`, `videoId`, `filePath`, `writeback`, … | Now previews the **curated** set (post-dedupe, suppressed/nowrite excluded) and **enqueues one job** rather than sequential per-field writes. Shows queue status. |
| `ConfirmDialog` | **Reuse** | `variant="accent"` | For destructive-ish confirms if needed (edit/remove are inline + reversible, so generally no modal — see Interactions). |

### Provenance inside a chip

Use a compact form of `ProvenanceBadge` — a `·tmdb` / `·tmdb+file` / `·manual` suffix in `text-muted` (file/manual) or `text-accent` (provider). Full source list goes to the chip's `aria-label` ("Science Fiction, from TMDB and file"). Per **dedup rule**, a value matched in N sources shows all contributing sources, primary first.

---

## States and Interactions

### Per-value (chip)

| Element | State | Behavior |
|---------|-------|----------|
| Chip | Default (guest) | Value + provenance, no controls. |
| Chip | Default (owner) | Value + provenance; edit (✎) + remove (✕) reveal on hover/focus (always present in DOM for keyboard/touch — opacity, not `display`, so they're focusable). |
| Chip | Hover/focus | Edit + remove icons go `text-muted` → `text-accent`. Chip border may go `border-accent`. |
| ✎ Edit | Click/Enter | Chip swaps to an inline text input pre-filled with the value; Enter commits (= suppress original + add edited, per spec), Esc cancels. Optimistic update; persists via curation API. |
| ✕ Remove | Click/Enter | **Suppress (tombstone).** Chip animates out; value is hidden everywhere and won't be written, and **stays gone across re-scan/re-enrich**. Reversible via the field's "Show removed (N)" toggle (below). No modal — removal is reversible and low-stakes; the persistence is the safety, not a confirm. |
| Don't-write toggle | Click | Marks the value `nowrite`: chip stays visible but gets a **struck-through / dimmed "won't write"** treatment (`opacity-60` + a small slashed-pencil indicator) and is excluded from the next file write. Toggling again restores. Distinct from remove (still shown vs. hidden). |
| Chip | `manual` | Carries a `·manual` provenance; visually identical otherwise. |
| Chip | `written` (after a successful write) | A subtle ✓ in `text-accent` may appear; not load-bearing (the file now carries it). |

### Per-field

| Element | State | Behavior |
|---------|-------|----------|
| **+ Add** chip | Click/Enter | Replaces itself with `AddValueInput`. Submit adds a `manual` chip immediately (optimistic) + persists. Empty submit / Esc cancels back to the **+ Add** chip. Duplicate (normalized) of an existing value is a no-op with a brief inline hint. |
| **Show removed (N)** | toggle | A muted link appears when ≥1 value is suppressed for the field; expands to show suppressed chips with a **restore (↩)** action. Hidden when N = 0. |
| Scalar field | edit | Editing the single chip sets a manual **override**; an "(override)" muted hint appears; a restore-to-source affordance reverts. |

### Write action (queue)

| Element | State | Behavior |
|---------|-------|----------|
| **⤓ Write to file** | Default | Shown when `isOwner && canWriteback` (a field has a writable, write-enabled value). Opens the evolved `WritebackFormDialog`. |
| Dialog | Preview | Lists each field's **curated, write-enabled** values (dedup applied; suppressed/nowrite excluded). Read-back of exactly what will be embedded. File path shown (`font-mono text-xs`). |
| Dialog confirm | Submit | Enqueues **one** write job (HTTP `202`); dialog shows **"Queued…"** then closes (does not block on the write). |
| Field/section | Queued | A small inline status near the section: **"Write queued"** → **"Writing…"** (spinner, `text-muted`) → **"Written ✓"** (`text-accent`) or **"Write failed"** (`text-warn`, with the error and a retry). Mirrors the activity feed (`kind=writeback`). |
| Field/section | Failed | `text-warn` message; the original file is untouched (ADR-041/ADR-048); offer retry. |

> **Concurrency:** writes are serialized by default (`WRITEBACK_CONCURRENCY=1`). With multiple files queued, status is **"Queued (position N)"** until the worker reaches it — keep the copy honest about waiting.

---

## Responsive Behavior

| Breakpoint | Changes |
|------------|---------|
| Desktop (≥640px, `sm`) | Metadata `<dl>` stays 2-col; chips wrap within a cell. |
| Mobile (<640px) | `<dl>` collapses to 1-col (existing). Chips wrap full-width; edit/remove icons are **always visible** (no hover on touch) at a ≥44px tap target. **+ Add** and **Show removed** stack below the chips. The write dialog is full-width (`px-4`, existing modal pattern). |

---

## Edge Cases

- **Empty field** — a field with no values (after suppression) is omitted from the resolved list (spec: resolver drops empty fields), so it simply doesn't render. If the owner suppressed the last value, show the field with only **+ Add** and **Show removed (N)** so they can recover.
- **Long value** — a single long chip wraps its text (`break-words`, `max-w-full`); it does not force horizontal scroll. Very long `genres` lists wrap to multiple rows naturally.
- **Long text fields (`overview`/`bio`)** — keep the block paragraph; edit opens a `textarea` (auto-resize, as in `WritebackFormDialog`), not a chip.
- **International text** — values may be RTL/CJK; chips are content-sized, no fixed width; rely on logical properties / default flow.
- **Many values** — chip wrap has no cap; if a field routinely exceeds ~12, consider a "+N more" expander (defer unless QA shows overflow).
- **Image fields** — no chip controls; writeback of cover art stays in the batch dialog under the ADR-039 asset-host allowlist.
- **No provider configured** — no "Enrich" button; file + manual chips still fully curatable and writable.
- **Non-owner** — chips render read-only (no ✎/✕/toggle/Add/Write); identical information to today.
- **Duplicate add** — adding a value equal (normalized) to an existing one is a no-op with a brief inline hint ("already present"); never creates a second chip.
- **Container with no tag mapping** (e.g. `.avi`) — the field is shown/curatable in-app, but the write dialog flags that field as **"can't write to this container"** (per-field, not a whole-batch failure — spec F30.4i) rather than hiding it.

---

## Animation / Motion

| Element | Trigger | Animation | Duration | Easing |
|---------|---------|-----------|----------|--------|
| Chip | add | fade + slight scale-in | 120ms | ease-out |
| Chip | remove (suppress) | fade + collapse width | 120ms | ease-in |
| Edit input | open | none (instant swap) | — | — |
| Icon buttons | hover/focus | color transition `text-muted`→`text-accent` | 100ms | ease |
| Write status | state change | text crossfade | 150ms | ease |

Keep motion subtle; respect `prefers-reduced-motion` (skip scale/collapse, keep instant state changes).

---

## Accessibility Notes

- **Roving tabindex over chips** (per project convention — see `EnrichPicker.svelte` and the keyboard-list memory): the chip group is one Tab stop; **←/→** move between chips, **Home/End** jump to ends. The chip's own edit/remove are reached via **Tab into** the focused chip (so Tab *does* reach the controls — do **not** use `aria-activedescendant` here, which would skip them). **+ Add** is the last item in the roving group.
- **Edit input** — when a chip enters edit mode, move focus into the input; on commit/cancel, return focus to the (new/edited) chip. **Add input** returns focus to the **+ Add** chip on cancel, or to the new chip on submit.
- **Remove** — after suppressing a chip, move focus to the next chip (or **+ Add** if it was last); announce via `aria-live="polite"`: "Removed {value}. {N} removed — restorable."
- **ARIA** — each chip `role="group"` with `aria-label="{value}, from {sources}"`; the field row labelled by its `<dt>`. The "Show removed" region is a disclosure (`aria-expanded`). The don't-write toggle is a `button` with `aria-pressed`.
- **Write status** — the queue status line is `aria-live="polite"` so "Write queued → Writing → Written ✓ / failed" is announced without stealing focus.
- **Dialog** — `WritebackFormDialog` keeps its existing focus trap + return-focus + Escape-to-close (when idle).
- **Contrast** — verify chip text (`text-ink` on `bg-surface-2`) and provider-accent provenance meet contrast in **all three skins**; the Brutalist accent on its surface is the usual offender.
- **Security (C4)** — all value/label/provenance rendering is plain text interpolation (Svelte auto-escapes); **never `{@html}`** on curated content.

---

## QA

A companion `docs/design/metadata-curation-qa-checklist.md` should mirror the repo's numbered, verifier-tagged convention (Setup/Smoke/Agent/Human, grouped by tag): per-value add/edit/remove/restore/nowrite, dedup-shows-once, suppress-survives-reenrich, scalar override, queue states (queued→writing→written/failed), non-owner read-only, the `.avi` no-mapping case, and a **3-skin render pass** of chips + provenance + the write dialog.
