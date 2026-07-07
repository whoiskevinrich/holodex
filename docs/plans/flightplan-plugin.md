# Flightplan — a portable session-state plugin

**Status:** proposal / tracking one-pager (not yet built). **Scope:** a Claude Code plugin, proven
in Holodex, then extracted for any Jira-tracked repo.

> **Prior art in this repo:** [`field-source-of-truth-rollout.md`](field-source-of-truth-rollout.md)
> and [`studio-entity-implementation.md`](studio-entity-implementation.md) are hand-rolled flight
> plans — session graphs + update protocols + per-session status. Flightplan **automates the pattern
> Kevin already maintains by hand.** That they exist, and work, is the evidence base.

---

## Problem

Large epics span many Claude Code sessions. Sessions die abruptly (context limit, crash, closed tab),
so any state that lived only in the agent's head is lost. Symptoms:

- Jira statuses go unmarked; sub-tickets and dependencies drift.
- Cross-session, cross-layer handoffs fail — architecture lands in one session, backend in another,
  frontend in a third, and each fresh session re-derives "where was I" from scratch.
- No record of which skills ran in a session.
- Ideas surfaced mid-flow evaporate when the session ends.

## The load-bearing principle

> **Never let durable state depend on agent discipline. Either derive it from an artifact (commit,
> branch, PR, file-on-disk), or fire it from a hook. The agent is allowed to *think*, never to
> *remember*.**

Evidence it's the right principle: the only Jira transitions that never fail today are the ones CI
fires off git events (ADR-058). The one agent-fired transition (`In Progress`) is the unreliable one.

Corollary for the wider ecosystem: most published context-management advice ("remember to `/compact`",
"remember to be terse", "add a Current Work section") is **discipline-dependent** and decays exactly
when you're deepest in the work. Flightplan's job is to convert those good habits into automatic
behaviors.

---

## Operating model

### 1. Epic hygiene — the container is loose; the worklog is the truth

Cleanup of muddled epics is **not** a Jira reorg. It's a bounded "reconcile reality" pass. Touch Jira
structure only to restore the invariant:

> **1 epic = 1 worklog = 1 definition of done.**

| Epic state | Action | Restructure Jira? |
|---|---|---|
| In-flight + coherent | Write the worklog now | No |
| Planned + coherent | Leave it; worklog on pickup | No |
| Muddled (two epics in one key, or work smeared across several) | Split at the seam / pull strays back | Yes — minimally |
| Vague / dead | Close, or demote the idea to the backlog inbox | Close only |

Timebox the pass. Don't gold-plate the board.

### 2. Token & session efficiency — the worklog *is* an optimization

A fresh session re-orienting from nothing reads code, diffs, and ADRs to reconstruct state — thousands
of tokens, often wrong. A curated ~20-line handoff replaces all of it. The plan pays for itself in
tokens; tracking is a side effect. Design rules:

- **Push a digest, pull the detail.** `SessionStart` injects a compact orientation (~150 tokens: top
  of up-next, `3/6` gate line, last handoff sentence, blockers). The full worklog stays on disk behind
  a pointer; read on demand only.
