---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-608
status: in-review
profile: ui                  # frontend bug fix; Back under an owner-only sort lands at the top, no UX change
depends-on: []
release_note: Going back to a list sorted by completeness now returns you to where you were, instead of the top of the list.
---

# HOLODEX-608 · Back from a video returns to the top of a list sorted by completeness

Completeness sorts are owner-only. `listController` started with `owner = false`, and the Media,
People and Studios pages set the owner only in an `$effect`. So the first render resolved the
default sort (key `""`), and `wholeListPage` spent Back's one-shot snapshot on that mismatched key.
Then the sort flipped to `completeness_asc`, the list restarted, and it scrolled to the top.
`listController(schema, path, owner)` now reads the owner before its first parse and follows it
after. EntityVideos' hand-rolled seed folds into the same call. Found after the whole-list scroll
(HOLODEX-604). Shed HOLODEX-609: a full reload under an owner-only sort still loses the position,
because capabilities load after mount on a cold load.

**Design package:** none (no UX change)

## Gates — definition of done

- [~] design `design-handoff` → `docs/design/**` — no UX change; restores the specified Back (EL7)
- [x] frontend → `web/src/**`
- [x] testing `testing-strategy` → `docs/testing-strategy.md` (whole-list section, HOLODEX-608) — nav harness `back-restores-owner-only-sort-scroll` (mutation-checked), live repro on `backend-stress-open-9700`

## Up next — ordered (position = priority)


## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-30 · owner-only sort Back fix
- skills: code-review (high --fix), handoff
- handoff: Fixed and verified (Media and People Back under `completeness_asc`, new nav scenario fails without the fix); PR open — next: merge, then sweep HOLODEX-608 to Done. HOLODEX-609 holds the reload case.

## Dropped — newest first (the reason is the point)
