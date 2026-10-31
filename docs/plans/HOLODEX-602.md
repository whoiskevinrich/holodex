---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-602                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: The owner can have Holodex rename media files from an ordered list of templates, with a preview first, names valid on Windows and Linux, stale-name review, and undo.  # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
approved:
  design:
    on: 2026-10-31
    at: e4aeee37
---

# HOLODEX-602 · Rename media files from a template (F86)

Done means the owner can rename library files in place from an ordered template list, through the
progressive plan → manual → writeback → job modes, without a video ever losing its identity or a
file ever being overwritten.

**Design package:** [spec F86](../specs/file-rename.md) · [architecture](../architecture/file-rename.md) · [handoff](../design/file-rename-handoff.md) · testing-strategy § (pending)   <!-- links; source of truth for *what*; this file is source of truth for *where it stands* -->

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

1. [ ] [testing] Strategy: read-back refusal, sanitisation, collisions, crash recovery, echo suppression — `docs/testing-strategy.md`
2. [ ] [security] `/security-review` of metadata-derived names touching the filesystem — `docs/architecture/security-perimeter.md`
3. [ ] [—] Owner test: rename one file by hand, check the media server's watch state; gates On-writeback/Job modes only — spec Open Questions
4. [ ] [—] Decide whether HOLODEX-192 (old rename-schema placeholder, now Relates-linked) closes as a duplicate of this epic
5. [ ] [backend] Build Plan then Manual modes first (spec Timeline); the rename keys are the first on the settings allowlist — `internal/`
6. [ ] [frontend] Plan, drift queue, lock, undo and template editor per the handoff — `web/src/`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-10-31 · session
- skills: architecture, code-review, handoff, design-handoff, implement
- handoff: Crossed into build — design signed off at e4aeee37 (handoff + 3 SVGs: File names tab, plan with Out of date filter, video-page File name row); origin/main merged in (security-perimeter.md kept both the rename and HOLODEX-599 poster sections); draft PR open. Start at the testing strategy.

### 2026-10-27 · session
- skills: write-spec, code-review, handoff
- handoff: Spec F86 landed (docs/specs/file-rename.md); owner chose refuse-on-bad-read-back, ordered templates, held original blocks rename, undo latest only — next is the architecture gate.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
