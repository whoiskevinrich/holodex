---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-203                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note:                # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
---

# HOLODEX-203 · Job history digest, pagination, and entity search

`/owner/status` answers "did anything fail", "what happened to this file", and "read it as an
audit trail" without loading 30 days of rows. Done when the digest is the default view, the log
reads through a keyset cursor, and job runs attribute to the entity they touched.

**Design package:** [`docs/specs/job-history-digest-and-search.md`](../specs/job-history-digest-and-search.md) · ADR-069 (reserved, not written) · handoff pending · testing-strategy pending

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [/] spec `write-spec` → `docs/specs/job-history-digest-and-search.md`
- [ ] architecture `architecture` → ADR-069 entity attribution + paginated read contract
- [ ] backend
- [/] frontend — phase 1 (ungate + error state) landed; digest/log views outstanding
- [ ] testing `testing-strategy`
- [x] security `security-review` — not required: endpoints stay inside the existing `requireOwner` group, no auth surface change

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [ ] [—] Owner answers Q1: does phase 1 alone fix the load time? Phases 3–5 are re-justified or dropped on the answer — `docs/specs/job-history-digest-and-search.md`
2. [ ] [architecture] ADR-069 — entity attribution columns + keyset read contract — `docs/architecture/`
3. [ ] [backend] Migration: `entity_type`/`entity_id`/`batch_id` on `job_runs`; populate from writeback/refresh/enrich; kill the Revert regex — `internal/db/migrations/`, `internal/writequeue/`, `internal/enrich/`  ⛔ blocked on #2
4. [ ] [backend] Digest + keyset-paginated history endpoints — `internal/api/`, `internal/repo/jobruns.go`  ⛔ blocked on #1
5. [ ] [testing] No frontend component-test harness exists (suite is pure-logic only), so "history paints while activity is pending" has no regression guard — `web/src/lib/*.test.ts`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->
