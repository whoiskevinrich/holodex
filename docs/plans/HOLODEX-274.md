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

Not a correctness fix — the prior path was already correct and tested; this is a
`/simplify`-flagged follow-up (see HOLODEX-272 session log, 2026-08-11) purely reducing
avoidable I/O on a low-frequency (user-initiated) admin path.

## Gates — definition of done

- [x] spec — not needed (no functional/behavior change; existing endpoint contract,
      request/response shapes, and collision semantics are unchanged)
- [x] architecture — not needed (no new ADR; reuses ADR-072's `RelinkVideoPeople`/
      `ReconcileVideoPeople` contract, generalizing HOLODEX-271's precedent)
- [x] design — not needed (backend-only; no frontend/UX surface touched)
- [x] backend — `internal/api/curation.go`'s `proposedPeopleNames` → `proposedPeopleLinks`,
      now returning the full `[]repo.PersonRoleName` link set (both roles) instead of a flat
      name list, computed once inside the `check()` closure `SetCurationChecked` runs under
      `writeMu` (so it stays valid even under `Override`, which only skips the
      `FindPeopleCollision` query); `internal/api/person_links.go`'s new
      `relinkPeopleWithContext` commits that link set straight to `ReconcileVideoPeople`
      (one `EnrichmentForEntity` fetch for `extIDByName`, no `loadRelinkContext`/`resolver.Resolve`)
- [x] testing — no new tests needed; `TestCurationAPI_PeopleCollision`,
      `TestCurationAPI_PeopleCollision_Suppress`, and
      `TestCurationAPI_NonPersonFieldSkipsCollisionGate` already exercise both the
      block-and-not-persist path and the override-then-relink path end-to-end via
      `PeopleForVideos`, and pass unmodified
- [x] security — not needed (no new query shape, no privilege-boundary change; same
      owner-gated endpoint, same parameterized queries)

## Up next — ordered (position = priority)

(none — this was the last flagged follow-up from HOLODEX-272's `/simplify` pass)

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
