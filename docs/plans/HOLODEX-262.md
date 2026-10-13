---
key: HOLODEX-262
status: in-progress
profile: chore
depends-on: []
release_note: ""
---

# HOLODEX-262 · Browse-page completeness sort + missing-facet filter chip

Already delivered before this session. The Completeness sort (F55.5) is live, owner-only, on
Media, People and Studios: F55 shipped it and F65 (`e7e765ec`, #350) moved it onto the store. The
Missing-facet chip (F55.6) shipped and was then removed on purpose by F73 R6 (`de97c861`, #403,
`docs/specs/list-toolbar.md`); the owner page's queue is its home now. This branch closes the
ticket and removes the frontend leftovers that removal stranded.

## Gates — definition of done

- [~] spec — no product rule changed; F73 R6 already records the chip's removal
- [x] frontend → `web/src/**` — dead `api.completenessFacets` + `FacetSummary`, and the never-passed `missingFacet` args on `listPeople`/`listStudios`, removed; `npm run check` 0 errors, 618 Vitest tests green

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge on green CI with subject `refactor(web): drop the client leftovers of the retired missing-facet filter`
2. [ ] [—] Not here: `filterPreference.ts` is dead too, but HOLODEX-474 owns it (its key list drives the legacy-key purge). `GET /completeness/facets` now has no SPA caller; removing it is a separate backend call

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-12 · found shipped; leftovers removed
- skills: code-review high --fix, handoff
- handoff: The story was already delivered, and its chip was retired by F73 R6; this branch only deletes the stranded client code. Next is the PR and a squash-merge on green CI.
