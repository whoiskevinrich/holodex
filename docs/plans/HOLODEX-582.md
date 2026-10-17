---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-582
status: in-review
profile: ui                  # one shared Details section; two small row fixes, no new visual
depends-on: []
release_note: "A studio's custom website now shows its custom label to visitors, and a Details field the owner chose to keep blank stays listed so that choice can be undone."
---

# HOLODEX-582 · One shared Details section for the film and studio pages

Spun out of HOLODEX-95. The film and studio pages carried two copies of the Details markup. Both
now render `web/src/lib/components/entity/EntityDetails.svelte`. Owner ruling (option A, chosen
from an inline mockup this session): short scalar rows keep `SourceBadge` as the owner branch, and
that is recorded as the exception in `web/src/routes/CLAUDE.md`. Also fixed: a custom url row's
`custom` pill for visitors, and an owner row for a standing keep-blank decision with no candidate
(spec P0-13 in `docs/specs/film-provider-enrichment-ux.md`).

**Design package:** none. The owner view is unchanged (option A), so there is no new mockup to persist.

## Gates — definition of done

- [~] design `design-handoff` → `docs/design/**` — no visual change; owner chose option A (keep SourceBadge) in session, recorded in `web/src/routes/CLAUDE.md`
- [x] frontend → `web/src/**`
- [x] testing `testing-strategy` — `EntityDetails.svelte.test.ts` covers both fixes (mutation-checked); film page tests pass unchanged

## Up next — ordered (position = priority)

1. [ ] [—] Merge the PR and confirm CI moves HOLODEX-582 to Done

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-17 · session
- skills: code-review (high --fix), handoff
- handoff: Film and studio Details share EntityDetails; owner kept SourceBadge on short rows (routes/CLAUDE.md exception). QA'd the studio page on backend-stress + enrich-stub; the film fixture has no Details fields beyond name, so page tests cover film. Next: merge the PR.

## Dropped — newest first (the reason is the point)
