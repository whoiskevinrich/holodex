# Spec: Poster View for the People list page (F55)

**Status**: Draft
**Phase**: F55 (Jira [HOLODEX-255](https://whoiskevinrich.atlassian.net/browse/HOLODEX-255), epic [HOLODEX-13](https://whoiskevinrich.atlassian.net/browse/HOLODEX-13) — People (pages, list, images))
**Owner**: Project owner
**Date**: 2026-08-05

**Depends on** (all shipped):
- People images / the `poster` core image role and its themed placeholder
  ([F25](people-images.md), `PersonImageFrame`/`PersonPoster.svelte`, ADR-038)
- The site-wide grid density preference ([`lib/density.svelte.ts`](../../web/src/lib/density.svelte.ts),
  introduced alongside [sort-persistence.md](sort-persistence.md)'s per-page preference pattern)
- `SortToggle` / the People list page's existing header controls (`web/src/routes/people/+page.svelte`)

**New ADRs required**: none. This is a new frontend display mode over an existing image
primitive (`.portrait-frame` / `PersonImageFrame`) plus one small, additive field on an
existing read (`ListPeople`) — no new table, no cross-cutting decision, no change to how
person images enter the system.

**Design provenance**: worked out interactively this session via a live HTML mockup
(comparing three card-chrome treatments) and a `/design-critique` pass on the border/padding
question — not committed to the repo (ephemeral artifact). This spec is the durable record of
what that process converged on; see "Resolved Decisions" below for the trail.

---

## Problem Statement

The People index (`/people`) has exactly one display mode: a dense list of small 1:1 avatar
rows (name + video count). That's efficient for scanning names, but Holodex already has a
richer visual browsing pattern for this exact use case — the Videos grid's poster-style
cards — and People doesn't get it. For a library with a couple hundred people, recognizing a
face is often faster than reading a name, and the existing `PersonPoster`/`portrait-frame`
2:3 well (already used on the person-detail credits surface) is sitting there unused as a
second way to browse the same list.

## Goals

1. **Add a photo-forward browsing mode to `/people`** — a Poster View toggle next to List,
   reusing the existing 2:3 image well rather than inventing a new visual language.
2. **Fix a real accessibility gap while touching this surface.** Poster-style cards
   (`PersonPoster`, and by extension this new grid) have no keyboard-focus indicator today —
   add one as part of this card's hover/focus treatment, not as an unrelated follow-up.
3. **One density control that feels like the one Videos already has**, not a second
   unrelated slider to learn — same shared preference, same interaction, calibrated for a
   narrower 2:3 card.
4. **Chrome-appropriate cards.** A border on every poster card was the default until a
   design-critique pass this session — the shipped version removes it where it isn't earning
   its keep (a real headshot) and keeps it where it is (a placeholder, for contrast reasons —
   see Resolved Decisions).
5. **List view keeps working exactly as today**, plus one small incidental layout fix bundled
   in because it touches the same page/session (see RD7).

## Non-Goals

- **Owner "Merge people…" multi-select mode in Poster view.** List view's checkbox-over-row
  affordance is unchanged and remains the only way to merge duplicates; Poster view has no
  select-mode in v1. *(Why: decided explicitly this session — see RD2. A checkbox-over-photo
  interaction wasn't designed, and merge is an occasional admin action already served by List
  view; forcing every future poster-card design decision to also accommodate a selection state
  would slow this down for a rare workflow.)*
- **Per-card loading/shimmer state for an in-progress headshot fetch/enrichment.** List view
  and `VideoCard` have this; Poster view doesn't get a designed treatment this round.
  *(Why: not designed this session; the existing "always-a-placeholder-or-real-image, never a
  broken glyph" server guarantee means this is a polish gap, not a correctness one — safe to
  fast-follow.)*
- **Tags page.** This spec is `/people` only. Tags has an analogous list-only surface but
  isn't in scope here. *(Why: keep this change provably scoped; if Poster View proves useful,
  extending it to Tags is a small, separate follow-up.)*
- **Person-detail page's own `PersonPoster` usage (credits surface).** That's a single hero
  card in a different, sparser context — its border stays as-is. *(Why: the "remove border
  chrome" reasoning in RD3 is specific to a *dense grid*, where the border on every card adds
  up; it doesn't apply to one card among a handful of credits.)*
- **Backend contract changes beyond the one additive field (RD8/P0-6).** No new endpoint, no
  pagination change, no new owner-gated mutation. *(Why: `ListPeople` already returns the full
  set in one call; Poster View is a rendering mode over data that (almost) already exists.)*

## Resolved Decisions

*(Locked this session via an interactive mockup, a `/design-critique` pass, and two
`AskUserQuestion` cards — recorded here for traceability.)*

- **RD1 — View toggle persists per page.** `localStorage` key `holodex:view:people`, values
  `'list' | 'poster'`, default `'list'`. Mirrors the validated-read/fallback-on-corrupt pattern
  from [sort-persistence.md](sort-persistence.md)'s SP1 (`holodex:sort:people`, a sibling key on
  the same page) — an unknown/malformed/missing value falls back to `'list'`, never throws.
- **RD2 — Owner multi-select ("Merge people…") is List-view-only for v1.** Resolved via
  question card; see Non-Goals. The "Merge people…" button's exact behavior while Poster view
  is active is Open Question Q1 below (hide it vs. auto-switch to List) — the *scope* decision
  is final, the *affordance detail* isn't.
- **RD3 — Card border is conditional, not removed outright.** No border on a card with a real
  headshot (the 14px grid gap alone separates tiles); a card still showing the themed
  placeholder keeps `border: 1px solid var(--rule)`. Verified by comparing `--bg`/`--surface-2`
  across skins: Brutalist (`#0a0a0a` vs `#111111`) and Broadcast (`#060814` vs `#0a0e1f`) sit
  only ~4–11 units apart per channel — close enough that a borderless placeholder would read as
  a blank hole in the page rather than an empty card. Cinémathèque has more natural separation
  but keeps the same rule for one consistent behavior across skins.
- **RD4 — Hover is a lift, not a border-color swap; focus gets its own ring.** Hover:
  `transform: scale(1.045)` + a soft `box-shadow`, replacing the old
  border-color-to-`--accent` swap (a 1px color change is easy to miss once cards get small at
  higher densities). Focus: a dedicated `:focus-visible` outline (2px `--accent`, 2px offset)
  on the frame — this is a **new** affordance, not a restyle; poster-style cards
  (`PersonPoster` included) have no keyboard-focus indicator today.
- **RD5 — Cinémathèque's decorative top bar is removed from poster cards specifically.** The
  existing 3px black/`opacity:.55` bar (`.video-frame`'s letterbox echo, also applied to
  `.portrait-frame`) was harmless while the card also had a border; once RD3 removes that
  border from photographed cards, the bar becomes the only chrome left and reads as a dead
  sliver cut into the top of the photo rather than a deliberate accent. Scoped removal —
  `.video-frame` itself (the actual Videos grid) is untouched; this only affects the new
  poster-card frame instance.
- **RD6 — Broadcast's scanline wash and Brutalist's catalog-number counter carry over
  unchanged.** Neither depends on the border; no new per-skin branch needed for them.
- **RD7 — List view: avatar row padding becomes `0 16px 0 0`** (was a uniform `10px 16px`), so
  the avatar sits flush against the row's top, left, *and* bottom edges; only the text column
  keeps 16px of right padding. Avatar size and the 3-column responsive grid/gap are unchanged.
  Bundled into this spec because it's a one-line CSS change on the same page/session, not
  because it's related in substance to Poster View.
- **RD8 — Density is one shared value, not two.** No second persisted preference. The existing
  `mediaDensity` value (`holodex:media-density`, `lib/density.svelte.ts`) drives both grids;
  People renders `mediaDensity.value × 2` columns, viewport-capped at double the video grid's
  existing per-tier caps (1536px→12, 1280px→8, 1024px→6, 480px→4, vs. video's 6/4/3/2).
  Rationale: a 2:3 poster reads fine at roughly half the width of a 16:9 video thumbnail, so
  doubling keeps both grids feeling comparably dense at any slider position, even though People
  always shows ~2× the columns Videos does at the same setting.

## User Stories

- As the owner, I want to browse People as a wall of photos instead of a list of names, so I
  can recognize a face instead of reading through names.
- As the owner, I want the People poster grid's density control to feel like the one on
  Videos, so I only have to learn one slider for both.
- As the owner, I want a person with no headshot yet to still look like a deliberate empty
  card (not a hole in the page) in every skin, so a partially-populated library doesn't look
  broken.
- As a keyboard user, I want to see which poster card is focused as I tab through the grid, so
  I can operate the page without a mouse — something today's poster-style cards don't offer.
- As the owner, I want my List/Poster choice remembered the next time I open `/people`, so I
  don't re-pick it every visit, consistent with how sort already behaves on this page.

## Requirements

### Must-Have (P0)

- **P0-1 — View toggle.** A List/Poster segmented control in the page header, visually
  identical to the existing `SortToggle` pattern (bordered container, `--accent` active fill,
  `--muted`/`--ink` inactive/hover), placed immediately after `SortToggle`
  (and after `SortReroll` when Random is active). Header order:
  `[Merge people… (owner only)] [SortReroll if random] [SortToggle] [ViewToggle] [Density (poster view only)]`.
- **P0-2 — Persisted view preference (RD1).** New module `holodex:view:people` read/write,
  mirroring `sortPreference.svelte`'s validated-read/fallback-on-corrupt shape. SSR-safe
  (`typeof localStorage !== 'undefined'` guard); write on toggle.
- **P0-3 — `PersonPosterCard` component.** New component: `PersonImageFrame` (`role="poster"`)
  + a name/video-count text block below, mirroring `VideoCard`'s title-below-thumbnail layout.
  Card chrome per RD3/RD4/RD5 (conditional border, lift+shadow hover, `:focus-visible` ring, no
  Cinémathèque top bar). Filed in `web/src/lib/components/person/` alongside `PersonPoster`
  (same domain folder, per that folder's `CLAUDE.md` classification rule).
- **P0-4 — `PersonPosterGrid` component.** New component mirroring `VideoGrid`'s density→column
  computation (RD8's doubled formula/tier caps) but for `PersonPosterCard`. Renders the poster
  grid's load-in animation consistent with `VideoGrid`'s existing `reel-rise` stagger.
- **P0-5 — Density slider (poster view only).** Same icon-pair/range-input UI as the existing
  media-list slider (small 4-square icon → range input → large single-square icon, inverted
  drag direction), bound to the same `mediaDensity` value (RD8). Visible only while Poster view
  is active; List view is unaffected and shows no density control (unchanged from today).
- **P0-6 — Backend: `poster_version` on `ListPeople`.** `headshot_version` (today's field) only
  reflects the `headshot` role, not `poster` — a person can have one without the other (e.g. an
  owner uploads only a headshot). The conditional-border logic (RD3) needs the *actual* signal
  for the role this grid renders. Add a second correlated subquery to `Repo.ListPeople`
  (`internal/repo/repo.go`), mirroring the existing `headshot_id` one exactly but for
  `role = 'poster'`, and a `PosterVersion int64 \`json:"poster_version,omitempty"\`` field on
  `model.Person` (parallel to `HeadshotVersion`). Frontend: add `poster_version?: number` to
  the `Person` interface in `web/src/lib/types.ts`. `PersonPosterCard` shows the border iff
  `poster_version` is `0`/absent.
- **P0-7 — Theming.** Tokens only, all new markup — `border-rule`, `bg-accent`/`text-accent`,
  `rounded-theme`, no hardcoded palette/radius. QA'd in **Cinémathèque, Broadcast, and
  Brutalist** (per the project's frontend-theming rule) before this ships — see Gate status.

### Should-Have (P1)

- **P1-1 — Extend Poster View to the Tags page**, if it proves useful on People. Explicitly
  deferred (Non-Goals) — not designed this round.
- **P1-2 — Per-card loading/shimmer state** for a headshot mid-fetch (mirrors `VideoCard`'s
  `thumb-shimmer`). Deferred (Non-Goals) — today's guarantee (placeholder-or-real, never
  broken) makes this cosmetic, not correctness-affecting.

### Future Considerations (P2)

- **P2-1 — Poster-view select-mode** (checkbox-over-card) if owner merge workflows outgrow
  List-view-only. Would need its own small interaction-design pass (checkbox placement over a
  photo, selected-state treatment) before implementation — deliberately not designed now (RD2).

## Behavior detail

### Conditional border — exact rule
```
PersonPosterCard border = none                         when poster_version > 0
                         = 1px solid var(--rule)        when poster_version is 0/absent
hover  → transform: scale(1.045) + box-shadow (both states — border presence is orthogonal to hover)
focus-visible → 2px solid var(--accent) outline, 2px offset (both states)
```
`poster_version` is the source of truth (P0-6) — **not** `headshot_version`, which reflects a
different, independently-fillable role.

### Density formula
```
cols = min(mediaDensity.value × 2, tierCapFor(viewportWidth))
tierCapFor: 1536px→12, 1280px→8, 1024px→6, 480px→4   (double the video grid's 6/4/3/2)
```
Same `invertDensity()` drag mapping as the existing slider (drag right = bigger cards = fewer
columns) — no new interaction to learn, just a different `cols` formula behind it.

## API

```
GET /api/v1/people    (existing endpoint, unchanged path/params)
```
Response gains one additive field per person:
```json
{ "id": 1, "name": "…", "video_count": 12, "headshot_version": 4, "poster_version": 0 }
```
`poster_version` is `omitempty` (absent = `0` = no poster image), matching `headshot_version`'s
existing convention. No version bump, no breaking change — purely additive.

## UI (grounded in real components)

- **`web/src/routes/people/+page.svelte`** — add the view-toggle state (`activeView`, backed by
  P0-2's persisted preference), the `ViewToggle` control in the header actions row, and a
  conditional render: `{#if activeView === 'poster'}<PersonPosterGrid people={displayed} />{:else}` …existing list markup… `{/if}`. The A–Z jump-nav (`sort === 'name' && !q.trim()`) stays
  List-view-only — its anchors (`#pl-{letter}`) are set on list rows and have no poster-grid
  equivalent designed this round.
- **New: `web/src/lib/components/person/PersonPosterCard.svelte`** — per P0-3.
- **New: `web/src/lib/components/person/PersonPosterGrid.svelte`** — per P0-4, update that
  folder's `CLAUDE.md` component table in the same change.
- **`web/src/lib/density.svelte.ts`** — no change to the module itself (RD8: one shared value);
  the doubled tier-cap table lives in `PersonPosterGrid`, not here.
- **`internal/repo/repo.go`** (`ListPeople`) / **`internal/model/model.go`** (`Person`) /
  **`web/src/lib/types.ts`** (`Person`) — per P0-6.

## Success Metrics

Single-owner personal server, so metrics are qualitative / self-observed:
- **Adoption:** the owner actually switches to and uses Poster view when browsing People
  (not just built-and-forgotten) — the persisted preference (RD1) sticking on return visits is
  a proxy signal.
- **Correctness:** a person with a poster image renders borderless; a person without one
  renders with the placeholder border, in all three skins — verified against `poster_version`,
  not `headshot_version` (the P0-6 fix actually matters, not just exists).
- **Accessibility:** Poster view is fully keyboard-navigable — Tab reaches every card with a
  visible focus ring, where none existed before this change.
- **No regression:** List view's existing behavior (rows, A–Z nav, sort, owner select-mode) is
  bit-for-bit unchanged except RD7's padding fix.

## Open Questions

- **Q1 (design, non-blocking):** exact behavior of the "Merge people…" button while Poster
  view is active — hide it entirely (forcing a manual switch to List), or leave it visible and
  have clicking it auto-switch to List + enter select-mode? RD2 fixed the *scope* (no
  select-mode in Poster view); this is only the entry-point affordance. Recommend: leave the
  button visible and auto-switch to List on click (fewer dead ends, no hidden functionality) —
  confirm before implementing P0-1.
- **Q2 (engineering, non-blocking):** should `PersonPosterGrid`'s doubled tier-cap table be
  derived programmatically from `VIDEO_TIERS` (`× 2` at read time) or hand-maintained as its own
  constant, per RD8's formula? Either is correct; the derived form is slightly more coupled but
  guarantees the two grids can't drift out of the stated 2:1 ratio if the video tiers ever
  change. Low-stakes, pick during implementation.

## Timeline / routing

No hard deadline. Per the project's change-routing rules:

1. **`/design-handoff`** — **needed, not yet done.** This introduces a real new UI surface
   (view toggle, two new components, a new density control, new hover/focus states, 3-skin
   QA) beyond what this spec's "UI" section covers structurally. The interactive mockup +
   critique done this session covers the *decisions*; a formal handoff spec covering exact
   spacing/breakpoints/component props should still land before implementation, per the
   project's UX-routing rule.
2. **`/testing-strategy`** — **needed, not yet done.** Add a People row to
   `docs/testing-strategy.md` covering: `poster_version` correctness (P0-6, backend unit test
   mirroring `TestListPeopleHeadshotVersion`), the conditional-border render logic, the density
   formula's tier caps, persisted-preference fallback-on-corrupt (mirroring SP1's test), and a
   keyboard-focus adversarial check (Q: does Tab actually reach every card and show the ring in
   all three skins?).
3. **`/security-review`** — **not needed, explicit call.** The only backend change (P0-6) adds
   one additive, unauthenticated-read field to an already-public list endpoint — no new
   owner-gated mutation, no new input surface, no change to what's already exposed at the
   person level (`poster_version` is no more sensitive than `headshot_version`, already public).
4. **`/architecture`** — **not needed, explicit call.** See header: no new ADR, incremental
   frontend feature over existing primitives + one additive field.

## Gate status

- [ ] `/design-handoff`
- [ ] `/testing-strategy`
- [x] `/security-review` — not required (see routing rationale above)
- [x] `/architecture` — not required (see routing rationale above)
