---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-407
status: in-progress
profile: backend
release_note: ""
---

# HOLODEX-407 · F-number claims — `scripts/feature-claims.mjs`

Filed 2026-09-17 from the F63 collision: HOLODEX-390 (#344) and HOLODEX-406 (#346) both took
F63 on their branches, 390 merged first, and 406 was renumbered to F64 across 53 references.
`scripts/adr-claims.mjs` already prevents exactly this for ADR numbers; there was no feature
twin. Agent-tooling only — no product behaviour, no ADR, no UI — so `chore(flightplan)` and
the routing-table gates are n/a except testing.

**Shape:** a sibling `scripts/feature-claims.mjs` (not a `--kind` flag) that imports the shared
pure helpers (`nextFree`, `rankRef`, `parseReservations`, `pruneReservations`, `daysSince`,
`describeRivals`, `mainWorktreeRoot`) plus two newly exported impure ones (`git`, `refsBySha` —
the by-commit ref dedupe extracted from the ADR scanner). Claims come from the H1 of every
`docs/specs/*.md` on every local + remote ref (`git grep "^# " <sha> -- docs/specs/*.md`, first
match per file). Cache: gitignored `.feature-claims` at the main worktree root; `--print`,
`--reserve <slug>`, `--release <n>` as the ADR script.

**Where the ADR rule had to bend, because the corpus already breaks it:**
- `# QA: … (F28)` companions restate a number and are not claims.
- Sub-features (`F56.2`, `F21.3b`) and phases (`F46 Phase 3`) are children — tracked by full id
  (`56.2`, `21.3b`, `46-phase3`), only the integer feeds "next free", never rivals of the parent.
- F55 and F56 are each claimed by **two** specs on main. Retro-renumbering is out of scope, so a
  collision is flagged only when a spec **not on main** shares a number — the in-flight case
  renumbering can still catch. Shared-on-main pairs render as a quiet `shared with …` line.

## Gates — definition of done

- [~] spec `write-spec` — n/a: dev tooling, no user-facing behaviour
- [~] backend — n/a
- [x] testing `testing-strategy` — `scripts/feature-claims.test.mjs` (11 tests: heading parser
  incl. sub/phase/QA cases, `git grep` output parsing, the 390/406 collision, two-in-flight
  collision, shared-on-main non-collision, sub-feature non-collision + `nextFree`, reservation
  round-trip through the shared parser, render). Picked up by the existing
  `make test-scripts` glob; 127/127 green. Live run against this repo: next free **F65**, zero
  collisions (F46 Phase 3 on `origin/HOLODEX-180` correctly a child, not a rival)

## Up next — ordered (position = priority)

1. `[S]` Open the PR (ready, not draft — every applicable gate is green) → CI fires In Review;
   merge → Done.
2. `[S]` Downstream: the `/write-spec` skill is the `product-management:write-spec` marketplace
   plugin, not a file in this repo, so its scaffold step could not be edited here. The
   CLAUDE.md Conventions bullet now instructs the agent to run `feature-claims.mjs` at that
   step, which is what the agent actually reads. If Kevin wants it in the skill itself, that is
   a Flightplan/plugin change (ADR-092 repo).

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-17 · branch cut, script + tests + wiring shipped
- skills: code-review (pre-commit)
- Cut `HOLODEX-407-feature-claims` from `origin/main` (faccc69, the F64 merge); fired In
  Progress via MCP.
- Wrote `scripts/feature-claims.mjs` + `.test.mjs`; refactored `adr-claims.mjs` only enough to
  share (`export git`, extract `refsBySha`, reservation parser tolerates an `F` prefix —
  ADR tests unchanged and green). `.gitignore` gains `/.feature-claims`; CLAUDE.md
  Conventions bullet now covers feature numbers and points at the script.
- Handoff: everything is built and tested; the PR is the only step left.
