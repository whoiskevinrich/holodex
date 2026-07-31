---
key: HOLODEX-239
status: In Progress
depends-on: []
release_note: "Tags can now be excluded from file writeback while staying searchable in Holodex, with a manual per-tag or bulk sync to catch already-written files up."
---

## Gates — definition of done
- [x] spec          docs/specs/tag-writeback-exclusion.md · S1
- [ ] architecture  not started
- [ ] backend       not started
- [ ] frontend      not started
- [~] testing       deferred until: backend/frontend implemented
- [~] security      deferred until: architecture + backend implemented (touches file I/O via the write queue)

## Up next   (ordered — position is the priority; top line is the next action)
1. Write the ADR covering the per-tag writeback flag + the manual sync batch seam    [architecture]
2. Migration: `writeback_enabled` column on Tag; filter it into `genreWritebackValuesForVideo`  [backend]
3. Manual sync endpoint: batch-enqueue via `writequeue.EnqueueMany`, reusing the `propagateMerge` pattern  [backend]
4. Bulk toggle/sync endpoints for multi-tag selection  [backend]
5. `tags/{id}` Details card: toggle + sync trigger, extending `WritebackFormDialog`  [frontend]
6. Extend `/tags` list's multi-select Manage-mode bar with the three new bulk actions  [frontend]
7. Regenerate testing-strategy for the new endpoints/flows  [testing]
8. Security review — file I/O via the write queue  [security]

## Session log   (append-only)
S1 · /product-brainstorming /write-spec — spec drafted, epic filed, Draft PR opened
