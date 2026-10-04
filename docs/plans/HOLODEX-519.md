---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-519
status: in-progress
profile: feature
depends-on: []
release_note: "Adding tags on a media page is faster: the input stays open after each tag, suggests existing tags as you type (Tab takes the suggestion), and your tags list pending file changes first."
approved:
  design:
    on: 2026-10-04
    at: c0195e12
---

# HOLODEX-519 · Media page tag input: Enter adds and clears, autocomplete, status-sorted chips

The owner's add-tag form on the media page closes after every tag, offers no existing tags while
typing, and lists chips in no useful order. Done = Enter adds and clears with the form staying open,
a suggestion list of existing tags (Enter takes the text, Tab takes the suggestion), and owner chips
sorted with pending writeback changes first.

**Design package:** spec F50 P0-8a (`docs/specs/tag-governance-and-video-enrichment.md`) · handoff
`docs/design/media-tag-input-handoff.md` · mockup `docs/design/media-tag-input-mockup.svg`

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — F50 P0-8a amendment
- [x] design `design-handoff` → `docs/design/**` — `media-tag-input-handoff.md` + mockup SVG
- [~] backend → `{cmd,internal,providers}/**` — no server change: `GET /tags` and `POST /media/{id}/tags` already serve it
- [x] frontend → `web/src/**` — `entity/TagAddInput.svelte`, `lib/tagInput.ts`, media page wiring; owner confirmed the build matches the mockup
- [x] testing `testing-strategy` — `lib/tagInput.test.ts` (sort + suggestion filter, 10 cases); keyboard paths QA'd in the dev server (web/ has no component-test harness); no strategy change

## Up next — ordered (position = priority)

1. [ ] [—] After merge, eyeball the status sort on a library with writeback status (on-file glyphs) — the AMV testbed has none

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-04 · build, review, QA
- skills: code-review (high --fix), handoff
- handoff: Built and QA'd; owner confirmed the build matches the mockup and re-confirmed the sign-off at c0195e12. PR #443 marked ready — merge, then eyeball the status sort on a writeback-enabled library.

### 2026-10-03 · mockups, design handoff, spec amendment, crossing into build
- skills: design-handoff, implement
- handoff: Crossed into build — design signed off at a81f8f04; draft PR open. Start at `sortTagsByStatus` + unit tests.

## Dropped — newest first (the reason is the point)
