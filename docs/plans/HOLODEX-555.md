---
key: HOLODEX-555
status: in-progress
profile: backend
depends-on: [HOLODEX-554]
release_note: "Removing a person from a video's cast or director now removes them under every spelling, so an alias on the file no longer brings them back."
---

# HOLODEX-555 · Removing a person chip leaves their alias spellings

Done when a suppress or no-write on a person-typed field covers the person's canonical name and every alias,
atomically, and clearing it restores them all. Stacked on HOLODEX-554 (PR #481), which collapsed those
spellings into one chip.

**Design package:** [entity-identity.md P0-11](../specs/entity-identity.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — entity-identity P0-11, removal scenario
- [x] backend → `{cmd,internal,providers}/**` — `curationSpellings`; `SetCurationChecked` / `ClearCurations` over all spellings in one transaction / statement
- [x] testing `testing-strategy` — `TestCuration_SuppressPersonCoversAliases` (fails before the fix)

## Up next — ordered (position = priority)

1. [ ] [—] Merge #481 first, then merge main in here and retarget this PR to main — `git merge origin/main`
2. [ ] [—] Sweep HOLODEX-555 to In Review when the PR is marked ready, Done on merge — Jira

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-09 · person removal covers alias spellings
- skills: code-review, handoff
- handoff: Fixed and tested, stacked on #481 — merge #481, merge main in here, retarget to main and mark ready.

## Dropped — newest first (the reason is the point)
