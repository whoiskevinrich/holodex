---
key: HOLODEX-587
status: in-progress
profile: full
release_note: "The owner queues (Trash, Enrichment, Completeness, Duplicates, Extraction) now load 50 items at a time with a Load more button, so they open quickly on large libraries."
---

# HOLODEX-587 · Owner queues paged (subtask of HOLODEX-354)

Done when every owner queue is bounded server-side (limit/offset, clamped like the browse grid) and
paged client-side with Load more, without skipping rows after resolve / restore / delete.

**Design package:** [owner-queue-paging.md (F81)](../specs/owner-queue-paging.md) · sibling
[entity-list-paging.md (F80)](../specs/entity-list-paging.md) on HOLODEX-354's branch

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — `owner-queue-paging.md` (F81)
- [~] architecture `architecture` → `docs/architecture/**` — no technology fork (in-handler paging over the existing reads)
- [~] design `design-handoff` → `docs/design/**` — no new surface: the browse grid's Load more, layouts and order unchanged — until: the owner wants Completeness's per-group Load more designed
- [x] backend → `{cmd,internal,providers}/**` — `internal/api/paging.go` + the six queue handlers
- [x] frontend → `web/src/**` — `web/src/lib/queuePaging.ts` + the five `owner/*` pages and `DuplicatesBanner`
- [x] testing `testing-strategy` — testing-strategy §31; `paging_internal_test.go`, `owner_queue_paging_test.go`, `queuePaging.test.ts`
- [x] security `security-review` — owner gate intact on all six routes; no new param reaches SQL; `?type=` allowlisted

## Up next — ordered (position = priority)

1. [ ] [—] Review draft PR #540, then mark it ready — every gate is settled
2. [ ] [—] Sweep HOLODEX-587 to In Review when the PR is marked ready, Done on merge — Jira

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-19 · crossing into build
- skills: implement
- handoff: Crossed into build. No sign-off was owed (design `[~]`), and main was already current. Draft PR #540 is open; start at reviewing #540.

### 2026-10-18 · owner queue paging
- skills: code-review, security-review, handoff
- handoff: All five owner queues page server- and client-side (commit f5156bdb, branch pushed, no PR yet); the next move is `/implement` to open the Draft PR, once the HOLODEX-354 session agrees the merge order.

## Dropped — newest first (the reason is the point)
