# ADR-106: The PR opens at the design → build crossing, not at the first gate artifact

**Status:** Accepted — the mechanism is already live and enforced (Flightplan ADR-007, accepted
2026-09-20); this ADR records it on the Holodex side and retires the instruction it replaced.
**Date:** 2026-09-23
**Deciders:** Project owner

**Supersedes:** [ADR-069](ADR-069-draft-prs-for-pre-implementation-gates.md)'s **§1 only** — the
*timing* rule ("as soon as the first gate artifact lands… open a **Draft** PR"). Everything else in
ADR-069 stands and is load-bearing: §2's amendment of ADR-058's `In Review` trigger
(`pull_request: opened` **when not draft**, or `ready_for_review`) is what CI implements today, §3's
account of what a Draft PR is for still describes the PR this ADR opens later, and the Draft state
itself is not retired — only its start date moves.
**Records, does not take, the decision:** **Flightplan** ADR-007 `pr-lifecycle-gates`
(`G:/source/flightplan/docs/architecture/ADR-007-pr-lifecycle-gates.md`, **Accepted** 2026-09-20,
plus its 2026-09-23 addendum). Not Holodex's ADR-007 (*Docker Image Structure*) — the two sequences
are separate by [ADR-092](ADR-092-flightplan-repo-extraction.md), and this collision is easy to make,
so every reference below says which repo it means.
**Relates to:** [ADR-064](ADR-064-flightplan-plugin.md) (the gate/worklog model), ADR-092 (why
Flightplan has its own ADR trail), [ADR-058](ADR-058-jira-transitions-via-rest-api.md) (the
transition ladder, untouched here). **Spec:** none — agent/dev-tooling process, no end-user surface.
**Issue:** [HOLODEX-455](https://whoiskevinrich.atlassian.net/browse/HOLODEX-455).

---

## Context

Holodex ADR-069 §1 (Proposed, 2026-07-22) answered a real problem: a pre-implementation gate
artifact — an ADR, a spec, a design handoff — wants review *before* the code exists, and there was
no good way to get eyes on it. Its answer was to push the branch and open a **Draft PR** the moment
the first such artifact landed.

Flightplan ADR-007 reached the same seam from the other end two months later, and named the failures
that answer *caused*, each observed in this repo more than once:

- **Docs-only draft PRs as parked epics** — two open at the time it was written, both `+N/-0`: a
  spec, a design handoff, and a worklog. The draft-PR list had become the only "what's parked"
  surface, and ADR-007 calls it "a list of costumes".
- **Epics going dark.** Design PR merged, branch deleted → the worklog sits on `main` with no branch
  carrying its key, and no session ever finds it again.
- **Implementation before sign-off.** `design [x]` meant "the handoff doc is committed", never "the
  owner saw the mockup and said yes" — and a later session built something quite different from what
  had been approved. ADR-069 §1's Draft PR never guaranteed otherwise.

Critically, ADR-007's options table rejects **D** ("keep the local branch unpushed until build") as
"the worst of the four": an unpushed parked epic is invisible from every other worktree and machine,
and Holodex routinely runs several worktrees at once. So **the push half of ADR-069 §1 survived and
only the PR half died.** Its §5 states it directly: *"the owner's 'local branch' was never the
requirement; 'no PR yet' was, and the push is what keeps a parked epic visible and backed up."*

### The repo currently instructs what its own tooling refuses

ADR-007 §4 ships a `PreToolUse` hook that **denies `gh pr create` while any design-phase gate is
open**, and `.claude/flightplan.yaml` opts this repo in with `phases.design: [spec, architecture,
design]`. Meanwhile four Holodex surfaces still say to open a Draft PR at the first artifact:
ADR-069 §1, `.claude/CLAUDE.md`, `docs/reference/workflow-idea-to-merge.md`, and
`docs/reference/jira-pipeline.md`. An agent following the documentation gets refused by the hook; an
agent following the hook is contradicting the documentation. That is the whole of what this ADR
fixes — **documentation drift, not an open decision.**

### The cost that surfaced it

HOLODEX-451, 2026-09-23. Design commits were pushed early per ADR-069 §1; `/implement` then rebased
onto fresh `main` before pushing, rewriting those published commits, so the push needed a
`--force-with-lease`. The repo had given the owner ADR-069 §1's early push (which is what sets up the
force-push) without ADR-069 §1's early Draft PR (the visibility that was the entire point of pushing
early) — the worst of both halves. Flightplan ADR-007's own addendum of the same date fixes the
rebase: the crossing now **merges** `origin/main` rather than rebasing onto it, since a branch already
on `origin` cannot be rebased without rewriting published commits. So the force-push failure mode is
already gone, independently of this ADR. What is left is the docs.

