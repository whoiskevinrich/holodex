---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-491
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: ui                  # a UX consistency fix, plus one owner affordance (header "+ Set edition")
depends-on: [HOLODEX-490]    # merged (#417 → 11c7b7da); this branch was rebased onto main
approved:
  design:
    on: 2026-09-29
    at: 7fcc32a8                # bc04db6c before the rebase onto main (#417 squash-merged); same tree
release_note: A media page now offers "+ Set edition" beside "+ Set part" when a file has no edition yet. On a film page, the "+ Set edition" link on a full-film file is readable and easy to hit, and the media page's add-a-film tile reads "Add film" like its neighbours.
---

# HOLODEX-491 · "+ Set edition": film-page text CTA, media-page header slot; "Add film" ghost tile

This is done when the last dashed missing-value pill, the film page's "+ Set edition", is the
`btn-quiet` text CTA; the Films ghost tile says "Add film" like PersonPicker's "Add person" tile and
the empty "+ Add film" CTA; and the media page header offers "+ Set edition" beside "+ Set part"
(owner ask, 2026-09-29). Spun off from HOLODEX-490 (Relates).

## Gates — definition of done

- [x] design `design-handoff` → `docs/design/**`. The film-page and ghost-tile work applies the HOLODEX-490 rule, with supersede notes on `entity-identity-card-handoff.md` §3b and `media-detail-reorder-handoff.md`; "Attach" wasn't deliberate (HOLODEX-328 §1). The header "+ Set edition" relaxes F60 RD6's display-only header for an empty edition. That's recorded as an amendment under the as-built table in `entity-identity-card-handoff.md`, as a cross-note in `media-parts-handoff.md`, and as a translation row in `ui-vocabulary.md`.
- [x] frontend → `web/src/**`. Film page: full-size text CTA; a long title wraps beneath it (row 78 px, was 67 px), a short one sits inline (50 px), and there's no x-overflow at 375 px. Media page: "Add film" tile, `aria-haspopup="dialog"` on both Films add buttons, and Part's editor generalized into one header-slot editor for edition and part. Both slots are gated on a curatable completeness facet. Verified on the stress fixture against a scratch mapping that adds `edition`: edition then part, one open at a time, Escape restores, Enter saves a standing manual decision and the pill replaces the CTA. On the stock stress mapping (no `edition`) only "+ Set part" shows. `npm run check`: 0 errors.
- [~] testing `testing-strategy`. Skipped deliberately: `web/` has no component harness, and the editor reuses the proven `decideField` path. The `part-pill-beside-the-title` geometry assertion is unaffected.

## Up next — ordered (position = priority)

1. [ ] [—] Mark #418 ready once CI is green on the rebased branch, then merge
2. [ ] [—] Sweep HOLODEX-490 to Done if jira-sync didn't (the squash merge should fire it)

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-09-29 · session
- skills: code-review, implement
- handoff: Crossed into build (design signed off at 4084de07, Draft PR #418), then added the owner's header "+ Set edition" beside "+ Set part" on a shared, facet-gated slot editor. Code-review skipped the identifier-derived CTA copy and the pre-existing blur-clears-error behaviour. You re-confirmed the design at bc04db6c (7fcc32a8 after the rebase). #417 is squash-merged, and this branch was rebased onto main at your request and force-pushed with lease. Start at Up next item 1: mark #418 ready once CI is green.
