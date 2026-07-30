---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-224                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: A denied term can never become a tag, tags form a searchable broader/narrower hierarchy, and video enrichment automatically fills in real, mergeable genre tags.
---

# HOLODEX-224 · F50 Tag governance & video enrichment

Extends F43's tag identity spine with a global deny-list, a strict one-parent hierarchy, automatic
tag-materialization from video enrichment (TMDB `genres`), manual add/remove tag chips on the media page,
and genre writeback. Done means all nine suggested slices (S1–S9) land, tested, and pass `/security-review`
against the final implementation diff.

**Design package:** [spec](../specs/tag-governance-and-video-enrichment.md) · [ADR-075](../architecture/ADR-075-tag-governance-and-video-enrichment.md) · [handoff](../design/tag-governance-and-video-enrichment-handoff.md) + [QA checklist](../design/tag-governance-and-video-enrichment-qa-checklist.md) · [testing-strategy §9](../../docs/testing-strategy.md) (F50 block)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/tag-governance-and-video-enrichment.md`
- [x] architecture `architecture` → ADR-075
- [x] design `design-handoff` → design handoff + QA checklist
- [/] backend — S1/S2/S3 landed (provenance fix, deny-list, hierarchy); S4–S7 remain
- [ ] frontend — media-page tag chips (S4), deny-list tab + hierarchy pill-menu action (S8) not started
- [/] testing `testing-strategy` → §9 F50 block written; per-slice tests land alongside each slice
- [/] security `security-review` — design-level review complete (ADR-075 item 10); must re-run against the
      final implementation diff before merge (standing policy)

## Up next — ordered (position = priority)

1. [x] [backend] S1 — `video_tags` provenance + `replaceAssociations` fix (P0-1, ADR-075 D3) — migration 0030
2. [x] [backend] S2 — deny-list table + `resolveOrCreateByName` enforcement + management endpoints (P0-2/3) — migration 0031, `internal/api/tag_denylist.go`
3. [x] [backend] S3 — hierarchy: `parent_tag_id` + cycle guard + descendant-inclusive filter/search (P0-4/5/6) — migration 0032, `internal/repo/tag_hierarchy.go`, `internal/api/tag_hierarchy.go`
4. [ ] [backend] S4 — video↔tag attach/detach endpoints + media-page UI (P0-7/8) — `internal/api`, `web/src/routes/media/[id]/+page.svelte`
5. [ ] [backend] S5 — enrichment materialization (P0-9) — `internal/api/enrich_review.go` `afterEnrichApply`
6. [ ] [backend] S6 — genre writeback wiring (P0-10) ⛔ blocked on #4 — `internal/writeback`
7. [ ] [backend] S7 — merge reparenting (P0-11) — `internal/repo/identity_ops.go`
8. [ ] [frontend] S8 — P1 UI: deny-list tab, hierarchy pill-menu row action — `web/src/routes/owner/tags`, `web/src/routes/tags/+page.svelte`
9. [ ] [—] S9 — QA + `/security-review` final pass before merge

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-07-30 · S3 — hierarchy
- skills: (implementation only, against the already-landed spec/ADR/handoff/testing-strategy), simplify
- handoff: migration 0032 (`tags.parent_tag_id` + `idx_tags_parent`) lands the strict one-parent tree.
  `internal/repo/tag_hierarchy.go`'s `SetTagParent`/`isTagDescendant` and `VideoFilter.build()`'s now
  descendant-inclusive `TagIDs` clause share one recursive-CTE subtree query (`tagSubtreeQuery`), so
  "descendant" means the same thing in both places. Owner-gated `POST /tags/{id}/parent` returns
  `400 {cycle:true}` on a cycle, `404` on an unknown tag/parent. A 4-level tag-tree fixture
  (`internal/repo/tag_hierarchy_test.go`) covers the four cycle-guard boundary cases the testing strategy
  calls out (self, direct-child-as-parent, deep-descendant-as-parent, unrelated sibling) plus
  descendant-inclusive filter parity; migration round-trip and API gating/validation covered too. Full
  `go test ./...` green. ADR-075 action items 1/4/7 marked done. Next: S4 (video↔tag attach/detach +
  media-page UI).

### 2026-07-30 · session
- skills: security-review, simplify

### 2026-07-29 · session
- skills: testing-strategy
