---
key: HOLODEX-239
status: In Progress
depends-on: []
release_note: "Tags can now be excluded from file writeback while staying searchable in Holodex, with a manual per-tag or bulk sync to catch already-written files up."
---

## Gates — definition of done
- [x] spec          docs/specs/tag-writeback-exclusion.md · S1
- [x] architecture  docs/architecture/ADR-077-tag-writeback-exclusion.md · S2
- [x] backend       S3 (commit cb88390)
- [x] frontend      S4 — docs/design/tag-writeback-exclusion-handoff.md + -qa-checklist.md
- [~] testing       deferred until: /testing-strategy regen (ADR-077 action item 7)
- [~] security      deferred until: /security-review (file I/O via the write queue, 4 new owner-gated endpoints)

## Up next   (ordered — position is the priority; top line is the next action)
1. Regenerate testing-strategy for the new endpoints/flows (ADR-077 action item 7)  [testing]
2. Security review — file I/O via the write queue, four new owner-gated mutation endpoints  [security]

## Session log   (append-only)
S1 · /product-brainstorming /write-spec — spec drafted, epic filed, Draft PR opened
S2 · /architecture — ADR-077 drafted (writeback_enabled flag filtered flat into TagNamesForVideo; manual sync recomputes GenreWritebackValues per video via a propagateMerge-style shared-batchID EnqueueMany; new batch-status endpoint aggregating writeback_queue/job_runs by batch_id); ADR index + spec cross-reference updated
S3 · backend implementation (commit cb88390) — migration 0033, TagNamesForVideo flat filter, VideoIDsForTag(s), syncTagWriteback, 4 owner-gated endpoints, GetWritebackBatchStatus + status endpoint; full suite green
S4 · /design-handoff + frontend implementation — tag-writeback-exclusion-handoff.md + -qa-checklist.md; new WritebackBatchDialog.svelte (sibling of WritebackFormDialog, aggregate progress not per-row); tags/[id] Details card (toggle + sync trigger); /tags Manage-mode bulk bar (3 actions, 2+ selected); waitForWritebackBatch in writebackJob.ts. `npm run check`/`test` green; live-verified end-to-end against backend-amv (toggle, single-tag sync incl. zero-value skip, bulk toggle on mixed state, bulk sync incl. zero-enqueued), all 3 skins measured (AA clear, same tokens as writeback-selection-handoff.md). Found (not fixed, out of scope): `Repo.ListTags` doesn't select `writeback_enabled`, so the Go zero-value serializes false on every `/tags` list row regardless of actual state — doesn't affect this frontend (list page never reads the field) but flagged for a follow-up.

### 2026-07-31 · session
- skills: simplify, design-handoff
