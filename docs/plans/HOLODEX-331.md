---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-331                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Improvement — detail pages now use the full width of large displays, placing tags, films, people and metadata in a rail beside the video instead of stacked below it, and the media grid picks its column count from the available width so it works on both ultrawide monitors and phones.
---

# HOLODEX-331 · Responsive page width: player column, metadata rail, intrinsic grid density

Started from "update all pages to take the full width of the page instead of being constrained."
The premise needed correcting first: `<main>` has no cap, so the browse grids and the person/studio
detail pages are already edge-to-edge. Only six files cap anything — media and film detail
(`max-w-4xl`) and the three owner pages, which additionally **double-wrap** (the owner layout caps
at 1024, then each child caps again inside it).

Removing those caps alone makes the page worse, not better. The player is `aspect-video w-full`, so
uncapping it produces a 1920x1080 player that overruns the fold before the title renders; and the
field list is hardcoded `sm:grid-cols-2`, so extra width widens two columns instead of adding any.

Measurement then found a third, larger problem at both ends of Kevin's actual device range
(5120x1440 ultrawide down to a Pixel 7 Pro). `TIERS` in `density.svelte.ts:57` tops out at
`{min: 1536, cap: 6}` and bottoms out at `{min: 480, cap: 2}`, so **nothing happens above 1536px**:
at 5120 the grid renders four 1252px cards and a single poster row is 1878px tall on a 1440px-tall
screen. At 412 it collapses to one column. Raising `DENSITY_MAX` fixes neither end.

The governing rule: **width is spent on more information, never on bigger information.** A page
uses extra width by adding columns; where there is no further column to add, content stops growing
and centres. That is why the 5120 case caps at a 2600px stage rather than going edge-to-edge — the
one deliberate departure from the original ask, taken because an uncapped rail puts a field's value
a screen-width from its label.

**Design package:** `docs/design/responsive-page-width-handoff.md` + two committed SVG mockups
(`responsive-page-width-mockup.svg`, four width options at 1920; `responsive-page-width-ladder.svg`,
the chosen option at 412 / 768 / 1024 / 1920 / 5120) + a numbered QA checklist.

**Overlaps:** the browse-grid half of this belongs to epic HOLODEX-24 (Browse & video list) while
the story is parented to HOLODEX-12 (Video detail page). The two halves must ship together — the
field-grid change is what makes the width change worth anything.

## Gates — definition of done

- [~] spec `write-spec` — **no longer required**. It existed solely to document the stored-preference
  remap the target-width model would have forced; that model was rejected (handoff §2e), so
  `holodex:media-density` keeps its existing column-count meaning and no migration happens
- [~] architecture `architecture` — not applicable as judged; layout convention plus a
  component-local storage change, no data-model or seam change. Revisit if the stage cap is meant
  to be a standing rule for all future pages rather than a per-page choice (handoff §9.2)
- [x] design `design-handoff` — `docs/design/responsive-page-width-handoff.md` +
  `responsive-page-width-mockup.svg` + `responsive-page-width-ladder.svg` +
  `responsive-page-width-qa-checklist.md`
- [ ] frontend
- [~] testing `testing-strategy` — partially done: `density.test.ts` covers the shipped ceiling
  (mutation-checked). Still needed for the ladder extension (QA §4.4–4.7) and the layout geometry
  assertions (QA §3)
- [~] security `security-review` — not applicable; presentation only, no auth, access, or
  infrastructure surface
- [~] three-skin QA — done for the density ceiling at 1536 and 1920. Still needed across
  412 / 768 / 1024 / 1280 / 2560 / 3840 / 5120, reloading (not resizing) at each width

## Up next — ordered (position = priority)

1. [x] [design] Measure the real geometry across the gamut, settle the direction with Kevin, write
   the handoff + two committed SVGs + QA checklist, file HOLODEX-331 under HOLODEX-12, rename the
   branch, fire In Progress — `docs/design/`
