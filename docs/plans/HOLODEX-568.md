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
- [ ] backend → `{cmd,internal,providers}/**`
- [~] frontend → `web/src/**` — no UI surface — until: P1-1 move history needs a visible surface
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

1. [ ] [backend] Migration: `videos.content_fp` + sampled-hash helper + lazy backfill in the scanner worker — `internal/scanner/`, `internal/db/migrations/`
2. [ ] [backend] Hold size-twin new files, match after the walk, repoint before `DeactivateExcept` — `internal/scanner/scanner.go`
3. [ ] [backend] Post-deactivation fold reusing `planCarry` (newest `created_at`/`fetched_at` wins) + legacy exact-facts rule — `internal/repo/`
4. [ ] [backend] Repointed/folded/ambiguous counts on the scan job row — `internal/scanner/`
5. [ ] [—] Measure how many of the 232 hidden rows carry curation/links/enrichment (read-only prod probe, anonymized)
6. [ ] [spec] After HOLODEX-567 merges and main is merged in: edit F12.7/F12.8 in place to drop their "until HOLODEX-568" clauses (moves keep their place; past moves heal) — `docs/specs/phase-2-mcp-polish.md`
7. [ ] [backend] Expect conflicts with HOLODEX-567 in `UpsertVideo` (conditional `indexed_at` bump) and the scanner — `internal/repo/repo.go`, `internal/scanner/scanner.go`

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
- handoff: In build on draft PR #498 — testing-strategy §26 is the test plan; start at the content_fp migration + sampled-hash helper (Up next 1), writing §26.3's fingerprint tests first.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
