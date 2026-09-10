---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-355
status: in-review
depends-on: [HOLODEX-342]
release_note: Fixed media and film detail pages scrolling sideways on tablet and phone widths.
---

# HOLODEX-355 · `stage-grid`'s single-column branch omits the `minmax(0, …)` guard

`@utility stage-grid` defined `grid-template-columns` only inside its `>= lg` media query, so
below `lg` the single implicit column was sized `auto` — which carries `min-width: auto` and
therefore resolved to the content's *min-content* width. Every media and film detail page
overflowed the viewport horizontally by a constant ~440px at 768px, independent of content.
Done means the guard the utility's own comment already documents for the two-column case also
covers the one-column case, with the geometry harness proving it.

**Design package:** no spec/ADR/design change — this restores the layout
[HOLODEX-331](HOLODEX-331.md) already designed and which `stage-grid`'s own comment already
states for the `>= lg` branch; only the branch below it was missing the guard. Regression
coverage is [`docs/testing-strategy.md`](../testing-strategy.md) §12 (the HOLODEX-349 harness).

## Gates — definition of done

- [~] spec `write-spec` — n/a: a defect fix, no requirement or scope change
- [~] architecture `architecture` — n/a: no seam, stack or data-model decision
- [~] design `design-handoff` — n/a: restores the intended layout, does not redesign it
- [~] backend — n/a: frontend-only
- [x] frontend — base `grid-template-columns: minmax(0, 1fr)` on `@utility stage-grid`
- [x] testing `testing-strategy` — the `no-horizontal-page-overflow` assertion already covered
  this (HOLODEX-349); its `blockedBy` narrowed from `HOLODEX-355, HOLODEX-356` to `HOLODEX-356`
  so the marker stays honest and does not silently disarm
- [~] security `security-review` — n/a: no auth, access or infrastructure surface

## Up next — ordered (position = priority)

1. [ ] [—] Sweep to `Done` on merge — CI transitions only the branch's own key, and this branch
   is stacked on `HOLODEX-342-stress-fixture`, so confirm the base merges first
2. [ ] [—] **HOLODEX-356** is the remaining blocker on `no-horizontal-page-overflow` — the header
   nav at 768px (+10px/+33px depending on skin) and the person hero on an `unbroken` name
   (+231px). Only when *that* lands does the assertion go green and `blockedBy` come off
   entirely → HOLODEX-356
3. [ ] [P2·M] **File: `blockedBy` mutes an assertion across every page it runs on.** Surfaced by
   the `code-review max` pass below and deliberately not fixed there. `no-horizontal-page-overflow`
   selects 30 of 84 fixture pages × 6 cells = 180 checks, and the single HOLODEX-356 marker mutes
   all 180 — so *this ticket's own fix is not regression-guarded*: reverting `minmax(0, 1fr)` moves
   the report from `70 known-open` to `79` and still exits 0. The fix is to narrow a marker to the
   cells/pages the bug actually reproduces on (per-cell, since at `narrow` all 30 fail while at
   `wide` only 1–2 do), which needs 356's real failing set measured first — guessing it would mute
   the wrong pages, the same defect pointed the other way
