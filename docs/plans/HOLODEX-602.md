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
    on: 2026-11-01
    at: 84362775
---

# HOLODEX-602 · Rename media files from a template (F86)

Done means the owner can rename library files in place from an ordered template list, through the
progressive plan → manual → writeback → job modes, without a video ever losing its identity or a
file ever being overwritten.

**Design package:** [spec F86](../specs/file-rename.md) · [architecture](../architecture/file-rename.md) · [handoff](../design/file-rename-handoff.md) · [testing-strategy §38](../testing-strategy.md#38-rename-media-files-from-a-template-f86-holodex-602)   <!-- links; source of truth for *what*; this file is source of truth for *where it stands* -->

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
- [/] backend → `{cmd,internal,providers}/**`. Plan (`acdf164d`) and Manual (`da951199`) are built and tested; R12/R24 gaps, On-writeback and Job remain (Up next).
- [ ] frontend → `web/src/**`
- [x] testing `testing-strategy`
- [/] security `security-review`. The design review (2026-10-31, `c25e21e4`) found no hole in the designed perimeter's intent, but nine gaps in how it was written. Each is closed in the doc that owns it: the template text and the whole name now face the character checks; the Windows reserved set is complete and judged before the first dot; invisible and invalid characters are removed or refused; the Linux root check is bound to a folder descriptor; the fallback `Lstat`s the target; the via name is built from the id; identity read-back is bounded; name state and job errors stay owner-only; and the apply body is capped. The tests are F7, F8, A8, A9 and §38.11. Held at `[/]`, as HOLODEX-58 and HOLODEX-493 did: the gate settles only on the implementation review of the backend diff.

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

1. [ ] [backend] Close Manual gaps: R12 identity test (thumbnail, montage, playlist, film link after rename+rescan), R24 one per-batch row with renamed count, hand-edit lock via the fold path — `internal/rename/`
2. [ ] [security] `/security-review` of the backend diff against §38.11 and the rename perimeter before marking ready; then tick the gate and clear `needs-security-review` — `docs/architecture/security-perimeter.md`
3. [ ] [frontend] Plan (incl. the Out of date filter), lock, undo and template editor per the handoff; APIs: `/rename/plan`, `/rename/plan/preview`, `/rename/apply`, `/admin/settings/{key}`, `/media/{id}/rename/*` — `web/src/`
4. [ ] [testing] Confirm F1/F8 (Linux-only primitive tests) pass in CI; the Windows dev box can't run them — `internal/rename/fs_linux_test.go`
5. [ ] [—] Owner yes on two spec wording fixes: P0-3's `]` example (only `] ` refuses under lazy capture) and P0-1's "as F48 refuses" (F48 accepts unknown tokens; rename refuses them) — `docs/specs/file-rename.md`
6. [ ] [testing] Pin undo-of-undo: the build allows undoing an undo (it is the latest rename), closing §38.10's open case — `internal/rename/worker_test.go`
7. [ ] [—] Owner test: rename one file by hand, check the media server's watch state; gates On-writeback/Job modes only — spec Open Questions
8. [ ] [backend] On-writeback mode: rename step inside the write job, after WriteBatch, before PostWriteFunc (R23) ⛔ blocked on item 7 — `internal/writequeue/`
9. [ ] [backend] Job mode: settle-window sweep over `video_file_names.desired_*` (R14, sweep half of R24) ⛔ blocked on item 7 — `internal/rename/`
10. [ ] [—] Decide whether HOLODEX-192 (old rename-schema placeholder, now Relates-linked) closes as a duplicate of this epic
11. [ ] [—] `TestSyncTagWritebackBulk_DedupsSharedVideo` fails on this Windows box at the pre-feature commit too (3 s drain deadline); not this epic's — `internal/api/tag_writeback_sync_test.go`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-11-01 · backend Plan + Manual
- skills: code-review, handoff
- handoff: The backend for Plan (`acdf164d`) and Manual (`da951199`) is in: migration 0068, the settings allowlist with the rename keys, builder and read-back, collision plan, plan/preview/apply/undo/lock routes, the no-overwrite primitive, the journalled worker with boot recovery, scanner and echo integration. `backend` stays `[/]` on the R12/R24 gaps and the blocked On-writeback/Job modes. Next is Up next 1, then the backend security review.

### 2026-11-01 · session
- skills: code-review, handoff
- handoff: The collision disambiguator is now ` {hx-<id>}` (braces, matching the marker grammar) across spec, arch, design and testing (`84362775`), and the whole-name check says braces are the builder's own; the owner re-confirmed the design at 84362775. Next is the backend: Plan, then Manual.

### 2026-10-31 · security review (design)
- skills: code-review, handoff
- handoff: The design-time security review is done (`c25e21e4`). Nine perimeter gaps are fixed in the arch, spec and testing docs, and §38.11 records them. The gate stays `[/]` until the implementation review of the backend diff (Up next 6), so `needs-security-review` stays on the ticket. The design sign-off still needs the owner's yes to re-pin `at:`; no file-rename design file changed since e4aeee37. Next is the backend: Plan, then Manual.

### 2026-10-31 · testing strategy
- skills: testing-strategy, code-review, handoff
- handoff: Testing gate done — §38 of docs/testing-strategy.md lays out builder/read-back, no-overwrite, crash-recovery, identity, echo, drift, lock and undo tests by layer, with a 21-item QA list and the order they land per mode. Design sign-off is flagged stale only by main-merge changes to other features' design docs (no file-rename-* file changed since e4aeee37); re-pin `at:` needs the owner's yes before ready. Next is the security review.

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
