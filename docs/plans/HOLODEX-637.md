---
# Flightplan worklog — one epic, one worklog, one definition of done.
key: HOLODEX-637
status: in-progress          # DERIVED from the Gates below; only `done` and `released` are read from here.
profile: feature             # a changed caching rule (spec) on one endpoint, plus one line on the person page
depends-on: []
release_note: After you add a provider on a person's page, the actor pop-over now links to it straight away instead of showing a dead badge for a few minutes.
---

# HOLODEX-637 · Person hover card shows stale provider links after enriching the person

Done means a provider link added on a person's page is clickable in that person's hover card on
the next hover, with no reload and no wait.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**`. `person-hover-card.md`: the card is `private, no-cache` and must never outlive a change to the person (R5); re-reading the profile drops the session copy (R6).
- [~] design `design-handoff` → `docs/design/**`. No surface change: the badge already had a linked state, and the card just stopped serving stale data.
- [x] backend → `{cmd,internal,providers}/**`. `getPersonCard` sends `Cache-Control: private, no-cache` (was `private, max-age=300`).
- [x] frontend → `web/src/**`. `applyPersonDetail` on `/people/[id]` calls `invalidatePersonCard` on every read.
- [x] testing `testing-strategy`. `TestPersonCard` asserts the header (failed on `max-age=300`); `go test ./internal/api -run TestPersonCard` and `npm run check` (0 errors) pass; live check on the stress fixture: first hover 1 request, re-hover 1, after the profile and Back 2. Freshness added to the hover-card row of `docs/testing-strategy.md`.

## Up next — ordered (position = priority)

1. [ ] [—] On merge, check HOLODEX-637 moved to Done (CI moves the branch's own key).

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-11-15 · actor pop-over provider badges not clickable
- skills: code-review (high --fix, 3 findings: 1 skipped, 2 no change needed), handoff
- handoff: The owner saw a dead "Known to" badge in the actor pop-over after adding a provider on the profile; a hard reload didn't fix it but waiting did, which pinned it on the card's 5-minute browser cache. Fix, spec and test landed with every gate settled. Next is merging the PR.

## Dropped — newest first (the reason is the point)

- [~] [frontend] Drop the session card when a person's links change somewhere other than their profile (enrichment review queue, background sweeps). Dropped 2026-11-15: a reload now fixes it, and the reported path is the profile. Revisit if it comes up.
