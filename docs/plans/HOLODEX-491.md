---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-491
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: ui                  # a UX consistency fix; behaviour unchanged
depends-on: [HOLODEX-490]    # stacked on its branch: the rule this amends lands with #417
release_note: On a film page, the "+ Set edition" link on a full-film file is now readable and easy to hit, and the media page's add-a-film tile reads "Add film" like its neighbours.
---

# HOLODEX-491 · Film page "+ Set edition" pill; Films ghost tile says "Attach film"

This is done when the last dashed missing-value pill, the film page's "+ Set edition", is the
`btn-quiet` text CTA, and the Films ghost tile says "Add film" like PersonPicker's "Add person" tile
and the empty "+ Add film" CTA. Spun off from HOLODEX-490 (Relates).

## Gates — definition of done

- [x] design `design-handoff` → `docs/design/**`. No new decision: it applies the HOLODEX-490 rule, which you signed off. Supersede notes are on `entity-identity-card-handoff.md` §3b (the dashed spec) and `media-detail-reorder-handoff.md` (origin of "Attach film"). The rule drops its holdout line and gains the dense-row and ghost-label clauses. "Attach" wasn't deliberate: HOLODEX-328 §1 calls the text CTA and the dashed tile "the same affordance at two densities".
- [x] frontend → `web/src/**`. Full-size text CTA, measured on the stress fixture with two full-film files attached to film 9000. A long title wraps beneath the title exactly as the old pill did (row 78 px, was 67 px). A short title sits inline (50 px). No x-overflow at 375 px. `btn-row` wasn't needed. The tile reads "Add film", and both Films add buttons gained `aria-haspopup="dialog"` to match PersonPicker. `npm run check`: 0 errors.
- [~] testing `testing-strategy`. Skipped deliberately: class and copy change, no behaviour change, and `web/` has no component harness.

## Up next — ordered (position = priority)

1. [ ] [—] Merge #417 (HOLODEX-490) first, then merge main into this branch and mark this PR ready

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-09-29 · session
- skills: code-review
- handoff: "+ Set edition" is now the full-size text CTA; the dense row wraps exactly as before, so there's no rule fork. The ghost tile says "Add film". This branch is stacked on HOLODEX-490, so its PR stays Draft until #417 merges.
