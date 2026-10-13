---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-568                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: A video whose file you move or rename on disk now keeps its place and everything you curated, instead of reappearing as a new video.  # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-568 · Keep a video's identity when its file is moved or renamed

Done when a moved or renamed file keeps its video (id, Date-added place, curation, links), and the
~232 past moves on production are healed automatically into their original videos.

**Design package:** [spec F79](../specs/video-identity-on-move.md) · [media-ingest.md § moves](../architecture/media-ingest.md#moves-recognised-by-a-sampled-content-hash-matched-after-the-walk) · no design handoff (no UI) · testing-strategy § (not yet written)

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
- [~] design `design-handoff` → `docs/design/**` — no UI surface (scan-summary counts ride the existing System activity line) — until: P1-1 move history needs a visible surface
- [/] backend → `{cmd,internal,providers}/**`
- [/] frontend → `web/src/**` — one display fix: the Status digest composes a failed scan's detail (move counts) with its error, as JobHistory does
- [/] testing `testing-strategy`
- [~] security `security-review` — no auth, access or perimeter change; scanner reads local library files only

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

1. [ ] [testing] Live QA §26.6 items 1–3 on a backend testbed (move a folder, copy, failed scan + move in the Status digest) — `docs/testing-strategy.md`
2. [ ] [—] Measure how many of the 232 hidden rows carry curation/links/enrichment (read-only prod probe, anonymized)
3. [ ] [backend] Share one file-layer SET list between `UpsertVideo` and `RepointVideo` so a new column can't go stale on moved rows — `internal/repo/repo.go`, `internal/repo/video_move.go`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-10-12 · own worktree + spec draft
- skills: write-spec, handoff, architecture, implement
- Moved 568 off the "Media sorting" session into its own worktree/branch; reserved F79. Owner decided: auto-heal past moves, newest owner edit wins on fold, ambiguity declined, no time limit on inactive matches; posture `full` with design/frontend/security skipped.
- Owner accepted the spec. Architecture (owner-chosen): SHA-256 over size + three 64 KiB samples; legacy hidden rows match on exact recorded facts; size-twin new files held and matched after the walk; fold reuses `planCarry`.
- Testing strategy §26 written (risks, near-twin fixtures, scanner/repo/integration tests, mutation checks); testing gate stays [/] until those tests exist.
- Built migration 0064 (`videos.content_fp` + file_size/content_fp indexes), `contentFingerprint` (SHA-256 over size + 3×64 KiB), fingerprint on every insert/re-extract/refresh, and the lazy backfill on the fast path; §26.3 fingerprint + backfill + refresh tests green; middle-sample mutation turns the near-twin test red. code-review high --fix: a fingerprint failure now stores NULL instead of hiding the file.
- Built hold-then-match: size-twin new files held, `matchHeld` repoints one-to-one pairs against missing rows (unseen-and-ENOENT or hidden) before `DeactivateExcept`, ambiguity declined; repo `HasSizeTwin` / `MoveCandidates` (fingerprint, or exact facts for NULL-fp rows) / `RepointVideo` (keeps id + indexed_at). §26.3 scanner + repo tests and a real-repo end-to-end move test green; 3 mutations red. code-review: Stat guard now requires ENOENT.
- Built the fold: `FoldPairs` (one-to-one, keep-separate and in-flight writeback/held-original pairs excluded) + `FoldMovedVideo` (re-keys enrichment per provider, dismissals, edits newer-wins via `applyCarry`, playlists/films/manual tags, writeback history/snapshots/reviews/job attribution, BEFORE deleting newer; survivor takes newer's file layer, keeps id + indexed_at); scanner `foldMoves` after deactivation + relink. Shared `sameContentSQL` now backs both MoveCandidates and the fold. Repo fold tests (schema-walking no-orphan check, atomicity) + real-repo heal test green; 4 mutations red. media-ingest.md + testing §26 updated (per-provider enrichment, thumbnails left like purge).
- Wired the counts: the scan's job row `detail` reads "N moved files recognised; M ambiguous matches skipped" (`moveDetail`), empty when both are zero; an in-pass ambiguous file counts once (real-repo test + mutation red). code-review: the Status digest's Recent Failures now shows detail AND error (was `detail || error`, which would hide a failed scan's error behind its move count) — `npm run check` clean, activity Vitest green; live render queued as §26.6 item 3.
- handoff: All backend work for F79 is in on draft PR #498 — next is live QA §26.6 items 1–3 on a backend testbed (Up next 1), then the post-567 merge items.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
