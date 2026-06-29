# Design Handoff: Refresh Metadata (per-item re-extract + re-enrich) (F31)

**Spec**: [Refresh Metadata (F31)](../specs/metadata-refresh.md) · **ADR**: [ADR-047](../architecture/ADR-047-per-item-metadata-refresh.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**
**Stack**: SvelteKit (Svelte 5 runes) + Tailwind v4 CSS-first (ADR-025).

---

## Overview

**One** owner-gated control on the media detail page (`/media/[id]`): **Refresh** — re-reads the file
(forced exiftool/ffprobe extraction, bypassing change-detection) **and** re-pulls the providers the
item is matched to, in a single click. It is the on-demand "bring this item back in sync with the
file as it exists right now" action (catches edits another system made — including ones that don't
change the file's mtime).

Refresh is **non-destructive and idempotent**, so — unlike delete — it has **no confirm dialog**. It
shows progress inline and reports the outcome on a single inline status line (there is still no toast
system; we follow the enrichment pattern of inline feedback). Nothing renders for a non-owner.

This is a **read** action (file + providers → Holodex's stores). Writing values *into* the file is the
separate **Write to file** control (F28). Rebuilding the thumbnail image is the separate **Regenerate
thumbnail** affordance (F11.6). Refresh sits beside the first; it may legitimately be followed by the
second.

### Design-system fit (the `/design-system` check)

**No new tokens, no new component.** Refresh is assembled entirely from idioms already on this page:

- **Ghost icon button** — identical treatment to the existing **Write to file** button
  (`+page.svelte` lines 331–338): `text-muted hover:text-accent`, an inline-SVG icon + label, no fill.
- **Spinner** — the **Regenerate thumbnail** button already spins its icon with `animate-spin` while
  busy (line 242) and reuses the **circular-arrow SVG path** (lines 247–249) — reuse both verbatim.
- **Inline status line** — the enrichment error line (`text-xs text-warn`, lines 342–344) is the
  template; success/no-change use `text-muted`, partial/file-error use `text-warn`.
- **Owner gating** — `activity.isOwner`, same predicate as Enrich / Write-to-file / Manage.
- **Refetch-after-mutate** — copy `onApplied` (lines 148–162): re-`getMedia(id)` so `resolved[]`
  reflects the new file + provider data; bump `thumbVersion` so a changed embedded cover busts.

Audit output: **add an inline ghost button + one status line + one `api.refreshMedia` call to
`+page.svelte`; reuse the icon, spinner, status-line, gating, and refetch idioms verbatim; introduce
no new tokens and no new primitive.**

---

## Placement decision (Option A)

The control lives **first in the Metadata section header cluster**, alongside Enrich / Clear /
Write-to-file — it is the umbrella "re-sync this item's metadata" action and belongs with its
siblings. Two alternatives were mocked and rejected:

| Option | What | Verdict |
|---|---|---|
| **A — Metadata header cluster (chosen)** | Ghost `⟳ Refresh` as the **first** item in the existing controls row | Co-located with Enrich/Write-to-file; no new region; reads as "redo the metadata sync". |
| B — Dedicated owner toolbar under the title | A new standalone row with one button | Always-visible, but invents a new owner toolbar for a single action — more chrome than the job needs. |
| C — In the **Manage** block (with delete) | Group it with Move-to-Trash / Delete | Rejected — Manage is the **destructive** zone (`--warn`); a safe, frequent action there muddies that block's meaning and risks mis-click proximity to delete. |

**Always-available rule.** The Metadata `<section>` today renders only when `resolved.length`. To give
owners a consistent home for Refresh even on an item with no resolved metadata (no mapping / file-only),
render the **section header row** when `isOwner || resolved.length || fields.length`. When the body is
empty, show a muted hint (`No metadata extracted yet.`) under the cluster. The Refresh button itself
gates on `isOwner` only (Enrich still needs a `provider`, Write-to-file still needs `canWriteback`).

---

## Reuse map (don't build from scratch)

| Need | Reuse from | Notes |
|------|-----------|-------|
| Owner gating | `activity.isOwner` (`activity.svelte.ts`) | Button renders only when owner. |
| Ghost button shape | **Write to file** button (`+page.svelte` 331–338) | Same `flex items-center gap-1 … text-xs text-muted hover:text-accent focus-visible:text-accent`. |
| Circular-arrow icon + spinner | **Regenerate thumbnail** (`+page.svelte` 242–250) | Same SVG path; same `animate-spin` while busy. |
| Inline status/error line | enrichment `enrichError` line (342–344) | `text-xs`; `text-muted` for success/no-change, `text-warn` for partial/file-error. |
| Owner-gated POST | `api.sendAuthed` (`api.ts` 85–96) | New `api.refreshMedia(id)` mirrors `enrichVideoApply` / `regenerateThumbnail`. |
| Refetch after mutate | `onApplied` (148–162) | Re-`getMedia(id)`; reset `resolved/enriched/extra/fields`; `thumbVersion += 1`. |
| Disabled-while-busy | `regenerating` pattern (235–236) | `disabled={refreshing}`. |

---

## Layout

| Region | Layout |
|--------|--------|
| **Refresh button** | First child of the Metadata header's right-hand controls `div` (`flex flex-wrap items-center gap-2`, lines 311–340). Order: **Refresh** → Enrich → Clear → Write to file. Ghost (no fill), so it sits quieter than the accent Enrich CTA. |
| **Status line** | A single line directly under the header row (where `enrichError` renders now, ~line 342), full width, `aria-live="polite"`. Shows progress and the outcome; replaced on the next action. |
| **Empty-metadata hint** | When the section renders for an owner but `resolved`/`fields` are empty, a muted `No metadata extracted yet.` line in place of the `<dl>`. |

No new breakpoints; the controls row already `flex-wrap`s and the status line is full width.

---

## Design tokens used

Tokens only — every value is a semantic utility backed by a CSS variable. **No `zinc-*`/`sky-*`/hex/
named-font/fixed-radius in markup.**

| Token | Usage |
|-------|-------|
| `text-muted` | Refresh at rest; in-flight label; success / no-change status line |
| `text-accent` | Refresh hover/focus (`hover:text-accent focus-visible:text-accent`); the ✓ glyph on a successful sync |
| `text-warn` | Partial-success and file-error status line **only** (attention/error — never the accent) |
| `rounded-theme` | Any focus-ring rounding on the button (matches Write-to-file) |
| `border-rule` / `bg-surface` | Inherited from the Metadata card; Refresh adds none of its own |

Refresh deliberately uses **no accent fill** — the page already has one accent CTA in this cluster
(Enrich), and CDS/Holuse restraint keeps a single filled control per group. Refresh is a ghost.

---

## Components

| Component | Variant / Props | Notes |
|-----------|-----------------|-------|
| Refresh control (**inline in `/media/[id]`**) | renders when `activity.isOwner` | A `<button onclick={refresh} disabled={refreshing}>` with the circular-arrow SVG (`animate-spin` when `refreshing`) + label. No new component file. |
| Status line (**inline**) | `refreshState: { tone: 'muted' \| 'warn'; text } \| null` | One reactive line; mirrors the `enrichError` idiom. Cleared when a new refresh starts. |
| `api.refreshMedia(id)` (**new client method**) | `POST /api/v1/media/{id}/refresh` → `RefreshReport` summary | Mirrors `enrichVideoApply`; owner cookie via `sendAuthed`. |

### Copy (exact — sentence case, verb-first, no "successfully")

| State | Label / line |
|---|---|
| Button (rest) | **Refresh** (icon + word; `aria-label="Refresh metadata from file and providers"`) |
| In flight | **Refreshing…** |
| Done · changed (file + provider) | `Synced · {f} from file, {p} from {provider} updated` |
| Done · changed (file only / no match) | `Synced from file · {f} fields updated` |
| Done · no change | `Already in sync — nothing changed` |
| Partial (file ok, provider failed) | `{provider} lookup failed — file metadata still updated` |
| File error (missing / locked) | `Couldn't read the file. Nothing changed.` |

`{f}`/`{p}` are counts from the `RefreshReport`; singular/plural the word "field". The provider name is
the matched provider (e.g. `TMDB`). No raw exception strings ever reach the line.

> **Single-item conflict display (v1 decision):** do **not** surface per-field `sources_disagree`
> prominently. Where file and provider differ, the resolver already picks by precedence and the
> existing **provenance chips** ("from TMDB" / "from file") show which won — that is sufficient at
> single-item scale. Rich disagreement/triage UI is deferred to the future batch feature (F31.11), per
> the spec. Keeping v1 to one status line avoids inventing a conflict surface no one asked for yet.

---

## States and interactions

| Element | State | Behavior |
|---------|-------|----------|
| Refresh button | not owner | **Not rendered** (absent from DOM). |
| Refresh button | owner, idle | Ghost `text-muted`; hover/focus → `text-accent`. |
| Refresh button | click | Sets `refreshing = true`, clears the status line, `POST /media/{id}/refresh`. Icon spins; button `disabled`. |
| Refresh button | in flight, second click | Ignored (button is `disabled`). Server also single-flights per item (ADR-047 / F31.7) so a stray duplicate returns "already running" and is treated as a no-op. |
| On `202` (done) | success | Re-`getMedia(id)` → repopulate `resolved/enriched/extra/fields`; `thumbVersion += 1`; render the changed / no-change line; `refreshing = false`. |
| On `202` (partial) | provider failed | Same refetch (file updates landed); render the `text-warn` partial line. |
| On file error | failure | No data change; render the `text-warn` file-error line; `refreshing = false`. |
| On `404` | item gone | Render `Already removed.`; (the detail page will 404 on reload — same as delete). |
| On `409` | soft-deleted | Shouldn't occur (control absent for deleted items, which 404 their detail), but render the server message inline if it does. |
| Status line | next action | Cleared at the start of the next refresh (and on navigating to a new id). |

No confirm dialog — refresh is safe and idempotent. (Contrast delete, which always confirms.)

---

## Responsive behavior

| Breakpoint | Changes |
|------------|---------|
| Desktop / tablet (≥640) | Refresh + Enrich + Clear + Write-to-file share the header controls row; Refresh leftmost. |
| Mobile (<640) | The controls row `flex-wrap`s; Refresh may wrap to its own line with the others. The status line is always full width below the header. Nothing truncates the label. |

---

## Edge cases

- **No provider match** — refresh re-reads the file only; the provider step is a clean no-op; the
  status line uses the *file-only* copy (no provider named). No picker is opened.
- **Provider down / slow** — the file re-extract still commits; the partial `text-warn` line names the
  provider. The 8 s per-call cap (ADR-033) bounds the wait; the spinner stays until it resolves.
- **File missing / locked** (unmounted volume, another process holds it) — refresh fails without
  changing the row's data or `active` state; file-error line. Never reports a change that didn't happen.
- **Item with no resolved metadata** — the section header (hence Refresh) still renders for owners; the
  body shows the muted `No metadata extracted yet.` hint.
- **Cover art changed by the refresh** — `thumbVersion += 1` busts the poster, same mechanism as
  Regenerate, so the new embedded art shows without a manual thumbnail rebuild.
- **Long provider name / counts** — the status line wraps; no truncation needed (it's body-width).
- **Rapid double-click** — client `disabled` + server single-flight; at most one pass runs.

---

## Animation / motion

- **Spinner** — reuse the Regenerate button's `class:animate-spin` on the icon while `refreshing`.
  Honor `prefers-reduced-motion`: gate the spin so reduced-motion users get a static icon (+ the
  "Refreshing…" label still conveys progress). Skin flourishes stay in `app.css` under `[data-theme]`,
  never per-component.
- **Status line** — appears instantly; an optional `prefers-reduced-motion: no-preference` opacity fade
  is fine. No transforms required.

---

## Accessibility notes

- Refresh is a real `<button>` with a **text label** ("Refresh") plus
  `aria-label="Refresh metadata from file and providers"`; the SVG icon is `aria-hidden="true"`. Color
  is never the only signal — the word carries the meaning.
- `disabled` while in flight; the label switches to "Refreshing…" so the state is conveyed without
  relying on the spin.
- The status line is `aria-live="polite"` so the outcome (synced / no change / failed) is announced
  without moving focus. Failure text is a full sentence (`text-warn` + words).
- Focus ring uses `focus-visible:text-accent` like Write-to-file; visible on every skin's accent.
- Owner control is **absent from the DOM** for non-owners — nothing misleading in the a11y tree.
- Reduced motion: the spinner does not convey anything the label doesn't; safe to freeze.

---

## Three-skin QA checklist (required before merge — CLAUDE.md)

> Numbered + verifier-tagged per the house convention. Switch skins via the header picker.

### Setup
- [ ] **0.1** `[smoke]` Set `ADMIN_TOKEN`, sign in on `/status` so `activity.isOwner` is true; index at
      least one media item (ideally one matched to a provider, e.g. TMDB).

### Smoke
- [ ] **1.1** `[smoke]` `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'`
      returns empty for the changed markup (raw hex only in `app.css`; `rounded-full` pills intentional).
- [ ] **1.2** `[smoke]` `svelte-check` + `vitest` green.

### Agent / human (per skin: Cinémathèque, Broadcast, Brutalist)
- [ ] **2.1** `[human]` On `/media/[id]` as owner, **Refresh** shows **first** in the Metadata controls
      row (before Enrich / Clear / Write to file) as a quiet ghost button — readable against the header
      on all three skins, and clearly **not** the accent CTA (Enrich keeps that).
- [ ] **2.2** `[human]` Clearing the token removes Refresh (and the rest of the cluster) entirely — a
      non-owner sees no Refresh button.
- [ ] **2.3** `[human]` Click Refresh on a provider-matched item: the icon **spins**, the label reads
      **Refreshing…**, the button is disabled, then a `text-muted` line reports
      `Synced · N from file, M from TMDB updated`; the metadata above updates without a reload.
- [ ] **2.4** `[human]` Edit a file's tags in another app **without changing its size/mtime**, then
      Refresh — the new values appear (proves the forced re-extract bypasses change-detection).
- [ ] **2.5** `[human]` Refresh an item with **no provider match** → file-only copy
      (`Synced from file · N fields updated`); **no picker opens**, no error.
- [ ] **2.6** `[human]` Refresh again immediately with nothing changed → `Already in sync — nothing
      changed` (muted, not warn).
- [ ] **2.7** `[human]` With the provider sidecar stopped, Refresh → the file still updates and the
      line reads `TMDB lookup failed — file metadata still updated` in **`--warn`** (distinct from the
      accent on every skin — load-bearing on Brutalist's bright accent).
- [ ] **2.8** `[human]` Refresh radius/focus per skin (`--radius` 2px/0/0): no stray rounding; the
      `focus-visible` accent ring is visible on each skin's accent; the ghost label hover→accent reads.
- [ ] **2.9** `[human]` Keyboard: Tab reaches Refresh, Enter/Space triggers it, focus stays put; the
      outcome line is announced (`aria-live`); reduced-motion freezes the spinner but the label still
      conveys progress.
- [ ] **2.10** `[agent]` `POST /api/v1/media/{id}/refresh` without owner auth → 401/403; on a
      soft-deleted id → 409 (or the detail already 404s); a double POST while one is in flight does not
      double-write (single-flight).

> Verify with `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'`
> returning empty for the changed markup.