2. [x] [frontend] **Max density → 8 columns**, shipped ahead of the remodel against the current
   column-count model: `DENSITY_MAX` 6→8, top tier derived as `cap: DENSITY_MAX` (both halves
   needed — `VideoGrid` takes `min(density, cap)`, so the ceiling alone leaves the last stops
   dead). `capForWidth` exported so the coupling is testable. Fixed an eager-loading regression
   the change introduced in `PersonPosterGrid` (`eager={i < 12}` → `{i < cols}`). Verified at
   1920 (8 cols / 220px) and 1536 (8 cols / 172px), three skins, `wide` + `poster`, 1280 and 1024
   unregressed — `density.svelte.ts`, `PersonPosterGrid.svelte`, `density.test.ts`
3. [~] [spec] Density preference remap — **dropped**: only needed under the rejected target-width
   model. Stored values keep their column-count meaning, so there is nothing to migrate
4. [x] [frontend] **Stage token.** `--container-stage: 2600px` in a **plain** `@theme` block (not
   `@theme inline` — that resolves the value into the utility without emitting the custom property;
   verified `--container-stage` was absent from `:root` under `inline`). Registered in
   `docs/design/theming.md` and `.claude/rules/frontend-theming.md`. `owner/+layout.svelte` and
   `media/[id]` use `max-w-stage`; `owner/status` drops its genuinely redundant cap, while
   `owner/keys`/`owner/trash` **keep** `max-w-4xl` — they were never redundant
5. [x] [frontend] **Two-zone split** on media detail. Measured: 547/390 at 1024, 1069/764 at 1920,
   1503/1073 at 5120 with the article at exactly 2600px, left offset 1253px; stacked at 768. Zero
   `order-*` utilities, computed `order: 0` on both zones, DOM order left→rail. Deep links still
   resolve while stacked
6. [x] [frontend] **Field grid** → `grid-cols-[repeat(auto-fit,minmax(320px,1fr))]`, `sm:col-span-2`
   → `col-span-full`, on all four grids in the page. **Not optional and not deferrable:** review
   caught that shipping item 5 without it left two 174px field columns at 1024 (they had ~426px in
   the old `max-w-4xl`) — a straight regression between 1024 and ~1714px. Now 1 column at 1024
   (356px), 2 at 1920 (361px), 3 at 5120 (341px)
7. [x] [frontend] **Film detail rail.** `max-w-stage` + the shared `stage-grid`; subject zone =
   banner + header (coupled by `-mb-14`), rail = Cast / scene coverage / Details / full-film file,
   **Scenes full width beneath both**. The banner question is answered: it wants the 1.4fr column,
   because at full stage width an 8:3 band is 975px tall. Extracted `@utility stage-grid` now that
   a second call site exists, so the ratio and rail floor live in one place
8. [x] [frontend] **Density ladder extended.** Rungs `{2560:12}` and `{3840:16}` above
   `{1536:8}`; `DENSITY_MAX` now `Math.max(...TIERS.map(cap))` so it survives reordering. Slider
   range tracks `viewportTierCap` (no inert stops), `invertDensity(n, max)` inverts against the
   current cap, and the control hides where no choice exists. Markup extracted to
   `components/sort/DensitySlider.svelte`; `effectiveDensity()`/`posterColumns()` centralise the
   narrowing rule and the People 2:1 ratio. Card width 172–302px from 1536 to 5120; a 5120 poster
   row is 453px tall, was 1878px
9. [~] [testing] Ladder covered by `density.test.ts`, stage-aligned grid by `stageGrid.test.ts` (tier boundaries, card-width bands for both
   grids, inversion against a dynamic cap). Layout geometry assertions per QA §3 still needed
10. [ ] [—] live three-skin QA on the real 5120x1440 and the real Pixel 7 Pro (QA §5 is written for
    a human on hardware, not an emulator), push, sync Jira
