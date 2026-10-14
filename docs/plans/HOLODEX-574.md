---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-574
status: in-review
profile: ui                  # test-harness fix for a frontend scenario; no app change
depends-on: []
release_note: No change for users; the navigation test suite's Back-after-trash check is reliable again.
---

# HOLODEX-574 · Nav harness back-after-write-refreshes-grid hangs in route.fetch on Windows

The ticket's diagnosis was wrong: nothing hung. The scenario waited only for the trashed card to be
absent, which is vacuously true right after Back, before the restored grid renders. The wait passed
at once, the grid then rendered with the card, and the check failed, depending on render timing.
`route.fetch` looked stuck only because the context closed under it. The scenario now waits for the
grid's own refetch response and for cards before checking. The §27 mutation (no stale revalidate)
fails it with "Back restored the grid without refetching it after the trash".

**Design package:** none (test harness only)

## Gates — definition of done

- [~] design `design-handoff` → `docs/design/**` — test harness only
- [~] frontend → `web/src/**` — no app change; the fix is in `web/geometry/nav.mjs`
- [x] testing `testing-strategy` — §27 updated; scenario 3/3 + full harness 10/10; mutation pass re-run

## Up next — ordered (position = priority)

1. [ ] [—] On merge, confirm CI moved HOLODEX-574 to Done

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-13 · fix the vacuous wait in back-after-write-refreshes-grid
- skills: code-review (high --fix), handoff
- handoff: Fixed: the scenario now waits for the grid's refetch and cards before checking, and the harness is 10/10 with the §27 mutation re-killed. Next: open the PR, merge, and confirm CI moves 574 to Done.

## Dropped — newest first (the reason is the point)
