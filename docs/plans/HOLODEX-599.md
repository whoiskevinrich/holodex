---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-599                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Choose a video's poster in the writeback dialog from the file's cover art, a frame you picked, your upload, the automatic frame or a provider poster, and it is written into the file on the next writeback.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
approved:
  design:
    on: 2026-10-27
    at: d8d11ea5
---

# HOLODEX-599 · Choose the video poster in the writeback dialog

The writeback dialog's Poster row offers every poster the owner can see: the file's cover art, the
picked frame, the upload, the automatic frame and each provider's poster. A frame pick or an upload
shows on the page at once and is written into the file on the next writeback. Child: HOLODEX-597.
Follow-up: HOLODEX-600 (several posters per provider).

**Design package:** [spec F85](../specs/writeback-poster-sources.md) · architecture (a29ef32b: [media-ingest.md](../architecture/media-ingest.md), [writeback.md](../architecture/writeback.md), [field-resolution.md](../architecture/field-resolution.md), [security-perimeter.md](../architecture/security-perimeter.md)) · [handoff](../design/writeback-poster-chooser-handoff.md) · testing-strategy § to come

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

1. [ ] [testing] /testing-strategy for F85; first a failing test for writeback overwriting an upload — `docs/testing-strategy.md`
2. [ ] [backend] Frame and upload as poster sources; writeback embeds local images — `internal/writeback/writeback.go`
3. [ ] [frontend] Grouped poster tiles; relabel the page button "Pick a frame" — `web/src/lib/components/curation/SourceImageTiles.svelte`
4. [ ] [security] /security-review of the local-image write path — `docs/architecture/security-perimeter.md`
5. [ ] [—] Sweep child HOLODEX-597 with the epic: In Review when the PR is ready, Done on merge — HOLODEX-597

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-10-27 · session
- skills: product-brainstorming, write-spec, code-review, architecture, design-handoff, handoff, implement
- handoff: Crossed into build: design was signed off at d8d11ea5 and the Draft PR is open. Start with /testing-strategy for F85, beginning with a failing test for writeback overwriting an upload.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
- [~] [design] Poster picker on the media page — dropped 2026-10-27, the owner chose to pick posters only in the writeback dialog (option A)
