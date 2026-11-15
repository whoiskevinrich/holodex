---
# Flightplan worklog — one epic, one worklog, one definition of done.
key: HOLODEX-636
status: in-progress          # DERIVED from the Gates below; only `done` and `released` are read from here.
profile: feature             # a changed duplicate rule (spec) on the backend, plus the error wording on two pages
depends-on: []
release_note: Writing one part of a multi-part video to file is no longer blocked as a duplicate of its other parts, and a duplicate warning now names the field instead of saying "undefined".
---

# HOLODEX-636 · Film parts collide as composite-key duplicates (writeback blocked)

Done means two videos whose resolved part differs never collide on the {title, people, date,
studio} duplicate check, whichever edit triggers it, and a collision from a source pick names the
field instead of quoting `undefined`.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**`. `video-composite-key-collision.md` gains the P0 rule "Different parts are never duplicates" and records the multi-part false positive it had predicted in Open Questions.
- [~] design `design-handoff` → `docs/design/**`. No new surface: the only UI change is the wording of an existing error line.
- [x] backend → `{cmd,internal,providers}/**`. `scanCollisionCandidates` takes a `PartLookup` and skips a candidate whose resolved part differs; Title, Studio (and the film-studio cascade) and People gates pass the API's `partsFor`.
- [x] frontend → `web/src/**`. `collisionMessage` in `web/src/lib/api.ts` replaces the three inline throws on the media and film pages.
- [x] testing `testing-strategy`. `TestCollision_PartsAreNotDuplicates` (API, failed with 409 on all three parts before the fix), the part cases in `TestFindStudioCollision`, `collisionMessage` Vitest; `go test ./internal/repo ./internal/api`, `api.test.ts` and `npm run check` (0 errors) pass. Row added to `docs/testing-strategy.md`.

## Up next — ordered (position = priority)

1. [ ] [—] On merge, check HOLODEX-636 moved to Done (CI moves the branch's own key).

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-11-14 · film part writeback blocked as a duplicate
- skills: code-review (high --fix, 2 findings fixed), handoff
- handoff: Production report: writing Part 3 to file said `"undefined" already matches another video`. The owner chose "part number counts"; fix, spec rule and tests landed with every gate settled. Next is merging the PR.

## Dropped — newest first (the reason is the point)

- [~] [frontend] "Save anyway" in the Write-to-file dialog — dropped 2026-11-14, the owner chose the part rule alone; revisit only if a non-part false positive shows up.
