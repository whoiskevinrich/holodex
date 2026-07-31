---
key: HOLODEX-239
status: In Progress
depends-on: []
release_note: "Tags can now be excluded from file writeback while staying searchable in Holodex, with a manual per-tag or bulk sync to catch already-written files up."
---

## Gates — definition of done
- [x] spec          docs/specs/tag-writeback-exclusion.md · S1
- [x] architecture  docs/architecture/ADR-077-tag-writeback-exclusion.md · S2
- [ ] backend       not started
- [ ] frontend      not started
- [~] testing       deferred until: backend/frontend implemented
- [~] security      deferred until: backend implemented (touches file I/O via the write queue)

## Up next   (ordered — position is the priority; top line is the next action)
1. Migration `0033_tag_writeback_enabled.{up,down}.sql`: `writeback_enabled` column on Tag; filter into `TagNamesForVideo`'s final projection (ADR-077 D1)  [backend]
2. `Repo.VideoIDsForTag` (single + bulk-union) and `syncTagWriteback`: batch-enqueue via `writequeue.EnqueueMany`, recomputing `GenreWritebackValues` per video (ADR-077 D2)  [backend]
3. Four owner-gated endpoints: `PATCH /tags/{id}/writeback`, `POST /tags/{id}/writeback/sync`, and their bulk counterparts (ADR-077 D2)  [backend]
4. `GetWritebackBatchStatus` + `GET /writeback/batches/{batchID}/status` (ADR-077 D3)  [backend]
5. `tags/{id}` Details card: toggle + sync trigger, extending `WritebackFormDialog`  [frontend]
6. Extend `/tags` list's multi-select Manage-mode bar with the three new bulk actions  [frontend]
7. Regenerate testing-strategy for the new endpoints/flows (ADR-077 action item 7)  [testing]
8. Security review — file I/O via the write queue, four new owner-gated mutation endpoints  [security]

## Session log   (append-only)
S1 · /product-brainstorming /write-spec — spec drafted, epic filed, Draft PR opened
S2 · /architecture — ADR-077 drafted (writeback_enabled flag filtered flat into TagNamesForVideo; manual sync recomputes GenreWritebackValues per video via a propagateMerge-style shared-batchID EnqueueMany; new batch-status endpoint aggregating writeback_queue/job_runs by batch_id); ADR index + spec cross-reference updated

### 2026-07-31 · session
- skills: simplify
