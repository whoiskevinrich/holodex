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
- [x] testing       docs/testing-strategy.md §4/§5/§6/§9/§10 (ADR-077 action item 7) — cross-references existing green coverage, no new tests needed
- [~] security      deferred until: /security-review (file I/O via the write queue, 4 new owner-gated endpoints)

## Up next   (ordered — position is the priority; top line is the next action)
1. Security review — file I/O via the write queue, four new owner-gated mutation endpoints  [security]

## Session log   (append-only)
S1 · /product-brainstorming /write-spec — spec drafted, epic filed, Draft PR opened
S2 · /architecture — ADR-077 drafted (writeback_enabled flag filtered flat into TagNamesForVideo; manual sync recomputes GenreWritebackValues per video via a propagateMerge-style shared-batchID EnqueueMany; new batch-status endpoint aggregating writeback_queue/job_runs by batch_id); ADR index + spec cross-reference updated
S3 · backend implementation (commit cb88390) — migration 0033, TagNamesForVideo flat filter, VideoIDsForTag(s), syncTagWriteback, 4 owner-gated endpoints, GetWritebackBatchStatus + status endpoint; full suite green
S4 · /design-handoff + frontend implementation — tag-writeback-exclusion-handoff.md + -qa-checklist.md; new WritebackBatchDialog.svelte (sibling of WritebackFormDialog, aggregate progress not per-row); tags/[id] Details card (toggle + sync trigger); /tags Manage-mode bulk bar (3 actions, 2+ selected); waitForWritebackBatch in writebackJob.ts. `npm run check`/`test` green; live-verified end-to-end against backend-amv (toggle, single-tag sync incl. zero-value skip, bulk toggle on mixed state, bulk sync incl. zero-enqueued), all 3 skins measured (AA clear, same tokens as writeback-selection-handoff.md). Found (not fixed, out of scope): `Repo.ListTags` doesn't select `writeback_enabled`, so the Go zero-value serializes false on every `/tags` list row regardless of actual state — doesn't affect this frontend (list page never reads the field) but flagged for a follow-up.
S5 · fix(tags) commit 3465782 — `Repo.ListTags` batch-attaches `writeback_enabled` via new `tagWritebackEnabled` helper (mirrors `tagParents`), closing the S4 gap; added `TestListTagsWritebackEnabled`. Full `go test ./...` green.
S6 · /testing-strategy — regenerated docs/testing-strategy.md (§0 date line, §4 backend table rows + 2 critical invariants, §5 frontend row distinguishing real writebackJob.test.ts coverage from the manual-QA-only component layer, §6 E2E flow 21, §9 phasing narrative, §10 Given/When/Then example); found ADR-077 action item 7's four scenarios (D1 flat-filter, D2 recompute, D2 bulk-dedup, D3 aggregation) were already fully covered by tests shipped alongside S3/S5 — checked off item 7 in the ADR rather than writing duplicate tests.

### 2026-07-31 · session
- skills: simplify, design-handoff, testing-strategy, testing-strategy
