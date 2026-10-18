---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-585
status: in-progress
profile: chore               # agent tooling: CI workflow + CLAUDE.md, no product change
depends-on: []
release_note: "Internal: the committed graphify code graph now refreshes through a rolling bot PR on every push to main."
---

# HOLODEX-585 · Auto-refresh the committed graphify graph via a bot PR on push to main

The committed `graphify-out/` graph was a month stale (last refreshed 66280fc4, 2026-09-18). Its
only refresh, the local post-commit hook, exits in linked worktrees and during merges, and GitHub
squash-merges never run it. `.github/workflows/graphify-refresh.yml` now runs the same code-only,
no-LLM `graphify update . --force` (PYTHONHASHSEED=0, graphifyy pinned) on push to `main` and on
dispatch, and opens or updates one `chore(graphify): ...` PR on `bot/graphify-refresh` using the
release-please PAT, so the required checks run. `.claude/CLAUDE.md` gains a Graphify section.

**Design package:** none (tooling)

## Gates — definition of done

_Posture `chore` holds no gates._

## Up next — ordered (position = priority)

1. [ ] [—] Merge this PR, then dispatch `Graphify refresh` on main and confirm it runs green
2. [ ] [—] Confirm the bot PR prunes `web/src/lib/adminMode.svelte.ts` from the graph, then merge it
3. [ ] [—] Confirm CI moves HOLODEX-585 to Done

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-18 · session
- skills: code-review (high --fix)
- handoff: The workflow and CLAUDE.md section are in. A scratch run showed the clean rebuild is deterministic apart from manifest mtimes and prunes adminMode. workflow_dispatch only works once the file is on main, so next: merge, then dispatch.

## Dropped — newest first (the reason is the point)
