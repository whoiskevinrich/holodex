---
key: HOLODEX-554
status: in-progress
profile: backend
release_note: "A cast member credited on the file under an alias now appears once in the writeback dialog, under their own name, instead of beside it."
---

# HOLODEX-554 · Person alias listed beside the linked person in writeback

Done when a video's person rows (actors / director) collapse values by person identity, so an alias of a
linked person shows and writes as that person's canonical name, once.

**Design package:** [entity-identity.md P0-11](../specs/entity-identity.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — entity-identity P0-11 (person alias collapses on the video)
- [x] backend → `{cmd,internal,providers}/**` — `collapsePersonAliases` in `internal/api/person_identity.go`, wired into `getMedia`
- [x] testing `testing-strategy` — `TestGetMedia_PersonAliasCollapsesIntoLinkedPerson` (fails before the fix)

## Up next — ordered (position = priority)

1. [ ] [—] Sweep HOLODEX-554 to In Review when the PR is marked ready, Done on merge — Jira

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-09 · person alias collapse in the writeback row
- skills: code-review, handoff
- handoff: Fixed, tested and spec'd; follow-up HOLODEX-555 (chip removal leaves alias spellings) filed and linked — next move is review and merge of the PR.

## Dropped — newest first (the reason is the point)
