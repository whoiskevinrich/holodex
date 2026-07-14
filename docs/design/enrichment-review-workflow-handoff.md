# Design Handoff: Enrichment review workflow — queue, auto-apply, unmatched, refresh (F47)

**Spec**: [enrichment-review-workflow.md](../specs/enrichment-review-workflow.md) · **ADR**: [ADR-066](../architecture/ADR-066-enrichment-auto-apply-and-dismissal.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**
**Stack**: SvelteKit (Svelte 5 runes) + Tailwind v4 CSS-first (ADR-025).

This handoff extends two shipped surfaces — `EnrichPicker.svelte` (F22) and `EnrichProviderChips.svelte`
(HOLODEX-136) — and adds one new one: an **Enrichment tab** in the `/owner` hub (F35), built as the
entity-generic sibling of the **Duplicates tab** (F43/ADR-061), whose dense-row layout it deliberately
mirrors. Nothing here introduces a new token, font, or radius.

---

## Overview

Three changes, in build order (mirrors the spec's S1–S4 slices):

1. **Enrichment tab** (`/owner/enrichment`, new) — a dense, grouped queue of Person/Studio/Media rows
   still missing provider data, each row showing one status chip per provider (P0-1, P0-6/RD9).
2. **`EnrichPicker` gains two things** — a **"None of these match"** action (durable dismissal, RD4) and
   an optional **"view source ↗"** link per candidate when the provider supplies `profile_url` (RD6).
3. **`EnrichProviderChips` flips its primary action once linked** — "Enrich" becomes **"Refresh"**
   (direct `apply()`, no picker); "Re-match…" and "Clear" move into the ⋯ overflow (RD7). A
   **"Refresh all"** action is added alongside the chip row (RD8), with per-provider partial-result
   fallout shown inline.

All three are **owner-gated** (`activity.effectiveOwner`, ADR-030) and, per the spec's Goals/Non-Goals,
**lazy** — the queue's list/count is a free DB read; nothing calls a provider until the owner opens a row
or clicks a chip.

---

## Design-system fit (the `/design-system` check)

Almost nothing new visually. This is F43's Duplicates-tab idiom (dense grouped rows, per-row verdicts,
resolve-without-refetch) applied to enrichment state, plus two additive states on components that
already ship:

- **Tab shell** — add one entry to `owner/+layout.svelte`'s `tabs` array. No new chrome.
- **Grouped dense rows** — `DuplicatePairRow`'s exact rhythm (`border-t border-rule px-3 py-2.5`, group
  header `text-xs uppercase tracking-wide text-muted`) — the new `EnrichQueueRow` is a sibling, not a
  reinvention.
- **Status chip** — `EnrichProviderChips` already owns the chip shell (`border border-rule bg-surface`,
  icon + name); the queue row reuses a **read-only, smaller variant** of the same chip rather than a new
  component family.
- **Picker additions** — `EnrichPicker`'s existing dialog, candidate row, and confidence-chip idioms are
  unchanged; "None of these match" is one more button in the existing footer area, and "view source ↗"
  is one more inline link on a candidate row that already shows a `disambiguation` line.
- **Refresh-all partial result** — reuses the picker's own inline error/status line pattern
  (`text-xs text-muted`/`text-warn`), scoped per chip.

Net new component files: `EnrichQueueRow.svelte`, `ProviderStatusChip.svelte` (the read-only queue-row
chip; `EnrichProviderChips` stays the interactive detail-page version and does **not** get merged with
it — the two have different affordances). New route: `owner/enrichment/+page.svelte`. **Introduce no new
tokens.**

---

## Reuse map (don't build from scratch)

