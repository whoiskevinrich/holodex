---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-459
status: in-review
profile: ui                  # frontend bug fix; anchor landing on existing sections, no new surface
depends-on: []
release_note: The person hover card's Videos and Films links now land on those sections of the profile instead of leaving you at the top, and studio and tag pages do the same.
---

# HOLODEX-459 · Person profile never scrolls to the hover card's #videos / #films anchors

`EntityVideos` (shared by person, studio and tag pages) jumps to `#videos` or `#films` once per mount,
after the grid's first page lands. Before that the page is too short to reach either. `#films` waits
for the shelf's own fetch until the reader first moves the page. Back still restores the scroll
snapshot. Any other arrival takes the anchor, using the navigation type the root layout records in
`lastNavigation`.

**Design package:** none. Owner's design decisions are on the ticket (2026-10-13 comment): instant
jump, no arrival cue, studio and tag pages in scope.

## Gates — definition of done

- [~] design `design-handoff` → `docs/design/**` — no new surface; the owner's landing decisions are on the ticket
- [x] frontend → `web/src/**`
- [x] testing `testing-strategy` — §28 + nav harness `hash-anchor-lands`, mutation-tested (no jump; no `popstate` guard), 9/9 green

## Up next — ordered (position = priority)

1. [ ] [—] On merge, confirm CI moved HOLODEX-459 to Done (Bug on its own keyed branch — jira-sync handles it)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-13 · land entity pages on #videos / #films
- skills: code-review (high --fix), handoff
- handoff: Fixed in `EntityVideos` and verified live on `backend-stress` plus the nav harness (9/9); spun out HOLODEX-573 (list snapshots restore on plain links, not only Back). Next: open the PR, merge, and confirm CI moves 459 to Done.

## Dropped — newest first (the reason is the point)
