---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-35                  # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: feature             # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: "Regenerate thumbnail now opens a frame picker: scrub the video silently (drag, arrow keys or the mouse wheel, one frame at a time), compare with the current thumbnail, and keep the exact frame on screen."  # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
approved:
  design:
    on: 2026-10-26
    at: a079d8f4
---

# HOLODEX-35 · Thumbnail frame picker (F84)

Regenerate thumbnail opens a picker: scrub a muted, never-played copy of the video, compare with the
current thumbnail, and keep the on-screen frame, captured in the browser and saved as an uploaded poster.

**Design package:** [spec F84](../specs/thumbnail-frame-picker.md) · [handoff](../design/thumbnail-frame-picker-handoff.md) · [testing-strategy §36](../testing-strategy.md#36-thumbnail-frame-picker-f84-holodex-35)   <!-- links; source of truth for *what*; this file is source of truth for *where it stands* -->

## Gates — definition of done

<!-- States: [ ] not started · [/] in progress · [~] deliberately skipped · [x] done. [~] and [x]
     both count as SETTLED (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs;
     ONLY /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**`
- [x] design `design-handoff` → `docs/design/**`
- [~] backend → `{cmd,internal,providers}/**` — no backend change: the frame goes through the existing poster upload (F52)
- [x] frontend → `web/src/**`
- [x] testing `testing-strategy`

<!-- Deliberate-skip example — always say why; `until:` records what would reopen the concern later
     (as a fresh up-next item or its own issue — the gate itself stays settled):
- [~] security `security-review` — until: a mutation endpoint exists (read-only slice so far) -->

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The CHECKBOX settles an item and nothing else does: [x] done, [~] dropped (ADR-008).
     This queue holds LIVE work only (ADR-010). -->

1. [ ] [backend] Writeback embeds the owner-uploaded poster (incl. a picked frame) as the file's cover art; today it only embeds the provider `poster_url` → HOLODEX-597 — `internal/writeback/tags.go`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-10-27 · session
- skills: docs
- handoff: The owner signed off the production test plan (Claude Doc https://claude.ai/artifact/4PWNvGRWb3eY2pRNjmHQbV) on 2026-10-27: every section passed, including frame accuracy on 60 fps, MPEG-TS and VFR files, so there are no follow-up bugs. The frame picker is done; the only live item is the writeback gap, HOLODEX-597.

### 2026-10-26 · session
- skills: code-review, handoff, implement, docs
- handoff: Built, agent-QA'd (saved poster matched frame 225 exactly) and merged via #560 at the owner's request so it can be tested on the production library. Production QA found the picked frame never reaches the file on writeback — spun out as HOLODEX-597 (Relates). Start at the owner's results from the production test plan doc (Up next 1).

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