---

## Decision

**A keyed branch is pushed during its design phase and carries no PR. The PR opens at the design →
build crossing (`/implement`) and is still the epic's one PR, maturing Draft → ready → merge.**

### 1. Push at the first gate artifact — unchanged

ADR-069 §1's push survives verbatim. A parked epic lives on `origin`, where another worktree, another
machine, and a cold session can all see it. This is not the concession in the decision; it is the half
that was independently re-confirmed.

### 2. No PR while a design-phase gate is open

`phases.design` for Holodex is `[spec, architecture, design]` (`.claude/flightplan.yaml`). The phase
is **derived** from gate state, never stored: `build` iff every design-phase gate present in the
epic's posture is `[x]` or `[~]`, **and** every `approve: true` gate among them is `[~]` or has an
`approved:` entry. `design` is `approve: true` here. Absent `phases:`, the check is off — configuration
absence is the feature being off, not a silent default.

A change with no build-phase artifact — this ADR, for instance — settles its design gates the moment
the artifact is committed, derives to `build` in the same session, and may open its PR immediately.
That is not a special case; it is the general rule with an empty build phase.

### 3. The review of a design artifact moves from a diff to the crossing

This is the cost, stated plainly: an ADR or spec no longer gets a GitHub diff and comment thread
while it is the only thing on the branch. What replaces it is **`/implement`**, which puts each
unsigned `approve` gate's artifact in front of the owner, asks, and records the yes in the worklog's
`approved:` frontmatter pinned to a commit — a fact about the owner, which is why it cannot be
derived from `[x]`. A later edit to the artifact makes the sign-off **stale**, which means *re-confirm
with the delta in front of you*, never a return to the design phase.

Weaker surface, stronger gate: no comment thread, but implementation cannot begin on an unseen mockup
— which ADR-069 §1 never prevented.

### 4. A parked epic is visible on the board, not in the PR list

`fp:ready-to-build` on the Jira issue (`tracker.labels.ready_to_build`). `/handoff` applies it when
every design-phase gate is settled and the sign-off is still outstanding; `/implement` removes it, so
a stale label self-heals at the next ritual. The dashboard query is:

```
project = HOLODEX AND labels = fp:ready-to-build AND status != Done
```

The **`Ready to Build` status also exists** in this Jira project (transition id 42), but the label is
what ships and `transitions.ready_to_build` stays commented out in `.claude/flightplan.yaml`. Adding
it is optional per-project sugar; the label is the mechanism, because add/remove is idempotent and
re-derivable from the worklog while a status transition is a path-dependent edge.

### 5. Holodex's docs carry the rule in full, not a pointer

The Flightplan repo is **private and not vendored** (ADR-092; Flightplan ADR-002/003), and its
`PreToolUse` guard is wired per **machine** in `~/.claude/settings.json` — not in this repo's
`.claude/settings.json`. So a contributor, or this same repo on a machine without the hook, reads
Holodex's documentation with no guard running and no access to the ADR that decided this. Holodex's
docs therefore state the rule completely and cite Flightplan ADR-007 as its origin, rather than
delegating to a file the reader may not be able to open.

### 6. The transition ladder does not move

Unchanged from ADR-058/069: **In Progress** at branch rename (agent-fired), **In Review** when the PR
is marked ready for review, **Done** on merge, **Released** on the `prod` deploy. A Draft PR still
fires nothing, and `Done` still cannot fire early because GitHub will not merge a draft. The only
change is that the draft now appears at the crossing rather than on day one.

---

## Options Considered

How should Holodex record a decision that was taken in the plugin's ADR trail?

| | ADR-069 stays readable as history | One place to read the current rule | Legible without the plugin repo |
|---|---|---|---|
| **A.** Edit ADR-069 §1 in place | ❌ breaks immutability | ✅ | ✅ |
| **B.** New ADR superseding §1 only, rule restated in full **(chosen)** | ✅ | ✅ | ✅ |
| **C.** New ADR that only points at Flightplan ADR-007 | ✅ | ❌ split across two repos | ❌ private repo |
| **D.** Mark ADR-069 `Superseded` wholesale | ✅ | ❌ | ✅ |

**A** is the convention this repo has held since ADR-001 — ADRs are immutable decisions, superseded
rather than rewritten — and the reasoning in ADR-069 is worth keeping legible, because the problem it
was solving is real and the trade it made is the one being reversed. **C** is the tidy answer and
fails on the third column, which is the one that matters for a rule an agent must follow on a machine
that may not have the guard. **D** is the tempting shortcut and is wrong on the second column: it
would retire §2's `In Review` trigger along with §1, and §2 is both live in CI and a dependency of
Flightplan ADR-007 ("a Draft PR fires no transition").

