---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-451
status: in-progress
approved:
  design:
    on: 2026-09-23
    at: b340a94
release_note: A flagged duplicate pair in the owner's Duplicates queue can now be opened side by side — both headshots, counts, aliases and provider links at once — so you can tell one person from two without leaving the page.
---

# HOLODEX-451 · Duplicates queue: side-by-side evidence so a person pair is decidable

`/owner/duplicates?type=person` shows two names, two video counts, the variation kind and the
match kind. That is not enough to decide whether a flagged pair is one person or two: the owner
is the entire inference engine, every pair arrives at equal weight, and the row carries no
supporting evidence. This epic gives the row enough to decide on.

**Shape: expand-to-compare.** Clicking a pair expands it in place into a two-column panel
showing both entities at once — headshot, counts, aliases, attributed provider facts, external
link badges — with the Merge / Keep-separate verdicts at the bottom. Profile images are the
fastest discriminator for a human reader.

## Decisions — do not re-litigate

- **The F68 person hover card is not the primary treatment here.** It is a one-at-a-time
  discovery affordance being asked to do a comparison job: one card open app-wide by design
  (so comparison happens from memory), 250 ms hover intent twice per row on a batch surface,
  hidden under `pointer: coarse`, and the pair row's names are `<span>`s rather than `<a>`s so
  `PersonLinkChip` is not free to adopt. It belongs later as the escape hatch on a hard pair,
  once the names become links.
- **Positive evidence can be decisive; negative evidence never is.** Providers carry their own
  duplicate entries, so "different ids on the same provider" does not mean two people.
- **Facts are only comparable inside a provider namespace.** Cross-provider disagreement
  describes the providers, not the people — `Nepal` vs `Federal Democratic Republic of Nepal`
  is a country-name update, `1990-01-01` vs `1990-03-17` is a fidelity difference. The panel
  reports who said what and never adjudicates: **no conflict chip.**
- Nationality normalization, if any, reuses whatever `NationalityFlags` maps names to flags with.

## Structural finding

`entity_external_ids` has `PRIMARY KEY (entity_type, external_id)` (migration 0046), so two
people can never share a `provider:id` there by construction. A shared external id is only
observable via `entity_enrichment.external_id`, which is not globally unique. Any auto-merge
rule depends on that being non-zero in practice — currently unverified.

## Probe results — live library, 2026-09-22 (1591 people, 1000 enriched)

`scripts/detect_person_duplicate_evidence.sql`, run on the host. Both blocking questions closed:

- **Symmetric panel.** 29 of 31 pairs have a headshot on **both** sides, 2 have one side. The
  asymmetric layout is not needed.
- **A strip, not one image.** Most people carry 40–50 images, so a short strip per side is
  clearly affordable and more discriminating than a single provider headshot.

What the numbers changed:

- **The queue is 100% weakest-signal.** All 31 pairs are `provider-alias` / `alias` — no
  whitespace, no punctuation. The F43 fuzzy set has been worked through.
- **Keep separate is the dominant verdict**: 185 person pairs already dismissed against 31 open.
  The current row styles `Merge` as the accent primary and `Keep separate` as a ghost — **the UI
  is optimized backwards and the verdict buttons should swap emphasis.**
- **The auto-merge rule is dead.** `same_xid` is 0 across all 31 pairs; nothing shares an
  external id. The panel is evidence-for-a-human, never a verdict engine.
- **`diff_xid` is 2 for nearly every pair** — both providers hold different ids for the two
  sides. Under the pre-correction ranking this would have stamped a confident "different people"
  on essentially the whole queue from a signal that means nothing. The negative-evidence rule
  above is load-bearing, not pedantry.
- Facts lean "different people": `birthdate` same-provider differs on 25 pairs, matches on 1;
  `nationality` 18 differ / 11 exact / 6 one-contains-other (the Nepal case is real and
  measurable); 3 pairs co-appear in a video.
- **Link row must not lean on `_source_url`** — only 86 of 1591 people have it. `website` covers
  688; provider link templates carry the rest.

Spun out: **HOLODEX-452** (detect duplicates by shared provider external id — the probe found 6
such collisions in `entity_enrichment`, none of them ever queued, which the name-based detector
structurally cannot see) and **HOLODEX-453** (should alias-only provider-alias pairs be flagged
at all). Scope call 2026-09-22: **panel first, detector follow-up.**

## Gates — definition of done

