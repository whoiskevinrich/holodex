---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-455
status: in-progress
release_note: No user-facing change. Agent-workflow documentation only — the repo no longer instructs opening a Draft PR at the first gate artifact, which its own PreToolUse guard refuses.
---

# HOLODEX-455 · Retire ADR-069 §1's draft-PR-early rule

**Documentation drift, not an open decision.** Flightplan ADR-007 `pr-lifecycle-gates` (Accepted
2026-09-20, in the plugin repo — *not* Holodex's ADR-007, Docker image structure) replaced "push and
open a Draft PR at the first gate artifact" with **push, no PR** plus a `PreToolUse` guard that
refuses `gh pr create` while a design-phase gate is open. Four Holodex surfaces still taught the old
rule, so the repo was instructing what its own tooling denies. Surfaced by HOLODEX-451: design
commits pushed early per ADR-069 §1, then `/implement` rebased onto fresh `main` and the push needed
a `--force-with-lease` — ADR-069 §1's early push without ADR-069 §1's early Draft PR, the worst of
both halves. The rebase half was already fixed upstream (ADR-007's 2026-09-23 addendum: the crossing
merges `main`); this ticket is the documentation half.

## Gates — definition of done

- [~] spec `write-spec` — **not applicable.** Agent/dev-workflow process. Nothing ships to a user,
  no product behaviour changes, no F## number involved.
- [x] architecture `architecture` — **[ADR-106](../architecture/ADR-106-push-early-pr-at-implementation.md)**,
  Accepted. Supersedes **ADR-069 §1 only**; §2's `In Review`-on-ready-for-review amendment to ADR-058
  is live in CI and a dependency of Flightplan ADR-007, so retiring ADR-069 wholesale was the wrong
  move and is recorded as rejected option **D**. ADR number taken from
  `node scripts/adr-claims.mjs --reserve push-early-pr-at-implementation`, never by eye.
- [~] design `design-handoff` — **not applicable.** No user-facing surface; no mockup to sign off.
  (This is the repo's one `approve: true` gate — `[~]` settles it, so the phase derives to `build`
  and the PR may open in the same session. A change with no build-phase artifact is the general
  rule with an empty build phase, not a special case; ADR-106 §2 says so explicitly.)
- [~] backend — **not applicable.** No `cmd/`, `internal/` or `providers/` change.
- [~] frontend — **not applicable.** No `web/**` change, so no three-skin QA.
- [~] testing `testing-strategy` — **deliberately skipped.** The change is five prose files; there
  is no behaviour to assert and nothing a test could hold that a reader cannot. A guard that greps
  the docs for "open a Draft PR" would be a lint against English, and the real enforcement already
  exists upstream as Flightplan's `PreToolUse` hook — which is what made this drift visible in the
  first place.
- [~] security `security-review` — **not applicable.** No auth, access, or infrastructure surface.
  Documentation of a process; no workflow, permission, secret, or perimeter touched.

`/code-review high --fix` is likewise **not applicable** — zero lines of code in the diff.

## What changed

| Surface | Change |
|---|---|
| `docs/architecture/ADR-106-*.md` | **new.** Records the decision Holodex-side, in full rather than as a pointer (§5: the plugin repo is private, and the guard is wired per *machine* in `~/.claude/settings.json`, so a contributor reads these docs with no guard running). |
| `docs/architecture/ADR-069-*.md` | header only — `Status:` qualified and a dated **§1 superseded** note. ADRs are immutable; §1's body is left intact as history. |
| `docs/architecture/README.md` | ADR-069's index row carries the partial supersession; new ADR-106 row. |
| `.claude/CLAUDE.md` | "Pre-implementation gates ship as a Draft PR" → **"The design phase pushes; it does not open a PR"**; the matching "Draft unless the gates are green" step under *Before pushing*. |
| `docs/reference/workflow-idea-to-merge.md` | Mermaid node (`DRAFT` → `PUSH` + `IMPL`), the *what's left* PR row, the Stage 4 section, Stage 6, the quick reference. |
| `docs/reference/jira-pipeline.md` | **not named in the ticket** — found by the sweep. The "A Draft PR is not In Review" paragraph and the docs-only-merge guard's rationale both asserted the early-PR shape. Its `In Review` *mechanics* are correct and untouched. |

**Deliberately left alone:** per-epic *records* that mention a Draft PR — specs' sequencing notes
(`instance-skin.md`, `video-playlists.md`, `person-hover-card.md`, `candidates-*.md`,
`provider-alias-collapse.md`, `film-studio-cascade-writeback.md`, `entity-identity-card.md`), design
handoff headers, `docs/testing-strategy.md` entries, and ADR-071/092/095/100's cross-references.
Each is an accurate account of how that epic actually shipped, not an instruction to a future
reader. Rewriting them would be falsifying history to match a later rule.

## Up next — ordered (position = priority)

1. [ ] [HOLODEX-455] Kevin's review — the PR opens **ready** (all gates settled, no Draft stage),
   so the issue moves to `In Review` on `opened`
2. [ ] [HOLODEX-455] on merge, sweep to Done **by hand** (CI transitions only the branch's issue)
3. [ ] [HOLODEX-455] release the ADR-106 claim — `.adr-claims` holds `106 RESERVED`; it self-clears
   once the ADR is on a pushed branch, but check `node scripts/adr-claims.mjs` reports no collision
4. [ ] *(unfiled, out of scope)* `workflow-idea-to-merge.md`'s closing "Today vs. when the flightplan
   plugin ships" table still describes shipped hooks as future work — pre-existing staleness with a
   different cause, worth its own ticket

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-23 · ADR-106 + the four instructional surfaces
- skills: architecture
- handoff: **Shipped, all gates settled.** The branch was renamed `claude/holodex-455-…` →
  `HOLODEX-455-adr069-draft-pr-drift` and the issue fired to In Progress before the first commit.
  **The fact that shaped the ADR:** ADR-069 §1 and Flightplan ADR-007 were not in disagreement about
  the *push* — ADR-007 rejects "keep the branch unpushed" as the worst of its four options, for the
  same reason ADR-069 pushed early. Only the PR half died, so the ADR supersedes a half of a section
  rather than a decision, and says in its trade-off analysis what is actually being traded: a
  reviewable diff on the artifact (which, in a single-owner repo, nobody else was ever going to
  review) for a sign-off that blocks implementation on an unseen design. Two costs are stated rather
  than glossed: no comment thread on an ADR until the crossing, and **no CI on design-phase pushes**
  — verified, not assumed, by reading `ci.yml`'s triggers (`pull_request` + push to `main` only).
  The sweep turned up a **fourth** surface the ticket did not name, `docs/reference/jira-pipeline.md`,
  and drew the line at per-epic records: a spec's "Sequence: … Draft PR" note is history, not an
  instruction. Left: Kevin's review, then mark ready.
