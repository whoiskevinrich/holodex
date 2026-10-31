---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-610
status: in-progress
profile: feature             # changed clear/rematch rule (backend) + a Remove control on the person hero
depends-on: []
release_note: Clearing or rematching a person's provider now removes the images, alternate names and link it brought, and owners can remove a person's headshot, banner or poster from the page.
---

# HOLODEX-610 · Clearing or rematching a person's provider leaves its images, aliases and link

Clearing a person's provider removed only its field values, and a rematch was additive, so the old
match's images, alternate names and external link stayed. Clear now removes them (uploads,
promotions and the owner's aliases stay), and a rematch to a different record clears the old match
first. The person hero gains a Remove control beside Edit on each filled core slot. Studio and film
clears are unchanged.

**Design package:** none — the Remove control reuses `CoreImageEditButton`'s chrome; the mockup was
approved in chat on 2026-10-30 (Remove beside Edit, all three core slots, removal may be refilled).

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — `people-images.md` F25.36, F25.37
- [~] design `design-handoff` → `docs/design/**` — one control in existing chrome; no handoff doc
- [x] backend → `{cmd,internal,providers}/**`
- [x] frontend → `web/src/**`
- [~] testing `testing-strategy` → `docs/testing-strategy.md` — no strategy change; covered by `TestClearPersonRemovesProviderImagesAliasesAndLink`, `TestRematchPersonClearsOldMatch`, `TestClearStudioLeavesImages`, `TestDeleteProviderPersonImages`, `TestSinkRemoveProviderAssets`, `coreImageEditButton.test.ts`; Remove QA'd live on `backend-stress-open-9600`

## Up next — ordered (position = priority)

- Mark the PR ready once review is done; merge, then confirm HOLODEX-610 reached Done.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-30 · clear/rematch cleanup + hero Remove
- skills: code-review (high --fix), implement
- handoff: Fixed and committed (Go + vitest green, Remove verified in the browser; clear itself covered by Go tests only, the stub sends no images); next: review the draft PR and mark it ready.

## Dropped — newest first (the reason is the point)