11. [ ] [—] `/simplify` then `/code-review` on the implementation diff, mark the PR ready

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-06 · Premise corrected, direction chosen, design package committed
- skills: design-handoff, simplify
- **Corrected the ask before building.** "All pages" was inaccurate — `<main>` has no cap and most
  pages are already edge-to-edge. Surfaced the six capping files, including the owner double-wrap,
  so the scope landed on the detail pages rather than a sweeping change.
- **Measured rather than assumed.** Ran the real stack (`backend-amv` + `web`) and read grid
  geometry at fresh page loads across 412 / 1024 / 1920 / 5120. Found the `TIERS` dead-end above
  1536px: four 1252px cards at 5120, a 1878px-tall poster row on a 1440px screen. One early reading
  looked like a resize-listener bug; a fresh-load control proved it an emulation artifact (the
  listener works when a real resize event fires) — reported as such rather than as a phantom bug.
- **Four options mocked, C chosen** (player column + rail). Kevin asked to flesh out mobile and to
  span 5120x1440 → Pixel 7 Pro, then took all three recommendations: cap-and-centre at ultrawide,
  target-card-width density, and stacking (not collapsing or tabbing) the rail on mobile.
- **Kept the mockup honest against the spec.** The 5120 frame originally showed four metadata
  columns; the rail math gives three, so the SVG was corrected rather than letting the picture and
  the spec disagree.
- Both SVGs render-verified in a browser before commit (temporary static page, removed afterwards).
- handoff: `docs/design/responsive-page-width-handoff.md`, mockups
  `responsive-page-width-mockup.svg` + `responsive-page-width-ladder.svg`, QA
  `responsive-page-width-qa-checklist.md`.
- **Next session:** start at item 3 — the density remap spec — because it is the only piece with a
  persistence consequence, and item 8 cannot be written safely until the remap is decided. No code
  has been written yet; the branch carries documentation only.

### 2026-09-06 (later) · Max density raised to 8 columns; remodel now blocked on a conflict
- skills: design-handoff (continued)
- **Shipped the ask directly.** "At max density I should see 8 videos in each row" needed *two*
  changes, not one: `DENSITY_MAX` 6→8 **and** the top `TIERS` entry 6→8. `VideoGrid` computes
  `min(density, viewportTierCap)`, so raising only the ceiling would have left the slider's last
  two stops silently dead — the exact trap the new code comment now warns about.
- **Verified live, not assumed:** 1920 → 8 columns / 218px cards; slider ends map correctly
  (leftmost = density 8, rightmost = density 2); 1280 still 4 and 1024 still 3, so the narrower
  tiers are unregressed; all three skins identical in both `wide` and `poster` layouts with no
  horizontal overflow. `npm run check` 0 errors, 212 + 6 tests pass.
- **Knock-on found and measured:** `PersonPosterGrid` is `min(density, cap) * 2` (RD8), so People
  poster now reaches 16 columns — 102x205px cards at 1920. Left the 2:1 ratio intact rather than
  special-casing it. Long names hard-clip at `line-clamp: 1` without breaking layout; whether the
  cut shows an ellipsis is unverified (pane zoom unsupported, screenshots unreliable here) so it
  became human QA item 5.9 rather than an asserted bug.
- **The requirement contradicts the chosen remodel, and that is now recorded rather than papered
  over.** Under target-width density, "8 per row" is emergent and no single target serves both
  5120 and 412. Handoff §2e carries a warning box with three resolutions; option 1 (keep column
  counts, extend `TIERS` upward) is flagged as the stronger candidate because the requirement was
  expressed in videos-per-row — evidence about the unit the user actually reasons in.
- **`/simplify` (4 agents) earned its keep on a two-constant diff.** Three of four converged on the
  same gap: the invariant was prose-only and the test asserted the wrong half. Mutation testing
  proved it — reverting the tier rung to a literal 6 left the original suite fully green. Fixes
  applied: derive the rung (`cap: DENSITY_MAX`) so the coupling is structural rather than
  comment-enforced; export `capForWidth` and assert `min(DENSITY_MAX, capForWidth(1920)) === 8`,
  mirroring `VideoGrid`'s own computation; drop three tautological `invertDensity` cases (a linear
  involution is self-inverse for *any* constant, so that test could never fail) and add tier
  boundary cases. Re-mutated to confirm: hardcoding the rung fails 3 tests, dropping `DENSITY_MAX`
  fails 2.
