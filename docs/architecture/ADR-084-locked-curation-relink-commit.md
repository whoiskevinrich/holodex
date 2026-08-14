# ADR-084: Locked curation-relink commit — extending `SetCurationChecked`'s `writeMu` to cover the People relink write

**Status:** Proposed
**Date:** 2026-08-14
**Deciders:** Kevin Rich

## Context

HOLODEX-274 (PR #240, merged) gave `setCuration`'s person-typed-field (`actors`/`director`)
add/suppress path a fast relink route: `proposedPeopleLinks`
([internal/api/curation.go:151](../../internal/api/curation.go)) computes the video's resulting
`[]repo.PersonRoleName` link set by reading the already-materialized `video_people` table
(`PeopleForVideos`) and patching in the one field/value delta being edited, and
`relinkPeopleWithContext` ([internal/api/person_links.go:54](../../internal/api/person_links.go))
commits that set straight to `ReconcileVideoPeople`
([internal/repo/person_links.go:44](../../internal/repo/person_links.go)) — skipping the old
path's `loadRelinkContext` (4 queries) + `resolver.Resolve` pass.

That fast path is faster, but it lost a property the old path had for free:
**self-healing convergence**. `ReconcileVideoPeople` is a full-replace, not a merge
(inserts `desired\current`, deletes `current\desired`, [person_links.go:86-108](../../internal/repo/person_links.go)).
The old path re-resolved from source (curation/decision/enrichment rows) at relink time, so
whichever of two concurrent relinks ran last always converged on the correct union. The new
fast path's link snapshot is instead captured from `PeopleForVideos` **before** either
concurrent request's own relink write has landed.

Concretely: request A adds actor "Alice" and request B adds director "Bob" on the same video,
near-simultaneously.

1. A's `check()` (under `writeMu`, via `SetCurationChecked`) reads `video_people` (empty),
   computes `links_A = [{Alice, actor}]`, commits its curation row, lock released.
2. B's `check()` runs before A's relink executes — reads `video_people` (still empty),
   computes `links_B = [{Bob, director}]`, commits its curation row, lock released.
3. A's `relinkPeopleWithContext` runs (unlocked, after `SetCurationChecked` already
   returned): `ReconcileVideoPeople` full-replaces `video_people = {Alice/actor}`.
4. B's `relinkPeopleWithContext` runs with its *stale* `links_B` (computed before step 3):
   full-replaces `video_people = {Bob/director}` — **silently dropping Alice**.

The root cause isn't that `proposedPeopleLinks` reads a snapshot instead of resolving fresh —
it's that the snapshot-read-and-write cycle spans **two separate lock acquisitions**
(`SetCurationChecked`'s and `ReconcileVideoPeople`'s own), with an unlocked gap between them
where another request's full read-modify-write cycle can interleave. `check()` already runs
under `writeMu` precisely to prevent this class of race for the *collision check*
(HOLODEX-272); the relink write was left outside that guarantee.

This is scoped to People specifically: HOLODEX-272 requires People's collision check to be
lock-consistent (unlike Studio's `resolveProposedStudioNames`, which the codebase already
accepts as an unlocked-then-relocked TOCTOU-light check — see
[internal/api/decisions.go:181-192](../../internal/api/decisions.go)). The race window itself
requires two concurrent curation edits to *different* person-typed fields on the *same* video —
narrow in practice for an owner-gated admin endpoint, but the failure mode is silent data loss,
not a visible error, so it's worth closing deliberately (HOLODEX-277).

## Decision

Extend `SetCurationChecked`'s contract with an optional `commit func(WriteLock)` callback that
runs — under the same `writeMu` lock, immediately after the curation row write succeeds — so
the People relink write becomes part of the same locked critical section as the check and the
curation write, instead of a separate, unlocked step run afterward. This closes the race
without reintroducing the resolve-from-source I/O HOLODEX-274 removed.

```go
// internal/repo/curation.go
func (r *Repo) SetCurationChecked(
    ctx context.Context, entityType string, entityID int64, fieldKey, value, action string,
    check func(WriteLock) (*VideoCollision, error),
    commit func(WriteLock),
) (*VideoCollision, error) {
    r.writeMu.Lock()
    defer r.writeMu.Unlock()
    var lock WriteLock
    if check != nil {
        if collision, err := check(lock); err != nil || collision != nil {
            return collision, err
        }
    }
    if err := r.setCurationLocked(ctx, entityType, entityID, fieldKey, value, action); err != nil {
        return nil, err
    }
    if commit != nil {
        commit(lock)
    }
    return nil, nil
}
```