- [x] spec `write-spec` — F70, `docs/specs/duplicates-pair-evidence.md`. The design's four deltas applied and approved by Kevin 2026-09-22 (no `+N`, Merge takes `btn-ghost` not `btn-quiet`, no fade, F68 `Videos`/`Films` anchors); OQ2/OQ4 struck through as closed. **OQ3 re-answered 2026-09-23** — the match-kind label moves into the panel on a person pair; two P0-1 criteria and RD10 updated with it
- [~] architecture `architecture` — n/a (Kevin, 2026-09-22): no endpoint, no migration, no cross-cutting decision; OQ1 closed without forcing one
- [x] design `design-handoff` — `docs/design/duplicates-pair-evidence-handoff.md` + committed `duplicates-pair-evidence-mockup.svg` (5 panels, Cinémathèque + a Brutalist radius-0 panel). **Amended 2026-09-23**: panel 1 redrawn at real type sizes as single-line 40px rows, panel 2's well opens with the label. **Signed off by Kevin 2026-09-23 at `b340a94`** (`approved.design` in the frontmatter)
- [~] backend — n/a for P0: the panel composes `GET /people/{id}/card` (F68) + `GET /people/{id}/images` (F26), both existing. P0 touches no Go code
- [x] frontend — `DuplicateComparePanel.svelte` (new), disclosure + verdict swap + label gate in
  `DuplicatePairRow`, single-open/Escape/focus-after-resolve in `+page.svelte`, `imageId` on
  `PersonImageFrame`, the two `app.css` doc comments, the `duplicates/CLAUDE.md` row. Built and
  QA'd against a seeded two-pair fixture 2026-09-23; `npm run check` 0 errors, 415 tests pass
- [x] testing `testing-strategy` — `docs/testing-strategy.md` **§16 + §16.1**, and 32 new tests
  across three files. The three hand-QA findings are all pinned except the one that is genuinely
  browser-only: `queue.test.ts` (id agreement, the person-only panel gate, the OQ3 label
  placement in both directions, the three-rung focus ladder), `personImages.test.ts` (P0-5
  asserted against **requests** via a stubbed `fetch`, plus `stripGallery` for P0-3),
  `verdictOwnership.test.ts` (source-shape, à la `playerElement.test.ts`: the panel owns no
  verdict, so a per-side failure *cannot* disable one). All five rules mutation-checked. The
  `sm:flex-nowrap` row-height boundary has **no CI home** — the stress fixture seeds no
  deterministic duplicate pair — filed as **HOLODEX-456** and recorded as §16.1's first gap
