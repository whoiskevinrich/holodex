---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-190
status: in-review
profile: ui                  # frontend bug fix; lists restored on Back go stale after writes, no UX change
depends-on: []
release_note: Going back to the Media grid after deleting, restoring or editing something now shows the change straight away, at the same scroll position, instead of the grid as it was before.
---

# HOLODEX-190 · Restored list shows stale data until hard refresh

Every successful owner write bumps one app-wide counter in the API client's write path
(`checkRedirect(res, true)` → `markDataChanged`). `createNavSnapshot` stamps it on save and `take`
reports `stale` when it moved, so the Media grid restores its scroll and then re-fetches the loaded
range in place. That replaces the per-site patches (HOLODEX-478's `forgetVideo`, HOLODEX-572's clear on
Restore) and covers any write and any future data-caching list without wiring. People, Studios,
Films, Tags, Search and entity video grids were checked: they re-fetch on mount and restore scroll
only, so they were never stale. The freshness fork is recorded in
`docs/architecture/browse-and-list-state.md`.

**Design package:** none (no UX change)

## Gates — definition of done

- [~] design `design-handoff` → `docs/design/**` — no UX change
- [x] frontend → `web/src/**`
- [x] testing `testing-strategy` → `docs/testing-strategy.md` §27 — unit (`navSnapshot.test.ts`, `api.test.ts`), nav harness `back-after-write-refreshes-grid` (8/8, mutation-checked), live QA on `backend-stress`

## Up next — ordered (position = priority)

1. [ ] [—] After the squash merge, replace "Decided in this commit" with the squash sha link — `docs/architecture/browse-and-list-state.md`
2. [ ] [—] On merge, confirm CI moved HOLODEX-190 to Done (Bug on its own keyed branch — jira-sync handles it)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-13 · launch profiles for the nav harness
- skills: —
- handoff: Added `backend-stress-open-9600` + `web-9600` to `.claude/launch.json.example` and a "Preview configurations" guide to `.claude/CLAUDE.md`; PR #507 is still ready — next: merge, then fill the architecture section's Decided-in sha.

### 2026-10-13 · write-count freshness for list snapshots
- skills: code-review (high --fix), handoff
- handoff: Fixed and verified (unit, nav harness 8/8, live trash/restore/title-edit on `backend-stress`); PR open and ready — next: merge, then fill the architecture section's Decided-in sha.

## Dropped — newest first (the reason is the point)