`commit` takes no error return and swallows its own failures (logs via the caller's closure),
matching `relinkPeopleWithContext`'s existing best-effort semantics — a relink failure has
never failed the user's curation write, and this doesn't change that. What changes is *when*
the relink runs: inside the same lock, so no other request's check-write-relink cycle can
interleave with it, regardless of whether the relink itself succeeds.

`ReconcileVideoPeople`'s existing body (already lock-and-transact,
[person_links.go:44-134](../../internal/repo/person_links.go)) splits into a private
locked core and the current public method becomes a thin locking wrapper around it:

```go
// ReconcileVideoPeopleLocked is ReconcileVideoPeople's implementation for a caller that
// already holds writeMu — obtainable only from inside a SetCurationChecked check/commit
// callback (see WriteLock). Do not call this without holding writeMu: it performs the
// same full-replace write ReconcileVideoPeople does, with no locking of its own.
func (r *Repo) ReconcileVideoPeopleLocked(ctx context.Context, _ WriteLock, videoID int64, links []PersonRoleName, extIDByName map[string]string) error {
    // ...current ReconcileVideoPeople body, minus the writeMu.Lock()/Unlock()...
}

func (r *Repo) ReconcileVideoPeople(ctx context.Context, videoID int64, links []PersonRoleName, extIDByName map[string]string) error {
    r.writeMu.Lock()
    defer r.writeMu.Unlock()
    var lock WriteLock
    return r.ReconcileVideoPeopleLocked(ctx, lock, videoID, links, extIDByName)
}
```

`WriteLock` is an empty struct with an unexported field, constructible only inside `internal/repo`:

```go
// WriteLock is proof the caller is executing inside a writeMu-locked repo callback
// (SetCurationChecked's check/commit). Only repo can construct one, so a Locked method
// that requires it cannot compile-time be called outside that lock.
type WriteLock struct{ _ [0]byte }
```

`internal/api/curation.go`'s `setCuration` then builds a `commit` closure alongside the
existing `check` closure, moving the `relinkPeopleWithContext` call from after
`SetCurationChecked` returns to inside `commit`:

```go
var commit func(repo.WriteLock)
if isPeopleField {
    commit = func(lock repo.WriteLock) {
        enrRows, err := h.repo.EnrichmentForEntity(r.Context(), model.EnrichEntityVideo, id)
        if err != nil {
            h.log.Warn("relink people: enrichment fetch", "video", id, "err", err)
            return
        }
        if err := h.repo.ReconcileVideoPeopleLocked(r.Context(), lock, id, links, personExternalIDsFromRows(enrRows)); err != nil {
            h.log.Warn("relink people: reconcile", "video", id, "err", err)
        }
    }
}
collision, err := h.repo.SetCurationChecked(r.Context(), model.EnrichEntityVideo, id, body.Field, value, body.Action, check, commit)
```

`links` is populated as a side effect of `check()`, which always runs before `commit()` within
the same lock — the same "populated once, read later in the same critical section" shape the
closure already relies on today (see the existing comment at
[curation.go:95-102](../../internal/api/curation.go)).

## Options Considered

### Option A: Re-resolve from source inside `check()` — rejected

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low — `proposedPeopleLinks` swaps `PeopleForVideos` for `loadRelinkContext` + `resolver.Resolve`, mirroring `relinkVideoPeople` ([person_links.go:196-216](../../internal/repo/person_links.go)) |
| Blast radius | Contained to People's `check()` closure only |
| Cost | Re-adds the 4-query fetch + resolve pass HOLODEX-274 exists to remove; extends `writeMu` hold time by that same amount |
| Correctness | Closes the race — `check()` already runs under the lock, so a fresh resolve there sees a fully-consistent snapshot |

**Rejected because:** it reverses HOLODEX-274's stated purpose for every People curation edit,
not just the racing ones, to fix a bug that only manifests under a narrow concurrent-edit
window. Option B buys the same correctness without paying that cost on every edit.

### Option B: Extend the lock to cover the relink write — chosen

| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium — new `WriteLock` token type, `SetCurationChecked` gains a `commit` parameter, `ReconcileVideoPeople` splits into locked-core + locking-wrapper |
| Blast radius | `internal/repo/curation.go`, `internal/repo/person_links.go`, `internal/api/curation.go`, `internal/api/person_links.go` — contained to the People path; `SetDecisionChecked` (Title/Studio) is untouched (see Non-goals) |
| Cost | No added I/O over what HOLODEX-274 already pays; `writeMu` hold time grows only by the relink write's own transaction, which was going to happen either way |
| Correctness | Closes the race by serializing each request's full check→write→relink cycle against every other request's, independent of whether any individual relink succeeds |

**Chosen because:** People-editing is an owner-gated, low-traffic admin path — the
architecturally cleaner fix costs nothing here that the narrower Option A patch would have
saved in practice, and it establishes a reusable `commit`-under-lock shape if a future
field ever needs the same guarantee.

### Sub-decision: how `commit` reaches the locked write across the `api`/`repo` package boundary

