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

## Session log — append-only (cap: last 8 sessions; older → archive/)

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
