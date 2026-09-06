# ADR-092: Flightplan repo extraction — relocate the plugin to its own repository

**Status:** Proposed
**Date:** 2026-09-06
**Deciders:** Project owner

**Supersedes:** [ADR-064](ADR-064-flightplan-plugin.md)'s **in-repo packaging decision only** — the
worklog schema, mechanical hooks (`SessionStart`/`PostToolUse(Skill)`/`Stop`), skill split
(`/handoff`/`/triage`), and load-bearing principle ("never let durable state depend on agent
discipline") all stand unchanged as the design-of-record; only *where that design lives* changes.
**Relates to:** [ADR-069](ADR-069-draft-prs-for-pre-implementation-gates.md) (this ADR ships as a
Draft PR per that convention). **Spec:** none — infra/tooling, no end-user-facing surface. **Issue:**
[HOLODEX-327](https://whoiskevinrich.atlassian.net/browse/HOLODEX-327) (task, parent epic
[HOLODEX-182](https://whoiskevinrich.atlassian.net/browse/HOLODEX-182)).

---

## Context

ADR-064 built Flightplan as a self-contained `flightplan/` directory inside Holodex from its first
commit, explicitly to make extraction later "a copy-out + a `.claude/flightplan.yaml` swap, not a
rewrite" — portability was a day-one constraint, never executed. Its own POC sequencing named the
plan outright: *"prove in Holodex, then extract."* Two things now make executing that plan the right
call, rather than leaving it dormant:

1. **ADR numbering collisions have already happened twice** between flightplan-adjacent tooling work
   and Holodex's own product architecture (`scripts/adr-claims.mjs` flags live collisions on ADR-059,
   ADR-068, and ADR-088 as of this writing; the frontend-component-reuse ADR, #257/HOLODEX-287,
   collided on its number against `main` more than once). Tooling-about-how-we-build and the product's
   own architecture decisions competing for one numbering sequence is a structural mismatch, not a
   process failure `scripts/adr-claims.mjs` can fully absorb.
2. **New Flightplan design work is queued**: a profile-driven pre-implementation gate selector (let
   the operator pick how strictly the CLAUDE.md routing table is enforced per epic — Quick fix /
   Standard / High-stakes) plus a `/code-review high --fix` gate. This is Flightplan evolving its own
   mechanism, not a Holodex product decision — it belongs in Flightplan's own architecture history.

**What this ADR is *not* deciding:** whether Flightplan becomes a packaged, installable Claude Code
plugin (the shape `hookify`/`commit-commands`/`claude-md-management` already take). No second
consuming repo exists yet — this is anticipatory, not urgent. Designing a plugin interface against
zero real second-consumer usage would be exactly the kind of speculative abstraction this repo's own
working agreements warn against. This ADR stops at "its own repo, still hand-copied per consumer,"
and revisits packaging only once a real second consumer exists (see Consequences).

### Constraints / forces

- **ADRs are immutable; supersede, don't rewrite.** ADR-064's design content is not wrong and is not
  being redone — only its placement decision changes. The supersession here is narrow (packaging
  location only), the same shape as ADR-091's "supersedes ADR-073 D4 only."
- **Portable by construction, never exercised.** The `.claude/flightplan.yaml` seam and the
  self-contained `flightplan/` directory already assume this move; nothing about the mechanism itself
  needs to change to support it.
- **No plugin distribution channel exists yet.** Without a second real consumer, a "copy-out" step
  stays manual — there is no installer to build against a sample size of one.
- **Holodex's own epic worklogs (`docs/plans/*.md`) are data, not mechanism.** Only the
  schema/hooks/skills move; the worklogs already on disk in `docs/plans/` are this repo's own state
  and stay here.

---

## Decision

Extract `flightplan/` into a new standalone repository — proposed name **`flightplan`**, visibility
and exact ownership pending confirmation before creation (see Action Items; this ADR documents the
decision to extract, not the repo's creation, per ADR-069's Draft-PR-gates-before-implementation
convention).

1. **The new repo becomes the canonical source** for the `flightplan/` directory (worklog template,
   hooks, `lib/`, skills) and for Flightplan's own architecture decisions going forward, seeded from
   ADR-064's content ported in as that repo's own first ADR — carrying forward batches 1–3 as already
   shipped/proven and batch 2 (`/handoff`, `/triage`, `INBOX.md`) as still pending there, not here.
2. **Holodex becomes a consumer**, not the source: it keeps a local copy of `flightplan/` (re-synced
   by hand on pull from the new repo — the "copy-out" step ADR-064 always described, just exercised
   for the first time) plus its own `.claude/flightplan.yaml` config and its own `docs/plans/*.md`
   worklogs, which stay Holodex-specific data.
3. **ADR-064 is not rewritten.** Its content remains Holodex's historical record of the design; this
   ADR narrowly supersedes only its "packaged as a `flightplan/` directory **in this repo**" placement
   clause. A reader wanting Flightplan's current design reads the new repo; a reader wanting *why
   Holodex originally built it this way* still reads ADR-064.
4. **The queued gate-selector / `/code-review` gate design is out of scope here** — it becomes new
   work filed in the new repo's own ADR trail once the repo exists, not part of this extraction.

---

## Options Considered

### Where does Flightplan's design live going forward?

#### A — standalone repo, hand-copied per consumer, own ADR trail (chosen)
**Pros:** fixes the ADR-numbering collision class immediately; gives Flightplan's own evolution a
home that doesn't compete with product architecture for review attention or numbering; costs nothing
speculative — the portability seam already exists and has just never been exercised. **Cons:** two
repos to keep in sync by hand until/unless real plugin packaging happens later; Holodex's own
`flightplan/` copy can drift from upstream between syncs.

#### B — package as an installable Claude Code plugin now
**Pros:** would be the real fix for copy-out drift (versioned, updatable, matches the precedent
already set by `hookify`/`commit-commands`/`claude-md-management`). **Cons:** there is no second real
consumer today to design the interface against — doing this now means guessing at an abstraction
boundary from a sample size of one, which is precisely the "no 'flexibility' that wasn't requested"
anti-pattern this repo's working agreements call out. Rejected for now; revisit once a second consumer
is real (see Consequences → What we'll need to revisit).

#### C — stay in Holodex; solve the numbering collision with a reserved ADR range instead
**Pros:** no repo move, no sync burden, `scripts/adr-claims.mjs` already does collision detection
mechanically. **Cons:** treats the symptom, not the cause — the actual friction isn't "numbers
collide," it's that a portable dev-tooling plugin's design and a personal media server's product
architecture don't belong in the same decision trail at all. A reserved range would still mean every
Flightplan-only conversation happens inside Holodex's docs/PRs. Rejected — doesn't achieve the actual
goal, only defers the symptom `scripts/adr-claims.mjs` was already built to catch.

---

## Trade-off Analysis

The recurring theme from the original brainstorm applies here too: don't build for a consumer that
doesn't exist yet, but don't let "no consumer yet" become an excuse to leave a structural problem
(numbering collisions, tooling-vs-product ADRs sharing one trail) unaddressed when the fix (a plain
repo move) is cheap and the portability seam it depends on is already built. Option A is the smallest
move that actually resolves the two concrete pains named in Context, without speculatively building
Option B's distribution mechanism ahead of real demand.

---

## Consequences

**What becomes easier**
- Flightplan's own future design work (starting with the gate-selector/`code-review` gate) gets a
  clean ADR sequence with no numbering collision risk against Holodex's product architecture.
- Holodex's `docs/architecture/README.md` index stops accumulating tooling-about-tooling decisions
  alongside product decisions — a reader scanning it for "how does Holodex work" isn't interrupted by
  "how does our dev process work."
- A future second consumer (if and when one appears) clones an existing repo instead of copy-pasting a
  subdirectory out of Holodex.

**What becomes harder**
- Two repos to keep in sync by hand — a Flightplan mechanism change means re-copying `flightplan/`
  into Holodex (and any other consumer) manually; no installer exists to do this for us.
- ADR-064 stays in Holodex as a historical record whose placement clause is now stale — a reader has
  to know to follow the supersession pointer to the new repo for the current design.
- Holodex's `.claude/settings.json` hook wiring and `docs/plans/*.md` worklogs need to keep working
  unchanged through the swap — the migration must be verified to not silently break `SessionStart`/
  `PostToolUse`/`Stop`.

**What we'll need to revisit**
- Packaging Flightplan as an installable Claude Code plugin (Option B) — deferred until a real second
  consuming repo exists to design the interface against, not designed speculatively here.
- Whether `scripts/adr-claims.mjs`-style numbering tooling is worth porting into the new repo once it
  accumulates enough ADRs of its own to need collision detection.

---

## Action Items

1. [x] ADR-092 recorded; added to `docs/architecture/README.md` (ADR-064's own row/content unchanged
   — narrow supersession, not a rewrite).
2. [x] Target repo name/local creation confirmed by the project owner — `flightplan`, created at
   `G:\source\flightplan`. **GitHub remote/visibility still unconfirmed** — the repo exists locally
   only; pushing it to a GitHub remote is a separate outward-facing step, not yet taken.
3. [x] New repo created; `flightplan/` ported unchanged (verbatim `diff -rq` against Holodex's copy
   is empty — the "copy-out, not a rewrite" claim held). Its test suite (13/13) passes unmodified
   from the new location.
4. [x] ADR-064's content ported into the new repo as its own ADR-001 (`Accepted`; batches 1–3 marked
   shipped/live, batch 2 — `/handoff`/`/triage`/`INBOX.md` — marked pending there).
5. [x] Re-sync verified — Holodex's `flightplan/` and the new repo's are currently byte-identical
   (nothing has diverged yet, so there was nothing to pull back); hooks are unchanged and untouched
   by this migration.
6. [ ] File the profile-driven gate-selector + `/code-review` gate design as new work in the new
   repo's own ADR trail — out of scope here, not started.
7. [x] Cleared the `needs-adr` label on HOLODEX-327.
