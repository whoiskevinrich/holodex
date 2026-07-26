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
- [/] backend — template + config seam + `jira-transition.mjs` + all three batch-1 hooks (SessionStart, PostToolUse, Stop) + shared `config.mjs`/`stdin.mjs` done; batch-1 plumbing complete, batch 2 (`/handoff`, `/triage`) remains
- [x] frontend — n/a: no web UI surface
- [ ] testing `testing-strategy` — hook unit tests (batch 2)
- [x] security `security-review` — clean (2026-07-11): key = matched substring under letters/digits/hyphen regex ⇒ no path traversal; argv (non-shell) spawns ⇒ no command injection; token confined to the auth header

## Up next — ordered (position = priority)

1. [x] [backend] SessionStart hook — branch-key → In Progress + scaffold + orientation banner — `flightplan/hooks/session-start.mjs`
2. [x] [backend] PostToolUse(Skill) hook — append skill runs to the session log; flip the gate to `[/]` — `flightplan/hooks/post-tool-use.mjs`
3. [x] [backend] Extract shared `config.mjs` (config + key + gates) & `stdin.mjs`; both hooks reuse them — `flightplan/hooks/`
4. [x] [backend] `Stop` hook — staleness nag (worklog behind code) + SessionStart "left no handoff" surface — `flightplan/hooks/stop.mjs`
5. [/] [testing] Hook unit tests — `parseWorklog`/`section`/`logSkillRun`/`flipGate` **done** in
   `flightplan/lib/worklog.test.mjs` (wired into `make test-scripts`); **config gates parse still open**
6. [x] [security] `/security-review` — clean; the matched-substring key charset already blocks traversal/injection
7. [x] [backend] Collapse `worklog.mjs`/`scripts/whats-left.mjs` onto one parser (shared schema) —
   done: `flightplan/lib/worklog.mjs` is canonical; both are now thin consumers
8. [ ] [backend] `/handoff` skill → batch 2 (own slice)
9. [ ] [backend] `INBOX.md` + `/triage` → batch 2 (own slice)  ⛔ blocked on the batch-1 retro checkpoint below

## Batch-1 retro checkpoint (unblocks item 9)

"Living with batch 1" was an open-ended vibe with no exit signal. Replacing it with a mechanical
trigger + a fixed set of questions, so nobody has to remember to check in:

- **Trigger** (first to occur): **3 real sessions** on this repo since PR #119 merged (2026-07-11),
  or **2026-07-25** (2-week hard stop) — checkable from `git log`/session-log count, not a feeling.
- **Who gathers what:** no passive feedback collection runs in between — the only ambient signal
  already captured is the `PostToolUse` skill-run log (existing session-log entries below). Kevin
  isn't expected to track anything as it happens. At the trigger, answer these once, append as a
  session-log entry, and decide from that whether item 9 opens as originally scoped:
  - **SessionStart** — did `In Progress` fire reliably? Was the orientation banner read/useful, or
    ignored noise?
  - **Stop nag** — did it catch real staleness, fire falsely, or miss staleness it should have caught?
  - **PostToolUse gate-flips** — did `[/]` transitions track what actually happened, or drift?
  - **Workarounds** — any manual step done by hand that a hook should have done instead?

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-07-11 · /security-review clean → merge #119
- skills: security-review
- handoff: `/security-review` on the batch-1 hooks came back clean — no HIGH/MEDIUM findings. All three flagged vectors are mitigated: `resolveKey` returns the regex-matched key substring (letters/digits/hyphen only) so the worklog path can't traverse; every subprocess is `execFileSync` argv (no shell) with the key passed as an env var, so no command injection; `JIRA_API_TOKEN` lives only in the Basic-auth header, never logged/urled/written. `security` gate + up-next item 6 flipped `[x]`. PR #119 merged → batch 1 landed on main. Next: live with the plumbing, then batch 2 — `/handoff` (item 8) then `INBOX.md`+`/triage` (item 9). Remaining open gate: `testing` (hook unit tests, batch 2).

### 2026-07-10 · Stop hook — worklog-staleness nag (batch-1 plumbing complete)
- skills: simplify
- handoff: Stop hook shipped (`flightplan/hooks/stop.mjs`) — nags via `systemMessage` when a changed file (git status) is newer than the worklog; stateless + self-clearing (worklog touch moves it ahead → quiet), `stop_hook_active`-guarded, exit-0, never re-enters Claude. /simplify dropped an early temp-file throttle (sole stateful wart; nag is user-only so repeats are cheap; ADR-064 defers any dampener to batch-4 tuning) and hoisted shared `hookout.mjs` (`emitJson`/`relPath`). Complementary SessionStart surface added: `parseWorklog.handoffPending` → banner shows "last session left no handoff" when the newest log entry has no handoff line. Tests green. **Batch 1 is done** (schema + config seam + all 3 hooks). Next: live with it, then batch 2 — `/handoff` (item 8) then `INBOX.md`+`/triage` (item 9). Pre-merge `security` gate (item 6) still open.

### 2026-07-10 · ADR-064 + worklog schema + SessionStart & PostToolUse hooks
- skills: architecture, simplify
- handoff: ADR-064 recorded (PR #119); batch-1 landed the worklog schema, SessionStart hook (In Progress + scaffold + orientation), and PostToolUse hook (skill-log + gate `[/]` flip) over shared `config.mjs`/`stdin.mjs`; all soft-fail offline and are unit-smoke-tested. Next: the `Stop` staleness nag (last batch-1 hook).
