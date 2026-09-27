---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-469                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: infra               # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are trimmed to match.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: The first owner page after a restart no longer stalls while every completeness score is recomputed; scores are recomputed only after an upgrade or a config change, and then in the background.
---

# HOLODEX-469 · First owner list page after a restart stalls on a full completeness re-score

Done means an ordinary restart does no completeness work, so the first owner People or Media load
is as fast as the second. A new build or a changed config re-scores everything once, in the
background, and pages opened meanwhile show the previous rings instead of waiting.

**Design package:** [ADR-112](../architecture/ADR-112-completeness-boot-fingerprint.md) (supersedes
ADR-099 D4's boot hook only) · [spec note](../specs/entity-completeness-score.md) ·
[testing-strategy § Completeness store — boot re-score](../testing-strategy.md)

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default — the list below is only what you get with no config. Once `profile:` is
     set, trim them to that posture. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] architecture `architecture` → `docs/architecture/ADR-*` — ADR-112 (supersedes ADR-099 D4's boot hook only)
- [x] backend → `{cmd,internal,providers}/**`
- [x] testing `testing-strategy`
- [~] security `security-review` — touches no auth, access, network or request input: boot reads its own executable and the two config files it already loads, and writes one internal `settings` key that no handler reads or exposes

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge #401 on Kevin's ping; CI moves HOLODEX-469 to Done (branch-keyed, no children)
2. [ ] [—] After deploy: restart twice; the second boot must NOT log "completeness inputs changed", and the first owner People load is instant
3. [ ] [—] Batch the per-video genre-writeback lookups in `completenessForVideos` → HOLODEX-470

## Session log — newest first

### 2026-09-27 · session
- skills: code-review high --fix, handoff, implement
- Diagnosed the post-restart People-list stall from prod aggregates (every entity re-scored in one ~8 s synchronous drain); filed HOLODEX-469 + follow-up HOLODEX-470; ADR-112 D1 boot fingerprint (executable + mappings + sources) and D2 background boot drain with request-path skip; spec + testing-strategy notes.
- handoff: Crossed into build (no approve gates in the infra posture) and every gate is settled; PR #401 marked ready. Next: squash-merge on Kevin's ping, then verify on prod with two restarts.

## Dropped — newest first (the reason is the point)
