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
8. [ ] [backend] `/handoff` skill → batch 2 (own slice) — **do this one first**, see retro verdict below
9. [ ] [backend] `INBOX.md` + `/triage` → batch 2 (own slice) — unblocked, retro completed 2026-07-29

## Batch-1 retro checkpoint (unblocks item 9) — ✅ completed 2026-07-29

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

### 2026-07-29 · Batch-1 retro — completed, answered from evidence + Kevin
- handoff: Trigger fired on the hard-stop date alone (2026-07-25, four days past); answered the
  four questions from real evidence in this repo's other worklogs and git history rather than
  vibes, plus one direct answer from Kevin. **Verdict: batch 1 works when it runs, but the retro
  surfaced a real gap that item 8 (`/handoff`) exists to close — do it before item 9.**
  - **SessionStart — In Progress fire + banner:** *Mixed.* At least one documented miss —
    [HOLODEX-186](HOLODEX-186.md)'s 2026-07-13 log: "Jira `In Progress` still not fired... no
    local `JIRA_*` creds this session" — a real reliability gap, though the hook's own
    soft-fail-offline design degrades safely rather than corrupting state. **Banner: Kevin
    confirms it's useful** — read and oriented from, not skipped. Keep as-is.
  - **Stop nag — catch, false-fire, or miss?** No logged false-fires or missed code-vs-worklog
    staleness. But the nag's scope is narrower than "worklog is stale" — it only compares a
    changed file's mtime against the worklog's, which structurally **cannot** catch worklog-vs-
    **Jira** drift (the epic's real status diverging from what the worklog frontmatter/gates
    say). That exact drift is what today's session existed to fix, across four separate plan
    files ([HOLODEX-193](HOLODEX-193.md), [HOLODEX-220](HOLODEX-220.md),
    [field-source-of-truth-rollout.md](field-source-of-truth-rollout.md),
    [studio-entity-implementation.md](studio-entity-implementation.md)) — none of it was code-
    freshness staleness, so the Stop hook had no way to see it. Not a bug; a scope gap worth
    naming.
  - **PostToolUse gate-flips — track reality or drift?** Drifted, and the mechanism explains why:
    a skill run flips a gate to `[/]`, but only `/handoff` sets `[x]` — and `/handoff` doesn't
    exist yet (item 8). HOLODEX-193's six gates sat at `[ ]`/`[/]` despite the epic being fully
    shipped and **Released** in Jira, because nobody had a tool to close them out. Separately,
    real worklog-parser bugs landed and got fixed post-batch-1 (`e04837e` — `--!>` HTML-comment
    masking, `5b78543` — commented-out examples, `1e210b4` — trailing YAML comments; `08d57a5`/
    `4b63d6e` — a missing `design` gate added to the routing table), so early gate-flips could
    have been misread before those fixes landed. Current parser is fixed; historical drift isn't
    retroactively correctable, just noted.
  - **Workarounds — manual steps a hook should've done?** Yes, concretely: this entire session.
    A human had to manually diff four plan files' `status:` fields against live Jira and
    hand-correct them, because nothing periodically reconciles worklog state against tracker
    ground truth — that's exactly the `/handoff` gap above, at epic-closure scale rather than
    real-time. Also HOLODEX-186's "flagged to the user, not actionable from here" note is itself
    a manual workaround substituting for automated recovery.
  - **Decision:** item 9 (`INBOX.md`/`/triage`) proceeds as originally scoped — the retro found
    no reason to change its shape, it addresses a different problem (idea capture) than what
    drifted. But **item 8 (`/handoff`) is more urgent than its queue position suggested** — it's
    the direct fix for the gate-closure and status-drift gap this retro surfaced, not just
    "batch 2, whenever." Build it first.

### 2026-07-11 · /security-review clean → merge #119
- skills: security-review
- handoff: `/security-review` on the batch-1 hooks came back clean — no HIGH/MEDIUM findings. All three flagged vectors are mitigated: `resolveKey` returns the regex-matched key substring (letters/digits/hyphen only) so the worklog path can't traverse; every subprocess is `execFileSync` argv (no shell) with the key passed as an env var, so no command injection; `JIRA_API_TOKEN` lives only in the Basic-auth header, never logged/urled/written. `security` gate + up-next item 6 flipped `[x]`. PR #119 merged → batch 1 landed on main. Next: live with the plumbing, then batch 2 — `/handoff` (item 8) then `INBOX.md`+`/triage` (item 9). Remaining open gate: `testing` (hook unit tests, batch 2).

### 2026-07-10 · Stop hook — worklog-staleness nag (batch-1 plumbing complete)
- skills: simplify
- handoff: Stop hook shipped (`flightplan/hooks/stop.mjs`) — nags via `systemMessage` when a changed file (git status) is newer than the worklog; stateless + self-clearing (worklog touch moves it ahead → quiet), `stop_hook_active`-guarded, exit-0, never re-enters Claude. /simplify dropped an early temp-file throttle (sole stateful wart; nag is user-only so repeats are cheap; ADR-064 defers any dampener to batch-4 tuning) and hoisted shared `hookout.mjs` (`emitJson`/`relPath`). Complementary SessionStart surface added: `parseWorklog.handoffPending` → banner shows "last session left no handoff" when the newest log entry has no handoff line. Tests green. **Batch 1 is done** (schema + config seam + all 3 hooks). Next: live with it, then batch 2 — `/handoff` (item 8) then `INBOX.md`+`/triage` (item 9). Pre-merge `security` gate (item 6) still open.

### 2026-07-10 · ADR-064 + worklog schema + SessionStart & PostToolUse hooks
- skills: architecture, simplify
- handoff: ADR-064 recorded (PR #119); batch-1 landed the worklog schema, SessionStart hook (In Progress + scaffold + orientation), and PostToolUse hook (skill-log + gate `[/]` flip) over shared `config.mjs`/`stdin.mjs`; all soft-fail offline and are unit-smoke-tested. Next: the `Stop` staleness nag (last batch-1 hook).
