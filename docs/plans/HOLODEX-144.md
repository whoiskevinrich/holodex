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
release_note: Chip rows such as Actors and Director show where their values came from with the provider's icon, like single values already do, instead of a source label on every chip.
approved:
  design:
    on: 2026-10-24
    at: 4d6cc4b9
---

# HOLODEX-144 · Chip-row provenance (epic 2: Provenance badges)

Epic 2 of the frontend backlog plan (Phase 4, Redesigns). It held HOLODEX-144 and HOLODEX-92; the
owner closed 92 as Won't Do on 2026-10-24, because its only possible link is the entity-level one
`ProviderLinkBadge` already shows.

**Design package:** [handoff](../design/chip-row-provenance-handoff.md) ·
[mockup](../design/chip-row-provenance-mockup.svg). The owner picked option B on 2026-10-24.

## Gates — definition of done

<!-- Rows match the `ui` posture. -->

- [x] design `design-handoff` → `docs/design/**` — `chip-row-provenance-handoff.md` + mockup SVG (option B); owner signed off 2026-10-24
- [x] frontend → `web/src/**` — `#lib/chipProvenance` (row rule, reusing `providerFromWinningSource`); `CurationFieldRow` renders the row badge and passes `mark`; `CurationChip` plain branch drops the `·` text for an optional 12px `ProviderIcon`
- [x] testing `testing-strategy` — `chipProvenance.test.ts` (10), `CurationFieldRow.svelte.test.ts` (6); live QA on media 902 visitor rows; testing-strategy §5 row

## Up next — ordered (position = priority)

1. [x] [—] Wait for PR #549 (HOLODEX-357) to merge, then merge `main` in
2. [x] [design] Owner sign-off on the handoff, then `/implement`
3. [x] [frontend] Build the row rule in `CurationFieldRow` and the icon in `CurationChip` — `web/src/lib/components/curation/`
4. [x] [testing] Row-key unit tests, component tests, QA list §1–7
5. [x] [design] The handoff's QA list and mockup use Actors, but on the media page Actors render in the People section, not a chip row. Name Director (and a visitor's replace fields) instead. Changing `docs/design/**` needs the owner to re-confirm the sign-off — `docs/design/chip-row-provenance-handoff.md`
6. [x] [—] Mark PR #551 ready once 5 is settled
7. [ ] [—] On merge, check HOLODEX-144 reached Done (CI moves the branch's own key)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-24 · session
- skills: design-handoff, implement, code-review
- decided: HOLODEX-357 first (adopt 24px, PR #549); for 144, option B (one row badge, icons only
  when mixed); HOLODEX-92 is Won't Do.
- then: the owner caught provider-specific fields in the mockup (Tagline, Genres, an invented
  Keywords). Redrew it with Actors and Director and a generic `provider-1`; Tags is out of scope
  (no provenance by design). The rule for future docs went to its own PR, #550.
- then: PR #549 squash-merged (87a08a42); HOLODEX-357 and HOLODEX-580 moved to Done; `main` merged in.
- then: built it. 716 frontend tests pass, type-check clean. Live QA on media 902 (visitor): the
  Language and Tagline rows end with one `from alpha` badge, with no `·alpha` text. The stress
  fixture has no mixed-source merge row, so the mixed case is covered by the component test.
  `/code-review high --fix`: reused `providerFromWinningSource` (it also excludes `computed` and
  `tag`); updated the curation CLAUDE.md table and testing-strategy. Skipped one finding: the
  handoff names a row the media page doesn't render this way (Up next 5).
- then: the handoff and mockup now name Director, not Actors (4d6cc4b9). The owner re-confirmed
  the design sign-off against that diff, and it's re-pinned at 4d6cc4b9.
- handoff: Every gate is settled and PR #551 is ready for review; next is the merge and checking HOLODEX-144 reached Done.

## Dropped — newest first (the reason is the point)

- HOLODEX-92 (per-field badge links to the provider's page): the only URL is per entity, so a
  per-field link repeats the title's link badge on every row. Won't Do, 2026-10-24.
