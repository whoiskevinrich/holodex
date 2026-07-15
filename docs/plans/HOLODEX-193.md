---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-193                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note:                # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
---

# HOLODEX-193 · On-demand metadata extraction from filenames & tags (F48)

Filenames become a real, structured metadata source (parsing + confidence-gated auto-apply/review),
and Person/Studio merges propagate to file tags automatically, with a snapshot/revert safety net.
Done means F48.1–F48.11 all shipped in code (docs already landed pre-branch) and the ADR-067 Action
Items closed.

**Design package:** [spec](../specs/metadata-extraction.md) ·
[ADR-067](../architecture/ADR-067-filename-extraction-confidence-and-rollback.md) ·
[design handoff](../design/metadata-extraction-handoff.md) ·
[testing-strategy](../testing-strategy.md) (§4/§5/Phase 3)

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [ ] spec `write-spec` → `docs/specs/**`
- [ ] architecture `architecture` → `docs/architecture/ADR-*`
- [ ] backend
- [ ] frontend
- [ ] testing `testing-strategy`
- [/] security `security-review`

<!-- Deferred gate example — carry the un-defer trigger so it isn't lost:
- [~] security `security-review` — until: a mutation endpoint exists (read-only slice so far) -->

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [backend] Phase 1 — filename pattern parsing (F48.1) + `filename:` shadow-store write path
   (F48.2), pure/unit-tested — `internal/extract/`
2. [ ] [backend] Phase 2 — confidence scoring (F48.3) + auto-apply/review routing (F48.4), behind a
   flag with auto-apply log-only until ADR-067 is Accepted — new `metadata_extraction_review` table
   (migration 0025)
3. [ ] [backend] Phase 3 — rollback foundation (F48.9), `file_writeback_snapshots` (migration 0026) —
   must land before Phase 4 enables auto-apply
4. [ ] [backend] Phase 4 — extraction triggers (F48.5): on-demand → batch → import-time, in that order
5. [ ] [frontend] Phase 5 — Extraction tab UI + preview (F48.6/F48.7)  ⛔ blocked on #2–#4
6. [ ] [backend] Phase 6 — merge → writeback propagation (F48.8)  ⛔ blocked on #3

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-14 · session
- skills: simplify, security-review

### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->
