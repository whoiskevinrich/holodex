# Design Handoff: Entity Completeness Score — Remediation Queue & Breakdown Panel (HOLODEX-260)

**Spec**: [entity-completeness-score.md](../specs/entity-completeness-score.md)
**ADR**: [ADR-081](../architecture/ADR-081-entity-completeness-score.md) — facet criticality, `facet_not_applicable` table, compute-on-read scoring
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**
**Prior art**: `owner/extraction` + `ExtractionQueueRow.svelte` (HOLODEX-199) — reuses the "individual apply only, no bulk" idiom and the `tier: 'conflict' | 'weak' | null` badge vocabulary. Does **not** reuse the component itself: extraction groups rows by *video* with heterogeneous per-field editors (entity chips vs. scalar diffs); this queue groups by *facet* with a uniform row shape (apply / search / upload). Forcing one shared component across both would mean a wide union-typed prop surface for two genuinely different interaction models — the spec's own Frontend section calls this out explicitly ("visual language, not shared code").
**Depends on**: F55.1–4 (registry criticality + tri-state resolution + score/actionability compute) and F55.10 (not-applicable mutation) — items #2–3 in the flightplan's Up next, not yet built. This handoff specs the UI to build against once that backend work lands; it does not block starting frontend scaffolding in parallel (empty/loading states don't need real data).
**Surfaces**:
- New route: `web/src/routes/owner/completeness/+page.svelte` (remediation queue)
- New components: `web/src/lib/components/completeness/CompletenessQueueRow.svelte`, `web/src/lib/components/completeness/CompletenessPanel.svelte`
- New card embedded in three existing detail pages: `web/src/routes/media/[id]/+page.svelte`, `web/src/routes/people/[id]/+page.svelte`, `web/src/routes/studios/[id]/+page.svelte`
- **Scope of this handoff**: the remediation queue and the breakdown panel, per the current request. § 9 covers the browse "Completeness" sort + "Missing facet" filter briefly — it's pure reuse of `FacetFilter.svelte`/`SortDropdown` with zero new components, so it doesn't need a full separate handoff, but it's included here so the design gate for F55 closes in one document.

---

## Overview

### Design-system-fit audit

Two new surfaces, both owner-mode-only diagnostic/remediation tools — the same category as
`owner/extraction` and the per-entity curation cards already on the detail pages. Nothing here
needs a new interaction primitive: the queue is a grouped list of rows with one action per row
(the extraction/enrichment queues already established this shape), and the panel is a card with a
`dt`/`dd`-style status list plus one toggle (the tag-detail "Details" card already established that
shape too, right down to the toggle's dt/dd + trailing icon-button anatomy). What's genuinely new
is the **content vocabulary** — a tri-state tier (curated / provider / missing / not-applicable)
that doesn't exist anywhere else in the app yet — so most of this handoff's decisions are about
which existing visual idiom best carries that new vocabulary, not about inventing new chrome.

---

## 1. The remediation queue

### DD1 — Grouped by facet, ordered critical-first then by count

Groups are keyed by canonical facet (`"Missing poster"`, `"Missing genres"`, …), each group split
into **Candidate-ready** and **Needs research** subsections. Group order: **critical facets before
nice-to-have**, and within the same criticality tier, **larger groups first** (mirrors
`ExtractionQueueRow`'s "most pending fields sort first, clears the most backlog per click," here
applied to which *kind* of gap matters most rather than which video has the most gaps). Within a
subsection, entities sort alphabetically by title/name — a plain, deterministic default; the spec
doesn't call for anything richer here.

**Chosen over**: pure count-descending order (simpler, but would let "Missing genres · 40" bury
"Missing poster · 3" below the fold, even though poster is a critical facet and genres isn't —
the spec's whole premise is that critical gaps matter more, so the queue's own ordering should
say so too).

Within a facet group, **Candidate-ready always renders above Needs research** — quick wins first,
which is the entire reason the actionability signal exists (User story 3).

### DD2 — Row anatomy

```
[thumbnail]  Entity name (link)              [provenance/candidate hint]        [action]
```

- **Thumbnail**: reuse each entity type's existing small-thumbnail treatment (`VideoCard`'s poster
  crop, `PersonAvatar`, the studio branding-image slot) at a compact fixed size — decorative,
  `alt=""`, the entity name link carries the accessible label.
- **Entity name**: links to the detail page (`media/[id]`, `people/[id]`, `studios/[id]`).
  `truncate` + a `title` attribute for overflow, matching `ExtractionQueueRow`'s convention for
  long video titles.
