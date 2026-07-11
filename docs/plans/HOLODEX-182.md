---
key: HOLODEX-182
status: in-progress
depends-on: []
release_note: Flightplan plugin — per-epic session-state worklogs that survive session boundaries.
---

# HOLODEX-182 · Flightplan — portable session-state plugin

Turn this repo's hand-rolled per-epic worklogs into a config-driven plugin: an on-disk worklog as
ground truth, mechanical hooks that fire status + orientation, and `/handoff`/`/triage` for the
judgment surfaces. **Done** = batches 1 and 2 shipped and lived-with; the plugin orients and
tracks HOLODEX epics with no reliance on agent memory.

**Design package:** [ADR-064](../architecture/ADR-064-flightplan-plugin.md) · [one-pager](flightplan-plugin.md) · prior art: [field-source-of-truth-rollout.md](field-source-of-truth-rollout.md), [studio-entity-implementation.md](studio-entity-implementation.md)

## Gates — definition of done

- [x] spec — n/a: infra/tooling, ADR-064 records no product spec (§Spec: none)
- [x] architecture `architecture` → [ADR-064](../architecture/ADR-064-flightplan-plugin.md)
- [/] backend — template + config seam + `jira-transition.mjs` + SessionStart & PostToolUse hooks + shared `config.mjs`/`stdin.mjs` done; Stop hook remains
- [x] frontend — n/a: no web UI surface
- [ ] testing `testing-strategy` — hook unit tests (batch 2)
- [ ] security `security-review` — branch-name → REST-call injection, token handling, no token in logs

## Up next — ordered (position = priority)

1. [x] [backend] SessionStart hook — branch-key → In Progress + scaffold + orientation banner — `flightplan/hooks/session-start.mjs`
2. [x] [backend] PostToolUse(Skill) hook — append skill runs to the session log; flip the gate to `[/]` — `flightplan/hooks/post-tool-use.mjs`
3. [x] [backend] Extract shared `config.mjs` (config + key + gates) & `stdin.mjs`; both hooks reuse them — `flightplan/hooks/`
4. [ ] [backend] `Stop` hook — staleness nag (code touched, worklog not updated) — last batch-1 hook
5. [ ] [testing] Hook unit tests — `parseWorklog`/`section`/`logSkillRun`/`flipGate` + config gates parse (batch 2)
6. [ ] [security] `/security-review` — branch-name → REST injection (anchor the key regex), token never logged
7. [ ] [backend] Collapse `worklog.mjs`/`scripts/whats-left.mjs` onto one parser (shared schema) — follow-up
8. [ ] [backend] `/handoff` skill → batch 2 (own slice)
9. [ ] [backend] `INBOX.md` + `/triage` → batch 2 (own slice)  ⛔ blocked on living with batch 1 first

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-07-10 · ADR-064 + worklog schema + SessionStart & PostToolUse hooks
- skills: architecture, simplify
- handoff: ADR-064 recorded (PR #119); batch-1 landed the worklog schema, SessionStart hook (In Progress + scaffold + orientation), and PostToolUse hook (skill-log + gate `[/]` flip) over shared `config.mjs`/`stdin.mjs`; all soft-fail offline and are unit-smoke-tested. Next: the `Stop` staleness nag (last batch-1 hook).
