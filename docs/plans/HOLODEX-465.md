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
release_note: Writing tags back to a file now makes its tags match Holodex — tags you removed or turned off for writeback are also cleared from Keywords and Category, so a rescan no longer brings them back, and MP4 taglines no longer turn into tags. # the ONE user-facing sentence; authored once by /handoff, flows to the
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

**Design package:** [spec § Amendment](../specs/tag-writeback-exclusion.md#amendment--the-file-tag-contract-holodex-465) · [ADR-110](../architecture/ADR-110-tag-writeback-file-contract.md) · no UI (no handoff) · [testing-strategy § file tag contract](../testing-strategy.md)   <!-- links; source of truth for *what*; this file is source of truth for *where it stands* -->

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
- [x] backend → `{cmd,internal,providers}/**`
- [~] frontend → `web/src/**` — backend-only; the UI shows whatever tags the scanner reads
- [x] testing `testing-strategy`
- [x] security `security-review`

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

1. [ ] [—] Mark PR #397 ready once CI is green; on merge sweep HOLODEX-466 to Done by hand (CI only moves 465) — `docs/plans/HOLODEX-465.md`
2. [ ] [backend] Verify an mkvmerge-written MKV (TagLanguage=und → `Keywords-und`?) is caught by ReadTagKeys — needs MKVToolNix — `internal/writeback/tagkeys.go`
3. [ ] [backend] Snapshot keys derived writes by bare name: two groups holding one key share a prior value on revert — `internal/writeback/snapshot.go`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-09-26 · session
- skills: implement, code-review, security-review
- decisions (owner): filter extra tag keys (not clear/mirror); attach/detach never write the file; fold HOLODEX-466 in; ignored tags are UI-only — removed from file, kept on the video
- build: probe found MP4 Keywords lives in the `Keys` group (write back under the reported Group:Name); round-trip test found the scanner reading MKV list-valued Keywords as one `[Drama Heist]` tag (fixed, D7); empty unions now clear the file (D6)
- handoff: All gates settled — ADR-110 D1–D7 built and tested (unit + real-file integration on MKV and MP4), security review clean; next move is marking PR #397 ready and merging, then sweeping HOLODEX-466 by hand.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
