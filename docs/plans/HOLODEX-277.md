---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-277
status: in-progress
depends-on: [HOLODEX-274]
release_note: null
---

# HOLODEX-277 · People curation relink fast path loses self-healing convergence, races on concurrent edits

Done means the People curation add/suppress fast path (`relinkPeopleWithContext`,
`internal/api/person_links.go`) no longer has an unlocked gap between reading its link
snapshot and committing it — two concurrent curation edits to different person-typed fields
(e.g. an actors add and a director add) on the same video must both survive in `video_people`,
not silently drop one. Found and confirmed by `/code-review xhigh` on HOLODEX-274 (PR #240);
not fixed there because the correct fix is an architectural decision (ADR-084) past a
review-fix's scope.

## Gates — definition of done

- [x] architecture — [ADR-084](../architecture/ADR-084-locked-curation-relink-commit.md):
      `SetCurationChecked` gains an optional `commit func()` run under the same `writeMu`
      lock right after the curation write; `ReconcileVideoPeople` splits into a locked core
      (`ReconcileVideoPeopleLocked`) and a thin locking wrapper, using this codebase's
      existing `XxxLocked`-plus-doc-comment convention (`setCurationLocked`/
      `setDecisionLocked`). Chose extending the lock over re-resolving from source inside
      `check()` (Option A, rejected — reintroduces the I/O HOLODEX-274 removed). An
      unforgeable `WriteLock` capability-token variant was drafted first, then dropped
      during `/simplify` as providing no real compile-time guarantee over the doc comment
      alone — see the session log below for why, and ADR-084 Sub-decision B2 for the record.
      `SetDecisionChecked` (Title/Studio) is explicitly untouched (non-goal).
- [x] spec — not needed (no contract/request-response change; internal locking fix)
- [x] design — not needed (backend-only)
- [x] backend — implemented ADR-084: `SetCurationChecked`'s `commit` parameter,
      `ReconcileVideoPeopleLocked`, moved `relinkPeopleWithContext`'s call from after
      `SetCurationChecked` into a `commit` closure in `setCuration`
      (`internal/api/curation.go`)
- [x] testing — regression test `TestCurationAPI_PeopleConcurrentDifferentFields_NoLostUpdate`
      (`internal/api/curation_concurrency_test.go`): 20 rounds of genuinely concurrent
      `setCuration` calls to `actors`/`director` on the same video, both survive in
      `video_people` every round; full suite (`go test ./...`) green
- [x] security — `/security-review` pass on the locking change: no findings (owner-gating
      unchanged, no new input path, sanitization unchanged, no new SQL/lock-hold-time
      DoS surface)

## Up next — ordered (position = priority)

1. ~~Implement ADR-084 (backend gate above).~~ Done.
2. ~~File a follow-up HOLODEX ticket for the secondary finding from HOLODEX-274's review
   (`proposedPeopleLinks`' suppress match widening blast radius on the real `video_people`
   write) — an entity-identity question, independent of this ADR, noted as a Non-goal in
   ADR-084.~~ Filed as [HOLODEX-278](https://whoiskevinrich.atlassian.net/browse/HOLODEX-278),
   linked "relates to" this ticket.
3. Push, open the PR — all 6 gates are green, so open ready for review rather than Draft
   (ADR-069).

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-14 · ADR-084 implemented, all gates green
- skills: architecture, simplify, security-review
- handoff: Implemented ADR-084 per the prior session's design: `SetCurationChecked` gained
  a `commit func()` parameter run under `writeMu` right after the curation write;
  `ReconcileVideoPeople` split into `ReconcileVideoPeopleLocked` (locked core) + thin
  wrapper; `setCuration` (`internal/api/curation.go`) now builds `check`/`commit` closures
  and moves the People relink into `commit`. Wrote regression test
  `TestCurationAPI_PeopleConcurrentDifferentFields_NoLostUpdate`
  (`internal/api/curation_concurrency_test.go`, 20 rounds of concurrent HTTP requests) —
  passes deterministically. Ran `/simplify`: 4 parallel review agents (reuse/simplification/
  efficiency/altitude) surfaced one real defect in the initial draft — the `WriteLock`
  capability-token's core claim ("unexported field blocks construction from another
  package") is false in Go, since an empty/zero-value composite literal
  (`repo.WriteLock{}`) always compiles regardless of unexported fields (verified with a
  standalone two-package build). Dropped `WriteLock` entirely, reverting to this
  codebase's existing `XxxLocked`-plus-doc-comment convention
  (`setCurationLocked`/`setDecisionLocked`) — simpler and equally safe, since the token
  never provided a real compile-time guarantee to begin with. Also deduplicated the new
  test's server fixture into the existing `peopleDecisionServer` helper
  (`internal/api/curation_collision_test.go`), parameterized as
  `peopleDecisionServerWithFields`, instead of forking a near-identical copy. Corrected
  ADR-084 to match (Sub-decision B2 now records the token as considered-and-rejected).
  `go build ./...`, `go vet`, and `go test ./...` all clean. `/security-review` (with the
  corrected diff, not the stale docs-only one the skill auto-gathered) found no
  vulnerabilities — owner-gating, input sanitization, and DB write shape are all unchanged
  by this diff; it's a pure lock-ordering change. All gates now green. Next: file the
  Non-goals follow-up ticket, push, and open the Draft PR.
