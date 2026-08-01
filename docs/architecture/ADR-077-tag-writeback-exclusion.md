# ADR-077: Tag writeback exclusion — per-tag Genre writeback flag + manual sync batch seam

**Status:** Proposed
**Date:** 2026-07-31
**Deciders:** Project owner

**Extends:** [ADR-075](ADR-075-tag-governance-and-video-enrichment.md) (`TagNamesForVideo`'s ancestor-expansion
query, `denied_tags`'s forward-only precedent — this ADR's D1 both reuses the former and contrasts with the
latter) · [ADR-048](ADR-048-metadata-curation-and-write-queue.md) (`writequeue.EnqueueMany`, shared-`batchID`
grouping — the batch-enqueue mechanism D2 reuses) · [ADR-067](ADR-067-filename-extraction-confidence-and-rollback.md)
(`propagateMerge`, the existing precedent for "one owner action batch-enqueues writeback across many videos" —
D2's closest structural relative) · [ADR-073](ADR-073-post-write-baseline-resync.md) (`GET
/writeback/jobs/{id}`, single-job status — D3 is its batch-scoped sibling). **Relates to:**
[ADR-041](ADR-041-metadata-writeback.md) (copy→write→rename file-safety model, unchanged by this ADR) ·
[ADR-013](ADR-013-metadata-field-mapping.md) (the `genres` canonical field this flag ultimately gates).
**Spec:** [tag-writeback-exclusion.md](../specs/tag-writeback-exclusion.md) (HOLODEX-239).

---

## Context

Tags already reach a video file's `Genre` tag on writeback: `genreWritebackValuesForVideo`
(`internal/api/genre_writeback.go`) unions a video's attached-tag names — ancestor-expanded via
`TagNamesForVideo` (`internal/repo/tag_hierarchy.go`) — with the raw resolved `genres` field, deny-list-filtering
only the raw side. The only owner lever over what reaches a file today is the global deny-list
(`denied_tags`, ADR-075 D2), which is forward-only and all-or-nothing: it blocks a *term* from ever becoming a
tag anywhere in the app. HOLODEX-239 asks for something narrower — a tag stays a normal, searchable, attachable
Holodex tag, but is excluded from the one specific output (`genres` writeback) it would otherwise feed.

### Current state (survey, 2026-07-31)

| Seam | Today | File |
|---|---|---|
| `tags` | `id, name, parent_tag_id` (ADR-075 D1) — no writeback-participation column | `internal/db/migrations/0032_tag_hierarchy.up.sql` |
| `genreWritebackValuesForVideo` | Unions `TagNamesForVideo`'s ancestor-expanded names (unfiltered) with the deny-filtered raw `genres` union | `internal/api/genre_writeback.go:39` |
| `TagNamesForVideo` | `WITH RECURSIVE` walk **up** `parent_tag_id` from a video's attached tags to the root, returning every name reached | `internal/repo/tag_hierarchy.go:104` |
| Batch writeback enqueue | `propagateMerge` (merge-triggered) builds one `writequeue.BatchJob` per affected video and calls `EnqueueMany` under one shared `batchID`, from a **precomputed static name list** (`namesByVideo`) | `internal/api/merge_writeback.go` |
| Batch job status | `GET /writeback/jobs/{id}` reports **one** queued job's state; no endpoint aggregates status across a shared `batchID` | `internal/api/writeback.go:45` |
| Batch revert | `POST /writeback/batches/{batchID}/revert` already exists, keyed purely on `batchID` — batch-scoped, not video-scoped | `internal/api/writeback.go:40` |
| Owner-gated tag mutation pattern | `mountTagDenylist`/`mountTagHierarchy`-style: small handler file, routes mounted in the shared `requireOwner` group, `?query`/`{id}` path shape | `internal/api/tag_denylist.go`, `internal/api/tag_hierarchy.go` |

### Forces

- **The flag must change one output, not the tag itself.** Attachment, search, browse-filter, and hierarchy
  membership must all be unaffected (spec P0 requirement) — ruling out anything that reuses the deny-list's
  enforcement point (`resolveOrCreateByName`), which exists specifically to stop a `tags` row from being
  created at all. This is the opposite lifecycle: the row already exists and stays exactly as useful as before.
- **Ancestor expansion raises a question the spec doesn't answer explicitly: does a name's own flag decide its
  fate, or does a disabled ancestor suppress everything below it?** `TagNamesForVideo` returns both a video's
  directly-attached tags and every ancestor reached by walking `parent_tag_id` upward. The spec's P0
  requirement — "a tag with the flag off contributes nothing to `GenreWritebackValues`, for any video" — reads
  naturally as a property of each *name in the output*, not of the walk that reached it. Deciding otherwise
  (e.g., a disabled tag also blocks every ancestor beyond it) would require the walk itself to carry
  per-hop flag state and is not asked for anywhere in the spec's requirements or non-goals.
- **Genre writeback is a computed union, not a stored name list — so the manual sync trigger can't reuse
  `propagateMerge`'s pattern unmodified.** `propagateMerge` batch-writes a *precomputed* `namesByVideo` map
  because a merge's affected-video name list is static once the merge transaction commits. `genres` is
  recomputed per video from tag attachments + the deny-list + (after this ADR) the writeback flag itself — the
  same function that would run automatically has to run explicitly at sync time, per video, not be assembled
  once ahead of the batch.
- **A batch can be large (the spec's own success metric: 50+ videos) and the existing job-status surface is
  single-job-scoped.** The P0 acceptance criterion ("shows progress/completion via the extended dialog")
  needs *some* aggregate signal; polling N individual `/writeback/jobs/{id}` calls from the SPA doesn't scale
  cleanly and duplicates work the queue can already answer in one query.
- **Bulk (multi-tag) sync must not double-enqueue a video attached to more than one selected tag.** The spec's
  bulk requirements treat "sync selected tags" as one action over the union of their videos, not N independent
  single-tag syncs that could each separately enqueue the same video.

---

## Decision

### D1 — `tags.writeback_enabled` column; filtered at `TagNamesForVideo`, uniformly per name regardless of how it was reached

```sql
ALTER TABLE tags ADD COLUMN writeback_enabled INTEGER NOT NULL DEFAULT 1;
```

Plain nullable-free boolean column, matching the codebase's existing convention (`videos.active`,
`0001_init.up.sql:15`) rather than a `CHECK` constraint SQLite doesn't need enforced at the schema layer here.
Default `1` (included) means the migration is a no-op for every existing tag's writeback behavior — required by
the spec's own P0 acceptance criterion ("a tag with the flag on behaves identically to current behavior") and
its open question ("confirm the flag defaults `true`... no silent behavior change on deploy").

**Enforcement point: `TagNamesForVideo`'s final `SELECT`, not `genreWritebackValuesForVideo`'s post-hoc
filtering (the deny-list's pattern) and not the ancestor-walk's recursive step.**

```sql
-- videoTagAncestorsQuery, amended:
WITH RECURSIVE tag_ancestors(id) AS (
    SELECT tag_id FROM video_tags WHERE video_id = ?
    UNION
    SELECT t.parent_tag_id FROM tags t
    JOIN tag_ancestors a ON t.id = a.id
    WHERE t.parent_tag_id IS NOT NULL
)
SELECT DISTINCT t.name FROM tags t JOIN tag_ancestors a ON t.id = a.id
WHERE t.writeback_enabled = 1
```

The recursive CTE's own walk is **unchanged** — it keeps climbing through a disabled tag to reach names above
it, exactly as it does today. Only the final projection drops a disabled tag's own name from the result. Per
the Forces discussion: a name's presence in the output is a property of that name's own flag, not of the path
that reached it — so disabling "Dog" removes "Dog" from a video's writeback set but does not also suppress
"Animal" (a further ancestor) or "German Shepherd" (a descendant, reached independently as a directly-attached
tag with its own flag). This is the simplest rule that satisfies "contributes nothing... for any video" without
inventing per-hop state the spec never asked for, and it composes cleanly with D1's own non-goal: the flag
affects only this one query's output, nothing about hierarchy membership or expansion itself.

`genreWritebackValuesForVideo` (`internal/api/genre_writeback.go`) needs **no change** — it already treats
`TagNamesForVideo`'s return value as "the tag names contributing to this video's writeback," so filtering
inside that function is sufficient and keeps the deny-list's separate, unrelated filter (applied only to the
raw `genres` side, per ADR-075 RD9) untouched.

### D2 — Manual sync batch-enqueues per-video via `genreWritebackValuesForVideo`, not a precomputed name list; shared `batchID` across single- and bulk-tag triggers

A new function, `syncTagWriteback(ctx, videoIDs []int64, batchID string)`, is `propagateMerge`'s structural
sibling with one necessary difference: where `propagateMerge` receives an already-built
`map[int64][]string` and batch-enqueues it directly, `syncTagWriteback` must call
`genreWritebackValuesForVideo` **per video**, because the value being written (the full `genres` union for
that video, not just the tag(s) being synced) is exactly what D1 changed the computation of. Enqueuing a
static "this tag's name" per video would be wrong twice over: it would drop the video's *other* contributing
tags and the raw-genres side entirely, silently narrowing the file's `Genre` tag instead of syncing it to the
owner's actual current decision.

```go
// internal/api/tag_writeback_sync.go (new file, sibling of merge_writeback.go)
func (h *Handlers) syncTagWriteback(ctx context.Context, videoIDs []int64, batchID string) (enqueued int, err error) {
    jobs := make([]writequeue.BatchJob, 0, len(videoIDs))
    for _, id := range videoIDs {
        values, err := h.GenreWritebackValues(ctx, id)
        if err != nil {
            return 0, err // aborts before any enqueue -- see below
        }
        if len(values) == 0 {
            continue // nothing to write for this video; matches propagateMerge's own skip
        }
        jobs = append(jobs, writequeue.BatchJob{
            VideoID: id,
            Fields:  []writequeue.JobField{{Field: "genres", Values: values, Source: fieldsource.Manual}},
        })
    }
    if len(jobs) == 0 {
        return 0, nil
    }
    ids, err := h.writeQueue.EnqueueMany(ctx, jobs, batchID)
    return len(ids), err
}
```

Per-video reads (`h.GenreWritebackValues`, itself a `GetVideo` + `TagNamesForVideo` + resolved-field call) run
before any enqueue — unlike `propagateMerge`, which tolerates a partial batch because the merge it's
propagating already committed, a sync trigger has committed nothing yet, so a read failure partway through
aborts the whole batch rather than silently enqueuing a partial one. This trades `propagateMerge`'s
best-effort posture for a stricter one, justified because sync is an explicit owner-initiated batch with
nothing else already committed to reconcile against.

**Endpoints** (owner-gated, mounted alongside the existing tag-mutation handlers per the
`tag_denylist.go`/`tag_hierarchy.go` pattern):

| Method + path | Body | Behavior |
|---|---|---|
| `PATCH /tags/{id}/writeback` | `{"enabled": bool}` | Sets one tag's flag. Never enqueues (spec P0: "changing the flag alone never enqueues a write"). |
| `POST /tags/{id}/writeback/sync` | — | Loads video IDs currently carrying tag `{id}` (new `Repo.VideoIDsForTag`, a plain `SELECT video_id FROM video_tags JOIN videos ... WHERE tag_id = ?`, active/non-deleted only — mirrors the existing `namedCountQuery` join predicate), then `syncTagWriteback` under a fresh `batchID`. |
| `PATCH /tags/writeback` | `{"tag_ids": [...], "enabled": bool}` | Bulk flag-set — one `UPDATE tags SET writeback_enabled = ? WHERE id IN (...)`, per-tag state ignored on the way in (spec P0: "applies... regardless of individual prior state"). |
| `POST /tags/writeback/sync` | `{"tag_ids": [...]}` | Bulk sync — loads the **union** of video IDs across every listed tag (`SELECT DISTINCT video_id FROM video_tags WHERE tag_id IN (...)` joined the same way), so a video attached to two selected tags is enqueued once, then `syncTagWriteback` under one shared `batchID`. |

`batchID` generation has no `mergeBatchID`-style deterministic derivation available: a merge only ever happens
once per surviving/losing pair, but a sync trigger is explicitly repeatable (the owner can sync the same tag
today and again next week), so each trigger mints a fresh id —
`fmt.Sprintf("tag-writeback-sync-%d", time.Now().UnixNano())` for the single-tag path, same shape for bulk.
This is a plain unique-string requirement, not an idempotency key: re-triggering a sync is expected to
re-enqueue, not be deduplicated against a prior run.

### D3 — `GET /writeback/batches/{batchID}/status`: a new batch-scoped status endpoint, aggregating `writeback_queue` (pending/running) and `job_runs` (done) by `batch_id`

```go
// internal/repo — sibling of GetWritebackJobStatus
func (r *Repo) GetWritebackBatchStatus(ctx context.Context, batchID string) (pending, running, done, failed int, err error)
```

`pending`/`running` come from `COUNT(*) ... FROM writeback_queue WHERE batch_id = ? GROUP BY status` (the
queue's existing `status` column, already populated per job); `done` and `failed` come from `job_runs WHERE
batch_id = ?` (already carrying `batch_id` since ADR-071's job-run attribution), counted by outcome. A row's
absence from `writeback_queue` combined with a `job_runs` hit is exactly `writebackJobStatus`'s own "row is
gone = done" rule, applied across every job sharing the batch instead of one job id — no new columns, no new
table, both source columns already exist for an unrelated reason (ADR-071, ADR-073) and this is their first
cross-referenced read.

`GET /writeback/jobs/{id}` is untouched and stays the shape the existing single-video `WritebackFormDialog`
flow (F28/F41) uses; the new batch endpoint is additive, mounted next to it
(`internal/api/writeback.go`'s `mountWriteback`), and exists specifically so a tag-scoped sync's dialog instance
polls one endpoint instead of fanning out to N.

---

## Options Considered

### D1 — where the flag is enforced

**A — Filter inside `TagNamesForVideo`'s final `SELECT` (chosen).** Pros: one query, one place, the function's
contract ("this video's writeback-contributing tag names") stays exactly as documented, every caller (today
just `genreWritebackValuesForVideo`) gets correct behavior for free. Cons: none identified.

**B — Filter in `genreWritebackValuesForVideo`, after `TagNamesForVideo` returns (the deny-list's own
pattern).** Pros: mirrors how the deny-list filters the raw-genres side, so a reader familiar with that code
recognizes the shape immediately. Cons: requires `TagNamesForVideo` to also return each name's flag (a second
column) purely so the caller can re-filter what the query could have filtered itself; strictly more plumbing
for the identical result. Rejected — no reader benefit outweighs the extra column-threading.

**C — A disabled ancestor suppresses every name beyond it in the walk.** Pros: arguably a stronger "excluded
means excluded" semantic if an owner mentally models the hierarchy as inheriting exclusion. Cons: not asked for
anywhere in the spec (P0 requirements and non-goals are both silent on hierarchy interaction); requires the
recursive CTE to track and propagate flag state per hop rather than a flat `WHERE` on the final projection —
real added complexity for a behavior nobody has asked for. Rejected as speculative; revisit only if an owner
actually wants inherited exclusion once the flat behavior ships.

### D2 — sync's value source

**A — Recompute `genreWritebackValuesForVideo` per video (chosen).** Pros: correct by construction — always
reflects the video's full current tag set, deny-list, and (per D1) writeback-flag state, the same function
automatic future writeback would use. Cons: one read per video instead of zero extra reads (the value was
never precomputed to begin with, unlike a merge's post-transaction name list). Accepted — there was never a
cheaper correct option; `propagateMerge`'s reuse of a precomputed map is only cheap because merge's own
transaction produces one as a side effect, which sync's flag-toggle does not.

**B — Enqueue only the toggled tag's own name(s), relying on the file already carrying every other genre from
prior writes.** Pros: no per-video read. Cons: this is not a sync in any meaningful sense, it is an append; it
cannot ever *remove* a genre that used to be written but is now excluded, defeating the feature's entire
purpose ("noise measurably drops... pre/post" is the spec's own success metric). Rejected outright.

### D3 — batch progress signal

**A — New `GET /writeback/batches/{batchID}/status`, aggregating existing columns (chosen).** Pros: additive
(no schema change — `writeback_queue.batch_id` and `job_runs.batch_id` both already exist), symmetric with the
already-existing `POST /writeback/batches/{batchID}/revert`, one poll per batch regardless of size. Cons: none
identified.

**B — SPA polls `GET /writeback/jobs/{id}` for every job id the enqueue response returned.** Pros: zero new
backend surface. Cons: N requests per poll tick for an N-video batch (the spec's own 50+-video success metric
makes this concretely bad), and the dialog would need to hold and reconcile N independent statuses itself
instead of reading one aggregate. Rejected — the cost scales with exactly the dimension (batch size) the
feature is designed to make routine.

**C — Reuse the existing `.activity-dot` background-activity mechanism (F21.5) instead of a dedicated status
endpoint.** Pros: zero new backend surface, matches the spec's own P1 "syncing indicator" language. Cons: that
mechanism signals *that* something is running, not *how far along* a specific batch is — it answers a
different question (spec P1, "is it still going after I left the page") than the in-dialog progress bar (spec
P0, "shows progress/completion via the extended dialog") needs. Not rejected — this remains the right tool for
the P1 requirement; it is simply not a substitute for D3, which is scoped to P0.

---

## Trade-off Analysis

**A flat per-name filter vs. inherited hierarchy exclusion (D1).** The flat rule is strictly less code and
matches every explicit requirement in the spec; inherited exclusion is a real, defensible alternative reading
of "excluded" but one the spec never states, and building it now would be designing past the actual ask. This
is the same posture ADR-075 D1 itself took on hierarchy depth (ship the simplest structure that's correct,
revisit only on evidence).

**Recomputing per video vs. any cheaper alternative (D2).** There is no cheaper *correct* alternative — the
value being synced is definitionally a function of current DB state (tags, deny-list, and now the writeback
flag), so computing it fresh is not a performance shortcut being declined, it is the only option that produces
a real sync rather than an append. The cost (one `GenreWritebackValues` call per video) is bounded by the same
video count the write itself already touches — no asymptotic difference from the write cost it's paired with.

**A new aggregation endpoint vs. reusing what exists (D3).** Both rejected alternatives (B, C) are cheaper to
build but answer the wrong question at the wrong granularity for a genuinely new (N-videos-per-owner-action)
progress-reporting need. `writeback_queue.status` and `job_runs.batch_id` already exist for other reasons; this
ADR's contribution is one query joining them by an id both already carry, not new state to maintain.

---

## Consequences

**What becomes easier**
- The manual-sync batch automatically inherits `POST /writeback/batches/{batchID}/revert` for free — it was
  built generically on `batchID`, not on merge-specific assumptions, so a sync gone wrong (e.g. the owner
  re-enables a tag right after syncing it off) is revertible the same way a bad merge propagation already is.
- `TagNamesForVideo` stays the single place "which tag names does this video's writeback see" is answered —
  the deny-list's raw-genres filter and this ADR's tag filter remain two independent, non-overlapping checks
  on two different halves of the union, exactly as ADR-075 RD9 originally split them.
- D3's batch-status endpoint is reusable by any future batch-writeback trigger keyed on `batchID` — the next
  one (a future tag-categories bulk action, per the spec's own stated follow-up) needs zero new backend work
  for progress reporting.

**What becomes harder**
- `syncTagWriteback`'s abort-on-first-read-error posture (unlike `propagateMerge`'s log-and-continue) means a
  single unreadable video in a large bulk sync blocks the whole batch rather than degrading gracefully. This is
  a deliberate choice (nothing is committed yet, unlike a merge), but a future maintainer reasoning by analogy
  to `propagateMerge` should not assume the same best-effort behavior here.
- A disabled tag's exclusion is genuinely per-name, not per-branch of the hierarchy (D1-C rejected) — a future
  UI affordance that visually implies "excluding a parent excludes its children" would be presenting behavior
  the backend does not implement; any hierarchy-aware messaging in the Details card must describe the flat
  rule accurately.

**What we'll need to revisit**
- **Inherited hierarchy exclusion (D1-C)** — only if an owner actually wants disabling a broad tag to suppress
  its whole subtree's writeback contribution; no evidence of that need yet, and the flat rule is strictly
  additive to extend later (a hierarchy-aware `WHERE` clause change, not a schema change).
- **`syncTagWriteback`'s all-or-nothing read failure handling** — revisit toward partial-batch tolerance if a
  real large-library sync hits a genuinely unreadable video mid-batch in practice; no evidence yet this is a
  real failure mode rather than a theoretical one.

---

## Action Items

1. [ ] Migration `0033_tag_writeback_enabled.{up,down}.sql`: `tags.writeback_enabled INTEGER NOT NULL DEFAULT
   1` (D1).
2. [ ] `TagNamesForVideo`: add `WHERE t.writeback_enabled = 1` to the final projection only, leaving the
   recursive walk unchanged (D1).
3. [ ] `Repo.VideoIDsForTag` (single) and its `IN (...)`-union bulk counterpart: active/non-deleted videos
   currently carrying a given tag id or set of tag ids (D2).
4. [ ] `internal/api/tag_writeback_sync.go`: `syncTagWriteback`, the four owner-gated endpoints (D2 table),
   `PATCH /tags/{id}/writeback` and its bulk counterpart as plain `UPDATE`s with no enqueue.
5. [ ] `Repo.GetWritebackBatchStatus` + `GET /writeback/batches/{batchID}/status`, mounted in `mountWriteback`
   alongside the existing per-job and revert routes (D3).
6. [ ] `tags/{id}` Details card + `/tags` bulk-action bar wiring (spec P0 frontend requirements) — out of this
   ADR's scope; covered by `/design-handoff` per the epic's gate table.
7. [x] `/testing-strategy`: cover D1's flat-filter behavior (a disabled ancestor does not suppress a
   still-enabled descendant/further-ancestor), D2's per-video recompute (a sync reflects the *current* full
   `genres` union, not just the synced tag), D2's bulk video-dedup (a video attached to two selected tags is
   enqueued once), and D3's status aggregation across `pending`/`running`/`done`/`failed`. All four were
   already covered by `internal/repo/tag_hierarchy_test.go`'s `TestTagNamesForVideo_WritebackFlagFlat` (D1)
   and `internal/api/tag_writeback_sync_test.go`'s `TestSyncTagWriteback_RecomputesFullUnion`,
   `TestSyncTagWritebackBulk_DedupsSharedVideo`, and `TestWritebackBatchStatusEndpoint` (D2/D3), shipped
   alongside the S3 backend commit — this pass cross-references that existing green coverage into
   `docs/testing-strategy.md` §4/§9 rather than adding net-new tests.
8. [x] `/security-review` before merge (file I/O via the write queue, four new owner-gated mutation endpoints)
   — per the spec's own note and this epic's `needs-security-review` gate. Reviewed authz (all 5 endpoints
   confirmed mounted behind `requireOwner` in `handlers.go`, not just handler-body checks), SQL injection
   (all new queries parameterized via the existing `placeholders()`/`toAnySlice()` pattern), batch-ID
   enumeration (server-generated, owner-gated, single-owner app — no cross-tenant concept), and bulk
   `tag_ids` scope (non-existent ids simply match zero rows; no write-target injection). No findings.
