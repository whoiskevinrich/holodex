---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-479                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: backend             # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are trimmed to match.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Writing metadata back to a fragmented MP4 now explains that the file needs a remux and shows the ffmpeg command to do it, instead of a raw exiftool error.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-479 · Writeback refuses fragmented MP4 with a remux hint

Done when a writeback to a fragmented MP4/M4V/MOV is refused before exiftool runs with a typed
`writeback_unsupported_container` error, and the media page tells the owner the file needs a remux
(with the ffmpeg one-liner) instead of showing a raw exit status.

**Design package:** [spec R3.7](../specs/fire-and-forget-writeback.md) · ADR-041 (unchanged — refusal happens before its copy → write → rename flow)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — R3.7 in fire-and-forget-writeback.md
- [x] backend → `{cmd,internal,providers}/**` — `internal/writeback/fragments.go`, sync-path 422
- [x] frontend → `web/src/**` — outside the `backend` posture, kept because it carries work: the R3.2 detail line's copy for this refusal (no new component, no layout change)
- [x] testing `testing-strategy` — box-walker table test, WriteBatch refusal test (no tools needed), ffmpeg-generated fMP4 + remux integration test; verified live on backend-amv

## Up next — ordered (position = priority)

1. [ ] [—] Open a PR for this fix, review, merge; then sweep HOLODEX-479 to Done — `docs/plans/HOLODEX-479.md`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-28 · session
- skills: code-review, handoff
- handoff: Fix committed and live-verified (fragmented file refused, page shows remux hint); follow-ups HOLODEX-480 (auto-remux) and 481 (scan-time flag) are filed and related — next is the PR.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-09-28, <why>
-->
