# Design handoff: Film provider enrichment (F59)

**Status:** Draft — pre-implementation gate
**Epic:** [HOLODEX-308](https://whoiskevinrich.atlassian.net/browse/HOLODEX-308)
**Owner:** Project owner
**Date:** 2026-09-04
**Spec:** [film-provider-enrichment-ux.md](../specs/film-provider-enrichment-ux.md) (F59)
**ADR:** [ADR-089](../architecture/ADR-089-film-enrichment-field-vocabulary.md) — D1 cast landing,
D2 merge rule, D3 year identity write, D4 banner-replaces-thumb, D5 SPA widening
**Supersedes:** [films-entity-handoff.md](films-entity-handoff.md) §2a (header) — the poster is no
longer the header's only image
**Theming contract:** [ADR-021](../architecture/ADR-021-theming.md) + [theming.md](theming.md) —
**tokens only, QA all three skins.**

![Film enrichment mockup](film-enrichment-mockup.svg)

## Overview

Three surfaces change on `/films/{id}`, and nothing else moves. The Details section gains the provider
chip every other entity page already has; the header gains a landscape banner behind the existing
poster row; and the Cast section gains a second, differently-meaning group below the scene union.

The load-bearing constraint throughout: **no component under `web/src/lib/components/enrichment/**` is
modified.** They are already entity-agnostic. A diff that touches them has gone wrong (ADR-089 D5).

## 1. Details section — provider chips

Structurally identical to `web/src/routes/studios/[id]/+page.svelte:419-436`. The film page already
has the Details `section` with the same `SourceBadge` / `baselineKey='record'` shape; this adds the
chip row to its heading line and the picker at page level.

| Element | Spec |
|---|---|
| Section shell | Unchanged — `space-y-3 rounded-theme border border-rule bg-surface p-4` |
| Heading row | Becomes `flex flex-wrap items-start justify-between gap-2`, matching studio |
| Heading | Unchanged — `text-xs uppercase tracking-wide text-muted`, "Details" |
| Owner control | `EnrichProviderChips` — one chip per film-capable provider (icon + name + Enrich), Clear/Refresh in the `⋯` overflow once linked |
| Visitor | Section-level note only: `Enriched from {provider}` in `text-xs text-muted`, accent on the provider name. **No per-row badge** when a single provider covers every field — same restraint as studio |
| Picker | `EnrichPicker`, page-level, `entityName={film.name}` |
| Error | `actionError` renders as `text-sm text-warn` above the field grid |

**Gate:** the whole chip row is hidden when `films_enabled` is off. The film enrich routes are not
registered in that state, so a rendered chip would 404 (F59 P0-3).

**Field rows.** `Description` and `Release date` render as they do today. `Year` joins them as a
read-only-looking row whose value changes only via the collision-checked apply (§4).

## 2. Header — banner behind the poster row

### 2a. Resolved: the banner sits behind an otherwise-unchanged header (spec Q1)

The alternative — promoting the banner to hero and demoting the poster to a Person-style overlapping
badge — was rejected. PR #292 deliberately made the portrait poster the film's identity image seven
days ago, and a film's poster carries more recognition weight than a person's headshot does. The
banner is atmosphere; the poster stays the subject.

So: the existing `flex flex-col gap-4 sm:flex-row` row (poster `w-40` + title column) is **unchanged**.
A banner band is inserted above it, and the row is pulled up to overlap the band's lower portion —
the same overhang relationship the Person page uses, with the poster rather than the headshot doing
the overlapping.

### 2b. Band geometry

| Property | Value | Note |
|---|---|---|
| Slot ratio | `aspect-[8/3]`, full content width | Matches Person's band; the provider asset is ~16:9 and is cropped, not letterboxed |
| Component | `EntityImageSlot` `variant="frame"` `role="banner"` | **No new component.** Second instance beside the existing poster slot |
| Overlap | Poster row pulled up so the poster overhangs the band's lower third | Name and poster read as one unit, not stranded below |
| Scrim | Bottom-anchored gradient over the band's lower ~45% | Reuses the person-bio scrim from PR #290 |
| Stacking | The overlap row and every child paint above the band | Same fix as PR #283 — the row must not sit behind |

### 2c. Empty states (spec Q2)

`EntityImageSlot` falls back to the entity monogram for every non-poster role. **A monogram stretched
across an 8:3 band is the wrong empty state** — this is the one behavioural change the component
needs, and it should be expressed as a prop, not a film special case.

| Film has | Owner sees | Visitor sees |
|---|---|---|
| Banner + poster | Both, scrim active | Both, scrim active |
| Poster only | No band; an `+ Add banner` control where the band would be | **No band at all** — header renders exactly as today |
| Banner only | Band + poster slot's existing owner upload affordance | Band + the poster role's dashed empty box |
| Neither | Today's header, plus `+ Add banner` | Today's header, unchanged |

The "poster only, visitor" row is the important one: a film with no banner must render byte-identically
to today. This mirrors F25.30's decision for Person — no placeholder band for visitors.

### 2d. Owner controls

Both roles keep `variant="frame"`'s corner overlay buttons (pencil = replace, `×` = remove). The
banner's sit top-right of the band; the poster's stay where PR #292 put them. Two `EntityImageSlot`
instances, two independent upload/remove paths, one `onchanged={reloadDetail}`.

## 3. Cast — the union, then the difference

The single most important rule in this handoff. See the mockup's panel 3.

### 3a. Structure

| Group | Source | Rendering |
|---|---|---|
| **Cast** | Scene union over `film_videos` — unchanged | Existing `PeopleGrid`, unchanged |
| **Billed on the release — in no scene you own** | `film_people_roles` **minus** the union | Second `PeopleGrid` instance, visually distinguished |

### 3b. What must not happen

- **Never render a name twice.** A performer in both sources appears once, in Cast. At realistic scale
  (a 10-person union against TMDB's 20-name `maxCastCredits` window) two full lists are roughly half
  duplicates.
- **Never merge the two into one list.** That destroys the distinction the second group exists to
  express — "in my footage" versus "on the release" — which is the whole value.
- **Never reorder or filter the union** to accommodate the second group. The union is primary.

### 3c. Difference styling

Dashed chip border in the accent role, accent text — reading as *provisional / not-yet-present* rather
than as an error or a warning. These are not problems; they are information about coverage. Do not use
`--text-warn` or `--text-danger`.

Difference is computed by **resolved person identity**, not display string, so an alias or a case
variant never manufactures a phantom entry (ADR-089 D2).

### 3d. Counts line (P1-1)

Right-aligned on the Cast heading row, `text-xs text-muted`: `Your scenes cover 10 of 14 billed cast`.
Hidden entirely when no provider credits exist — a film with no enrichment shows exactly today's Cast
section.

### 3e. Empty and degenerate states

| State | Rendering |
|---|---|
| No provider credits | Cast section exactly as today. No second group, no counts line |
| Credits exist, difference is empty | Counts line reads `Your scenes cover all 14 billed cast`; the second group is **omitted**, not shown empty |
| Union is empty, credits exist | Cast section shows the empty-union state; the second group renders in full — this is a film whose scenes are all unattached or uncredited |
| One difference name | Same group, singular counts copy |

## 4. Year — the collision error

The apply is atomic: a colliding year changes nothing at all (ADR-089 D3).

- Copy, on collision: `Can't set 1999 — "The Matrix" (1999) already uses that name and year.` The
  occupying film's name links to it.
- Placement: the Details section's `actionError` slot, `text-sm text-warn`.
- The picker stays open so the owner can choose a different candidate.
- **No auto-bump, no silent swap** — the same posture as scene-number collisions on attach.

## 5. Out of scope

- Film rename / title editing (spec Non-Goal 1) — no affordance appears anywhere in this handoff.
- Any change to how Studio is edited on a film; F57's cascade dialog is untouched.
- Multi-provider picker UI (HOLODEX-85/168) — films inherit it whenever it ships.
- Tags on a film — still a read-only union, unchanged.

## 6. Accessibility

- The banner is decorative; its `EntityImageSlot` renders `alt=""` and the film name stays the `h1`.
- The scrim is a presentational overlay, not a focusable element, and must not intercept pointer
  events on the controls beneath it.
- The difference group needs a real heading, not a styled `div` — screen-reader users must get the
  "billed but absent" framing, which is the entire meaning. Colour and dashes alone do not carry it.
- Owner overlay buttons on both image roles keep discernible names: `Replace banner`, `Remove banner`,
  and the existing poster equivalents.
- Chip contrast: the dashed accent chips must clear AA against `--surface` in all three skins — this
  is the pairing most likely to fail, since accent-on-surface is decorative elsewhere but load-bearing
  text here.

## 7. Theming

Tokens only. The band, scrim, chips and dashed borders all resolve from existing tokens — no new
token is introduced. The scrim is the one risk: it is a gradient to `--surface`, and a skin whose
surface is light inverts the legibility problem it solves. QA the banner + title contrast in all three
skins explicitly, not by inspection of one.

## 8. QA checklist (3-skin)

Numbered `section.item`, grouped by verifier ([[feedback-qa-checklist-numbering]]).

### §1 Setup
- **1.1** Preview stack running with `films_enabled: true` and a TMDB provider configured.
- **1.2** A film with at least 4 attached scenes and a known TMDB match.
- **1.3** A second film sharing the first's name but a different year (for 4.x collision tests).

### §2 Smoke — automated (green in CI)
- **2.1** `[smoke]` With `films_enabled: false`, `/films/{id}` renders no enrichment chip and issues no
  `/films/*/enrich/*` request.
- **2.2** `[smoke]` Applying provider cast to a film leaves every attached video's `video_people` and
  `field_source_decisions` unchanged (ADR-089 D1 invariant).
- **2.3** `[smoke]` A union of 10 and a billed list of 14 sharing 10 names renders 10 + 4 chips, never 24.
- **2.4** `[smoke]` A colliding-year apply returns the occupant's name and leaves both films unchanged.
- **2.5** `[smoke]` `web/src/lib/components/enrichment/**` is unchanged by the epic's diff.

### §3 Agent live QA (preview tools against the §1 stack)
- **3.1** `[agent]` Enrich a film; confirm description, release date, year, poster and banner all
  populate from one apply. **All 3 skins.**
- **3.2** `[agent]` Measure computed contrast of the film title over the banner scrim. **All 3 skins.**
- **3.3** `[agent]` Measure computed contrast of a dashed difference chip's label against `--surface`.
  **All 3 skins.**
- **3.4** `[agent]` Confirm via `getBoundingClientRect` that the poster overlaps the band and that no
  control sits beneath the scrim's hit area.
- **3.5** `[agent]` Clear the provider; confirm the difference group and counts line disappear and the
  Cast union is untouched.
- **3.6** `[agent]` A film with a poster but no banner renders a header geometrically identical to
  `main`'s.

### §4 Human
- **4.1** `[human]` Open a film you know well and click **Enrich** on the provider chip in the Details
  box. Pick the right match. Does the page now show a wide image across the top, with the poster still
  in front of it and the title readable over the image? Nothing should look washed out or hard to read.
- **4.2** `[human]` Look at the Cast area. The top group is who's in the video files you actually have.
  The dashed group below is people credited on the real film who aren't in any of your files. Is that
  distinction obvious without reading the labels twice? Is anyone listed in *both* groups? (Nobody
  should be.)
- **4.3** `[human]` Switch skins using the theme control and repeat 4.1 and 4.2 in each. The wide image
  and the title should stay readable in all three.
- **4.4** `[human]` Try enriching the film from §1.3 — the one that shares a name with another film.
  You should get a clear message naming the film it clashed with, and nothing on the page should
  change.
- **4.5** `[human]` On a film with no wide image set, does the page look the same as it did before this
  change? (It should — no empty grey band.)
- **4.6** `[human]` As a visitor (owner view off), confirm you see no Enrich button, no pencils, and no
  "Add banner" control anywhere.
