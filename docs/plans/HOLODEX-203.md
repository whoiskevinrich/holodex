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

**Design package:** [`docs/specs/job-history-digest-and-search.md`](../specs/job-history-digest-and-search.md) · [ADR-071](../architecture/ADR-071-job-run-attribution-and-paginated-history.md) · handoff pending · testing-strategy pending

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [/] spec `write-spec` → `docs/specs/job-history-digest-and-search.md`
- [/] architecture `architecture` → [ADR-071](../architecture/ADR-071-job-run-attribution-and-paginated-history.md) entity attribution + paginated read contract (drafted, Proposed)
- [/] backend — phase 2 (migration 0028, attribution, `batch_id`) done; digest + keyset endpoints outstanding
- [/] frontend — phase 1 (ungate + error state) and the Revert-on-column switch landed; digest/log views outstanding
- [ ] testing `testing-strategy`
- [x] security `security-review` — not required: endpoints stay inside the existing `requireOwner` group, no auth surface change

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [ ] [—] Owner answers Q1: does phase 1 alone fix the load time? Phases 3–5 are re-justified or dropped on the answer — `docs/specs/job-history-digest-and-search.md`
2. [ ] [backend] Digest + keyset-paginated history endpoints — `internal/api/`, `internal/repo/jobruns.go`  ⛔ blocked on #1
3. [ ] [testing] No frontend component-test harness exists (suite is pure-logic only), so "history paints while activity is pending" has no regression guard — `web/src/lib/*.test.ts`
4. [ ] [frontend] Entity label resolution — render `#<id> (deleted)` when attribution no longer resolves (P0-1's remaining criterion; lands with the log view)

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->