| Need | Reuse from | Notes |
|------|-----------|-------|
| Owner gating | `activity.effectiveOwner` (`activity.svelte.ts`) | Same predicate every other owner surface uses. |
| Tab shell | `owner/+layout.svelte` L13–18 | Add `{ href: '/owner/enrichment', label: 'Enrichment' }` to `tabs`. Active-tab styling (`bg-surface-2 text-ink`) unchanged. |
| Grouped queue layout | `owner/duplicates/+page.svelte` (whole file) | Same shape: `$state` rows, `$derived` groups by entity type, `$effect` load, resolve-in-place on action (no refetch). Copy the section/group-header markup verbatim (L74–88). |
| Dense row rhythm | `DuplicatePairRow.svelte` | `flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-rule px-3 py-2.5 text-sm`, `role="group" aria-label=...`. `EnrichQueueRow` follows this exactly, swapping the pair-of-names layout for name + per-provider chips. |
| Provider chip shell | `EnrichProviderChips.svelte` L62–70 | `rounded-theme border border-rule bg-surface`, icon via `ProviderIcon`. The read-only `ProviderStatusChip` (queue row) borrows this shell without the button/menu wiring. |
| Picker dialog/list | `EnrichPicker.svelte` (whole file) | Dialog chrome, roving-tabindex candidate list, `matchLabel()` confidence chip, focus-trap/Esc/return-focus — all unchanged. Add two things only (below). |
| Confirm-before-act idiom | `DuplicatePairRow`'s "choosing" state (L74–96) | Not reused directly (dismissal needs no survivor choice), but the "reveal a confirm row" pattern is the reference for keeping "None of these match" a single click, not a second modal. |
| Banner precedent (if wired later) | `DuplicatesBanner.svelte` | Not required by P0/P1, but if a future "N need review" banner is added to entity list pages, this is the component to copy — same `role=status`, `border-warn`, deep-link idiom. |

---

## 1. `/owner/enrichment` — the review queue tab

Structurally identical to `/owner/duplicates`: `$state` list, `$derived` grouping, `$effect` load-once,
resolve-in-place removal/update on action. The differences are what a row contains and how groups order.

### Resolved: Q3 — grouping and ordering

**Group by entity type, in nav order: People → Studios → Media** (not tags-first like Duplicates —
that ordering was frequency-driven, "41 of 56 pairs are tags"; enrichment has no such skew, since every
entity type rides the identical shadow-store mechanism and any of the three can dominate a given
library). Matches the header nav order (`People`, `Studios`, then the media grid) so the grouping reads
as "the same three kinds you already browse," not a new taxonomy.

**Within a group, actionable rows sort first:** rows with ≥1 provider in `needs_review` or `unreviewed`
state sort above rows that are fully `auto_applied`/`not_matched` (nothing left to do). This directly
serves Goal 1 ("one click resolves the common case") and Goal 2 ("one place to work the backlog") — the
owner's next click is always visible without scrolling past already-handled rows. Ties broken by name.

### Row anatomy (`EnrichQueueRow.svelte`)

```
[entity icon] Name                    [tmdb: chip]  [provider2: chip]     Review →
```

- Leading icon (`ti-user`/`ti-building`/`ti-video`, `text-muted`) — mirrors `DuplicatePairRow`'s
  entity-icon idiom.
- Name (`text-ink`, `truncate`), linking to the entity's detail page (opens in place, doesn't leave the
  queue — clicking the **row** action, not the name, is what triggers resolve).
- One **`ProviderStatusChip`** per outstanding/relevant provider (P0-6/RD9 — never a single collapsed
  flag). States below.