- **The efficiency pass caught a real regression I introduced**, not a style nit: `eager={i < 12}`
  in `PersonPosterGrid` was one row only while `DENSITY_MAX` was 6; at 16 columns the top row's
  last four posters would lazy-load above the fold. Now `eager={i < cols}`.
- **Skipped deliberately:** person images have no width variant or `srcset`
  (`personImageURL`/`PersonImageFrame`), so a ~1333x2000 source decodes into a 102px box — the new
  ceiling amplifies that ~1.8x but did not create it. Out of scope for a ceiling change; filed as
  **HOLODEX-332** rather than fixed here. Also skipped a `PERSON_DENSITY_MAX` bound on the 2:1 doubling — the
  reviewers called it low priority, it is documented and measured, and adding it would change
  behavior beyond the ask.
- **Next session:** get a decision on §2e's three options *before* writing any remodel code. Item 2
  is done and independently useful, so the branch is no longer docs-only — it now carries a small,
  verified behavior change that stands on its own if the remodel is deferred.

### 2026-09-06 (later still) · Density model decided: column counts kept, ladder to be extended
- skills: design-handoff
- **The §2e conflict is resolved in favour of the column-count model.** Kevin chose to keep
  columns-per-row and extend `TIERS` upward rather than move to target card widths — consistent
  with how he expressed the requirement in the first place. The target-width stop table is deleted
  from the handoff; §2e now carries the replacement ladder (`{2560: 12}`, `{3840: 16}` above the
  existing `{1536: 8}`) with card widths held in a ~170–320px band from 1536 to 5120.
- **The spec gate closed as a consequence, not by being satisfied.** It existed only to document
  the stored-preference remap that the target-width model would have forced. With column counts
  retained, `holodex:media-density` keeps its meaning and nothing migrates — so the gate is `[~]`
  not `[x]`, and the QA section that tested the migration was rewritten to test the ladder instead.
- **Surfaced a new constraint rather than declaring the decision done:** extending the ladder
  raises `DENSITY_MAX` to 16, which makes slider stops 9–16 inert at 1920 — the same dead-stop
  defect the shipped work just fixed, at eight times the scale. The fix (slider `max` tracks
  `viewportTierCap`) cascades into `invertDensity` and into whether `clamp()` stores the raw or
  clamped preference, since clamping on write would ratchet density down when moving from the
  ultrawide to a laptop. Written into §2e as a warning box and QA §4.5–4.7 so implementation is
  mechanical.
- **Deliberately not implemented this session.** The ladder is fully specified but is a three-file
  change touching the inversion math and preference persistence; tacking it onto the end of a long
  session is how the ratcheting bug would get shipped unnoticed. It is item 8, ready to start cold.
- Kevin also confirmed keeping People poster at the derived 2:1 ratio (16 columns at max) rather
  than giving it a separate ceiling.
- **Next session:** item 4 (stage token) or item 8 (ladder) — both are unblocked and independent.

### 2026-09-06 (later still) · Ladder extended, slider range made viewport-aware
- skills: design-handoff, simplify
- **Implemented item 8.** Rungs `{2560:12}` and `{3840:16}`; card width now holds 172–302px from
  1536 to 5120 instead of ballooning to 1252px, and a 5120 poster row is 453px tall rather than
  1878px — three visible rows where there was not one.
- **Solved the dead-stop trap rather than scaling it up.** Slider range is
  `DENSITY_MIN..capForWidth(viewport)`, `invertDensity` takes the cap, and the control hides at
  caps 1–2 (where it would be a one-position slider or an invalid `min > max`). Verified: 1024
  spans 2–3 with both positions live; 800 and 412 render no slider.
