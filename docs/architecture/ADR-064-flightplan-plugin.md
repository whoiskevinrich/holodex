# ADR-064: Flightplan — a portable session-state plugin (mechanical worklog + hooks over judgment)

**Status:** Proposed
**Date:** 2026-07-10
**Deciders:** Project owner
**Extends:** [ADR-058](ADR-058-jira-transitions-via-rest-api.md) (Jira REST transitions — Flightplan's `SessionStart` hook fires `In Progress` over the same uncounted REST path, not Automation; this ADR does not re-derive that mechanism). **Relates to:** [ADR-044](ADR-044-automated-version-and-release-pr.md) (Release Please's Done/Released split — the worklog's `release_note` promotion threads through it), `docs/reference/jira-pipeline.md` (the transition pipeline Flightplan's hook calls into). **Spec:** none — infra/tooling with no end-user-facing surface; the working agreement it automates is documented in CLAUDE.md, not a product spec. **Prior art:** [`field-source-of-truth-rollout.md`](../plans/field-source-of-truth-rollout.md), [`studio-entity-implementation.md`](../plans/studio-entity-implementation.md) — hand-rolled flight plans this formalizes; their existence and continued use is the evidence base. **One-pager this ADR supersedes as the design-of-record:** [`docs/plans/flightplan-plugin.md`](../plans/flightplan-plugin.md). **Issue:** [HOLODEX-182](https://whoiskevinrich.atlassian.net/browse/HOLODEX-182) (epic).

---

## Context

Large epics span many Claude Code sessions. Sessions die abruptly — context limit, crash, closed
tab — so any state that lived only in the agent's head is lost between them. In practice this shows
up as: Jira statuses going unmarked and sub-tickets drifting; cross-session, cross-layer handoffs
(architecture in one session, backend in another, frontend in a third) failing because each fresh
session re-derives "where was I" from scratch by reading diffs, ADRs, and Jira; no record of which
skills ran in a session; and ideas surfaced mid-flow evaporating when the session ends because
capture depends on stopping to file them.

Two hand-rolled worklogs already exist in this repo (`field-source-of-truth-rollout.md`,
`studio-entity-implementation.md`) — session graphs + update protocols + per-session status,
maintained by hand because the pain of *not* having them was worse. That they work is the evidence
Flightplan is the right shape; the ADR's job is to stop requiring a human to remember to write them.

### The load-bearing principle

> Never let durable state depend on agent discipline. Either derive it from an artifact (commit,
> branch, PR, file-on-disk), or fire it from a hook. The agent is allowed to *think*, never to
> *remember*.

The evidence for this principle is already in the repo: of the four Jira transitions in
[ADR-058](ADR-058-jira-transitions-via-rest-api.md), the three fired by CI on a git event
(`In Review`/`Done`/`Released`) never fail; the one fired by agent judgment at start-of-work
(`In Progress`) is the unreliable one. Flightplan generalizes that lesson from one transition to the
whole session-state problem: push what a hook can fire mechanically, and reserve the agent's
judgment for the one thing it's actually needed for — deciding a gate is *done*, not just touched.

### Constraints / forces

- **Token budget is the scarce resource, not developer time.** A fresh session re-orienting from
  nothing reads code, diffs, and ADRs to reconstruct state — thousands of tokens, and often wrong.
  Per this repo's own "Context discipline" rule (CLAUDE.md), that cost is paid on *every* turn of the
  session it happens in, not once — so cheap orientation is not a nice-to-have, it's a budget fix.
- **Hooks must stay mechanical.** CLAUDE.md's Hook Safety rule forbids a hook command that re-enters
  Claude (fork-bomb risk). This is a hard boundary on what Flightplan's hooks can do: they can append,
  transition, nag, and scaffold; they can never write the "is this epic actually done" judgment call.
- **The flaky connector must never be load-bearing.** Jira/MCP access has already proven unreliable
  enough that ADR-058 moved transitions off it onto CI-fired REST calls. Flightplan's own use of Jira
  (orientation, capture) must degrade to the local, offline worklog/inbox rather than block on it.
- **Portable by construction.** The pattern is proven in Holodex but not Holodex-specific — the design
  must separate the mechanism (worklog schema, hooks, skills) from the repo-specific facts (tracker,
  project key, gate list) so it can be extracted to another Jira-tracked repo without a rewrite.

---

## Decision

Build **Flightplan**, a Claude Code plugin packaged as a `flightplan/` directory from its first
commit, that converts the session-handoff discipline this repo already practices by hand into
config-driven hooks and skills. It has three parts, each solving one of the Context's failure modes:

### 1. A per-epic worklog is the ground truth; Jira status is a projection of it

One epic, one worklog file (`docs/plans/HOLODEX-<key>.md`, generalizing the two hand-rolled
precedents), carrying frontmatter (`key`, coarse `status`, `depends-on: [KEY…]`, `release_note:`), a
**gates checklist** keyed to the repo's routing table (spec/architecture/backend/frontend/testing/
security — see the `.claude/flightplan.yaml` config below) with states `[ ]`/`[/]`/`[~]`/`[x]`, an
**ordered up-next queue** (position *is* the priority — no `[P·effort]` tags to keep in sync), and an
append-only, capped **session log**. A gate moves to `[/]` mechanically when its producing skill runs
(hook-observable); only `[x]` — "done" — is a judgment call, made by `/handoff`, never by a hook.