- **Candidate-ready rows** show a `ProvenanceBadge` for the cached candidate's source (which
  provider it would come from) so the owner isn't applying a blind guess.
- **Needs-research rows** show nothing in that slot — there's no candidate to describe yet.

### DD3 — One accent action per row; navigation and mutation share the same visual weight

- **Candidate-ready** → a single **Apply** button, `.btn-accent`. Applying is an in-place mutation:
  button shows a spinner and disables while in flight; on success the row is removed from local
  state immediately and the group's count re-derives (same pattern as `ExtractionQueueRow`'s
  `onhandled`/`dropRow`) — no toast, the row disappearing *is* the confirmation.
- **Needs-research** → a single **Search** button, also `.btn-accent`, but it *navigates* to the
  entity's detail page (anchored at that facet's enrichment control) rather than mutating anything
  here. For the three image-typed facets only (`poster_url`, `photo`, `branding_image`), a second,
  lighter affordance appears beneath it: **"or upload an image directly"** as a `.btn-quiet` text
  link, also navigating to the entity page's upload control.

**Chosen over**: giving Search/Upload a different color than Apply (e.g. `.btn-ghost`) to signal
"this navigates, it doesn't mutate." Rejected — from the owner's point of view every row has
exactly **one** obvious next step, whether that step happens to mutate in place or hand off to
another page; splitting the palette by mutate-vs-navigate would fragment the three-role button
vocabulary (`.btn-accent`/`.btn-ghost`/`.btn-quiet`, all already meaning something specific
elsewhere) for a distinction that doesn't matter to the owner deciding what to click. The label
text plus a trailing external-link glyph on Search/Upload carries the "this takes you elsewhere"
signal instead.

**Chosen over** (for the Upload affordance specifically): two co-equal `.btn-accent` buttons
side by side (Search / Upload). Rejected — two same-weight accent buttons on one row reads as
"pick either," which isn't true (search is the default path for every facet; upload only exists
for images, and even then is the secondary path). Demoting Upload to a quiet inline link keeps one
clear primary action per row.

### Empty / loading / error states

This route hand-rolls its own loading/error/empty block inline — the same choice `owner/extraction`
already made rather than the shared `AsyncState.svelte` wrapper, because both pages need extra
chrome above the basic loading state (here: nothing extra today, but keeping the pattern
consistent with the nearest sibling queue is more valuable than a marginal reuse).

| State | Copy | Markup |
|---|---|---|
| Loading | "Loading…" | `py-16 text-center text-sm text-muted` |
| Error | (the fetch error message) | `py-16 text-center text-sm text-warn`, `role="alert"` |
| Empty (no missing facets anywhere) | "Nothing to remediate — every scored facet across your library is resolved or marked not applicable." | `py-16 text-center text-sm text-muted` |
| Empty subsection (e.g. a group with zero candidate-ready rows) | — | The subsection heading itself is omitted, not shown with a "(0)" count |

---

## 2. The completeness breakdown panel

### DD4 — Placement: a new owner-only card, high in the stack

A new `<section>` card following the existing two-tier shell (`rounded-theme border border-rule
bg-surface p-4` outer, `<h2 class="text-xs uppercase tracking-wide text-muted">Completeness</h2>`
header) on each of the three detail pages. Positioned near the top of the owner-only card stack —
right after the page's primary Metadata/Details card — since it's a diagnostic summary the owner
wants on first glance at a low-scoring entity, not something to scroll past.

The whole card is owner-gated (`{#if isOwner}`, entire card, not just its controls) — mirrors the
tag-detail page's Details card, and matches the spec's Access control section directly: score and
actionability are not exposed to non-owners at all.

### DD5 — Header stats: a score bar, and actionability only when it means something

```
Completeness
██████████████░░░░░  65%
2 of 2 missing facets have a cached candidate ready — 100% actionable
```

- **Score**: a large number (`font-display`, same heading treatment as other card titles) plus a
  thin horizontal bar — `bg-surface-2` track, `bg-accent` fill sized to the score percentage. Only
  `--surface-2`/`--accent` are used; no new tokens.
- **Actionability**: a smaller secondary line, shown **only when the entity has at least one
  missing facet**. When there are zero missing facets, this line is replaced with "Fully
  complete" rather than computing a 0-of-0 ratio, which the spec's formula leaves undefined.

### DD6 — Facet list: grouped by criticality, one row per facet

