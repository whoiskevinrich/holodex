# Design handoff: Media detail stage layout

**Status:** Implemented (pending review)
**Phase:** HOLODEX-363 (Story) under epic HOLODEX-12 "Video detail page"
**Owner:** Project owner
**Date:** 2026-09-10
**Spec:** none — rearrangement and re-gating of existing elements; no new behavior surface
**ADR:** none — see §5. An earlier revision of this handoff required one; the unconditional
overview rule removed the need.
**Branch/PR:** `HOLODEX-363-media-detail-stage-layout`

## Overview

Three layout decisions for `web/src/routes/media/[id]/+page.svelte`, arrived at from a
`/design-critique` of the page under the HOLODEX-342 stress fixture. The trigger was the
**visitor view**: the metadata rail is sized for an owner, and when the owner-only sections drop
out a visitor is left with Tags and People in a column built to hold six blocks — roughly 70%
empty, and at ultrawide that empty column is 1073px wide.

![Block order and width scope, owner and visitor, at three viewports](media-detail-stage-layout-mockup.svg)

## The threshold

**2648px** = `--container-stage` (2600) + `<main class="px-6">`'s 48px of padding.

- **Below it** the stage fills the window; there is no gutter. "Stage width" and "window width"
  are the same pixel, so any rule phrased as justification or breakout is a no-op.
- **At or above it** the stage stops growing and gutters open either side.

**Exactly one rule keys off this: the shelf justification in §2.** An earlier revision made the
overview move viewport-conditional too, and the threshold was shared. That is no longer the case —
§3's overview rule is unconditional — so this number belongs to the shelf alone and should not be
generalised into a page-wide breakpoint.

## 1. File joins the enrichment group

`File` leaves the metadata rail and sits **directly above** the `Enrichment data:` disclosures at
the bottom of the page. It stays owner-only (`isOwner`, i.e. `activity.effectiveOwner`). The three
never separate: File, `Enrichment data: File Extraction`, then one `Enrichment data: <provider>`
block per provider — one audit group, in that order.

It sits **inside** `max-w-stage`, following the enrichment disclosures rather than going
window-scoped, so its `field-grid` resolves **7-across** at 2600px (not 8 — `gap-2` costs a column:
8×320 + 7×8 = 2616 > 2568). That is a much better home for the `col-span-full truncate` `Path:` row
than a 320px rail ever was.

Supersedes the "File moved above Completeness" point in
[`media-detail-reorder-handoff.md`](media-detail-reorder-handoff.md) — Completeness stays in the
rail, File does not.

## 2. More-with shelves break the width cap

![Shelf justification rule and the CSS that already implements it](media-detail-stage-layout-shelf-rule.svg)

Shelves span the window, not the stage. **Cards left-justified while the stage still fills the
window; the box centred once it does not.**

This is **not new CSS**. `.video-grid.stage-aligned` in `app.css` already encodes exactly this
rule (HOLODEX-331 §9.6):

| Declaration | Effect | The rule it delivers |
|---|---|---|
| `justify-content: start` | tracks pack left inside the box | left-justified under the cap |
| `min-width: min(var(--container-stage), 100%)` | box never narrower than the stage | short row still aligns to the player |
| `max-width: 100%` | box grows to the container | edge-to-edge for a long row |
| `margin-inline: auto` | box centred in its container | centred once it overhangs the stage |

Track maths live in `$lib/stageGrid` (`stageGridTracks` / `stageGridWidth`), pinned by
`stageGrid.test.ts`. One call site uses it today: the film detail page's Scenes grid, via
`VideoGrid`'s opt-in `stageAligned` prop. The behavioural rule and its traps are recorded in
[`web/src/lib/components/video/CLAUDE.md`](../../web/src/lib/components/video/CLAUDE.md) so it is
not re-decided per component.

### Two traps

1. **`RelatedShelf` is not a `VideoGrid`.** It is a `flex … overflow-x-auto` horizontal scroller
   with fixed-width cards (`w-32`/`w-52` per `data-layout`), borrowing the `.video-grid` class only
   to reset the Brutalist `reel` counter and inherit `data-layout` sizing. There is no
   `stageAligned` prop to pass — the class goes on directly, and `width: fit-content` against
   `overflow-x: auto` is **not** the same layout as against a grid. **Verified live: it is.** At
   5120, 5 cards pin the box to 2600 with zero overhang; 16 cloned cards grow it to 3824 with a
   612px overhang each side, symmetric to the pixel; 30 cards cap it at the window and scroll
   internally with no document overflow. Shipped as `.stage-band`, a second selector on the
   existing `.video-grid.stage-aligned` rule — on the `<section>`, so the heading travels with
   the cards.