### 2. Orientation is pushed as a digest at session start; detail is pulled on demand

`SessionStart` regex-matches the branch for a tracker key, fires `In Progress` idempotently over the
existing ADR-058 REST path, scaffolds the worklog if missing, and prints a compact (~150-token)
banner: top of up-next, gate count (`3/6`), the last handoff sentence, open blockers. The full worklog
stays on disk behind that pointer, read only when more is needed. When more *is* needed — reconstructing
state from diffs, an epic-triage pass, `/handoff`'s "what changed" — that expensive read runs in a
**disposable subagent** that returns only the distillate, so the main thread never eats the raw reads.
`/compact` remains an intra-session hygiene tool, complementary to this, prompted at the handoff moment
— not a substitute for durable state. **Rejected:** a "Current Work" section in `CLAUDE.md` (see
Options Considered) — it loads every session regardless of epic, so it's stale for every epic but the
one currently active and taxes every other session's budget for free.

### 3. Idea capture is split from triage, and capture never depends on the connector

Capture is an instant, offline, always-works append to `docs/backlog/INBOX.md` — one line, no
fields, no connector round-trip. A separate, deliberate `/triage` ritual files each line into Jira or
slots it into a live epic's up-next queue, then clears it. `/handoff` and the `Stop` hook sweep the
session for surfaced ideas / `IDEA:` markers as a safety net, so a forgotten capture is still caught
even if the human never typed the command; `spawn_task` chips feed the same inbox.

### Portability seam

Everything repo-specific lives in one config the plugin reads; nothing is compiled in:

```yaml
# .claude/flightplan.yaml   (each repo writes its own)
tracker:
  system: jira                 # or linear, github-projects, ...
  project: HOLODEX
  branch_key: 'HOLODEX-\d+'
  transitions:
    in_progress: { via: rest, script: scripts/jira-transition.sh }   # ADR-058, not Automation
worklog:
  dir: docs/plans
gates:
  - { id: spec,         skill: write-spec,   artifact: 'docs/specs/**' }
  - { id: architecture, skill: architecture,  artifact: 'docs/architecture/ADR-*' }
  - { id: backend }
  - { id: frontend }
  - { id: testing,      skill: testing-strategy }
  - { id: security,     skill: security-review }
```

### Hooks — mechanical only

- `SessionStart` → branch → key regex → fire `In Progress` (idempotent) → scaffold worklog if
  missing → print orientation banner.
- `PostToolUse(Skill)` → append the skill run to the session log; move its gate to `[/]`.
- `Stop` → mechanical freshness check: if the session touched code but never updated the worklog,
  print a loud nag (cannot write the note itself — the fork-bomb rule forbids a `Stop` hook invoking
  Claude); the next `SessionStart` also surfaces "last session left no handoff."

### Skills — where judgment lives

- `/handoff` — ticks gates, writes deferred `until:` notes, updates up-next, sets `release_note`,
  syncs coarse Jira status. The one place a gate is marked `[x]`.
- `/triage` — drains `INBOX.md` into Jira issues or up-next entries.

### Release-note promotion

The worklog, the commit/PR, and the release notes live at different lifecycles (per-epic / per-change
/ per-release) and can't be synced 1:1. Instead the user-facing sentence is authored once, in the
worklog's `release_note:` frontmatter, and flows downhill through a `Release-Note:` git **trailer**
(not the subject, to satisfy the clean-Conventional-Commit rule `release-please`/`git-cliff` depend
on) into the aggregated release notes — joined across all three by the issue key. An epic cannot close
with every gate `[x]` but no `release_note` set — that's an enforceable gate, not a convention.

### POC sequencing — prove in Holodex, then extract