4. [ ] [P3·S] `tag-chips-stay-tappable` measures the inner `<a>`, which is correct today (the owner
   branch's padding sits on a non-clickable wrapper, so 20px is the real tap target) but becomes a
   trap if HOLODEX-357 is resolved by padding `.curation-chip` instead of the link — the assertion
   would keep failing and its stale-marker signal would never fire. Whoever rules on 357 should
   re-check this selector against the ruling; a comment in `assertions.mjs` now records why

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-09 · `code-review max --fix` over the whole stacked branch
- skills: code-review
- verified: 10 finder angles + per-candidate verification over the 39 reviewable files (~9.9K
  lines; `graphify-out/` and the lockfile excluded). 15 findings, 13 fixed. Most land in
  HOLODEX-342's half of the stack rather than this one. The load-bearing four: **the harness
  could pass on measuring nothing** (`applies: 'count'` returned before the vacuity guard, so a
  renamed class read as a pass *and* triggered "HOLODEX-354 looks fixed, drop `blockedBy`"), and
  `blockedBy` swallowed `vacuous` on 4 of 5 assertions so a stale selector exited 0; **`reset()`
  left six no-FK tables behind** — `job_runs` (which permanently disables the person-link backfill
  and its "SHRANK" loss guard from the second boot on), `denied_tags` (one QA deny aborts every
  later reseed *after* the wipe), and the four owner-decision tables (re-attached to the next
  seed's entities at the same steered addresses); **6,065 lines of Go never ran in CI**, since the
  toolchain excludes any directory named `testdata` from `./...`; and **`npm ci` installs no
  browser**, which the docs never said and which surfaced only after preflight had passed.
  Green after: `go test ./testdata/stressseed` ok 94s, `npx vitest run` 269/269, `npm run check`
  0 errors, and a full live harness run reproducing the baseline exactly — 146 passed / 88
  known-open / 1 skipped, 234 page loads, exit 0. Seed + reseed into a scratch path both clean.
- handoff: two findings not applied, both now items 3 and 4 in *Up next* — the `blockedBy`
  granularity gap (which is why this ticket's own fix still is not regression-guarded) and the
  tag-chip selector's future-fix trap. Neither blocks the merge. Note the CI change adds ~94s to
  the backend job (confirmed in CI: `backend` now runs 2m31s); that is the price of compiling the
  seeder at all, and tuning it is a separate call. Two evaluate.mjs tests were updated rather than
  kept green: both were pinning the defective behaviour they described.
- then: **#313 squash-merged, and the stack needed a sync.** Squash gave 342's commits a new SHA,
  so all 17 files this branch shares with it conflicted as add/add and #314 went `DIRTY`. Resolved
  by first proving `origin/main`'s copy of every one is byte-identical to 342's old tip `8015dd9`
  — which makes this branch's copy "main plus the review fixes", so `--ours` is correct for all 17
  rather than merely convenient. `graphify-out/graph.json` *auto-merged* without conflicting,
  which for a generated 200K-line file means git blended two lineages; restored from HEAD and
  re-derived (graphify then reported no topology change, confirming HEAD's copy was canonical).
  #314 is now `MERGEABLE/CLEAN` against `main`, all ten checks green, and its diff is exactly this
  branch's own work. Pre-existing `gofmt` drift on ten `internal/**` files is visible locally but
  is untouched by this branch and already on main — left alone, worth its own cleanup.

### 2026-09-10 · `code-review xhigh` over PR #314, all 14 findings applied
- skills: code-review
- verified: reviewed this PR's own diff (656 reviewable lines) rather than the epic's, which
  caught **a regression the previous pass introduced**: un-muting `vacuous` was right, but
  `reconcileBlocked`'s `stillBroken` still enumerated only `blocked`/`error`, so a group of some
  passes and some stale selectors would have printed "every check now passes — drop `blockedBy`"
  for an assertion that measured nothing on half its pages. Now stated as an exclusion
  (`s !== 'pass' && s !== 'skipped'`) so an unknown status suppresses the verdict by default.
  Second self-inflicted one: `cd web` as step 0 of the quickstart broke the block as a paste in
  both the README and §12 — the following `go run ./testdata/stressseed` would have run from
  `web/`. Subshelled. The rest cluster as "the fix left two hand-maintained copies": the stub's id
  template was built twice per candidate and `idNamespaceFor` restated the `flood`/`twins`
  prefixes, so `/describe` and `/resolve` could disagree again through a one-sided edit — both now
  derive from one `candidate()` helper, and `stub.js` is requireable (`require.main` guard) so a
  new `stub.test.mjs` guards the §4.1 relationship under `make test-scripts`. Same shape in the
  build config: CI now calls `make vet` / `make test-go` instead of restating `GO_PKGS`, and
  `test-integration` uses it too.
  **Both new guards were mutation-tested**: reinstating `namespace: 'tmdb'` fails 2 of the 4 stub
  tests, and reverting `stillBroken` fails the 2 new reconcile tests.
  Green after: `go vet ./... ./testdata/stressseed` clean · full Go suite passing · 109 node tests
  (105 + 4 new) · vitest 272/272 · a full harness run reproducing the baseline exactly (146/88/1,
  exit 0) · and a narrowed run now prints "known-open markers not checked".
- handoff: nothing deferred — all 14 applied. Committed as `chore(testing)` this time; the
  previous review commit used `fix(testing)`, which broke the convention every other commit in
  this epic follows and which exists to keep dev-fixture churn out of the CHANGELOG.
  Note `make` is not on PATH on this Windows box, so the two new CI steps were verified by running
  their target commands directly — CI's `scripts` job already proves `make` works on the runner.

### 2026-09-09 · fixed, measured before/after against the stress fixture
- skills: code-review
- verified: reproduced the ticket's exact measurement on `/media/200` at 768px
  (`grid-template-columns: 1184px` in a 704.8px container, 455px of page overflow), then
  confirmed the fix at 320/768/1440 across all three skins. The `>= lg` two-column layout is
  unchanged (789px/563px ≈ 1.4:1, rail above its 320px floor, zero overflow). Harness
  before/after on the same fixture: **101 passed / 79 known-open → 110 / 70**, zero errors.
  Every residual overflow is HOLODEX-356's (10px/33px nav, 231px `unbroken`); the 440px
  content-independent signature is gone from every page type and skin.
- handoff: HOLODEX-355 is fixed and verified live; the fix is one base track declaration plus
  the comment explaining why that branch needs the guard too. The assertion it unblocks is
  still held open by HOLODEX-356 alone, so `blockedBy` was narrowed rather than dropped —
  whoever fixes 356 should expect the harness to report the assertion as newly-passing and
  must then remove `blockedBy` entirely to arm it.
