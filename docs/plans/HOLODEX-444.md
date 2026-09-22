---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-444
status: in-progress
release_note: The person hover card no longer gets cut off at the top of a short window — when it has no room below or above the name, it opens below and the page scrolls to it.
---

# HOLODEX-444 · Person hover card clips off the top when it fits neither below nor above

Corner of the F68 placement rule (HOLODEX-431, #369). `placeCard` flipped the card above
whenever it did not fit below **and** above had more room than below — even when above did not
fit either. Above the viewport is a negative `y` nothing can scroll to, so in a short window
(a 364 px browser pane, a People tile near the top) the card's top was cut off. Fix: flip above
only when the card actually fits above (`trigger.top >= cardH + gap`); otherwise stay below,
where the card extends the page and scrolls into view. R3/RD10 and the handoff now say so.

## Gates — definition of done

- [x] spec `write-spec` — R3 + RD10 in `docs/specs/person-hover-card.md` amended with the neither-fits clause
- [~] architecture `architecture` — n/a: pure placement math, no ADR
- [x] design `design-handoff` — flip paragraph in `docs/design/person-hover-card-handoff.md` amended
- [x] frontend — `personCard.svelte.ts` `placeCard`
- [x] testing `testing-strategy` — `personCard.test.ts` pins the neither-fits-above-has-more-room case
- [~] security `security-review` — n/a
- [x] `code-review high --fix` — no findings

## Up next — ordered (position = priority)

1. [ ] [—] Merge; HOLODEX-444 → Done via CI (branch-keyed)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-21 · found while triaging "hover cards not working", fixed, verified live
- skills: code-review high --fix
- The report itself was a not-yet-released `1468d29` (ships in 1.16.0, release PR #210); this
  clip surfaced while reproducing on a short pane. Test written first (failed on HEAD), rule
  fixed, live-verified on `/media/2`: neither-fits → below at `top: 259` and scrolls into view;
  a bottom-of-pane tile with room above still flips above, fully visible.
- handoff: fix + test + spec/handoff wording shipped; PR open, ready for review.