---

## Trade-off Analysis

The reversal is not "ADR-069 was wrong". It optimized for the **artifact being reviewable**; ADR-007
optimized for the artifact being **approved before code exists** and for a parked epic being
**findable a month later**. Those are different goals, and the second pair is what was actually
failing in practice while the first was being satisfied only nominally — ADR-007's Context puts it
bluntly, that the doc PR "asks for review ceremony around a document nobody else reviews". In a
single-owner repo that is the honest reading, and it is the load-bearing premise of the whole
reversal: the review that mattered was always the owner's, and a GitHub comment thread was a proxy
for it that nobody used.

The second trade is giving up **CI on design commits**. `ci.yml` triggers on `pull_request` and on
push to `main` — never on a branch push — so a pushed design branch with no PR gets no checks until
the crossing. For a spec or an ADR that costs nothing. For the tooling work that often rides the same
design phase (a `scripts/` change, a hook, a docs link), a failure now surfaces at the crossing
instead of on the next push. Accepted rather than worked around: widening `ci.yml` to all branch
pushes would double CI minutes to cover a class of change that `make test-scripts` catches locally,
and the fix if it bites is a narrow `push:` filter, not a rethink of when the PR opens.

The third is that the parked-epic surface is now a **label a skill applies** instead of a state
GitHub maintains. A draft PR existed whether or not anyone remembered it; `fp:ready-to-build` exists
only if `/handoff` ran. That is exactly the "durable state depends on remembering" failure ADR-064
forbids — and it is covered, not by discipline, but by ADR-007 §4 making a fresh handoff a
**precondition of `git push`**. The residual exposure is a machine where the guard is off
(`FLIGHTPLAN_GUARD=off`, or the hook simply not wired): there, neither the label nor the worklog
freshness holds, and nothing in this repo can tell.

---

## Consequences

**What becomes easier**
- The repo stops instructing what its own hook refuses. An agent can follow either the docs or the
  guard and land in the same place.
- The PR list is only work being built — no `+N/-0` docs-only drafts to read past.
- "What's parked?" is one Jira query instead of a PR list plus a reading of each diff.
- The HOLODEX-451 force-push has no setup left: the branch is still pushed early, and the crossing
  merges `main` instead of rebasing onto it.
- Implementation cannot start on an unapproved design handoff — a guarantee the previous rule did not
  offer at all.

**What becomes harder**
- A design artifact has no GitHub comment thread until the crossing. Review is a conversation in
  session plus the committed artifact; a line-level comment on an ADR now has to wait for the PR.
- No CI on design-phase pushes (`ci.yml` has no branch-push trigger), so tooling changes that ride
  the design phase fail later than they used to.
- The parked-epic surface depends on `/handoff` having run, which depends on the push guard being
  wired on that machine.
- One more repo-vs-repo ADR-number ambiguity to keep straight in prose: Flightplan ADR-007
  (PR lifecycle) vs. Holodex ADR-007 (Docker image structure).

**What we'll need to revisit**
- **If Holodex ever gains a second contributor**, review ceremony around a design artifact stops
  being theatre, and the case for a reviewable pre-implementation diff returns on its own merits.
  This ADR's premise is explicitly single-owner.
- **An epic that goes dark *mid*-design** — before every design gate settles, so before the label is
  applied — is the case the label cannot see. Flightplan ADR-007 defers it (its OQ4 / spec R8:
  enumerating keyed remote branches at `SessionStart`), and the first occurrence is the trigger.
- Per-epic *records* that mention a Draft PR — specs' sequencing notes, design handoffs' headers,
  `docs/testing-strategy.md` entries, and ADR-071/092/095/100's cross-references — are deliberately
  **left alone**: each is an accurate account of how that epic actually shipped, not an instruction
  to a future reader. Only the four instructional surfaces changed.

---

## Action Items

1. [x] This ADR; `Superseded (§1)` pointer in ADR-069's header; `docs/architecture/README.md` index
   rows for both.
2. [x] `.claude/CLAUDE.md` — retitle and rewrite "Pre-implementation gates ship as a Draft PR", and
   the matching step in "Before pushing or opening a PR".
3. [x] `docs/reference/workflow-idea-to-merge.md` — the Mermaid node, the `what's left` PR row, the
   Stage 4 section, Stage 6, and the quick reference.
4. [x] `docs/reference/jira-pipeline.md` — the "A Draft PR is not In Review" paragraph and the
   docs-only-merge guard's rationale. The `In Review` mechanics themselves are correct and untouched.
5. [ ] Owner review. All gates are settled, so the PR opens **ready** (no Draft stage) and
   HOLODEX-455 goes to `In Review` on `opened`; sweep it to `Done` by hand on merge.
