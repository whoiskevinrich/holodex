---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-439
status: in-review
depends-on: [HOLODEX-431, HOLODEX-349]
release_note: Internal only — the layout test harness now checks that the person hover card never pushes a page sideways.
---

# HOLODEX-439 · a hover preparation so the person card's clamp becomes a harness rung

F68's person hover card (HOLODEX-431) is mounted on hover, and the §12 geometry harness had no
pointer step, so the card's "no horizontal scroll" and "16 px right gutter" invariants were live
`[agent]` checks only. This adds the `person-card-open` preparation, a `gutterRight` probe metric
and two rungs — `person-card-no-page-overflow` and `person-card-inside-gutter`.

**The host moved from film pages to media pages, on evidence.** The ticket suggested a film's Cast
grid; a live survey found its right-most tile ends 40 px+ short of the clamp zone at every width
cell, so the card never clamps there and both rungs would pass with the clamp deleted. A media
page's People grid with 25+ people clamps at all three widths (`translateX` −170 / −159.3 / −112),
landing at exactly 16 px. **Mutation-tested:** `placeCard` returning `shiftX: 0` fails all 12
cinematheque checks.

Out of scope and filed: every broadcast/brutalist cell errors for every rung since ADR-102 made
the skin instance identity — the harness's localStorage skin pin is dead → HOLODEX-460. The
flip-above half of the card (bottom-edge trigger) stays a live check.

## Gates — definition of done

- [~] spec `write-spec` — n/a: test tooling, no requirement change
- [~] architecture `architecture` — n/a: no seam, stack or data-model decision
- [~] design `design-handoff` — n/a: no UI change
- [~] backend — n/a: harness-only
- [~] frontend — n/a: `web/geometry/` only; no `web/src/**` change
- [x] testing `testing-strategy` — rungs + preparation in `web/geometry/`; §4 F68 row and §12.4
  updated in `docs/testing-strategy.md`; 12/12 pass (cinematheque), mutation-tested
- [~] security `security-review` — n/a: dev-only harness, no auth/access/infra surface

## Up next — ordered (position = priority)

1. [ ] [—] Fix the harness skin pin so broadcast/brutalist cells measure again → HOLODEX-460

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-26 · session
- skills: code-review high --fix (1 fixed: stale "Cast tile" wording; 2 skipped: vacuous-pass
  guard needs HOLODEX-395's relative measure, scrollbar basis mirrors `placeCard`), handoff
- handoff: rungs shipped and mutation-tested on the cinematheque cells; PR is ready — next is HOLODEX-460 so the other two skins measure again.
