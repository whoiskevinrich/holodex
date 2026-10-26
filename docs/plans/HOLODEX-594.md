---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/archive/ADR-064-flightplan-plugin.md
key: HOLODEX-594                 # the tracker key; must match the branch key regex
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
profile: chore
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: None — test-harness maintenance only, no user-facing change.
---

# HOLODEX-594 · Geometry/nav harness: two checks gone stale since F80 entity paging

Spun out of HOLODEX-32 (F82 list views). F80 moved `scroll-mt-16` from each People/Studios row onto
the list wrapper and paged the lists to 50 rows, which left three harness checks measuring nothing
or aiming past the page.

**Design package:** none (test-only; no spec/architecture/design churn)

## Gates — definition of done

<!-- chore posture: no gates. -->

## Up next — ordered (position = priority)

1. [x] [—] `az-rail-clear-of-rows` + `list-first-row-near-top-phone` select `main ul > li` — `web/geometry/assertions.mjs`
2. [x] [—] `two-tabs-independent` random tab targets 600px, inside the first page — `web/geometry/nav.mjs`
3. [x] [—] push, open PR #553
4. [ ] [—] Move HOLODEX-594 to Done if the merge doesn't (branch-keyed, so jira-sync should)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-24 · Repointed the three stale harness checks
- skills: code-review
- handoff: Selectors and scroll target fixed and verified against the 9600 stress pair. The checks touched pass, nav is 12/12, and geometry's 36 errors are the `enrich` profile run without `enrich-stub`. PR #553 is ready to merge once CI is green.