2. **Breaking the cap means leaving the article.** `max-width: 100%` resolves against the
   *container*, so a shelf inside `<article class="mx-auto max-w-stage">` caps at 2600 forever.
   The shelves must become **siblings** of that article, which also drops them out of its
   `space-y-6` rhythm — restate the vertical gap explicitly at the new call site.

### Free win: this fixes the one-column order

The shelves currently live in the subject column. Below `lg`, `stage-grid` collapses to one column
and the rail stacks under the subject in DOM order, so today a phone shows
`… studio → shelves → tags → films/people → metadata`. Moving the shelves to a sibling band
*after* the grid lifts the entity links above the recommendation shelves at no extra cost. Visual
and focus order still match the DOM — no `order-*` anywhere.

## 3. Visitor rail regains read-only field values, and the overview moves into it

### 3a. Read-only values for visitors

Reverses the owner-only re-gate of the Metadata section made in
[`media-detail-reorder-handoff.md`](media-detail-reorder-handoff.md) §2 (the comment at
`+page.svelte` reads "Owner-only (media-detail-reorder) — visitors previously saw a filtered
subset").

Visitors see the resolved field values again. They do **not** get:

- `SourceBadge` (the ADR-051 precedence chip row)
- the `Enrich` / `Refresh` / `Extract from filename` controls
- `Write decisions to file`
- `PromotedFieldEdit` or any `isReplaceField` edit affordance

They **do** get each value's `ProvenanceBadge`, consistent with the rest of the page. Follow the
idiom already used at the Tags, Studio and Overview blocks — `{#if isOwner || hasValue}` — rather
than a bare `{#if isOwner}`.

### 3b. The overview moves to the rail — unconditionally

The overview block relocates to the **top of the rail, above Tags**. At every viewport, for both
roles. No media query, no viewport branch.

Below `lg` the grid is one column and the rail stacks under the subject in DOM order, so the rule
still reads correctly there: the overview simply becomes the first block after Studio rather than
sitting between the meta line and Studio. That is the one visible cost — on a phone the synopsis is
separated from the title by the studio card. Accepted as the price of one rule instead of two.

**Why the rail is the right home.** `stage-grid`'s 320px rail floor was chosen, in HOLODEX-331, as
"where a field label, its value and its `SourceBadge` chip row still fit on one line". The
overview's owner rendering *is* a `SourceBadge`. The rail was already sized for this block.

**What this does not change.** The overview stays its own block above Tags — it does **not** go back
into the Metadata field list. The `media-detail-entity-ux` reasoning that "the synopsis reads as page
content, not as a data-management row" survives; only its column changes.

### 3c. Divergence from the film detail page — being resolved, not accepted

`films/[id]/+page.svelte` currently does the opposite: the description renders as `ExpandableText`
in the header (subject column, all roles) and the whole "Details" section is owner-only, reasoned in
a comment as *"the description a visitor wants already renders in the header above, and everything
else here exists to serve editing decisions."* Both pages share `stage-grid`.

The owner's call is to **generalise, as a follow-on**: the media page changes here, and
**HOLODEX-364** moves the film page's description into its rail on the same rule — which also means
un-gating that part of `Details` for visitors exactly as §3a does here, and re-checking the
header/banner `-mb-14` overlap once the header loses a block. The column contract itself is recorded
in [`web/src/routes/CLAUDE.md`](../../web/src/routes/CLAUDE.md) so the film work inherits it rather
than re-deriving it. Do not treat this handoff as licence to leave the two pages disagreeing.

## 4. Measured tracks

`stage-grid` is `minmax(0, 1fr)` below `lg` (1024px) and
`minmax(0, 1.4fr) minmax(320px, 1fr)` at or above it, gap 24.

| Viewport | CSS px | Subject | Rail | Shelf | Metadata `field-grid` | File `field-grid` | Overview |
|---|---|---|---|---|---|---|---|
| Ultrawide 5120×1440 | 5120×1440 @100% | 1503 | 1073 | 5072, centred | 3-across | 7-across | in rail |
| Desktop 4K @200% | 1920×1080 | 1078 | 770 | 1872, left | 2-across | 5-across | in rail |
| Desktop 4K @150% | 2560×1440 | 1451 | 1037 | 2512, left | 3-across | 7-across | in rail |
| Narrowest two-column | 1024×768 | 555 | 397 | 976, left | 1-across | 2-across | in rail |
| Pixel 7 Pro | 412×892 @DPR 3.5 | 364 (one col) | — | 364, left | 1-across | 1-across | in stack |

Gutter is 1260px each side at ultrawide and zero everywhere else in this table.
`field-grid` is `repeat(auto-fit, minmax(min(320px, 100%), 1fr))` with a `gap`; column counts above
are the largest *n* with `n×320 + (n−1)×gap ≤ container − 32px of p-4`. Measured live at 5120,
1920, 1024 and 412: subject/rail 1503/1073, 1069/764 (15px scrollbar), 547/390, 364.

**Measure check.** As prose, the overview reads at roughly 52 characters per line in a 397px rail,
100 at 770px and 140 at 1073px. The subject column would give 73, 140 and 200 respectively. So the
rail is the better measure at wide viewports and the worse one at the narrow end of two-column —
this is a simplicity win, not a typographic one, and should not be argued as the latter.

## 5. Why this no longer needs an ADR

An earlier revision made the overview move viewport-conditional. That could not be a second render:
`#field-overview` is a deep-link anchor, and the codebase already guards this exact hazard for
`#field-actors` ("rendered in exactly one branch below, so the id is never duplicated"). A
media-query branch would have duplicated the id, so the move would have had to be CSS *placement* —
collapsing the two column wrappers into one flat grid with named areas, restructuring `stage-grid`,
which the film detail page shares. That is a cross-cutting change to a layout primitive, and it is
what carried the `needs-adr` label.

**Making the rule unconditional removes all of it.** The overview is simply the rail column's first
child. `stage-grid` is untouched, the film page inherits nothing new from this change, and
`#field-overview` renders exactly once by construction. No ADR; `needs-adr` cleared.

## 6. States and edge cases

| Element | Case | Behavior |
|---|---|---|
| Overview | absent | nothing renders; Tags becomes the rail's first block |
| Overview | owner, field mapped `multi`, no value | section does not render — the gate is `(isReplaceField && isOwner) \|\| value`, exhaustive with its two branches, and `hasPageAnchor('overview')` mirrors it so the completeness fallback anchor stays in step (found in code review: the old header gate would have produced an orphan "Overview" heading) |
| Overview | visitor, 60+ character unbroken token, 390px rail | wraps — `ExpandableText` now carries `wrap-anywhere`. Before the fix it overflowed by 34px and `line-clamp`'s `overflow:hidden` cut it off silently (found on the first live check at 1024) |
| Metadata (visitor) | fold | **always open, no chevron** — the fold is owner noise control; a visitor has no controls, the values are the section. The design said "visitors see the values"; a collapsed fold would have shown them a count and a chevron |
| Metadata (visitor) | every resolved field is elsewhere or valueless | section does not render — `metadataFieldCount` is 0. The file-only `fields` fallback is guarded on `!resolved.length`; without that guard the first visitor render fell into it and re-rendered Overview/Actors/Studio from raw file tags as a second copy |
| Metadata (visitor) | valueless resolved field | dropped from `visibleResolved` (owner keeps it so the pin stays changeable). Note this is a *different* rule from the pre-reorder `visibleResolved`, which showed visitors only provider-won fields and hid file-derived ones; the contract in `routes/CLAUDE.md` is "visible whenever a value exists" |
| Audit wrapper | visitor | the whole `max-w-stage` wrapper is gated, not just its children — an empty div would still collect the article's `space-y-6` and leave 24px of dead space at the page bottom (found in code review) |
| Overview | present, one line | no expand chevron (fixed in #319, `ExpandableText` gates on `clamps`) |
| Overview | owner | renders `SourceBadge`, as today — the chip row the 320px rail floor was sized for |
| Overview | visitor | renders `ExpandableText`, as today |
| Metadata (visitor) | no resolved fields | section does not render at all — no empty heading |
| Metadata (visitor) | 1 field at ultrawide | one row, 3-across grid, two empty tracks; acceptable — do not special-case |
| Shelf | fewer cards than fill the stage | box pinned to stage by `min-width`, cards left-justified |
| Shelf | no items | `RelatedShelf` self-omits; the band renders nothing and contributes no gap |
| File / Enrichment | visitor | neither renders; the page ends after the shelf band |
| File | very long `file_path` | `col-span-full truncate` with `title` attribute, unchanged |

## 7. Accessibility

- **Focus order continues to match visual order everywhere.** Both moves in this change are DOM
  moves, not CSS repositioning, so the two orders stay locked together. The earlier named-areas
  approach would have decoupled them above the threshold; that risk is gone with it.
- **The shelf band's new position is a focus-order change on one column** — Tags, Films and People
  are now reached before the recommendation shelves. That is the intended improvement, not a
  regression.
- **`#field-overview` must appear exactly once** at every viewport and in both roles. Asserted:
  `field-overview-renders-once` in the geometry harness, 288 checks across 9 skin/width cells.
- No new interactive elements. The visitor Metadata list is non-interactive text plus
  `ProvenanceBadge`, so it adds no tab stops.

## 8. QA

Skin coverage per `.claude/rules/frontend-theming.md` — all three skins, no hardcoded styling.

### Setup

1.1 Seed the HOLODEX-342 stress fixture and open a video with 3+ resolved fields, 3 tags,
2 people, 1 studio, an overview long enough to clamp, and a sibling for the More-with shelf.
`[smoke]`

### Smoke

2.1 Owner view, desktop: File no longer appears in the rail; it appears once, at the page bottom,
immediately above the first `Enrichment data:` disclosure. `[smoke]`
2.2 Visitor view, desktop: Metadata renders with values and `ProvenanceBadge`, and with no
`SourceBadge`, no `Enrich`, no `Refresh all`, no `Write decisions to file`. `[smoke]`
2.3 The overview appears at the top of the rail, above Tags, in both roles. `[smoke]`
2.4 No horizontal page scroll at 320, 412, 768, 1024, 1920, 2560, 5120. `[smoke]`

### Agent

3.1 At 1920: shelf container width equals the stage width and its first card's left edge aligns
with the player's left edge (compare `getBoundingClientRect().left`). `[agent]`
3.2 At 5120 with few shelf cards: shelf box width equals 2600 and is centred in the window.
`[agent]`
3.3 At 5120 with many shelf cards: shelf box is wider than 2600, and its left and right overhang
past the stage edges are equal to within 1px. `[agent]`
3.4 `#field-overview` appears exactly once in the DOM at 412, 1024, 1920 and 5120, in both roles,
and its computed `left` places it in the rail column at every two-column width. `[agent]` — **done**,
and now `field-overview-renders-once` in the harness.
3.5 At 1024 (narrowest two-column) the overview does not overflow its 390px track — measured on the
`<p>` as visitor, since `line-clamp` clips and the rail itself never reports overflow. `[agent]` —
**done; found the clip; fixed; now `overview-fits-the-rail` under the `visitor-view` preparation
at the new `lg` cell, mutation-tested.**
3.6 Tab order from the player reaches Tags before the More-with shelves at 412px. `[agent]`
3.7 File's `field-grid` computes 8 columns at 5120 and 1 at 412. `[agent]`

### Human

4.1 Open a video page as owner on the ultrawide monitor. The synopsis should sit at the top of the
right-hand column, above the tag chips — not under the title. Nothing should look stranded in the
right column. `[human]`
4.2 On the same page, scroll to the bottom. The file details block and the collapsible
"Enrichment data" rows should read as one group, with nothing between them. `[human]`
4.3 Still on the ultrawide monitor, look at a "More with …" row that has only two or three
thumbnails. The row should start directly under the video, lined up with its left edge — not
drift to the middle of the screen. `[human]`
4.4 Find a video with many siblings so the row is long. It should now run wider than the rest of
the page, overhanging evenly on both sides. `[human]`
4.5 Sign out (or switch to visitor view) and reload. You should still see the field values —
year, runtime and so on — but no buttons for changing or fetching them. `[human]`
4.6 Shrink the window until the page becomes a single column. The synopsis should now read just
after the studio card, and the tags and people should come before the "More with …" rows. `[human]`
4.7 Repeat 4.1–4.6 in each of the three skins, then on the phone. `[human]`
