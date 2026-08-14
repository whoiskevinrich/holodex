---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-274
status: in-review
depends-on: [HOLODEX-272]
release_note: null
---

# HOLODEX-274 · Thread relinkContext through People curation commit (F30/ADR-072)

Done means `setCuration`'s person-typed-field (actors/director) add/suppress path no longer pays
for a redundant `loadRelinkContext` (4 queries) + `resolver.Resolve` pass after the write commits
— the People-flavored sibling of HOLODEX-271's Studio fix (`resolveProposedStudioNames` /
`relinkStudiosWithContext`), adapted for People's two constraints: (1) `ReconcileVideoPeople`
needs role-tagged `[]repo.PersonRoleName` links across *both* `actors` and `director`
(`registry.PersonTypedFields`), not a flat name list, and (2) unlike Studio's accepted
unlocked-then-relocked collision check, People's current-links read must run *inside* the
`writeMu` lock (HOLODEX-272's TOCTOU fix) — so the reusable link set has to come out of the same
locked closure the collision check runs in, not a separate unlocked pre-fetch.

Originally scoped as a pure I/O-reduction follow-up (see HOLODEX-272 session log,
2026-08-11) with no intended behavior change. `/code-review xhigh --fix` on the initial
implementation found that framing was wrong: reusing the People link set introduced two
real correctness changes (see 2026-08-13 session log below) — one fixed in this same
push, one left open as a follow-up. Status reflects that: this is now a correctness
change, not a pure perf one.

## Gates — definition of done

- [x] spec — not needed (endpoint contract, request/response shapes, and collision
      semantics are unchanged; the correctness fixes below are internal to the relink
      mechanism, not a contract change)
- [x] architecture — not needed (no new ADR; reuses ADR-072's `RelinkVideoPeople`/
      `ReconcileVideoPeople` contract, generalizing HOLODEX-271's precedent) — **revisited**:
      the concurrency follow-up below did turn into a `SetCurationChecked` contract change,
      tracked as [ADR-084](../architecture/ADR-084-locked-curation-relink-commit.md) under
      HOLODEX-277
- [x] design — not needed (backend-only; no frontend/UX surface touched)
- [x] backend — `internal/api/curation.go`'s `proposedPeopleNames` → `proposedPeopleLinks`,
      now returning the full `[]repo.PersonRoleName` link set (both roles) instead of a flat
      name list, computed once inside the `check()` closure `SetCurationChecked` runs under
      `writeMu` (so it stays valid even under `Override`, which only skips the
      `FindPeopleCollision` query); `internal/api/person_links.go`'s new
      `relinkPeopleWithContext` commits that link set straight to `ReconcileVideoPeople`
      (one `EnrichmentForEntity` fetch for `extIDByName`, no `loadRelinkContext`/`resolver.Resolve`);
      gated on `personFieldMapped` so an unconfigured person field never reaches this path
- [x] testing — `TestCurationAPI_PeopleCollision`, `TestCurationAPI_PeopleCollision_Suppress`,
      and `TestCurationAPI_NonPersonFieldSkipsCollisionGate` cover the block/override/relink
      flow unmodified; added `TestCurationAPI_PersonFieldNotMapped` to cover the
      config-guard fix specifically (an unmapped person field must not reach
      `relinkPeopleWithContext`/write `video_people`)
- [x] security — not needed (no new query shape, no privilege-boundary change; same
      owner-gated endpoint, same parameterized queries)

## Up next — ordered (position = priority)

1. **Concurrency follow-up — filed as [HOLODEX-277](https://whoiskevinrich.atlassian.net/browse/HOLODEX-277), needs design:** `proposedPeopleLinks`
   builds the People fast-path write from the current `video_people` table snapshot
   rather than a fresh resolve, so it lost the self-healing convergence the old
   resolver-based relink had. Two concurrent curation edits to *different*
   person-typed fields (e.g. an actors add and a director add on the same video,
   near-simultaneous) can race: whichever `relinkPeopleWithContext` call's
   `ReconcileVideoPeople` full-replace lands second silently drops the other's link,
   since each request's link snapshot was captured before either request's write ran.
   Found and confirmed via `/code-review xhigh` (2026-08-13); not fixed here because a
   correct fix means either re-resolving from source inside the locked `check()`
   closure (partially reintroducing the I/O this ticket exists to remove) or extending
   `SetCurationChecked`'s contract so the relink write itself happens inside the same
   `writeMu` critical section as the curation write (a shared Title/Studio/People
   mechanism change) — both are design decisions past a review-fix's scope.
2. (secondary, lower priority) `proposedPeopleLinks`' suppress match (role +
   case-folded name) now feeds the actual `video_people` write, not just the advisory
   collision-check name list — widens the blast radius of the pre-existing
   name-based (not person-ID-based) match if two Person rows ever share a name+role
   on one video. Same review pass, same reason for not fixing inline (pre-existing
   matching logic, broader entity-identity design question).

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-13 · implementation, /simplify pass, all gates green
- skills: simplify
- handoff: Implemented the fix: `proposedPeopleLinks` (renamed from `proposedPeopleNames`)
  now returns role-tagged `[]repo.PersonRoleName` built from the same locked
  `PeopleForVideos` read the collision check already did, and `setCuration` reuses that
  link set via the new `relinkPeopleWithContext` (person_links.go) instead of calling
  `relinkIfEntity` → `RelinkVideoPeople` → a fresh `loadRelinkContext` + `resolver.Resolve`.
  Preserved HOLODEX-272's locked-check invariant (unlike Studio's accepted unlocked
  precedent): `check()` now always runs (even under `Override`, which only skips the
  `FindPeopleCollision` query itself) so `links` is populated from the same writeMu-locked
  snapshot the write commits against. `/simplify` (4 parallel review agents) flagged one
  real simplification — `proposedPeopleLinks` was building two parallel slices
  (`links`/`names`) in lockstep even though `names` is fully derivable from `links` —
  fixed by returning only `links` and deriving `names` at the single call site right
  before `FindPeopleCollision`. Two other findings (relinkPeopleWithContext's signature
  diverging from `relinkStudiosWithContext`'s `rc *relinkContext` shape; the closure
  side-effect relying on `SetCurationChecked` calling `check()` exactly once) were
  reviewed and left as-is — both are the deliberate, documented tradeoffs the ticket
  itself calls out (accepting Studio's `relinkContext` shape here would reintroduce the
  4-query fetch this change exists to eliminate). `go build ./...`, `go vet ./internal/api/...`,
  and `go test ./internal/...` (full suite) all clean; `TestCurationAPI_PeopleCollision{,_Suppress}`
  and `TestCurationAPI_NonPersonFieldSkipsCollisionGate` pass unmodified. Backend-only, no
  frontend touched. Next: commit/push/PR/Jira sync.

### 2026-08-13 · /code-review xhigh --fix, two correctness bugs found and one fixed
- skills: code-review
- handoff: Ran the full xhigh review (10 finder angles, verify pass, gap sweep) on
  the implementation above — it found the "not a correctness fix" framing was wrong.
  **Fixed:** `relinkPeopleWithContext` had no guard against an unconfigured/unmapped
  person field, unlike `relinkVideoPeople`'s `h.mappings`/`len(fields)==0` guard
  (the exact HOLODEX-256 regression class) — `isPeopleField` (curation.go) now also
  requires the new `h.personFieldMapped(body.Field)` (person_links.go), so an
  unmapped field falls back to the guarded `relinkIfEntity` path instead of reaching
  `relinkPeopleWithContext` at all; added `TestCurationAPI_PersonFieldNotMapped` to
  cover it. A `/simplify` pass on the fix itself (run per the pre-commit checklist)
  flagged that this only guards at the call site — `relinkPeopleWithContext` itself
  still trusted its caller, unlike `relinkVideoPeople`'s self-contained guard — so
  added `anyPersonFieldMapped` as a second, self-contained no-op check inside
  `relinkPeopleWithContext` too, so it stays safe to call regardless of caller
  discipline (defense-in-depth against the same HOLODEX-256 class). Also fixed a
  minor log-message ambiguity (relinkPeopleWithContext's two failure branches now
  log distinct messages). **Found, not fixed, tracked in Up
  next:** a concurrency data-loss bug (concurrent edits to different person-typed
  fields can race and silently drop a link) and a suppress name-match blast-radius
  widening — both are real but need a design decision beyond a review-fix's scope,
  see Up next above. `go build ./...`, `go vet ./internal/api/...`, and
  `go test ./internal/...` all clean. Next: commit/push/PR/Jira sync, and file a
  follow-up HOLODEX ticket for the concurrency issue before closing this one out.
