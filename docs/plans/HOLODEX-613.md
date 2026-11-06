---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-613                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Trim unwanted footage off the start or end of a video from its details page, frame-accurately in the background, replacing the original or saving the cut as a new edition with all its metadata.  # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
approved:
  design:
    on: 2026-11-06
    at: 59c94fbe
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-613 · Video trimming (F88)

The owner picks the first and last frames to keep on Media Details. The cut runs in the background
as the second F77 Maintenance entry, is checked against those frames, and then either replaces the
original (held for the F77 window) or lands as a new edition item copied from the original.

**Design package:** [spec F88](../specs/video-trimming.md) · architecture (59d55a75: [file-maintenance.md](../architecture/file-maintenance.md)) · [handoff](../design/video-trimming-handoff.md) · testing-strategy § pending   <!-- links; source of truth for *what*; this file is source of truth for *where it stands* -->

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default; an empty roster leaves no rows at all, and status then comes from the
     release note alone. The rows below are only the unscaffolded template's. Once `profile:` is set,
     match them to that posture, adding rows as well as removing them. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**`
- [x] architecture `architecture` → `docs/architecture/**`
- [x] design `design-handoff` → `docs/design/**`
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`
- [ ] security `security-review`

<!-- Deliberate-skip example — always say why; `until:` records what would reopen the concern later
     (as a fresh up-next item or its own issue — the gate itself stays settled):
- [~] security `security-review` — until: a mutation endpoint exists (read-only slice so far) -->

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced in the SessionStart banner, clipped if long (the trailing path
     survives; the middle is dropped) — so keep it to one line.
     The CHECKBOX settles an item and nothing else does: [x] done, [~] dropped, same states as a gate
     row. A strikethrough is emphasis, not a state — `~~half~~ now do the rest` is a live item
     (ADR-008). This queue holds LIVE work only: delete a done item, move a dropped one to
     ## Dropped at the end of this file (ADR-010). The banner counts any settled item left here. -->

1. [ ] [testing] Fixture matrix for frame accuracy and edition copy — `docs/testing-strategy.md`
2. [ ] [backend] Trim op, frame checks and edition clone; settle the edition's rename state (proposed: starts with no rename history or lock) in the doc — `docs/architecture/file-maintenance.md`
3. [ ] [frontend] `TrimPicker`, the review step and the File row — `docs/design/video-trimming-handoff.md`
4. [ ] [security] `/security-review` of the capture upload and the new maintenance routes — `docs/architecture/security-perimeter.md`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-11-06 · session
- skills: design-handoff, implement
- handoff: Crossed into build — design signed off (given at 6af567af, pinned at 59c94fbe, the merge of main that left the trim docs unchanged); Draft PR open. Start at the testing fixture matrix, then the trim op; F86 file rename landed on main, so settle the edition's rename state while building the clone.

### 2026-11-02 · session
- skills: architecture
- handoff: Spike done, R6 rewritten, and architecture decided in file-maintenance.md (59d55a75): full re-encode of the kept range, audio copied and cut on packets, cut half a frame early, exact cut check plus shape-only screen check, edition cloned by the trim job in a journaled transaction. Next: /design-handoff for the trim picker and edition confirm step.

### 2026-11-01 · session
- skills: product-management:brainstorm, write-spec, handoff
- handoff: Q1 spike ran silently on an 84-min MP4 and a 92-min MKV (results on HOLODEX-613): cut half a frame early, verify against ffmpeg's own frame; MP4 shows no browser/ffmpeg drift, MKV browser side needs a re-run with the Browser pane visible. Next: that re-run, then the R6 rewrite.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
