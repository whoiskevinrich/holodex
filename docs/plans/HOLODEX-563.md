---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-563                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: backend             # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Rescan library now tells you when no media folder is configured, instead of claiming a scan is already running.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-563 · Rescan with no MEDIA_PATH reports "A scan is already running."

Done when a rescan with no media path configured answers 503 with that reason, which the Status page
toast shows, while an in-flight scan still answers `202 started:false`. Spun out of HOLODEX-560.

**Design package:** [system-activity spec](../specs/system-activity.md) (Rescan library row) ·
[testing-strategy "Admin rescan — no media path"](../testing-strategy.md)

## Gates — definition of done

<!-- States: [ ] not started · [/] in progress · [~] deliberately skipped · [x] done. [~] and [x] both
     count as SETTLED (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**`
- [x] backend → `{cmd,internal,providers}/**`
- [x] testing `testing-strategy`

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority. The CHECKBOX settles an item: [x] done, [~] dropped.
     This queue holds LIVE work only (ADR-010). -->

1. [ ] [—] Squash-merge the PR once CI is green (clean `fix(scanner): …` subject) — `internal/api/handlers.go`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-10-11 · rescan reports a missing media path
- skills: code-review (high --fix), handoff
- notes: the frontend is unchanged, since the existing error toast already shows the server's 503 `error` line. I merged main after #490 landed; the only conflict was the `scanner.go` import block (both changes added `errors`).
- handoff: Fix 40dcc411 is committed with spec and testing-strategy rows, and all gates are settled; the PR is open and ready, so squash-merge it once CI is green.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
