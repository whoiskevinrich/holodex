---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-492
status: in-review            # chore posture has no gates, so the merge guard reads this field
profile: chore               # dev tooling + a test fixture; no product surface
depends-on: []
release_note: ""
---

# HOLODEX-492 · Stress testbed: commit the :9300 preview entries and map `edition` in the fixture

This is done when a fresh worktree can start the stress pair off :7800 from the committed
`.claude/launch.json.example` (Windows can reserve TCP 7781–7880), and the stress fixture registers
`edition` so the edition UI (the header "+ Set edition", the pill, and the `#field-edition` landing)
can be exercised without a scratch mapping. Found while verifying HOLODEX-490/491 (Relates 491).

## Gates — definition of done

(chore posture: no gates)

## Up next — ordered (position = priority)

1. [ ] [—] Merge the PR after CI; HOLODEX-492 moves to Done via jira-sync

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-09-29 · session
- skills: code-review
- handoff: The example gains `backend-stress-9300` and `web-9300` with no credentials (ADR-094), plus a note to run one stress backend only. The fixture mapping gains `edition` with no seeded data. `go test ./testdata/stressseed` passes, and served live, 9146 offers "+ Set edition" with `edition` as a curatable facet. A full-film seeder rung was declined for now. Next: merge after CI.
