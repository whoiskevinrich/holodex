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

- [ ] spec `write-spec` — **required**: the density preference changes meaning (stored column count
  → target card width), so `holodex:media-density` values 2–6 need a documented remap and a new key
- [~] architecture `architecture` — not applicable as judged; layout convention plus a
  component-local storage change, no data-model or seam change. Revisit if the stage cap is meant
  to be a standing rule for all future pages rather than a per-page choice (handoff §9.2)
- [x] design `design-handoff` — `docs/design/responsive-page-width-handoff.md` +
  `responsive-page-width-mockup.svg` + `responsive-page-width-ladder.svg` +
  `responsive-page-width-qa-checklist.md`
- [ ] frontend
- [ ] testing `testing-strategy` — density remap is pure and unit-testable; layout needs the
  geometry assertions in QA §3
- [~] security `security-review` — not applicable; presentation only, no auth, access, or
  infrastructure surface
- [ ] three-skin QA — Cinémathèque / Broadcast / Brutalist at 412 / 768 / 1024 / 1280 / 1920 / 5120,
  reloading (not resizing) at each width

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
3. [ ] [spec] Density preference remap: old `holodex:media-density` (column count, 2–6) → new
   `holodex:media-card-width` (target px), read-migrate and leave the old key intact for rollback
4. [ ] [frontend] Stage token + wrapper: `--container-stage: 2600px` in `app.css` `@theme`, apply
   `max-w-stage` on the detail and owner pages, delete the three nested owner caps
5. [ ] [frontend] Two-zone split on media detail: `minmax(0, 1.4fr) minmax(320px, 1fr)` at ≥1024px,
   rail carries Tags / Films / People / Metadata / Manage / File in that order, stacks below 1024
   with no reordering (focus order must stay DOM order) — `media/[id]/+page.svelte`
6. [ ] [frontend] Field grid `sm:grid-cols-2` → `repeat(auto-fit, minmax(320px, 1fr))`, and
   `sm:col-span-2` → `grid-column: 1 / -1` on long-text and image fields
7. [ ] [frontend] Same two-zone treatment on `films/[id]/+page.svelte` — confirm the banner's
   aspect ratio wants the same 1.4:1 split as the video player (handoff §9.3)
8. [ ] [frontend] Density remodel — **BLOCKED on a design decision, see handoff §2e's warning
   box.** "8 videos per row at max density" is exact under the column-count model but emergent
   under the target-width model, and no single target width satisfies both ends: 220px gives 8
   columns at 1920 but only 1 at 412, while ≤174px gives 2 at 412 but 10 at 1920. Resolve as
   (1) keep column counts and extend `TIERS` upward for ultrawides — now the stronger candidate,
   since the requirement was expressed in videos-per-row; (2) target width, accepting drift;
   or (3) hybrid with a column clamp. Item 3's remap only matters under (2)/(3)
9. [ ] [testing] Unit tests for the density remap incl. the garbage-value fallback; geometry
   assertions per QA §3
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