- **Subagent-delegated re-orientation.** When more than the digest is needed, run the expensive reads
  (reconstructing state from diffs, epic-triage, `/handoff`'s "what changed") in a **disposable
  subagent** that returns only the distillate. The main thread never eats the raw reads.
- **Up-next items are session boundaries.** The ordered queue (below) is also the session-sizing unit —
  each item is a candidate for its own clean session. Atomic sessions are safe *because* handoff is cheap.
- **Never inject raw MCP/Jira JSON.** The hook distills; if the flaky connector fails, it falls back to
  the local worklog (offline, free). The connector is never in the hot path.
- **Bound the artifact.** Cap the session log to the last N sessions; archive older. Keep everything
  terse and structured (checkboxes, one-liners), not prose.
- `/compact` is an **intra-session** hygiene tool, complementary to — not a substitute for — the durable
  worklog. Prompt it at the handoff/checkpoint moment.
- **Rejected:** a "Current Work" section in `CLAUDE.md`. It loads every session regardless of epic —
  stale for every *other* epic and taxes every budget. The branch-scoped per-epic worklog is strictly
  better in a multi-epic repo.

### 3. Idea capture — split capture from triage

Capture fails today because it depends on the flaky connector or on stopping to fill fields. Fix:

- **Capture** must be instant, offline, always-work → append-only `docs/backlog/INBOX.md`, one line, no
  fields, no connector.
- **Triage** can be deliberate and batched → a `/triage` ritual files each line into Jira *or* slots it
  into a live epic's up-next, then clears it.
- **Safety net:** `/handoff` and the `Stop` hook sweep the session for surfaced ideas / `IDEA:` markers
  and auto-append, so a forgotten capture is still caught. `spawn_task` chips can feed the same inbox.
- Symmetric with the queue: a triaged inbox line becomes a Jira issue *or* an up-next entry — same
  shape, different scope.

---

## Plugin spec

### Portability seam — config, not code

Everything repo-specific lives in one config the plugin reads; nothing is compiled in.

```yaml
# .claude/flightplan.yaml   (each repo writes its own)
tracker:
  system: jira                 # or linear, github-projects, ...
  project: HOLODEX
  branch_key: 'HOLODEX-\d+'
  transitions:
    in_progress: { via: rest, script: scripts/jira-transition.sh }   # ADR-058, not Automation
worklog:
  dir: docs/plans              # this repo's convention
gates:                         # the routing table, per-repo
  - { id: spec,         skill: write-spec,        artifact: 'docs/specs/**' }
  - { id: architecture, skill: architecture,      artifact: 'docs/architecture/ADR-*' }
  - { id: backend }
  - { id: frontend }
  - { id: testing,      skill: testing-strategy }
  - { id: security,     skill: security-review }
```

### Worklog schema (`docs/plans/HOLODEX-<key>.md`)

- **Frontmatter:** `key`, coarse `status`, `depends-on: [KEY…]` (cross-epic deps), `release_note:` (the
  one user-facing sentence — see promotion pipeline).
- **Gates — definition of done:** checklist keyed to config. States: `[ ]` not started · `[/]` in
  progress · `[~]` deferred (with `until: <event>`) · `[x]` done. A gate moves to `[/]` mechanically
  when its producing skill runs; only `/handoff` (judgment) sets `[x]`.
- **Up next — ordered:** numbered queue; **position is the priority** (no P1/P2 noise). Each item
  carries a gate tag and optional file path; `→ KEY` promotes a separable item out to its own issue.
  Blocked items marked. Top item is surfaced verbatim in orientation.
- **Session log — append-only:** which skills ran, per session. Capped/archived.

### Hooks (mechanical only — never touch the "done" judgment)

- `SessionStart` → regex branch for key → fire `In Progress` (idempotent) → scaffold worklog if missing
  → print the compact orientation banner.
- `PostToolUse(Skill)` → append the skill run to the session log; move its gate to `[/]`.
- `Stop` → **mechanical freshness check**: if the session touched code but never updated the worklog,
  print a loud nag. Can't write the note (fork-bomb rule: a Stop hook must not invoke Claude), but makes
  the omission impossible to miss; next `SessionStart` also surfaces "last session left no handoff."

### Skills

- `/handoff` (judgment) — tick gates, write deferred `until:` notes, update up-next, set `release_note`,
  sync coarse Jira status. Belt-and-suspenders with the Stop nag.
- `/triage` — drain `INBOX.md` into Jira issues or up-next entries.

### Release-note promotion — one authored line, three altitudes

Worklog, commit/PR, and release notes live at different lifecycles (per-epic / per-change / per-release),
so they can't be synced 1:1. Instead the user-facing sentence is authored **once** in the worklog and
flows downhill; the join key threads all three:

```
worklog release_note:  ──/handoff at merge──▶  Release-Note: git trailer  ──git-cliff──▶  release notes
   (authored once)                             (subject stays clean;                       (aggregated,
                                                release-please still bumps)                 user-facing)
        └──────────────── join key: HOLODEX-118 → PR #NN links all three ────────────────┘
```

The `Release-Note:` **trailer** (not the subject) satisfies the clean-Conventional-Commit rule while
git-cliff (ADR-034) parses it. Enforceable gate: an epic can't close with all gates green but no
`release_note` set.

---

## POC sequencing (prove here, then extract)

Package as a `flightplan/` plugin dir from the first commit so extraction is a copy-out + config swap.

1. **Worklog template** — formalize the schema above (generalize the two existing hand-rolled plans).
2. **`SessionStart` hook** — fire `In Progress` via the real ADR-058 script + print orientation. Highest
   value, lowest risk; kills the two worst pains (unmarked status, no orientation) alone.
3. **`PostToolUse(Skill)` hook** — free "which skills ran" tracking.
4. **`/handoff` skill + `Stop` nag** — gate-ticking, deferred notes, `release_note` promotion, staleness.
5. **`INBOX.md` + `/triage`** — capture/triage.

Ship 1–3 first (pure plumbing, immediate relief), live with it for a few epics, then add 4–5 once the
real gaps are felt.

## Open threads

- Exact orientation-banner format + token budget (target ~150 tokens).
- Subagent contract for delegated re-orientation (inputs, distillate shape).
- `/triage` UX — how much auto-filing vs. confirm-each.
- Multi-tracker adapters beyond Jira (Linear, GitHub Projects) — deferred until extraction.
