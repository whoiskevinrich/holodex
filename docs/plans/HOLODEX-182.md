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
- [/] backend — worklog template + config seam + `jira-transition.mjs` + SessionStart hook done; PostToolUse + Stop hooks remain
- [x] frontend — n/a: no web UI surface
- [ ] testing `testing-strategy` — hook unit tests (batch 2)
- [ ] security `security-review` — branch-name → REST-call injection, token handling, no token in logs

## Up next — ordered (position = priority)

1. [x] [backend] SessionStart hook — branch-key → In Progress + scaffold + orientation banner — `flightplan/hooks/session-start.mjs`
2. [ ] [backend] PostToolUse(Skill) hook — append skill runs to the session log; flip the gate to `[/]` — `flightplan/hooks/`
3. [ ] [testing] Hook unit tests — `parseWorklog` / `section` / banner + transition soft-fail
4. [ ] [security] `/security-review` — branch-name → REST injection (anchor the key regex), token never logged
5. [ ] [backend] Extract `flightplan/lib/` (structured config + shared config/branch-key/parser helpers) so PostToolUse reuses them and `worklog.mjs`/`scripts/whats-left.mjs` collapse onto one parser — do when #2 lands
6. [ ] [backend] `/handoff` skill + `Stop` staleness nag → batch 2 (own slice)
7. [ ] [backend] `INBOX.md` + `/triage` → batch 2 (own slice)  ⛔ blocked on living with batch 1 first

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-07-10 · ADR-064 + worklog schema + SessionStart hook
- skills: architecture
- handoff: ADR-064 recorded (PR #119); worklog template/config/example landed (batch 1.1); SessionStart hook + jira-transition.mjs built and smoke-tested — In Progress soft-fails cleanly without local creds. Next: PostToolUse(Skill) hook.