Two `<h3 class="text-xs uppercase tracking-wide text-muted">` subheadings — **Critical** and
**Nice to have** — matching the vocabulary the ADR/spec already use, each listing its facets as
compact flex rows (`justify-between`): label on the left, status pill on the right. A flat
single-column list, not the two-column `dl` grid the Metadata card uses — with 15+ facets on a
video, a dense single-column list scans faster than a two-column grid would.

### DD7 — Status pill vocabulary (new, but built from existing idioms)

| Tier | Rendering | Rationale |
|---|---|---|
| **Curated** | small pill, `border-accent text-accent`, "Curated" | Outlined accent already means "an affirmative, owner-touched state" elsewhere (Stage/Apply buttons); a curated value is exactly that. |
| **Provider** | the existing `ProvenanceBadge` component (provider icon + name) | Richer than a generic "Provider" label — tells the owner *which* provider resolved it, and it's a zero-new-code reuse. |
| **Missing** | dashed pill, `border-dashed border-rule text-muted`, "Missing" | Echoes `CurationChip`'s existing "pending" motif (dashed ring = a signal state that isn't a committed decision) rather than inventing a new one. Deliberately **not** `text-warn` — most missing nice-to-have facets are a normal, expected state, not an error, and reserving `--warn` for genuine attention states (like a failed mutation) keeps that token meaningful. |
| **Not applicable** | plain `text-muted`, no border, "Not applicable" | The quietest state on purpose — it's an intentional exclusion the owner asserted, not a gap needing attention. |

### DD8 — The not-applicable toggle: a direct reversible flip, no confirm dialog

Rendered only on the `external_provider_id` row (the only v1 not-applicable UI target per spec
scope) as a trailing icon-button after the status pill — reusing the tag-detail Details card's
`dt`/`dd` + icon-button toggle shape exactly: `border-rule text-muted` when off, `border-accent
text-accent` when on, `aria-pressed` carries state, a busy state disables the button mid-request,
and an error surfaces as `text-xs text-warn` directly below the row.

**Chosen over**: wrapping the toggle in `ConfirmDialog`. Rejected — ADR-081 and the spec's Access
control section both frame this as "a simple owner-gated boolean-ish flag," fully reversible with
one more click. That puts it in the same category as the tag writeback toggle (no confirm) rather
than a reparent or delete (confirm required) — the existing repo convention already draws this
line, and this flag falls on the no-confirm side of it.

---

## 3. Components

| Component | Status | Notes |
|---|---|---|
| `owner/completeness/+page.svelte` | New | Queue page shell — own loading/error/empty block, not `AsyncState` (see § 1) |
| `completeness/CompletenessQueueRow.svelte` | New | One (entity, facet) row — DD2/DD3 |
| `completeness/CompletenessPanel.svelte` | New | Breakdown panel card — DD4–DD8. May internally split a `CompletenessFacetRow` sub-component if the facet-row markup grows; not required as a separate file for v1's scope. |
| `ExtractionQueueRow.svelte` | Not reused | Visual vocabulary only (tier badge colors) — see Prior art above |
| `ProvenanceBadge.svelte` | Reused unmodified | Candidate-ready queue rows (DD2) and panel Provider pills (DD7) |
| `VideoCard` / `PersonAvatar` / studio branding-image slot | Reused (thumbnail markup only) | Row thumbnails (DD2) |
| `FacetFilter.svelte`, `SortDropdown.svelte` | Reused unmodified | § 9 |
| A new `web/src/lib/components/completeness/CLAUDE.md` | New | Required by this repo's component-folder convention — a new domain folder needs its own file-purpose table (see `web/src/lib/components/CLAUDE.md`) |

## 4. Tokens

No new tokens. Everything above uses the existing set: `bg-surface`, `bg-surface-2`, `text-ink`,
`text-muted`, `border-rule`/`border-dashed`, `bg-accent`/`text-accent`/`text-accent-ink`,
`text-warn`/`border-warn`, `rounded-theme`, `font-display`, plus the shared `.btn-accent`/`.btn-quiet`
button roles.

## 5. States

