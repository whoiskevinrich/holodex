---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-144                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: ui                  # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: [HOLODEX-357]    # chips at 24px land first (PR #549)
release_note: Chip rows such as Genres and Actors show where their values came from with the provider's icon, like single values already do, instead of a "·tmdb" label on every chip.
---

# HOLODEX-144 · Chip-row provenance (epic 2: Provenance badges)

Epic 2 of the frontend backlog plan (Phase 4, Redesigns). It held HOLODEX-144 and HOLODEX-92; the
owner closed 92 as Won't Do on 2026-10-24, because its only possible link is the entity-level one
`ProviderLinkBadge` already shows.

**Design package:** [handoff](../design/chip-row-provenance-handoff.md) ·
[mockup](../design/chip-row-provenance-mockup.svg). The owner picked option B on 2026-10-24.

## Gates — definition of done

<!-- Rows match the `ui` posture. -->

- [x] design `design-handoff` → `docs/design/**` — `chip-row-provenance-handoff.md` + mockup SVG (option B); sign-off asked at /implement
- [ ] frontend → `web/src/**` — `CurationFieldRow` row rule, `CurationChip` plain branch drops the `·` suffix for an optional 12px icon
- [ ] testing `testing-strategy` — unit tests for the row-key rule, component tests for both chips, handoff QA list

## Up next — ordered (position = priority)

1. [ ] [—] Wait for PR #549 (HOLODEX-357) to merge, then merge `main` in
2. [ ] [design] Owner sign-off on the handoff, then `/implement`
3. [ ] [frontend] Build the row rule in `CurationFieldRow` and the icon in `CurationChip` — `web/src/lib/components/curation/`
4. [ ] [testing] Row-key unit tests, component tests, QA list §1–7

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-24 · session
- skills: design-handoff
- decided: HOLODEX-357 first (adopt 24px, PR #549); for 144, option B (one row badge, icons only
  when mixed); HOLODEX-92 is Won't Do.
- handoff: The design handoff for chip-row provenance is committed and waiting on the owner's sign-off; build after PR #549 merges.

## Dropped — newest first (the reason is the point)

- HOLODEX-92 (per-field badge links to the provider's page): the only URL is per entity, so a
  per-field link repeats the title's link badge on every row. Won't Do, 2026-10-24.
