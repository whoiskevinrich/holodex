---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-556                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: "Owners can flip a mirrored video horizontally from its details page; every tag, track and chapter is kept, and the original is held for a week in case you want it back."
approved:
  design:
    on: 2026-10-11
    at: d58dadc4
---

# HOLODEX-556 · F77 Video file maintenance — flip horizontally

Done when the owner can flip a video's picture from the Media Details page, with the new file verified
against the original (streams, tags, chapters, picture shape) before it replaces it, and the original
held for a configurable window with Restore and Keep change. Aspect-ratio operations are later entries.

**Design package:** [spec](../specs/video-file-maintenance.md) · [file-maintenance.md](../architecture/file-maintenance.md) · [handoff](../design/video-file-maintenance-handoff.md) · [testing-strategy §25](../testing-strategy.md)

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default; an empty roster leaves no rows at all, and status then comes from the
     release note alone. The rows below are only the unscaffolded template's. Once `profile:` is set,
     match them to that posture, adding rows as well as removing them. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**` — video-file-maintenance.md (F77), Q1–Q3 resolved
- [x] architecture `architecture` → `docs/architecture/**` — file-maintenance.md (new topic doc) + writeback.md lane note
- [x] design `design-handoff` → `docs/design/**` — handoff + mockup SVG; signed off 2026-10-10, re-confirmed 2026-10-11 at d58dadc4
- [x] backend → `{cmd,internal,providers}/**` — migration 0062, queue lanes, `internal/maintenance` (pipeline, verifier, swap, worker, expiry), owner API, purge; proven on real MP4 and five MKV shapes in the runtime image
- [x] frontend → `web/src/**` — Maintenance section + dialogs; live-QA'd; owner confirmed the build matches the approved mockup 2026-10-11
- [x] testing `testing-strategy` — §25: unit, API, live QA and the `make test-image` real-file matrix all pass; coverage 87.3% vs a 90% target (gap = deep tool-failure branches, recorded in §25.9)
- [x] security `security-review` — design review 2026-10-10 and implementation review 2026-10-11: no findings ≥8/10, all 7 design checks pass

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

1. [ ] [backend] Merge origin/main: renumber `0062_file_maintenance` → `0063` (+ `TestMigration0062…` → 0063, Migrate(61/62) → 62/63), resolve `purge_test.go` (keep both fakes' methods), rerun suites, then `gh pr ready 486` — `internal/db/migrations/`, `internal/purge/`
2. [ ] [—] Sweep HOLODEX-556 to In Review on ready and to Done when #486 merges (epics skip jira-sync) — Jira
3. [ ] [testing] Owner: watch a flipped real library file end to end, no visible quality loss, audio in sync (§25.7 item 12) — after merge

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-10-11 · maintenance package: probe, encoder table, argv, verifier
- skills: code-review, security-review, handoff
- handoff: Every gate settled (owner confirmed the build matches the mockup; implementation security review clean) but #486 stays draft — merging origin/main conflicts: `internal/purge/purge_test.go`, and main's `0062_film_soft_delete` collides with our `0062_file_maintenance`. Start at Up next #1: renumber ours to 0063, resolve, merge, then mark ready.

### 2026-10-10 · F77 design package: spec, design, architecture, testing, security
- skills: write-spec, design-handoff, architecture, testing-strategy, security-review, handoff, implement, code-review
- handoff: Build started on PR #486 — migration 0062 (`writeback_queue.kind`, `held_originals`) and lane-aware claims with one-running-job-per-file are in and tested (mutation-checked); start at Up next #1, the `internal/maintenance` package and its worker.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