| Element | State | Behaviour |
|---|---|---|
| Queue Apply button | Default → busy → gone | Spinner + disabled while applying; row removed from local state on success (no toast) |
| Queue Search/Upload | Default | Navigates immediately, no busy state (it's a link, not a mutation) |
| Not-applicable toggle | Off / On / Busy / Error | `border-rule text-muted` ↔ `border-accent text-accent`; disabled mid-request; error text below the row, not a dialog |
| Score bar | Populated | Width = score%; `font-display` number above it |
| Score bar | Fully complete (0 missing) | Actionability line replaced with "Fully complete" (DD5) |
| Queue page | Loading / Error / Empty | See § 1 table |

## 6. Accessibility

- Facet-group headings are real `<h3>` elements so screen-reader users can jump between groups.
- Row action buttons carry an explicit `aria-label` beyond the visible "Apply"/"Search" text — e.g.
  `aria-label="Apply candidate poster to {entity name}"` — since the visible label alone doesn't
  say which entity or facet the button acts on.
- The not-applicable toggle carries `aria-pressed` and an `aria-label` describing the action that
  will happen next (mirrors the existing tag-writeback toggle), not just its current state.
- Focus order within a row is plain DOM order (thumbnail is non-focusable/`alt=""` → entity link →
  action button) — **no roving-tabindex pattern here**. Roving tabindex applies to typeahead/popup
  result lists where Tab should skip past individual results; this queue's rows are ordinary
  in-page links and buttons, so normal Tab order is the correct (and expected) behavior.
- The score bar is not decorative — give it `role="img" aria-label="{score}% complete"` (or an
  equivalent visually-hidden text node), since its width and fill color convey information a
  screen reader can't otherwise recover from an empty `<div>`.

## 7. Edge cases

- **Zero missing facets on an entity**: the breakdown panel shows "Fully complete" instead of an
  actionability ratio (DD5); such an entity never appears in the remediation queue by definition.
- **A facet group with only candidate-ready or only needs-research rows**: the empty subsection's
  heading is omitted entirely rather than rendered with a "(0)" count.
- **Every scored facet on an entity is not-applicable** (only theoretically possible today, since
  v1's not-applicable UI is scoped to one facet): the score formula's denominator would be zero.
  The panel should defensively render "No scored facets" rather than dividing by zero — flagging
  this for the backend/frontend implementer even though it can't occur in practice yet, since the
  underlying tri-state model is generic (ADR-081) and a future facet's not-applicable UI (F55.16)
  could make it reachable.
- **Long entity names/titles** in queue rows: `truncate` + `title=` tooltip, matching
  `ExtractionQueueRow`'s existing convention.
- **A facet's cached candidate becomes stale mid-session** (another tab/process applies or clears
  it): out of scope for this handoff — the queue reflects whatever it loaded; a stale Apply
  failing server-side surfaces as a normal row-level error, no special UI beyond that.

---

## 8. Visual reference

See the rendered mockup delivered alongside this handoff (Cinémathèque skin, both surfaces). Real
token values used (Cinémathèque, for reference — QA all three skins before merging per § 10):
`--surface:#15110e`, `--surface-2:#181310`, `--rule:#2a2622`, `--ink:#f3ece1`, `--muted:#9b9082`,
`--accent:#e8a33d`, `--accent-ink:#1a1206`, `--warn:#e2603f`, `font-display: Fraunces Variable`.

---

## 9. Browse "Completeness" sort + "Missing facet" filter (F55.5/6)

Included here only for completeness of the design gate — this is pure reuse, no new component:

- **Sort**: add a `completeness` entry to the shared sort-options source `SortDropdown` already
  reads from (`$lib/filters`'s `MEDIA_SORTS`-equivalent for each entity type), labeled
  "Completeness" with ascending/descending directions like every other sort option. No new markup.
- **Filter**: a new `FacetFilter.svelte` instance per list page, same component and interaction as
  the existing Tags filter, populated with that entity type's scored facet list (`Lookup()`
  entries carrying a criticality) instead of tag names — selecting one filters to entities where
  that facet's tri-state status is `missing`.
- Both read the same backend predicate the remediation queue uses (F55.6's explicit requirement),
  so no separate design decision is needed here beyond what § 1–2 already specify.

---

## 10. QA gate

Per `.claude/rules/frontend-theming.md`: render and eyeball **Cinémathèque, Broadcast, and
Brutalist** for every state in § 5 before merging — loading/error/empty on the queue, and
populated/fully-complete on the panel, in all three skins. `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` must stay empty for the new files. A dedicated QA
checklist doc (following this repo's numbered-`section.item`, `[smoke]`/`[agent]`/`[human]`-tagged
convention) can be authored alongside frontend implementation or as part of the testing-strategy
gate (flightplan item #7) — not required to land with this handoff.
