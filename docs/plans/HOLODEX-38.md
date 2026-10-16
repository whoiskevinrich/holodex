---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-38
status: in-progress
profile: feature             # new user-facing behaviour (spec SP6) + UI; the backend already serves it
depends-on: []
release_note: Person, tag and studio pages can now sort their videos, Random included, with the same sort control as the Videos page; the choice carries across every entity page and into Play all.
---

# HOLODEX-38 · Sort (incl. Random) on entity video grids

Child of HOLODEX-13 (People). Rescoped 2026-10-13: the title filter already exists; what's missing
is sort, including Random, on the shared `EntityVideos` grid behind the person, tag and studio
pages. It absorbs the sort half of HOLODEX-45, and it reuses the F73 list-toolbar sort control. The
backend already accepts `sort`/`seed` on the `/media` query those grids page through (HOLODEX-501),
so the change is frontend only.

**Design package:** [entity-grid-sort-handoff.md](../design/entity-grid-sort-handoff.md) ·
[entity-grid-sort-mockup.svg](../design/entity-grid-sort-mockup.svg)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — `sort-persistence.md` SP6
- [x] design `design-handoff` → `docs/design/**` — handoff + SVG; owner picked layout A and the shared preference, 2026-10-15
- [~] backend → `{cmd,internal,providers}/**` — `/media` already takes `sort`/`seed` for entity facets (HOLODEX-501); no change
- [ ] frontend → `web/src/**` — `EntityVideos` toolbar + `entityVideosSchema`
- [ ] testing `testing-strategy` — schema unit tests; HOLODEX-38 entry in `docs/testing-strategy.md`

## Up next — ordered (position = priority)

1. [ ] Build the toolbar in `EntityVideos`, verify live on person/tag/studio pages
2. [ ] [—] Open the PR, merge, and confirm CI moves HOLODEX-38 to Done

## Session log — append-only (cap: last 8 sessions; older → archive/)

## Dropped — newest first (the reason is the point)
