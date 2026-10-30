---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/archive/ADR-064-flightplan-plugin.md
key: HOLODEX-607                 # the tracker key; must match the branch key regex
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
profile: backend
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Fix — merging a person or studio no longer throws away the merged one's provider match and photos; the survivor keeps whatever it was missing.
---

# HOLODEX-607 · Merging people or studios drops the merged entity's enrichment and photos

Bug fix. `mergeEntities` deleted the loser's `entity_enrichment` rows (the v1 rule) while repointing its
provider ids, so a bare survivor held ids with no data and read as unmatched; `person_images` /
`studio_images` cascaded away and their files were orphaned. Owner decision 2026-10-29: survivor wins
per provider; images fill empty slots; people and studios both.

**Design package:** spec only — [entity-identity.md RD6a](../specs/entity-identity.md) (the rule),
pointed at from person-aliases.md F23.9 and people-source-of-truth.md RD5. No technology fork, no UI.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — entity-identity.md RD6a; person-aliases.md F23.9 (open question 2 closed); people-source-of-truth.md RD5
- [x] backend → `{cmd,internal,providers}/**` — enrichment + image carry-over in the merge tx; `settleMergedImages` moves files
- [x] testing `testing-strategy` — `internal/repo/merge_carryover_test.go`, `internal/api/merge_images_test.go`

## Up next — ordered (position = priority)

1. [ ] [—] Open the PR, mark it ready, sweep HOLODEX-607 to Done on merge — `gh pr create`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-29 · Root-caused and fixed the merge data loss
- skills: code-review (high --fix), handoff
- handoff: merge now carries the loser's per-provider enrichment (honoring survivor dismissals), photo suppressions, and images into empty slots (person gallery to the cap, deduped; studio and film slots), with files moved after commit; all Go tests green — next is opening the PR.