- **No ratcheting on reload**, verified: stored `16` renders 8 columns at 1920 and comes back
  intact. Corrected a comment that overclaimed this — dragging the slider on a small screen *does*
  lower the stored value, because the range spans only that viewport. That is intended (moving a
  control is an explicit choice) but the code previously asserted the opposite.
- **`/simplify` (2 agents) again earned its keep.** Both converged on the narrowing rule being
  triplicated → extracted `effectiveDensity()`; both flagged `DENSITY_MAX = TIERS[0].cap` as
  depending on an undeclared caps-descending invariant → now `Math.max(...)`, leaving only the
  `min`-descending invariant the boundary tests already guard. Also moved `DensitySlider` from
  `video/` to `sort/` per the folder rule (its scope line is index-page controls shared across
  browse/people/tags; `video/` is card/grid primitives), inlined a single-use predicate, and
  collapsed tests that restated one another.
- **The review caught a real defect in the People band.** Adding the suggested poster-band test
  failed at 78px — the 1536 rung is the tightest point on the ladder for People, well below the
  ~85px the reviewer assumed. Recorded as a bounded test plus human QA 5.10 rather than silently
  widening the band. If 78px reads as too small, the fix is a People-specific rung.
- **Also caught my own slip:** restructuring `density.svelte.ts` dropped `invertDensity` entirely;
  the type-check caught it before commit.
- Deferred: the ladder is tuned only to 5120 — above 3840 the cap holds at 16, so a 7680px panel
  would grow cards to ~462px. Documented in the TIERS comment and pinned by a test so it is a
  known boundary rather than a rediscovered bug.
- **Next session:** item 4 (stage token) — the remaining layout work is untouched.

### 2026-09-07 · Stage token + two-zone rail (items 4–6)
- skills: design-handoff, simplify
- **Stage token** lives in a plain `@theme` block, not `@theme inline`. Review flagged that `inline`
  resolves the value into the utility *without* emitting the custom property; verified directly —
  `--container-stage` was absent from `:root`, so `var(--container-stage)` would have been
  undefined, which is the one thing a named token buys over `max-w-[2600px]`. Registered in
  `theming.md` and the frontend-theming rule so the next full-width page finds it.
- **Two-zone rail** on media detail, measured at every breakpoint (see item 5). Structural integrity
  of the ~860-line move was verified independently by normalized diff: the only content delta is 3
  balanced wrapper divs, the `<article>` class, and comments — nothing dropped or duplicated.
- **Review caught a real regression I introduced.** Moving the field list into the rail without
  item 6 left two 174px columns at 1024. My own handoff §4b says the two must ship together, and I
  had split them. Item 6 landed in the same change; verified 1/2/3 columns at 1024/1920/5120.
- **Corrected a claim I had repeated since the first analysis.** "The three owner pages double-wrap
  redundantly" was true only for `owner/status` (5xl inside 5xl). `keys` and `trash` were
  `max-w-4xl` inside `max-w-5xl` — *tighter*, so binding. Removing them was a ~3x widening, not
  cleanup, and the wrong call: both are flat `flex` rows with a `flex-1` label and `shrink-0`
  actions, so stage width would strand a Delete button ~2400px from its title. Both keep
  `max-w-4xl`; the handoff, PR and Jira wording are corrected.
- **Deliberately deferred:** `films/[id]` keeps `max-w-4xl` — giving it the stage without its rail
  would make it a 2600px single column, the exact option-B failure this design rejects. Rail
  sections also kept their existing chrome; turning the rail into a column of cards is a visual
  decision, not something a layout move should smuggle in.
- **New risks recorded rather than guessed at:** the metadata fold still animates to a magic
  `max-height: 6000px` sized for an 864px-wide list (QA 3.19), and the Films/People `side-by-side`
  branch inherited a `max-w-[50%]` split tuned for the old column (QA 5.11).
- **Next session:** item 7 — the film detail page's rail, which unblocks giving it the stage token.