- [ ] security `security-review` — owner-gated surface; re-confirm the existing gate covers the new fields
- [x] `code-review high --fix` — run on the frontend diff 2026-09-23. One finding applied: a
  failed side rendered the five-slot loading strip, claiming images it would never fill. Two
  logged and deliberately skipped — the image cache has no invalidation hook (mirrors
  `loadPersonCard`'s own contract; this page never mutates images) and the Escape listener is
  per-row rather than per-page. **Re-run it after the testing gate adds code**

## Up next — ordered (position = priority)

1. [ ] [—] `/security-review` — owner-gated surface; re-confirm the existing gate covers
   `/people/{id}/card` and `/people/{id}/images` reached from this page
2. [ ] [—] `gh pr ready` on #382 once security is green (`/code-review high --fix` was re-run
   over the testing gate's own code this session, closing the old item 3)
3. [ ] [—] Kevin's eyeball pass on the built panel in a prod skin before ready-for-review

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-23 · the testing gate — the hand-QA findings pinned, and one of them provably can't be
- skills: testing-strategy, code-review
- **The gate's real work was making the behaviours testable at all.** All four things the last
  session asked to pin lived inside `.svelte` components, and this repo has **no component-test
  harness** — `@testing-library/svelte` is named in §5's Phase-1 table but is not installed, and
  §14 already records "no component tests" as the standing posture. So the choice was extract or
  don't test. Two small modules came out, both of which remove real duplication rather than
  existing only for the tests:
  - `duplicates/queue.ts` — the page, the row and the panel each spelled the same per-pair id
    strings themselves. Now one `pairKey`, plus `showsComparePanel`, `matchKindLabel`,
    `labelPlacement` and `focusLandingIds`.
  - `person/personImages.ts` — the image cache lifted out of the panel's `<script module>`,
    sibling to `personCard.svelte.ts` and the same contract, plus `stripGallery`.
- **`labelPlacement` is the OQ3 trap turned into an assertion.** The last design session found by
  hand that moving the label into the panel without gating on entity type would delete it from
  studio/tag/film rows. That is now asserted in both directions for all four kinds, and the
  mutation that ignores entity type fails the suite.
- **P0-5 is asserted against requests, not against the client.** `fetch` is stubbed rather than
  `api`, because the criterion is "reopening issues no second request" — mocking
  `api.getPersonImages` would pin a call count and miss a bespoke `fetch` added later.
- **The per-side-failure criterion is held structurally rather than behaviourally.**
  `verdictOwnership.test.ts` (source-shape, the `playerElement.test.ts` vehicle) asserts the panel
  renders `{@render verdicts()}` and owns no verdict control or resolve call at all. That makes
  "a failure can't disable a verdict" *unbreakable* rather than merely currently-true — the
  verdicts are not the panel's to disable — and it pins RD7's emphasis swap in the same file.
- **The `sm:flex-nowrap` row-height boundary has no CI home, and I did not pretend otherwise.**
  The geometry harness could reach `/owner/duplicates?type=person` (it runs as owner, ADR-030),
  but `stressseed` seeds **no deterministic near-miss person pair** — the ones `FlagNearMiss`
  happens to file from the name palette are incidental and unaddressed by the manifest, so an
  assertion against them would pass *vacuously* on a reseed, which is the exact failure mode
  §12.2 warns about. Filed **HOLODEX-456** (seed a long-name pair, then add a `urls:` assertion
  with a `requires` gate) rather than writing a test that could not fail. It is §16.1's first gap.
- 32 new tests; `npm run test` 447 pass, `npm run check` 0 errors. **Seven mutations verified to
  break the suite** — the gate would otherwise be a claim rather than a guard.
- `/code-review high --fix` on the gate's own code found two, both applied: the loading skeleton's
  slot count was left hardcoded at 5 while the real cap moved to `personImages.ts`, so the two
  could drift and the column would jump when images land (now derived from
  `STRIP_GALLERY_SLOTS + 1`); and the emphasis-swap assertions matched attribute *order*, which an
  inline arrow's `=>` already broke — they now match whole `<button>` elements.
- handoff: **testing is closed; only security remains.** Start at `Up next` 1 — re-confirm the
  owner gate covers `/people/{id}/card` and `/people/{id}/images` as reached from this page. No
  behaviour changed this session: the id strings are byte-identical, the label gate is the same
  predicate, and no CSS moved, so the build session's three-skin QA still stands. `gh pr ready` on
  #382 after security, and Kevin's prod-skin look is still open.

### 2026-09-23 · the frontend is built, and QA found three things the design could not have
- skills: (none — straight build from the handoff's checklist), code-review
- Built the whole P0 surface: `DuplicateComparePanel.svelte`, the disclosure + verdict swap +
  label gate in `DuplicatePairRow`, single-open/Escape/focus-after-resolve in `+page.svelte`,
  the two `app.css` doc comments, the `duplicates/CLAUDE.md` row.
- **The `CardLease` question is answered: `release()` REFCOUNTS.** It decrements `waiters` and
  aborts only while `controller` is still non-null — which it is only until the request settles.
  A settled entry stays in the cache. So the panel holds its leases for its own lifetime and
  releases on teardown: collapsing mid-flight cancels correctly, collapsing after the data
  landed keeps the cache warm. That is recorded in the component's header comment.
- **The design's build checklist was not sufficient for P0-5, and the browser is what caught it.**
  `loadPersonCard` caches, but `api.getPersonImages` has no cache of its own — the network log
  showed `/people/{id}/images` re-fetched on *every* reopen while `/card` was fetched once.
  P0-5 held for half the payload. Added a matching module-level image cache in the panel's
  `<script module>`, same contract (shared across panels, a rejected promise evicted so a retry
  re-requests). Re-measured: zero requests of either kind on reopen.
- **Dropping `flex-wrap` outright broke the phone row, and the handoff's escape hatch never
  fired.** With `min-w-0` the inner block shrank to nothing instead of letting the ROOT wrap, so
  at 375px both names collapsed to ellipses and the `shrink-0` meta spans overflowed their own
  box and painted *under* the Keep-separate pill. Two changes fix it without touching the
  signed-off desktop behaviour: `min-w-64` gives the block a floor so the root's `flex-wrap`
  actually fires, and the inner block is `flex-wrap sm:flex-nowrap` — nowrap from 640px up (the
  one-line truncating scan row P0-1 asks for), wrapping below it, where the meta spans together
  outweigh the row. Measured after: 49px single-line rows with no overlap at 660px and 866px;
  at 375px names render at full width in a taller row.
- **Focus after resolving the last row in a group fell to `<body>`.** The group heading the
  handoff names as the fallback is removed along with its own group when the group empties, so
  there was nothing to land on. Added a third landing spot — the queue container, which always
  survives and is what holds "No possible duplicates."
- One shared component needed a prop the handoff assumed already existed: `PersonImageFrame`
  serves by ROLE only, and the strip's slots 2–5 are gallery images by id. Added an optional
  `imageId` so the strip still routes through it (P0-3) rather than hand-rolling a second frame.
- QA'd against a seeded two-pair fixture (one `alias` pair → `text-warn`, one `mixed` →
  `text-muted`; 6 images on one side to prove the 5-frame cap, a bare side for the placeholder
  path). All three skins: radius 0 in Broadcast/Brutalist, the scanline lands on ten 44px frames
  in Broadcast only, `--surface-2` well reads as a recess in each. Verified by measurement:
  frames exactly 44×44, columns equal-width, stacked at 375px with each border intact and no
  horizontal page scroll, Escape returns focus to its own disclosure, a second disclosure
  collapses the first, a forced per-side failure shows one broken column with Retry while all
  six verdict buttons stay enabled, and Retry recovers.
- Both theming greps are clean for the changed files; `npm run check` 0 errors, 415 tests pass.
  (Prettier is not in this repo's tooling — no config, no devDependency — so there is no format
  gate to run.)
- `/code-review high --fix` found one more: the strip's loading branch is keyed on `card === null`,
  which is also true after a failure, so a broken column rendered five empty wells it would never
  fill — the same overstatement P0-3 rules out for a person with no images. A failed side now
  holds one frame. Two findings logged and skipped (see the gate line).
- handoff: **the frontend and code-review gates are closed; testing and security remain.** Start
  at `Up next` 1. Nothing is blocked. The three QA findings above are the behaviours most worth
  pinning in tests, because none of them were visible from the design and two only appeared at a
  specific viewport. Committed as `0cf025d` and pushed to draft PR #382, whose description now
  carries the same gate posture and the QA write-up.

### 2026-09-23 · the sign-off was refused, the mockup was the reason, and the redraw carried it
- skills: implement, design-critique, design-handoff
- `/implement` put the design package up for sign-off. **Kevin pushed back** — "the collapsed
  queue has overlapping text" — so nothing was recorded: no `approved:`, no rebase, no PR.
- The overlap was real and measurable: the weak-signal label ran 28–32 px under the
  `Keep separate` pill in both person rows. But it was not a drawing slip. At the component's
  **real** type sizes (`text-sm` names, `text-xs` meta) that row needs **~836 px** to stay on
  one line, so `:77`'s `flex-wrap` wraps it — and the probe already said **100 % of the queue
  carries that label**, making the two-line row the *default*, not an edge case, on a surface
  built for scanning. The mockup had drawn the label overlapping the pill instead of wrapping,
  which is exactly why the cost was invisible when OQ3 was answered on 2026-09-22.
- **Kevin chose: move the label into the panel, keep rows 40px.** OQ3 reversed. It now opens
  the panel well above both columns — it describes *the pair*, so it can't sit in a column, and
  RD3/RD4 keep it out of the footer.
- Verifying that turned up a trap: `matchKindLabel` (`:84–92`) is **not** gated on entity type,
  and the panel is person-only (RD10), so a naive move would have deleted the label from
  studio/tag/film rows. Non-person rows keep it in the row — which also keeps P0-1's existing
  "byte-identical to today's" criterion true.
- Fixed two **pre-existing** defects in the same artifact while redrawing: panel 4's title ran
  145 px into panel 5's, and three captions (two of them untouched by this change) ran off the
  720 px canvas and were being clipped. Panel 1's group rect was also 18 px too short for its
  own tag row.
- Verified by measurement, not eyeball: 0 text collisions and 0 clipped captions across all 82
  text elements, SVG parses clean.
- Re-asked with the redraw rendered inline — rows back to one 40px line (widest run ends at
  x=489, the pill starts at x=540), the weak-signal label opening the panel's well, tag rows
  keeping their own. **Kevin approved.** `approved.design` recorded at `b340a94`, the rebase
  onto `0dfea65` (`1.16.1`) was clean, and **draft PR #382** is open. The branch had been pushed
  before the rebase, so the push needed a `--force-with-lease` — asked for and granted; the
  remote tip `ed1bc14` was patch-identical to the rebased `5e361c7`, so nothing was lost.
  Jira needed no transition (already `In Progress`) and carried no `fp:ready-to-build` label,
  but its description was stale on the sign-off and the OQ3 reversal and has been brought current.
- handoff: **Crossed into build — design signed off at `b340a94`; draft PR #382 open.** Start at
  `Up next` 1: build the frontend from the handoff's build checklist, and settle whether
  `CardLease.release()` evicts or refcounts before writing P0-5. Nothing in the design is open
  any more; the next gates are frontend, testing and security.

### 2026-09-22 · brainstormed the gap, ruled out the hover card, shipped the probe
- skills: product-brainstorming, implement, write-spec, design-handoff
- Kevin asked whether the new person hover card would fix the undecidable rows. Argued it
  would not — wrong shape of affordance for a comparison task — and put five treatments up as
  an inline mockup; he picked expand-to-compare on the strength of profile images. He then
  corrected two of my evidence claims (providers self-duplicate; cross-provider facts are not
  comparable), which demoted the whole auto-verdict line and produced the standing rules above.
  Chasing that down found a third error of my own: the `entity_external_ids` PK makes a shared
  external id impossible in that table, so the auto-merge rule I had ranked first is not even
  reachable there.
- Probe written, anonymized in the same shape as `detect_entity_collisions.sql`, and verified
  against a seeded fixture covering canonical/mixed/alias matches, both/one/neither headshots,
  same vs different external ids, and both cross-provider fact-disagreement shapes.
- Kevin ran the probe on the host. Both blocking questions closed (symmetric panel, image strip)
  and the queue turned out to be 100% weakest-signal with keep-separate as the dominant verdict.
  Spun out HOLODEX-452 and HOLODEX-453; he chose panel-first.
- handoff: nothing is blocked any more — start at `/write-spec` for the symmetric two-column
  panel with an image strip per side and the verdict emphasis swapped, then `/design-handoff`,
  then `/implement` to cross and open the draft PR.

### 2026-09-22 · design handoff — the column is the F68 card laid flat
- skills: design-handoff
- **The organising idea:** each column is `PersonHoverCard`'s anatomy, same fields in the same
  order with the same absent-is-absent rule, with a strip of five 44 px frames where its single
  48 px headshot was. That makes RD2 concrete — the hover card was never the wrong *content*,
  only the wrong *container*. Everything else in the panel falls out of reusing it.
- All three open questions closed by Kevin against an inline mockup: **OQ2 — there is no `+N`
  at all** (he rejected both the modal and the navigate; the strip is a sample, not an index),
  **OQ3 keep both match-kind labels** in the collapsed row only, **OQ4 five frames at 44 px**
  (`5 × 44 + 4 × 4 = 236` inside a 296 px column — measured, not guessed).
- Four design calls the spec didn't make: Merge takes **`.btn-ghost`, not `.btn-quiet`** (that
  class is documented as "no side effect" and Merge is irreversible); the link row **keeps F68's
  `Videos`/`Films` anchors**, which is what stops the panel being a navigational dead end now
  that `+N` is gone and the names are still `<span>`s; **each column is its own bordered card
  with no divider between them**, because `--surface-2` is ~2 % off `--surface` in Cinémathèque
  and the stacked layout would otherwise need a border-orientation flip `field-grid` can't
  signal; and **no summary line in the footer** — a first draft had one and it was exactly the
  cross-side adjudication RD3/RD4 forbid.
- Three things the exploration turned up that the spec had wrong or missing: **the row never
  fades** (`+page.svelte:47–49` is an unanimated filter; both the spec and the component's own
  header comment claim a fade), **`app.css` names these two buttons by name** in the
  `.btn-ghost`/`.btn-accent` doc comments so the swap makes them wrong, and **`api.videos({personId})`
  does not exist** — the P1-0 co-appearance call would be `api.listMedia({ person: [id] })`.
- Kevin approved the four spec deltas the same session; they are applied, OQ2/OQ3/OQ4 are struck
  through as closed, and RD7/RD12 reworded. The spec and the handoff no longer disagree.
- handoff: **every design-phase gate is green** (spec `[x]`, architecture `[~]` n/a, design
  `[x]`). Next is `/implement` — record the sign-off, rebase, push, open the draft PR. Then
  build straight from the handoff's build checklist; the first unknown to settle in code is
  whether `CardLease.release()` evicts or refcounts, because P0-5 depends on it.
