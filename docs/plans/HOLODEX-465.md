---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-465                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are trimmed to match.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note:                # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-465 · A genres writeback makes the file's tags match the UI

Done means after any genres writeback, every tag key the scanner reads (Genre, Genres, Keywords,
Category, Categories) holds only the UI's tags minus ignored ones. Ignored tags are UI-only and survive
rescans, and the MP4 tagline no longer leaks into tags (folds in HOLODEX-466).

**Design package:** [spec § Amendment](../specs/tag-writeback-exclusion.md#amendment--the-file-tag-contract-holodex-465) · [ADR-110](../architecture/ADR-110-tag-writeback-file-contract.md) · no UI (no handoff) · testing-strategy § pending   <!-- links; source of truth for *what*; this file is source of truth for *where it stands* -->

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default — the list below is only what you get with no config. Once `profile:` is
     set, trim them to that posture. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**`
- [x] architecture `architecture` → `docs/architecture/ADR-*`
- [~] design `design-handoff` → `docs/design/**` — no UI surface; until: HOLODEX-401 builds the dialog tags row
- [ ] backend → `{cmd,internal,providers}/**`
- [~] frontend → `web/src/**` — backend-only; the UI shows whatever tags the scanner reads
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

1. [ ] [backend] Probe exiftool write names for Keywords/Category/Genres on MP4+MKV and the MP4 tagline target (ADR-110 D2/D5) — `internal/writeback/tags.go`
2. [ ] [backend] Delete-capable FieldWrite on all three writers (exiftool `-TAG=`, ffmpeg `key=`, mkvpropedit drop Simple) — `internal/writeback/writeback.go`
3. [ ] [backend] Filter extra tag keys inside the genres job, matched via the name spine; ignored names off the raw side (D1/D2) — `internal/writequeue/writequeue.go`
4. [ ] [backend] Snapshot/audit/revert keyed by tag name for derived keys (D3) — `internal/writeback/snapshot.go`
5. [ ] [backend] On success, flip file→manual links for ignored tags (D4) — `internal/repo/video_tags.go`
6. [ ] [testing] Integration round trips per spec § Amendment checkboxes + testing-strategy row — `internal/writeback/`
7. [ ] [security] /security-review (file I/O + new key deletion) — `internal/writeback/`
8. [ ] [—] On merge, sweep HOLODEX-466 to Done by hand (folded in; CI only moves 465)

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-09-26 · session
- skills: implement
- decisions (owner): filter extra tag keys (not clear/mirror); attach/detach never write the file; fold HOLODEX-466 in; ignored tags are UI-only — removed from file, kept on the video
- handoff: Spec amendment + ADR-110 written and pushed, awaiting Kevin's read of them before crossing to build — then start at up-next #1, the exiftool key-name probe.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
