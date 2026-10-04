---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-519
status: in-progress
profile: feature
depends-on: []
release_note: ""
approved:
  design:
    on: 2026-10-04
    at: a81f8f04
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
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`

## Up next — ordered (position = priority)

1. [ ] [frontend] `sortTagsByStatus` helper + unit tests, apply in the owner chip branch — `web/src/routes/media/[id]/+page.svelte`
2. [ ] [frontend] `TagAddInput.svelte` combobox (Enter/Tab/↑↓/Esc, aria-activedescendant) — `web/src/lib/components/entity/`
3. [ ] [frontend] Page wiring: stay-open add, Cancel → Done, near-miss keeps typing; `entity/CLAUDE.md` row + combobox exception note
4. [ ] [testing] `/testing-strategy`, then QA in the dev server (Cinémathèque)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-03 · mockups, design handoff, spec amendment, crossing into build
- skills: design-handoff, implement
- handoff: Crossed into build — design signed off at a81f8f04; draft PR open. Start at `sortTagsByStatus` + unit tests.

## Dropped — newest first (the reason is the point)
