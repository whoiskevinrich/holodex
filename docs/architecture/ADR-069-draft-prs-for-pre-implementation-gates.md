# ADR-069: Draft PRs carry pre-implementation gates; `In Review` fires on ready-for-review

**Status:** Proposed
**Date:** 2026-07-22
**Deciders:** Project owner

**Amends:** [ADR-058](ADR-058-jira-transitions-via-rest-api.md) — replaces the `In Review`
row of its transition-trigger table (`pull_request: opened` → *ready for review*). Everything
else in ADR-058 (the REST mechanism, the idempotent/soft-fail contract, `In Progress`
agent-only, `Done` on merge, `Released` as a batch) stands unchanged.
**Relates to:** [ADR-064](ADR-064-flightplan-plugin.md) (the gate/worklog model this makes
visible on the board) · `docs/reference/workflow-idea-to-merge.md` (Stage 4/6).

---

## Context

Holodex routes every change through **gates** — spec, architecture, backend, frontend,
testing, security (CLAUDE.md's change-routing table; a worklog checklist per ADR-064). The
first two gates are **pre-implementation**: an ADR or a spec is written and wants review
*before* any code exists, precisely so the design is challenged while changing it is still
cheap.

There has been no good way to get eyes on that artifact. The options were both bad:

- **Don't open a PR.** The ADR sits on an unpushed branch. Nothing appears in the Jira
  development panel, no CI runs, and review means reading a local file.
- **Open a PR.** ADR-058's `jira-sync.yml` fires **In Review** on `pull_request: opened`,
  so the ticket jumps to In Review on day one of a multi-session epic. The board then
  claims the *work* is in review when only a design document is — and it stays wrong for
  however many sessions the implementation takes. Worse, it's silently wrong: the status
  is right for the 90% of PRs that are complete changes, so nobody notices the 10%.

GitHub already has the exact primitive for "open, linked, reviewable, not finished":
a **draft pull request**. The mismatch is only that ADR-058's trigger doesn't distinguish
it, because when ADR-058 was written every Holodex PR was a finished change.

## Decision

**A pre-implementation gate artifact is published as a Draft PR, and a Draft PR fires no
Jira transition. `In Review` moves from "a PR opened" to "a PR is ready for review."**

### 1. Draft is the state of in-flight work

As soon as the **first gate artifact lands** on a branch — an ADR, a spec, a design
handoff — push it and open a **Draft** PR. The Draft is the epic's PR from that moment on:
it accumulates the remaining gates over the following sessions, and is marked **ready for
review** only when the worklog's gates are green.

One PR, matured — not a gate-only PR merged ahead of the implementation. That keeps the
ladder honest with no new machinery: because a Draft PR *cannot be merged*, the `Done`
trigger can never fire early, so no opt-out marker, label, or trailer is needed to protect
it. The rule is carried entirely by a state GitHub already models and already displays.

### 2. Amended trigger table

| Transition | Trigger (ADR-058) | Trigger (this ADR) |
|---|---|---|
| **In Progress** | start-of-work (branch rename) | *unchanged* |
| **In Review** | `pull_request: opened` | `pull_request: opened` **when not draft**, or `ready_for_review` |
| **Done** | `pull_request` merged | *unchanged* |
| **Released** | `prod` deploy (batch) | *unchanged* |

The gate lives in the workflow's `if:` condition, not in `scripts/jira-branch-sync.mjs` —
the script stays a pure "transition the keys in this branch to this status", with the
question of *whether this event deserves a transition* left where the event shape is
already in scope. `ready_for_review` is added to the workflow's `types:` list, and
`JIRA_TARGET_STATUS` now selects `Done` on `closed` and `In Review` otherwise (rather than
`In Review` on `opened` and `Done` otherwise), so the new event lands on the right target
without a second branch.

### 3. What a Draft PR is still *for*

Firing no transition is not the same as being invisible. A Draft PR still:

- populates the **Jira development panel** (branch, commits, PR) — better linkage than the
  no-PR option ever gave;
- runs **CI** on every push, so an epic's implementation is green-by-default rather than
  discovering breakage at the end;
- gives the artifact a **reviewable diff and a comment thread** — the actual goal;
- makes "what's in flight" readable from the GitHub PR list, with Draft marking the
  in-progress set.

## Consequences

- **The board stops lying about long epics.** A ticket sits at `In Progress` for the whole
  build and reaches `In Review` on the day someone is actually asked to review it. The
  `In Review` column becomes a real queue — a thing to drain — instead of a bucket holding
  everything that has ever been branched.
- **Reviewing a design costs nothing extra.** The ADR gets a diff and a comment thread on
  the same PR that will later carry the code, so the design discussion and the
  implementation stay threaded together instead of split across a merged doc PR and a
  later code PR.
- **`In Review` now depends on a human action.** Marking a PR ready is a deliberate step;
  a PR left in Draft forever leaves its ticket at `In Progress`. That's the correct
  reading (unfinished work is not in review), and `scripts/whats-left.mjs` plus the
  worklog's gate checklist already answer "what's actually left" better than a status
  column can — but a stale Draft is now the failure mode to watch for, in place of a stale
  `In Review`.
- **Converting a ready PR *back* to draft does not move the ticket back.** `converted_to_draft`
  is deliberately not wired: it's rare, and the reverse transition (`In Review` →
  `In Progress`) is the only backwards edge in the ladder — worth adding on evidence, not
  on speculation. Symptom if it bites: a ticket parked in `In Review` behind a Draft PR.
- **Fork PRs and un-keyed branches are unaffected.** Both still no-op for the reasons in
  ADR-058 (secrets withheld from fork `pull_request` runs; no `HOLODEX-<n>` key, no
  transition). Draft-ness is evaluated from `github.event.pull_request.draft` on the same
  untrusted-input-safe `pull_request` event — a boolean GitHub sets, not attacker-supplied
  text, and it reaches only a workflow-level `if:`, never a shell.

## Alternatives considered

- **A `gate-only` PR label that suppresses both `In Review` and `Done`.** This supports the
  other shape — an ADR-only PR reviewed and merged *before* implementation begins. Rejected
  for now: it needs a new convention nothing enforces (forget the label and the merge marks
  the ticket `Done` prematurely — a silent, wrong-direction failure), and it splits one
  change's review across two PRs. Draft needs no marker because GitHub's own merge block
  does the enforcing. Revisit if separately-merged artifact PRs become the norm.
- **Leave CI alone; just say "don't open the PR until you're done."** The status quo. It's
  what produced the problem: the pre-implementation gates get no review at all, which
  defeats the point of having them.
- **A `[draft]`/WIP title prefix parsed by the script.** A string convention reimplementing
  a first-class GitHub state, with none of the merge protection and one more thing to
  typo.