### 2026-09-07 (later) · Film detail rail; shared stage-grid utility
- skills: design-handoff, simplify
- **Film page zoned differently from media detail, for measured reasons, not symmetry.** The banner
  is `aspect-[8/3]`: at full stage width it would be 975px tall and swallow a 1440px screen, so it
  sits in the 1.4fr subject column (401px at 1920, 563px at 5120) with the header, which overlaps
  it via `-mb-14` and cannot be separated from it. The Scenes list went **full width** beneath both
  zones because it renders `VideoGrid`, whose column count comes from `effectiveDensity()` — the
  *viewport*, not the container. In the 1.4fr column it would ask for 16 columns inside 1503px
  (~88px cards); at full stage width it gets 218px cards at 1920, matching the browse grid.
- **Extracted `@utility stage-grid`** now that a second call site exists — a prior review predicted
  exactly this ("defensible at one call site; two would not be"). The ratio, the 320px rail floor
  and the stacking breakpoint now live once in `app.css`. Verified compiled: `display:grid`,
  24px gap, two tracks at 1024 and one at 1023.
- **Found and filed HOLODEX-333** while setting up: with `FILMS_ENABLED=true` the header nav has
  five links plus the owner cluster and overflows horizontally at 768 (scrollWidth 778 vs 753).
  Attributed properly rather than assumed — zero overflowing elements inside either page's own
  container, and media detail overflows identically, so it is the shared header, not this work. It
  fails WCAG 1.4.10 on every route.
- **Setup gotcha worth remembering:** `films_enabled` is an env var (`FILMS_ENABLED`), not implied
  by the films-specific config paths in `launch.json`. Without it `/films` 404s and the nav link is
  hidden, so the film page cannot be verified at all. Added to the local (gitignored) launch config
  and to QA §1.1.
- **Next session:** items 9–11 — testing strategy, live three-skin QA on real hardware, then
  `/simplify` + `/code-review` on the whole implementation before marking the PR ready.

### 2026-09-07 (later still) · Stage-aligned Scenes grid
- skills: design-handoff
- **Kevin's question turned a binary into a better third option.** I had framed §9.6 as "cap the
  Scenes grid or break it out". He asked whether it could stay left-justified while it fits the
  stage and centre once it reaches the edges — which is both, and removes the sparse-row cost that
  made me hesitate to break it out at all.
- **Implemented as `stageAligned`, opt-in on `VideoGrid`,** used only by the film Scenes list.
  Three pieces, all load-bearing: fixed tracks instead of `1fr` (`1fr` always consumes the
  container, so `fit-content` could never shrink); track **count** capped at the card count
  (declared-but-empty tracks hold the grid open at full width — this was the bug in my first
  prototype, caught by measuring rather than by reasoning); and the CSS floor
  `width: fit-content; min-width: min(var(--container-stage), 100%); margin-inline: auto`.
- **Measured at 5120:** ≤8 scenes → grid 2600px with its left edge exactly on the stage's (1260px),
  302px cards; 9 → 2846px; 12 → 3800px; 16 → 5072px at the page edge. Card size constant throughout.
  Below the stage the mode is inert — 1872px/220px at 1920 and 2512px/194px at 2560, both identical
  to pre-change.
- **Track maths extracted to `$lib/stageGrid` and unit-tested** (11 cases), following
  `filmsPeopleLayout`'s precedent: the interesting behaviour lives at card counts the dev fixture
  cannot reach — its only film has two scenes, and the attach dialog only offers studio/cast
  matches — and the repo has no component-test harness. The tests pin the 9-card threshold *and*
  that it is emergent: a 1600px stage would flip at 6 with no code change.
- **Left for a human:** whether a 9–12 scene film, sitting partly outside the stage on both sides
  while the hero stays centred, reads as deliberate (QA 6.9). Arithmetic is pinned; taste is not.
- **Next session:** items 9–11 — testing strategy, real-hardware QA, then `/simplify` +
  `/code-review` over the whole implementation before marking the PR ready.