Package as a `flightplan/` plugin directory from the first commit so extraction is a copy-out + config
swap, not a rewrite. Ship in two batches:

1. **Worklog template** — formalize the schema above (generalizes the two existing hand-rolled plans).
2. **`SessionStart` hook** — fire `In Progress` via the real ADR-058 script + print orientation.
   Highest value, lowest risk: kills the two worst pains (unmarked status, no orientation) alone.
3. **`PostToolUse(Skill)` hook** — free "which skills ran" tracking.

— live with 1–3 for a few epics —

4. **`/handoff` skill + `Stop` nag** — gate-ticking, deferred notes, `release_note` promotion,
   staleness detection.
5. **`INBOX.md` + `/triage`** — capture/triage.

---

## Options Considered

### Where does durable per-epic state live?

#### A — a per-epic worklog file on disk, `docs/plans/<KEY>.md` (chosen)
| Dimension | Assessment |
|---|---|
| Complexity | Low — formalizes a pattern already hand-maintained twice in this repo |
| Cost | One file per epic, capped session log |
| Scalability | Scoped to the active epic's branch; doesn't tax unrelated sessions |
| Team familiarity | High — same shape as the two existing hand-rolled plans |

**Pros:** git-native (diffable, survives a session crash, reviewable in a PR); scoped so it never taxes
a session working a *different* epic. **Cons:** one more file type to maintain conventions for —
mitigated by the schema being config-driven rather than free-form.

#### B — a "Current Work" section in `CLAUDE.md`
**Pros:** always loaded, zero extra file. **Cons:** loads on **every** session regardless of which
epic is active — stale for every epic but the current one, and taxes every other session's budget for
nothing. Explicitly rejected in the source one-pager. Rejected here for the same reason: in a
multi-epic repo the branch-scoped worklog strictly dominates a global section.

#### C — Jira only (no local artifact)
**Pros:** single source of truth, no file to keep in sync. **Cons:** the connector is flaky enough
that ADR-058 had to route *transitions* off it onto CI; leaning on it for read-heavy orientation and
the session log (which Jira has no field for) would make Flightplan's core value depend on the exact
system it's designed to route around. Rejected — Jira stays the coarse-status projection, not the
detail store.

### How is session-start orientation delivered?

#### A — `SessionStart` hook pushes a compact digest; full detail stays behind a pointer, pulled on demand (chosen)
**Pros:** bounds the token cost to ~150 tokens on the common path; expensive re-derivation (diffs,
epic-triage) is delegated to a disposable subagent so the main thread never eats the raw reads.
**Cons:** the banner format has to be genuinely compact — see Open Threads; a bloated banner defeats
the point.