- **B1 — exported `XxxLocked` method + doc-comment discipline (chosen):** matches the
  convention this codebase already uses for `setCurationLocked`/`setDecisionLocked` — a
  `Locked`-suffixed method whose safety is a documented precondition, not a compiler
  guarantee, on the "trust the doc comment" model this codebase already relies on elsewhere
  (`relinkVideoPeople`'s optional `rc`, `anyPersonFieldMapped`'s defense-in-depth checks).
- **B2 — unforgeable `WriteLock` capability token (chosen, refines B1):** a zero-size struct
  with an unexported field, constructible only inside `internal/repo`, threaded through
  `check`/`commit` and required by `ReconcileVideoPeopleLocked`'s signature. This is strictly
  additive to B1 — it costs one small type and turns "don't call this without the lock" from a
  comment into something the compiler enforces at the one place that matters (a call from
  outside `internal/repo`, where the existing unexported-name convention can't reach). Adopted
  because the failure mode (silent data loss) is exactly the kind of mistake a doc comment
  alone has already failed to prevent once (this ADR *is* that mistake, for the read-write
  cycle rather than the lock itself).

## Trade-off Analysis

The real choice was never "resolve fresh" vs. "lock harder" in the abstract — both close the
race. It was whether to pay the resolve cost on *every* People curation edit (Option A) or pay
a small, one-time API-surface cost to make the *existing* relink write lock-compatible (Option
B). Given this is an admin-only path with no throughput requirement, Option B's slightly larger
diff is worth it for not undoing HOLODEX-274. The `WriteLock` token is the one piece of new
machinery in this ADR; it's justified specifically because the boundary it guards is a
cross-package one where the codebase's usual doc-comment discipline has less bite.

## Consequences

- People's curation add/suppress + relink becomes a single atomic-with-respect-to-other-writers
  operation; the HOLODEX-277 race is closed.
- `commit` failures stay non-fatal (logged only) — unchanged from today's best-effort relink
  semantics; a relink failure still never fails the owner's curation write.
- `writeMu` hold time for a People curation edit grows by `ReconcileVideoPeopleLocked`'s own
  transaction — the same work `relinkPeopleWithContext` already did, just moved inside the lock
  instead of immediately after it. No new queries.
- `SetDecisionChecked` (Title/Studio) is untouched — see Non-goals. A future tightening of
  Studio's TOCTOU-light check could reuse the identical `commit`/`WriteLock` shape, but nothing
  here requires or assumes that.
- Composing the curation-row write and the relink write under one mutex is **not** the same as
  wrapping them in one SQL transaction — they remain two separate `db.Exec`/transaction calls.
  The lock guarantees *no other writer interleaves* between them, not that they succeed or fail
  together. If `ReconcileVideoPeopleLocked` fails after `setCurationLocked` already committed,
  `video_people` can still drift from `metadata_curation` — the same partial-failure exposure
  `relinkPeopleWithContext` already has today, unchanged by this ADR. Full write-write atomicity
  (one transaction spanning both tables) is out of scope here; nothing observed in HOLODEX-277
  requires it, since the race is about interleaving between requests, not about one request's
  write partially failing.

## Non-goals

- **Extending `SetDecisionChecked` (Title/Studio) with the same `commit` parameter.** Studio's
  `resolveProposedStudioNames` accepts an unlocked-then-relocked check by existing design
  (referenced in Context); nothing in HOLODEX-277 requires changing that. Signature parity is a
  possible future convenience, not a requirement of this fix.
- **The secondary finding from HOLODEX-274's review** — `proposedPeopleLinks`' suppress match
  (role + case-folded name) now feeding the real `video_people` write instead of just the
  advisory `FindPeopleCollision` list — is an entity-identity question (name-based, not
  person-ID-based, matching), independent of this ADR's locking fix. Tracked separately; see
  Action Items.
- **Wrapping the curation write and the relink write in one SQL transaction.** See Consequences
  — not required to close the race this ADR addresses.

## Action Items

1. [ ] Implement `WriteLock`, `SetCurationChecked`'s `commit` parameter, and
   `ReconcileVideoPeopleLocked` in `internal/repo`.
2. [ ] Move `relinkPeopleWithContext`'s call from `setCuration` (after `SetCurationChecked`
   returns) into a `commit` closure passed alongside `check`.
3. [ ] Regression test: two concurrent `setCuration` calls to different person-typed fields
   (`actors` add + `director` add) on the same video both survive in `video_people`.
4. [ ] File a follow-up HOLODEX ticket for the suppress-match blast-radius finding (Non-goals),
   referencing this ADR and HOLODEX-274's original review.
5. [ ] `/testing-strategy` pass per the change-routing table (multi-file backend behavior
   change).
6. [ ] `/security-review` — no new query shape or privilege boundary, but the locking change
   touches the owner-gated write path; confirm no regression.
