---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/archive/ADR-064-flightplan-plugin.md
key: HOLODEX-220                 # the tracker key; must match the branch key regex
status: in-progress
profile: ui
depends-on: [HOLODEX-114]    # [KEY-…] cross-epic deps that must land first
approved:
  design:
    on: 2026-10-21
    at: 0b80a9db
release_note: The media page's metadata header has one Sync button and a sources summary in place of four refresh controls, fields that match the file fold into one line, and the raw source data sits in one disclosure.
                             # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
---

# HOLODEX-220 · Media page restructure — one sync verb, render once

Done means `/media/[id]` carries a single sync verb instead of four "refresh" variants, and renders
each enriched value once instead of two or three times — with no capability removed, no new endpoint,
and no data-model change. Both halves are subtractive.

**Design package:** spec n/a · ADR n/a · [handoff](../design/media-page-restructure-handoff.md) · testing-strategy § TBD

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] design `design-handoff` → `docs/design/media-page-restructure-handoff.md`
- [x] frontend
- [x] testing `testing-strategy`

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [ ] Sweep HOLODEX-97 with this epic (it has no branch of its own): In Review when the PR is
   ready, Done on merge
2. [ ] [—] Sync's rate-limit message, which Refresh all had → HOLODEX-591
3. [ ] [—] Visitor thumbnail regenerate: gate the endpoint or make the button touch-reachable → HOLODEX-592

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-21 · session
- skills: design-handoff, implement, code-review, handoff
- handoff: All gates settled. Parts A, B and C are built, the owner re-confirmed the design at
  0b80a9db and matched the build to the mockup, and PR #546 is ready. Main merged in for the
  gitleaks allowlist (#547), which clears `secrets`. Next: sweep HOLODEX-97 to Done on merge.

### 2026-07-29 · status reconciled against Jira — In Progress
- handoff: Frontmatter said `in-review` (stale from PR #179's design-handoff-only review round);
  Jira shows **In Progress** as of 2026-07-28 — the design PR merged and the branch is back in
  implementation, consistent with the gates below (frontend/testing still open). Corrected to
  match. HOLODEX-114 (the Part-B blocker) is **In Review** in Jira, not yet merged — item 3 stays
  blocked until it lands.

### 2026-07-26 · design critique → handoff → review
- skills: design-critique, design-handoff
- handoff: The handoff is written and PR #179 is up for design review ahead of implementation. Start at
  Part A (item 1) — it is unblocked and self-contained. Do not start item 3 until HOLODEX-114 lands.
