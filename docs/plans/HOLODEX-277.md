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
      `SetCurationChecked` gains an optional `commit func(WriteLock)` run under the same
      `writeMu` lock right after the curation write; `ReconcileVideoPeople` splits into a
      locked core (`ReconcileVideoPeopleLocked`, gated by an unforgeable `WriteLock` token)
      and a thin locking wrapper. Chose extending the lock over re-resolving from source
      inside `check()` (Option A, rejected — reintroduces the I/O HOLODEX-274 removed).
      `SetDecisionChecked` (Title/Studio) is explicitly untouched (non-goal).
- [ ] spec — not needed (no contract/request-response change; internal locking fix)
- [ ] design — not needed (backend-only)
- [ ] backend — implement ADR-084: `WriteLock` type, `SetCurationChecked`'s `commit`
      parameter, `ReconcileVideoPeopleLocked`, move `relinkPeopleWithContext`'s call from
      after `SetCurationChecked` into a `commit` closure in `setCuration`
      (`internal/api/curation.go`)
- [ ] testing — regression test: two concurrent `setCuration` calls to different
      person-typed fields on the same video both survive in `video_people`
- [ ] security — `/security-review` pass on the locking change (owner-gated path, no new
      query shape, but touches the write-path critical section)

## Up next — ordered (position = priority)

1. Implement ADR-084 (backend gate above).
2. File a follow-up HOLODEX ticket for the secondary finding from HOLODEX-274's review
   (`proposedPeopleLinks`' suppress match widening blast radius on the real `video_people`
   write) — an entity-identity question, independent of this ADR, noted as a Non-goal in
   ADR-084.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-14 · ADR-084 drafted, branch renamed, ticket In Progress
- skills: architecture
- handoff: Surveyed the current code (`internal/api/curation.go`'s `setCuration`/
  `proposedPeopleLinks`, `internal/api/person_links.go`'s `relinkPeopleWithContext`,
  `internal/repo/curation.go`'s `SetCurationChecked`, `internal/repo/person_links.go`'s
  `ReconcileVideoPeople`, and the Studio precedent in `internal/api/decisions.go`) to
  confirm exact signatures and the locking gap before deciding. Kevin chose Option
  B (extend the lock to cover the relink write) over Option A (resolve fresh inside
  `check()`) — admin-only/low-traffic path favors the architecturally cleaner fix over
  the lower one-time-cost patch. Wrote [ADR-084](../architecture/ADR-084-locked-curation-relink-commit.md)
  proposing a `commit func(WriteLock)` parameter on `SetCurationChecked` and a
  `ReconcileVideoPeopleLocked` locked core gated by an unforgeable `WriteLock` token
  (chosen over plain doc-comment discipline alone, since the boundary it guards crosses
  the `api`/`repo` package split where this codebase's usual `xLocked`-naming convention
  loses its unexported-ness protection). Renamed the worktree branch to
  `HOLODEX-277-people-relink-atomic-lock` and fired the Jira In Progress transition
  (ADR-058) as the first actions, per project convention. Next: implement the backend
  gate, write the concurrency regression test, `/testing-strategy` and
  `/security-review` passes, then push and open the Draft PR (ADR-069).
