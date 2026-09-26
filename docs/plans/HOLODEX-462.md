---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-462
status: in-progress
profile: backend
release_note: No user-facing change. CI's Jira sync no longer moves an issue to an earlier status category, so a closeout PR on a finished issue's branch can't drag it from Done back to In Review.
---

# HOLODEX-462 · jira-sync: never move an issue backwards

HOLODEX-358 was already `Done` when its worklog closeout PR (`chore/holodex-358-worklog-closeout`)
was marked ready. `extractKeys` matches case-insensitively, so `jira-branch-sync.mjs` moved it
`Done` → `In Review`. On merge the docs-only guard then skipped `Done`, which stranded the issue.
Fix: `syncOne` refuses any transition whose destination `statusCategory` ranks earlier than the
current one (`new` → `indeterminate` → `done`). Sideways moves still happen, and so do unranked
categories. The docs-only guard and case-insensitive key matching stay as they are. This matches
Flightplan ADR-005's addendum (whoiskevinrich/flightplan#11).

## Gates — definition of done

- [~] spec `write-spec` — **not applicable.** CI tooling, and no product behaviour changes.
- [~] backend — **not applicable.** No `cmd/`, `internal/` or `providers/` change.
- [x] testing `testing-strategy` — five new cases in `scripts/lib/jira-sync.test.mjs`: `Done` →
  `In Review` refused with and without `docsOnly`, In Progress → In Review allowed, an unranked
  category allowed, plus `isBackwards` itself. `Done` → `Released` now carries real categories.
  A mutation that disables the guard fails exactly the two refusal tests. `docs/testing-strategy.md`
  names the file as critical-invariant.

## Up next

- On merge: CI fires `Done` for HOLODEX-462 from the branch key. No children to sweep.
- Separate follow-up, once whoiskevinrich/flightplan#31 is released: remove the
  `transitions.in_progress` block from `.claude/flightplan.yaml` and remove `scripts/jira-transition.mjs`,
  after confirming nothing outside the repo still calls them.

## Session log

### 2026-09-26 · session
- skills: code-review (high --fix: 1 fixed, 1 skipped), handoff
- Filed HOLODEX-462 (Relates HOLODEX-358). Implemented the statusCategory guard in `syncOne`,
  with 5 new tests, the pipeline doc and the testing-strategy note. Merged `origin/main` in.
- handoff: Every gate is settled and all 144 script tests pass. The PR opens as ready for review;
  the next move is to merge it, and CI fires Done.