- Right-aligned row action, derived from the row's chip states (RD9): **"Review"** (≥1 `needs_review` or
  `unreviewed` provider — clicking triggers P0-2's resolve-then-route flow for those providers),
  **"Try again"** (all outstanding providers are `not_matched` — reopens the picker for the dismissed
  ones), or **nothing** (row is fully `auto_applied`/handled elsewhere — such rows sort last and are rare
  since they'd typically drop out of the queue once every provider is linked).

### `ProviderStatusChip.svelte` (new, read-only)

The queue-row sibling of `EnrichProviderChips` — same shell, no button/menu, since the *row's* action
(not the individual chip) drives resolution:

| State | Rendering |
|---|---|
| `unreviewed` | icon + name, `text-muted` — "not yet reviewed" |
| `auto_applied` | icon + name + `text-accent` "✓ Auto-applied" |
| `needs_review` | icon + name + `text-ink` "N possible" (N = candidate count once resolved; before the row's first click, just "Needs review") |
| `not_matched` | icon + name + `text-muted` "Not matched" |

Shell: `rounded-theme border border-rule bg-surface px-2 py-1 text-xs` (the `size='xs'` variant already
defined in `EnrichProviderChips` — reuse the same `txt`/`pad` derivation rather than redefining it).

### Zero-cost load, resolve-on-click (P0-1/RD2/RD3)

`GET /owner/enrich-queue` returns rows + per-provider `state` with **no** `/resolve` calls made (mirrors
`GET /owner/duplicates`'s zero-cost list). Clicking "Review" fires `/enrich/resolve` for that row's
outstanding provider(s) only, then routes per RD1: exactly one strong match → auto-apply immediately,
update the chip in place to `auto_applied` (no picker shown); anything else → open `EnrichPicker` scoped
to that provider, same as the detail page.

### Empty / loading / error

Identical wording pattern to Duplicates: `py-16 text-center text-sm text-muted` "Loading…" /
"Nothing left to review." / `text-warn` error line. A one-line intro above the groups, matching
Duplicates' descriptive paragraph:

> "Entries missing metadata from at least one source. Opening a row resolves it — an obvious match
> applies right away; anything ambiguous still asks you."

---

## 2. `EnrichPicker` additions

Both additions live in the existing dialog footer/candidate-row area — no new dialog, no new modal.

### "None of these match" (RD4/P0-4)

A secondary action below the candidate list (visible once a search has returned results, i.e. not while
`loading` or on the empty-query hint state):

```
rounded-theme px-2 py-1 text-xs text-muted hover:text-ink
"None of these match"
```

Clicking it calls the new `POST /{entity}/{id}/enrich/{provider}/dismiss`, then closes the picker via the
existing `onclose()` and reports the dismissal up (a new `ondismissed` event, sibling to `onapplied`) so
the caller (queue row or detail-page chip) can flip its own state to `not_matched` without a refetch.

Placement: left-aligned in the same row as the "Enriching…" status line, so it never competes visually
with a candidate's own confirm click. Not shown once `candidates.length === 0` (nothing to reject) — in
that state the "No matches for…" text already communicates the same outcome without needing a button.

### "View source ↗" link (RD6/P1-1)

When a candidate carries a scheme-validated `profile_url`, add it to the candidate row, after the
disambiguation line:

```html
{#if c.profile_url}
  <a href={c.profile_url} target="_blank" rel="noopener noreferrer"
     onclick={(e) => e.stopPropagation()}
     class="text-xs text-accent hover:underline">view source ↗</a>
{/if}
```

`stopPropagation` matters: the row's `onclick` applies the candidate, and the link must open in a new tab
instead of triggering apply. Absent/invalid `profile_url` → nothing rendered, no layout shift (matches
the existing `{#if c.disambiguation}` pattern immediately above it).

### Try again (clears a dismissal — reachable from queue + detail page, not inside the picker itself)

"Try again" is **not** a picker control — it's the queue row's / provider chip's action when the state is
`not_matched` (`DELETE /{entity}/{id}/enrich/{provider}/dismiss`), which then **opens** `EnrichPicker`
fresh (re-runs `/resolve`). No new picker state is needed for this.

---

## 3. `EnrichProviderChips` — Refresh, Re-match, Clear, Refresh-all

### Flipped primary action once linked (RD7/P0-5)

Today the chip's primary button always says "Enrich" (opens the picker) and "Clear" is the only overflow
item, gated on `linked(p)`. Change:

| `linked(p)` | Primary button | ⋯ overflow |
|---|---|---|
| `false` | "Enrich" (opens picker, unchanged) | none (menu absent, as today) |
| `true` | **"Refresh"** — calls `apply(p, storedExternalId)` directly, no picker | **"Re-match…"** (opens picker to pick a different candidate — today's "Enrich" behavior, relabeled) + **"Clear {p} data"** (unchanged) |

The chip needs the stored `external_id` to call `apply()` directly — either passed in via a `linkedId`
prop (`(p: string) => string | undefined`) alongside the existing `linked` predicate, or folded into one
richer `status(p)` prop returning `{ linked: boolean; externalId?: string }`. **Recommended:** the latter,
since it replaces one boolean prop with one richer one rather than adding a second — check with the
implementer whether the parent pages (`people/[id]`, `studios/[id]`, `media/[id]`) already have the
`external_id` on hand from their existing enrichment fetch (they should, since provenance badges already
need it to distinguish sources).

Button label change only — no new visual treatment: "Refresh" and "Enrich" are both the same
`text-accent` verb-in-chip styling already shipped (`EnrichProviderChips.svelte` L81).

### Refresh-all (RD8/P1-2)

One new control **next to** the chip row (not inside it — it acts on all chips at once):

```
rounded-theme border border-rule px-2.5 py-1.5 text-sm text-accent hover:bg-surface-2
"Refresh all"
```

Placement: trailing the chip `flex flex-wrap` row, same line when it fits, wrapping below on narrow
widths (`flex-wrap` already on the parent). Fires one call per configured provider (2–4, per RD8): linked
providers refresh directly, unlinked ones resolve-and-route per RD1. Results land back on each
**individual chip** — a provider that resolved ambiguously does **not** get silently dropped; it flips to
the same "needs review" affordance an individual "Enrich" click would produce (inline expand or opens
`EnrichPicker` for that one provider — implementer's choice, but it must not be silent). While in flight,
"Refresh all" itself shows the existing `busy`-string convention already used per-chip (`disabled`,
label → "Refreshing…").

---

## Layout

| Region | Layout |
|---|---|
| Enrichment tab | Same `max-w-5xl` shell as the rest of `/owner` (inherited from `+layout.svelte`); intro paragraph, then grouped sections, identical spacing to Duplicates (`space-y-5` outer, `space-y-0` per group). |
| Queue row | `flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-rule px-3 py-2.5 text-sm` — name + chips wrap together on the left, row action right-aligned, wraps below on mobile like `DuplicatePairRow`. |
| Provider chip row (detail pages) | Unchanged `flex flex-wrap items-center gap-2`; "Refresh all" appends after it in the same flex container or directly below — implementer's call based on how many providers a given entity typically has (2 fits inline; 4 may want its own line). |
| Picker additions | No new layout — both additions sit inside the existing `max-w-lg` dialog, in existing flow positions (footer area / candidate row). |

No new breakpoints anywhere in this feature.

---

## Design Tokens Used

All inherited — **no new tokens, no new radius, no new font.** ([theming.md](theming.md) reference):

| Token | Usage here |
|---|---|
| `bg-surface` / `bg-surface-2` | Queue section card bg / hover on rows if added, active tab |
| `text-ink` / `text-muted` | Entity names, chip labels / group headers, secondary chip text, "not yet reviewed", disambiguation |
| `text-accent` / `bg-accent` / `text-accent-ink` | "Auto-applied" ✓, Refresh/Re-match/Refresh-all verb text, "view source ↗" link, Try again |
| `border-rule` / `border-accent` | Chip/row borders / focus + active states (unchanged from existing components) |
| `text-warn` / `border-warn` | Errors only — a failed resolve/apply/dismiss call, exactly like every other surface. **Never** for "needs review" or "not matched" (those are neutral/advisory, not error states) |
| `rounded-theme` | Chips, rows-as-cards, buttons |
| `skin-title` / `font-display` | The `/owner` hub `<h1>` already covers this — no new heading introduced by the tab itself |

**Load-bearing distinction (mirrors the F43 regression risk):** "needs review" / "not matched" are
**not** warn states — they're normal backlog states, styled `text-ink`/`text-muted`. Only an actual
request failure (resolve/apply/dismiss erroring) gets `text-warn`. Conflating "there's a candidate to
review" with "something went wrong" would make the queue read as broken when it's just... a queue.

**Token guard**: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` stays
empty for the new/changed files.

---

## States and Interactions

| Element | State | Behavior |
|---|---|---|
| Enrichment tab | loading | `py-16 text-center text-sm text-muted` "Loading…" |
| Enrichment tab | empty | "Nothing left to review." |
| Enrichment tab | error | `text-warn` inline, `role="alert"` |
| Queue row | click "Review" | Fires `/enrich/resolve` for outstanding providers only; single strong match → chip flips to `auto_applied` in place (no picker); else → `EnrichPicker` opens scoped to that provider |
| Queue row | click "Try again" | `DELETE .../dismiss` for the row's `not_matched` provider(s), then opens `EnrichPicker` for it |
| Queue row | after auto-apply | Chip → `auto_applied`; row action recalculates (may disappear if that was the last outstanding provider) |
| Provider chip (unlinked) | click primary | Opens `EnrichPicker` ("Enrich"), unchanged |
| Provider chip (linked) | click primary | Direct `apply()` call ("Refresh") — no `/resolve`, no picker; busy label "Refreshing…" |
| Provider chip (linked) | ⋯ → Re-match | Opens `EnrichPicker` to pick a different candidate (today's default "Enrich" behavior) |
| Provider chip (linked) | ⋯ → Clear | Unchanged existing behavior |
| Refresh-all | click | Fans out per RD8; each provider's own chip reflects its own outcome; button shows "Refreshing…" while any are in flight |
| Refresh-all | partial ambiguous result | The ambiguous provider's chip/row surfaces "needs review" inline — never silently skipped |
| Picker | "None of these match" click | Records dismissal, closes picker, caller flips state to `not_matched` |
| Picker candidate | has `profile_url` | Shows "view source ↗", opens new tab, does not trigger apply |
| Picker candidate | no `profile_url` | No link, no layout shift |
| Any owner control | not owner | Absent from the DOM, not merely hidden (existing convention) |

---

## Responsive Behavior

| Breakpoint | Changes |
|---|---|
| Desktop / tablet (≥640) | Queue rows single-line where they fit; chips wrap within the row if an entity has many providers. Refresh-all sits inline after the chip row. |
| Mobile (<640) | Queue row action wraps below name+chips (`flex-wrap`, matching `DuplicatePairRow`); Refresh-all wraps to its own line below the chip row. |

No new breakpoints — this is a worklist like Duplicates, not a grid.

---

## Edge Cases

- **Entity has zero configured providers for its type** — doesn't appear in the queue at all (P0-1's
  membership rule already excludes it: "missing an `entity_enrichment` row for at least one provider
  whose `entity_types` includes its type").
- **All providers already `not_matched`** — row shows only "Try again"; no "Review" text competes with it.
- **Row resolved elsewhere while the tab is open** (owner enriches via the detail page in another tab) —
  mirrors Duplicates' "stale row 404s → treat as already-handled, no error toast, drop the row on next
  action" pattern.
- **Refresh on a Person whose `external_id` isn't actually usable for identity yet** (ADR-055 gap,
  HOLODEX-125, spec Open Question Q1) — until that lands, Person's Refresh/Refresh-all should either be
  scoped out (chip primary stays "Enrich"/"Re-match" only) or the refresh call is expected to fail
  gracefully with the existing error-line treatment. **Confirm scope with engineering before S4 ships for
  Person** — this handoff assumes Studio/Media get Refresh first regardless.
- **Large queue** — same as Duplicates: single scroll, grouped, no pagination at personal-library scale.
- **`profile_url` with a hostile scheme** (`javascript:`, `data:`) — never reaches the client; validated
  server-side per P1-1's own requirement, same posture as the rest of the provider contract.
- **International names (CJK, diacritics)** — queue row `name` truncates with `title` full text, same as
  every other entity list in the app.
- **Refresh-all on an entity with a provider mid-cooldown/unreachable** — that provider's own error
  surfaces on its chip (`text-warn`), the rest of the fan-out proceeds independently.

---

## Animation / Motion

No new motion. Reuses exactly what ships today, gated behind
`@media (prefers-reduced-motion: no-preference)`:

| Element | Trigger | Animation | Duration | Easing |
|---|---|---|---|---|
| `EnrichPicker` dialog | open (from queue row or chip) | `enrich-rise` (existing) | 150 ms | `cubic-bezier(0.2,0.7,0.2,1)` |
| Queue row | resolved / dismissed / auto-applied | none (in-place text/chip swap, no transform) — matches Duplicates' "row fades out" only applying to *removal*, not state change | — | — |
| Duplicates-style row removal | N/A here — enrichment rows don't disappear on resolve (they update in place); only truly "nothing left to do" rows sink to the bottom via re-sort | — | — |

---

## Accessibility Notes

- **Queue rows** are a `role="group" aria-label="{name}: enrichment status"` (mirrors
  `DuplicatePairRow`'s pattern), so a screen reader gets the row as one unit before its chips.
- **`ProviderStatusChip`** is not interactive (no button semantics) — it's a labelled status; ensure the
  full state reads via text content, not color alone ("✓ Auto-applied", not just an accent checkmark).
- **"None of these match"** is a real `<button>`, keyboard-reachable in the existing dialog's tab order
  (falls naturally after the candidate list, before nothing else needs to follow it).
- **"view source ↗"** link needs an accessible name beyond the arrow glyph — `aria-label="View {c.label} on {provider}'s site (opens in a new tab)"` or equivalent visually-hidden text, since "↗" alone isn't announced meaningfully.
- **Refresh-all** busy state: `aria-live="polite"` region (or `aria-busy` on the button) so "Refreshing…"
  is announced, matching the picker's existing `aria-live="polite"` status line convention.
- **Tab addition**: `owner/+layout.svelte`'s nav is already a labelled `<nav aria-label="Owner tools">` —
  no change needed beyond adding the entry.
- **Owner-only controls** absent from the DOM for non-owners (existing convention, not new here).

---

## QA checklist (3-skin)

Conventions ([[feedback-qa-checklist-numbering]]): every item numbered `section.item`, tagged by
verifier — `[smoke]` automated, `[agent]` agent-driven live QA, `[human]` needs human eyes. Skins:
**Cinémathèque · Broadcast · Brutalist**, switched via the header picker. (The exhaustive functional
matrix is `/testing-strategy`'s job, still pending per the spec's Timeline — this is the design-surface
subset.)

### §1 Setup
- **1.1** `[agent]` Start a backend with ≥1 Person/Studio/Media entry missing provider data across at
  least two providers ([[reference-holodex-preview-testbeds]]); enter the admin token
  (`activity.effectiveOwner`), then confirm the tab/controls are absent as a visitor (no token).

### §2 Smoke
- **2.1** `[smoke]` `GET /owner/enrich-queue` returns grouped rows with per-provider `state`, zero
  provider calls made on load (network log stays empty until a row is clicked).
- **2.2** `[smoke]` Dismiss/undismiss/refresh/refresh-all endpoints are `requireOwner`-gated (401/403 when
  unauthenticated).

### §3 Agent live QA (all 3 skins)
- **3.1** `[agent]` **Enrichment tab** renders grouped rows (People → Studios → Media), chips per provider
  legible in each skin, tab entry styled like the existing Status/Keys/Duplicates/Trash tabs (no visual
  outlier).
- **3.2** `[agent]` **Auto-apply on open**: a row whose sole outstanding provider resolves to one
  `>=0.85` candidate flips straight to "✓ Auto-applied" (`text-accent`) with no picker shown; a row with
  two strong candidates opens the picker instead.
- **3.3** `[agent]` **"None of these match"**: dismiss a candidate set from the picker (opened from a
  queue row), confirm the chip reads "Not matched," reopening the picker for it does not re-fire
  `/resolve` until "Try again," and the queue's actionable-first sort moves the row down.
- **3.4** `[agent]` **View-source link**: a candidate with `profile_url` shows "view source ↗", opens a
  new tab, and does **not** trigger apply (click doesn't close the picker or apply the candidate).
- **3.5** `[agent]` **Refresh vs Re-match vs Clear**: on a linked provider chip, primary action is
  "Refresh" (direct apply, no picker/network `/resolve` call); ⋯ menu offers "Re-match…" (opens picker)
  and "Clear {p} data" (unchanged); on an unlinked provider, primary is still "Enrich."
- **3.6** `[agent]` **Refresh-all partial result**: on an entity with one linked + one ambiguous
  unlinked provider, "Refresh all" silently refreshes the linked one and surfaces "needs review" inline
  for the ambiguous one — never drops it silently.
- **3.7** `[agent]` **Warn vs neutral separation** (the F43 regression risk, re-verified here): "needs
  review"/"not matched" chips read `text-ink`/`text-muted`, never `text-warn`; only an actual
  resolve/apply/dismiss failure shows `text-warn`. Check this holds on **Brutalist** (bright lime accent
  vs. hot red-orange warn) where the two are most likely to visually collide.

### §4 Human
- **4.1** `[human]` Open the Enrichment tab in each skin. It should feel like the Duplicates tab's
  sibling — same density, same "tidy worklist" feeling — not a bespoke new screen.
- **4.2** `[human]` Work a row end-to-end: click Review on an obvious match and watch it resolve with no
  extra click; then click Review on an ambiguous one and confirm the picker still feels like the same
  picker you already know, just reached from a new place.
- **4.3** `[human]` On a person/studio/media detail page, click "Refresh" on an already-linked provider —
  it should feel instant (no search, no picker flash) compared to the "Enrich" flow on an unlinked one.

> Verify with `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` returning
> empty for the new/changed markup.

---

## Open questions for the build (non-blocking)

1. **Chip prop shape for Refresh** — `linkedId(p)` (new prop) vs. folding `linked`+id into one
   `status(p)` prop. Recommended: the richer single prop (see §3 above); confirm with whoever implements
   `EnrichProviderChips` since it's a breaking prop-signature change for three call sites
   (`people`, `studios`, `media` detail pages).
2. **Refresh-all's ambiguous-result UI** — inline expand within the chip row vs. auto-opening
   `EnrichPicker` for just that one provider. Either satisfies P1-2's "never silently drop it"
   requirement; pick whichever is less code given the existing chip/picker wiring.
3. **Person Refresh/Refresh-all scope** — gate on HOLODEX-125 landing (Q1 in the spec) or ship
   Refresh/Refresh-all for Studio/Media only at first and add Person once ADR-055's identity gap closes.
   Recommended: scope Person out of P0-5/P1-2 initially rather than blocking the whole slice on HOLODEX-125.