#### B — status quo: the agent re-derives state each session from diffs/ADRs/Jira
**Pros:** no new mechanism. **Cons:** this is precisely the failure mode in Context — thousands of
tokens, paid on every turn of the session, and prone to reconstructing the wrong picture (e.g. a gate
that's actually deferred read as simply missing). Rejected — this is the status quo Flightplan exists
to fix, not an option to keep.

### How is idea capture made reliable?

#### A — offline `INBOX.md` append (capture) + deliberate `/triage` (filing), chosen
**Pros:** capture always works, even mid-context-limit, even if the connector is down; triage is
batched so it doesn't interrupt flow. **Cons:** two steps instead of one — mitigated by the `/handoff`
and `Stop`-hook sweep as a safety net for anything never explicitly triaged.

#### B — capture directly to Jira via MCP at the moment the idea surfaces
**Pros:** no intermediate file, no triage step. **Cons:** ties capture — the one step that must never
fail — to the flaky connector; a dropped idea is unrecoverable (no local trace). Rejected for the same
reason ADR-058 moved transitions off Automation: don't put the unreliable dependency in the critical
path of something that must always succeed.

### What is a hook allowed to do?

#### A — hooks stay strictly mechanical; judgment lives only in `/handoff`, invoked by the agent (chosen)
**Pros:** satisfies CLAUDE.md's Hook Safety rule (no `Stop`/other hook may re-enter Claude); keeps the
"is this actually done" call where it belongs — with a human-in-the-loop-invoked skill, not an
unattended trigger. **Cons:** the `Stop` hook can only *nag*, not *write*, when a session forgets to
hand off — accepted, because a loud, impossible-to-miss nag plus a `SessionStart` callback is enough
signal without violating the safety rule.

#### B — a hook invokes Claude (e.g. `Stop` calls `claude /handoff` unattended) to close the loop automatically
**Pros:** would make handoff fully automatic, no nag needed. **Cons:** directly violates the documented
fork-bomb rule (a hook command must never start with or wrap `claude`) — this repo's own CLAUDE.md
forbids it outright. Rejected, not merely deprioritized.

---

## Trade-off Analysis

The recurring shape across all four decisions is the same: **push what can be derived mechanically
from an artifact or a git event; reserve the agent (and the human behind it) for the one judgment call
that genuinely needs one.** That's why the worklog beats a `CLAUDE.md` section (scope beats
always-loaded), why the digest-plus-pointer beats re-derivation (bounded cost beats completeness by
default), why offline capture beats connector-direct (capture must never fail, even at the cost of an
extra triage step), and why hooks stay mechanical (the safety rule isn't negotiable, and "is this epic
done" was never a fact a hook could observe anyway — it's authored).

The two-batch POC sequencing (worklog + `SessionStart` + `PostToolUse` first; `/handoff`/`Stop`/inbox
second) is itself a trade-off call: batch one is pure plumbing with near-zero judgment risk and kills
the two worst pains (unmarked Jira status, cold-start re-orientation) alone, so it ships and gets lived
with before the higher-judgment pieces (deferred-note semantics, staleness thresholds, triage UX) are
designed against real epics rather than guessed upfront.

---

## Consequences

**What becomes easier**
- A session that dies mid-epic no longer loses state — the next session's `SessionStart` banner
  reconstructs "where was I" for ~150 tokens instead of a multi-thousand-token re-read.
- Jira `In Progress` stops depending on the agent remembering to fire it manually at exactly the right
  moment — it becomes a `SessionStart`-hook side effect of the branch already carrying the key.
- The two hand-rolled worklogs in this repo get a schema and tooling instead of being maintained by
  convention alone; new epics get the pattern for free instead of being copy-pasted by hand.
- Ideas surfaced mid-session are captured even when nobody stops to file them properly.

**What becomes harder**
- One more artifact type (`docs/plans/<KEY>.md`) with a schema to keep honest — mitigated by the
  config-driven gate list rather than free-form prose, and by `/handoff` being the only writer of the
  `[x]` state.
- Hooks that only nag (not fix) leave a residual chance a handoff is genuinely skipped across a session
  boundary — accepted per the Hook Safety constraint; the next session's banner surfaces the gap.
- A second config file (`.claude/flightplan.yaml`) to maintain alongside CLAUDE.md's routing table —
  the two must be kept in sync by hand until/unless the routing table itself is generated from the
  config (out of scope here).

**What we'll need to revisit**
- Multi-tracker adapters beyond Jira (Linear, GitHub Projects) — deferred until extraction to a
  non-Jira repo actually happens; the config schema is designed for it but untested against it.
- `/triage` UX (how much auto-filing vs. confirm-each-line) — design once batch 1–3 has been lived with
  and real inbox volume is known.
- The exact orientation-banner token budget and staleness-nag threshold — tune against real sessions
  in batch 4, not upfront.

---

## Action Items

1. [ ] ADR-064 recorded; add to `docs/architecture/README.md`.
2. [ ] **Worklog template** — formalize the schema (frontmatter, gates, up-next, session log) as a file
   template under `flightplan/`; generalize `field-source-of-truth-rollout.md` /
   `studio-entity-implementation.md` into it as a migration example.
3. [ ] **`.claude/flightplan.yaml`** — write Holodex's own config (tracker, worklog dir, gate list
   mirroring the CLAUDE.md routing table).
4. [ ] **`SessionStart` hook** — branch-key regex, idempotent `In Progress` fire via the ADR-058
   script, worklog scaffold-if-missing, orientation banner.
5. [ ] **`PostToolUse(Skill)` hook** — append skill runs to the session log; move the matching gate to
   `[/]`.
6. [ ] Live with 1–3 for a few epics before starting `/handoff`/`Stop`/`INBOX.md` (batch two) — capture
   real staleness/triage friction as HOLODEX follow-up issues rather than guessing upfront.
7. [ ] **`/handoff` skill** — gate-ticking, deferred `until:` notes, up-next update, `release_note`
   promotion, coarse Jira status sync.
8. [ ] **`Stop` hook** — mechanical freshness nag (code touched, worklog not updated); `SessionStart`
   surfaces the prior session's unmet handoff.
9. [ ] **`INBOX.md` + `/triage`** — offline capture file; triage skill to file into Jira or up-next;
   `/handoff`/`Stop` sweep for stray `IDEA:` markers as a safety net.
10. [ ] File the HOLODEX epic + phase sub-tasks matching the two batches above; link this ADR.
