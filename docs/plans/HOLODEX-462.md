---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-462
status: in-progress
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
- [~] architecture `architecture` — **not applicable.** This is a guard inside ADR-058's pipeline,
  the same kind of change as the Epic skip (HOLODEX-185) and the docs-only guard (HOLODEX-173/220).
  Neither of those needed an ADR. It's recorded in `docs/reference/jira-pipeline.md`.
- [~] design `design-handoff` — **not applicable.** No UI.
- [~] backend — **not applicable.** No `cmd/`, `internal/` or `providers/` change.
- [~] frontend — **not applicable.** No `web/**` change.
- [x] testing `testing-strategy` — five new cases in `scripts/lib/jira-sync.test.mjs`: `Done` →
  `In Review` refused with and without `docsOnly`, In Progress → In Review allowed, an unranked
  category allowed, plus `isBackwards` itself. `Done` → `Released` now carries real categories.
  A mutation that disables the guard fails exactly the two refusal tests. `docs/testing-strategy.md`
  names the file as critical-invariant.
- [~] security `security-review` — **not applicable.** No auth, secret, permission or workflow
  trigger change. The guard only narrows which transitions CI makes.

## Up next

- On merge: CI fires `Done` for HOLODEX-462 from the branch key. No children to sweep.
- Separate follow-up, once whoiskevinrich/flightplan#31 is released: remove the
  `transitions.in_progress` block from `.claude/flightplan.yaml` and remove `scripts/jira-transition.mjs`,
  after confirming nothing outside the repo still calls them.

## Session log

- 2026-09-26 — filed HOLODEX-462 (Relates HOLODEX-358), implemented the category guard with
  tests and docs, and ran `/code-review high --fix`, which fixed 1 finding and skipped 1.
  - handoff: Code, tests and docs are done and all 144 script tests pass. The PR is open and
    ready; next is the merge.
