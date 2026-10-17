---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-474
status: in-review
profile: ui                  # dead-module removal + a storage purge; no visual change
depends-on: []
release_note: "Filters saved by older versions of Holodex are now cleared from the browser, so they can never reappear on a list page."
---

# HOLODEX-474 · Delete filterPreference.ts and purge the legacy filter keys

F73 fast-follow (parent HOLODEX-472, Done). Filters have lived only in the URL since F73, and
nothing has read `web/src/lib/filterPreference.ts` since. The module and its test are deleted. Its
key list (`holodex:filters:{media,people,studios,tags}`) moved to `purgeLegacyFilters` in
`web/src/lib/listState.ts`, which the root layout runs once per app load.

**Design package:** none. Nothing on screen changes.

## Gates — definition of done

- [~] design `design-handoff` → `docs/design/**` — no visual change
- [x] frontend → `web/src/**`
- [x] testing `testing-strategy` — U3 in `listState.test.ts` covers the purge (all four keys, nothing else); verified in the dev server

## Up next — ordered (position = priority)

1. [ ] [—] Merge the PR and confirm CI moves HOLODEX-474 to Done

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-17 · session
- skills: code-review (high --fix)
- handoff: filterPreference.ts deleted; purgeLegacyFilters (listState.ts) clears the four SP5 keys on every load, wired in +layout.svelte; specs and testing-strategy updated. Next: merge the PR.

## Dropped — newest first (the reason is the point)
