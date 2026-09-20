# Spec: Person hover card — floating preview with headshot, age, counts and a link row on text-only person links (F68)

**Status**: Draft
**Phase**: Phase 3 (presentation) — rides the resolver (F27), person images (F26), aliases
(F23) and unified external ids (F60); adds one read endpoint and one component, no new subsystem
**Owner**: Project owner
**Date**: 2026-09-19
**Jira**: [HOLODEX-431](https://whoiskevinrich.atlassian.net/browse/HOLODEX-431)
**Feature block**: **F68** — hovering (or focusing) a **text-only** person link opens a small
floating **hover card**: headshot, name with nationality flags, current age, title and film
counts, "also credited as" aliases, and a **link row** (Profile · Titles · Films · external ids ·
owner-only Enrich / Edit). Person gets its first shared link component, `PersonLinkChip`, which is
the only place the card is mounted.

## Problem Statement

On a film page ("Billed on the release"), the `/search` results and the "More with …" shelf
title on a media page, a person is a bare name. The viewer has no recognition cue — no face, no
"how much of them do I have" — and must navigate to the profile and back to learn who a name is.
Person is also the one entity with **no shared link component**: `StudioLinkCard` and
`TagLinkChip` exist, but the six person-link sites each write their own
`<a href="/people/{id}">`.

Not solving it costs a round trip per unfamiliar name. Solving it is mostly flair — but flair
that lands on the exact surfaces where the profile's facts are furthest away.

## Goals

1. **Recognition without navigation** — from any v1 surface a viewer learns face, age, counts
   and aliases for a person in one hover, and can hop to the profile, its Titles/Films sections,
   or an external provider page from the card.
2. **One person-link component** — every text-only person link renders through
   `PersonLinkChip`; a later surface gets the card by adopting the chip, not by re-implementing
   it.
3. **Cheap** — the card's data is one small request per person per session; the payload for a
   video/film page does not grow.
4. **Accessible and honest on touch** — keyboard users get the same card via focus; pointers
   that cannot hover get the plain link with nothing broken.

## Non-Goals

- **Surfaces that already show the face** — `PeopleGrid` tiles, `PersonPosterCard`, the people
  index list rows. A card re-showing the headshot beside the headshot is noise; an *in-tile*
  stats reveal is a separate follow-up (HOLODEX follow-up, see Deferred).
- **The header search dropdown** — it is itself a popover; the card lands on the `/search` page
  only (RD6). Popover-on-popover is deferred until it earns a design.
- **Curation chips** (`CurationChip` person values) — the owner's chip row already opens its own
  `PopoverMenu`; a hover card beside an editing menu invites collisions (RD7).
- **Studio / Film / Tag cards** — Studio and Film may follow with their own facts; Tag has no
  image and likely never gets one.
- **Age at release** — "27 when this title came out" needs the video's year threaded into the
  card; deferred, the endpoint leaves room for it (P2).
- **Touch long-press** — no hover, no card; the link works.
- **Prefetching profiles / any change to `video.people[]`** — it stays `id + name + role`.

## Resolved Decisions

Locked during the 2026-09-16 brainstorm and the 2026-09-19 spec session.

| # | Decision | Rationale |
|---|---|---|
| RD1 | **Interaction model B — card with a link row**, not a read-only preview (A) or an enriched chip (C). | Kevin's call; the link row is the point. Costs hover-intent + leave-grace + Tab order, all specified below. |
| RD2 | **Surfaces (v1):** film "Billed on the release" chips, `/search` page person rows, "More with …" shelf title on media pages. | Text-only links only — where the face is furthest away. |
| RD3 | **Content:** headshot · name + `NationalityFlags` · current age · "N titles" · "N films" · "also credited as …" (aliases, up to 3) · link row. | The profile's recognition cues, nothing editable. |
| RD4 | **Link row:** Profile · Titles (`#videos`) · Films (`#films`) · one badge per external id (same `ExternalLink[]` as `EntityVideoMeta`) · **owner-only** Enrich · Edit. | All four link groups chosen by Kevin. Owner links follow the visitor/owner rule: content stays visible to visitors, only the two owner actions are gated. |
| RD5 | **Age = the resolver's derived `age`** (→ `age_at_death` when a death date exists), same rule as the profile. | One truth; no client-side date math. |
| RD6 | `/search` page only, **not** the header dropdown. | Avoids a floating card inside a floating panel and the dropdown's own outside-click dismiss. |
| RD7 | Curation chips **deferred**. | Editing surface; collides with `PopoverMenu`. |
| RD8 | **Data = new `GET /people/{id}/card`**, fetched on hover-intent, cached for the session, aborted on leave. | `GET /people/{id}` ships up to 500 videos + a full resolve — not a hover fetch. The card endpoint runs a 3-field resolve subset (RD5) plus cheap counts. |
| RD9 | **Hidden under `@media (pointer: coarse)`**; hover-intent delay 250 ms; leave grace 150 ms. | First hover-only affordance in the app — state the touch posture once, in CSS. |
| RD10 | **No positioning library in v1.** Card is absolutely positioned beside the trigger and flips horizontally/vertically from one `getBoundingClientRect` measure. `@floating-ui/dom` only if the prototype proves ugly → then an ADR. | The app has no floating-ui / anchor positioning today; keep the dependency decision behind evidence. |
| RD11 | `PersonLinkChip` lives in `web/src/lib/components/person/`; the card is `PersonHoverCard.svelte` beside it, mounted **only** by the chip. | Single entity → `person/` per the components `CLAUDE.md`. |

## User Stories

**Viewer (visitor or owner)**

- As a viewer on a film page, I want to hover a billed name and see the face, age and how many
  titles I have with them, so I can decide whether to click through.
- As a viewer, I want to jump from the card straight to the person's Titles or Films section, or
  to their TMDB page, so the card saves a navigation rather than adding one.
- As a keyboard user, I want the same card when I Tab onto the link, and to Tab into its links,
  so the affordance is not pointer-only.
- As a viewer with a mouse who moves diagonally from the chip into the card, I want the card to
  stay open, so the links are actually reachable.
- As a viewer on a tablet, I want the link to just work — no half-open card, no double-tap.

**Owner**

- As the owner, I want Enrich and Edit in the card's link row, so a missing headshot or wrong
  age seen in the card is one click from fixable.

**Edge cases**

- A person with no headshot shows the same placeholder frame `PersonImageFrame` shows on the
  profile; the card still opens.
- A person with no birthdate shows no age segment (no "—", no "unknown").
- A person with zero films omits the "N films" segment and the Films link.
- A person whose card request fails shows nothing — the chip stays a plain link; no toast.
- Twelve chips in a row: only one card is open at a time; moving along the row swaps cards
  after the intent delay, not on every pixel.

## Requirements

### Must-Have (P0)

**R1 — `PersonLinkChip` component.** Props `{ id, name, display_name?, class? }`; renders the
existing pill (`rounded-full border border-rule bg-surface-2 px-2.5 py-0.5 text-sm`) or the
consumer's supplied classes around `<a href="/people/{id}">`; owns the hover/focus state that
mounts `PersonHoverCard`.
- [ ] The three RD2 surfaces render through the chip; no other `<a href="/people/…">` is touched.
- [ ] Given `pointer: coarse`, the chip renders and navigates; the card never mounts.

**R2 — Open / close behavior.**
- [ ] Given a hovering pointer, when it rests on the chip ≥ 250 ms, then the card opens; leaving
  before 250 ms opens nothing and cancels any in-flight fetch.
- [ ] Given the card is open, when the pointer leaves chip *and* card for ≥ 150 ms, then it
  closes; re-entering either within the grace keeps it open.
- [ ] Given the chip receives keyboard focus, then the card opens immediately (no intent
  delay); Tab moves into the card's link row in DOM order; Shift+Tab returns to the chip; Tab
  past the last link closes the card and continues.
- [ ] Escape and outside-click close the card (`use:dismissable`) and return focus to the chip
  when the card had it.
- [ ] Only one card is open at a time app-wide (extend `PopoverMenu`'s single-open state or a
  module-level equivalent).
- [ ] Clicking the chip itself always navigates — the card never intercepts the primary link.

**R3 — Positioning.** Card is rendered beside the chip (preferred: below-start), flips above
when there is < card height below the viewport edge, and flips to end-aligned when it would
overflow the right edge. One measure on open, re-measure on window resize/scroll while open.
- [ ] A chip at the bottom-right corner of the viewport shows a fully visible card.
- [ ] The card never causes horizontal page overflow (HOLODEX-356 guard applies).

**R4 — Card content (RD3/RD4).** Layout: headshot frame left (`PersonImageFrame`
role=`headshot`, 1:1, ~56 px), name line (display name when set, else name) + `NationalityFlags`,
meta line "`{age}` · `N titles` · `N films`" with absent segments dropped, optional "also
credited as a, b, c" (max 3, `+N more` not shown), then the link row.
- [ ] Link row order: Profile · Titles · Films · external-id badges (`ProviderLinkBadge`, sorted
  by `sortExternalLinks`) · Enrich · Edit. Films omitted when `film_count = 0`. Enrich/Edit
  rendered only when `isOwner`.
- [ ] Titles → `/people/{id}#videos`; Films → `/people/{id}#films` — **these two anchors do not
  exist yet**; the build adds `id="videos"` to the profile's video-grid section and `id="films"`
  to its `FilmsRow` heading. Enrich → `/people/{id}#enrich-providers`; Edit →
  `/people/{id}#field-photo-upload` (both exist).
- [ ] `role="dialog"` is **not** used; the card is `role="group"` with `aria-label="{name}"`
  and the chip carries `aria-describedby` only while open.

**R5 — `GET /people/{id}/card`.** Public read (no owner branch — every field is visitor-visible
on the profile today). Shape:

```json
{
  "id": 12, "ref": "person:12",
  "name": "Maya Rodriguez", "display_name": null,
  "headshot_version": 3,
  "video_count": 27, "film_count": 3,
  "age": 41, "age_at_death": null,
  "nationality": ["Brazilian"],
  "aliases": ["M. Rodrigues"],
  "external_links": [{ "provider": "tmdb", "label": "TMDB", "url": "https://…" }]
}
```
- [ ] Implementation budget: `repo.GetPerson` (name/display_name/video_count/aliases) +
  `personImageVersions([id])` + one `COUNT` over the `ListFilmsForEntity` EXISTS clause +
  `externalLinksForEntity` + a **3-field** `ResolveFields` subset (`birthdate`, `deathdate`,
  `nationality`) followed by `resolver.Derive` — promotions/claims/auto-register skipped.
- [ ] 404 for an unknown or soft-deleted person; `Cache-Control: private, max-age=300`.
- [ ] `age` and `age_at_death` are mutually exclusive, exactly as `deriveAge` emits them.

**R6 — Client fetch discipline.**
- [ ] One request per person per session (module-level `Map<id, Promise<Card>>`); a failed
  request is not cached, so the next hover retries.
- [ ] The request starts at intent (250 ms), not at pointer-enter; leaving before it resolves
  aborts via `AbortController`.
- [ ] While loading, the card shows the name line only (no spinner) and fills in when data lands.

**R7 — Theming.** Tokens only: `bg-surface border-rule rounded-theme shadow-lg text-ink
text-muted` — the same floating-panel recipe as the header search dropdown; there is no per-skin
shadow hook in the app and this does not introduce one. QA all three skins.

**R8 — Tests.** Component tests for R2 (timers, focus order, single-open), a positioning test
for R3 at the corner, an API test for R5 (shape, 404, age exclusivity), and a geometry-harness
rung asserting no horizontal overflow with the card open at max density.

### Nice-to-Have (P1)

- **Keyboard shortcut hint** in the card footer (ties into the F62 hotkey sheet) — only if the
  card grows a second row anyway.
- **Alias overflow** "+N more" as a link to the profile's Aliases section.

### Future Considerations (P2)

- **Age at release** — `GET /people/{id}/card?year=2019` adds `age_at: 27`; the media page
  passes its resolved year. The endpoint shape reserves the field name.
- **In-tile stats reveal** for `PeopleGrid` — same card data, rendered inside the tile's
  existing `group-hover:` reveal.
- **Studio / Film cards** — a `StudioHoverCard` over `StudioLinkCard` reusing the chip's
  open/close logic (extract to a `use:hoverCard` action when the second consumer arrives, not
  before).
- **Header-dropdown search rows and curation chips** — once a popover-in-popover posture exists.
- **Touch long-press.**

## Behavior detail

```
pointer enters chip ──250 ms──▶ fetch card (cached?) ──▶ open beside chip
        │ leave < 250 ms                       │ leave chip+card ≥ 150 ms
        ▼                                      ▼
   cancel timer + abort                     close
focus chip ──▶ open immediately ──Tab──▶ links… ──Tab past last──▶ close, continue
Escape / outside click ──▶ close (+ return focus if the card had it)
another chip opens ──▶ this one closes
```

## Data model

No migration. `film_count` is computed; nothing new is stored.

## API

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/people/{id}/card` | public | R5 shape; 404 unknown/soft-deleted; `Cache-Control: private, max-age=300` |

Registered beside `/people/{id}/images` in `handlers.go`. Not exposed on MCP in v1.

## UI

- `web/src/lib/components/person/PersonLinkChip.svelte` — the link + state.
- `web/src/lib/components/person/PersonHoverCard.svelte` — presentation, receives `card` and
  `isOwner`.
- `web/src/lib/api.ts` — `getPersonCard(id, signal)`.
- Consumers: `routes/films/[id]/+page.svelte` (billed chips), `entity/SearchResultsPanel.svelte`
  (person rows, `/search` page context only — prop `hoverCards: boolean`, default `false`,
  the dropdown never sets it), `routes/media/[id]/+page.svelte` (`RelatedShelf` title for the
  person shelf).
- Design handoff + committed SVG (all three skins, corner-flip state, loading state, no-headshot
  state) precede code.

## Success Metrics

This is a single-owner instance; "adoption" is Kevin using it. Concrete checks instead:

- **Leading:** card-open → link-click ratio on a real film page during human QA ≥ 1 in 3
  (the link row earns its cost); zero horizontal-overflow regressions in the geometry harness.
- **Lagging:** in a month of use, no card-related bug filed against the three v1 surfaces, and
  at least one deferred surface asked for (evidence the pattern wants to spread) — or none
  (evidence it doesn't, and P2 stays P2).

## Open Questions

- **OQ1 (engineering, non-blocking):** does a single `getBoundingClientRect` flip hold up inside
  the `RelatedShelf` header (a flex row that may itself be clipped)? Prototype there first; if
  it needs `position: fixed` + scroll listeners, that is the trigger for the floating-ui ADR
  (RD10).
- ~~**OQ2 (design):** dashed vs solid pill for the film-billed chips~~ — **resolved in the design
  handoff:** the chip is a transparent wrapper that keeps the consumer's classes. The dashed
  accent border encodes "billed but in no owned scene" (`films/[id]/+page.svelte:752-783`) and
  must survive; only the card is uniform. Note that surface is **owner-only**.
- **OQ3 (data, non-blocking):** should `film_count` count films via `film_videos ⋈ video_people`
  (what the profile's Films row shows) or `film_people_roles` (billing)? The spec says the
  former so the card and the profile agree; confirm nothing on the profile uses the latter for
  its count.

## Timeline / routing

| Gate | Artifact | Status |
|---|---|---|
| Spec | this document | ✔ |
| Design | [`docs/design/person-hover-card-handoff.md`](../design/person-hover-card-handoff.md) + [SVG](../design/person-hover-card-mockup.svg) | ✔ |
| ADR | only if RD10 falls (floating-ui) | n/a unless triggered |
| Testing strategy | `docs/testing-strategy.md` section | pending |
| Security review | public read of visitor-visible fields — not required unless the endpoint grows an owner branch | n/a |

Single story; no epic. Draft PR opens with this spec (ADR-069).
