---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-433
status: in-progress
release_note: Picking a provider poster (or any provider value you had not decided on yet) in the "Write to file" dialog now actually writes it to the file — previously it only saved the choice and left the file untouched. Poster tiles in that dialog also show the whole image at its own proportions instead of cropping it.
---

# HOLODEX-433 · Poster picker: undecided provider tile never writes + tiles crop the image

Two bugs in the poster chooser shipped by HOLODEX-403 (#364), reported by the owner.

**1. Write never happens.** The resolver returns `in_sync: true` for every *undecided* replace
field by construction (`replaceMarkers`, nothing decided ⇒ nothing to lag). The `rowClass` clause
#364 added — `in_sync === true && value === values[0]` ⇒ `matches` — therefore read an undecided
provider poster as already in the file, so picking its tile classified as decision-only: one
`PUT …/decision`, no writeback, and the page poster (served from the file's extracted cover)
never changed. Same regression for every undecided provider-winning text row (overview, tagline,
release_date, original_language on the films testbed). Fix: gate the clause on
`decision.standing === true` — the backend only consults the ledger witness for a standing
decision, matching ADR-101 D1.

**2. Cropped tiles.** `SourceImageTiles` used a fixed `h-20 w-14` box with `object-cover`. Now
`h-20 w-auto min-w-14 object-contain`: the image's own ratio, a 56 px floor so a broken URL keeps
a clickable box, letterboxing only for a narrower-than-2:3 image.

## Gates — definition of done

- [~] spec `write-spec` — n/a: bug fix; the handoff's QA item 8.3 already specifies the restored behaviour
- [~] architecture `architecture` — n/a: ADR-101 stands (the frontend now matches its D1)
- [~] design `design-handoff` — n/a: sizing fix inside the existing tile design
- [x] frontend — `writebackCockpit.ts` standing-decision gate; `SourceImageTiles.svelte` sizing
- [x] testing `testing-strategy` — `writebackCockpit.test.ts` undecided case (image + text); §5 row amended
- [~] security `security-review` — n/a
- [x] `code-review high --fix` — one finding (broken image collapsed the `w-auto` tile), applied

## Up next — ordered (position = priority)

1. [ ] [—] Owner's look at the tiles on the prod skin, then `gh pr ready`; on merge HOLODEX-433 → Done via CI (branch-keyed)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-19 · root-caused against live API data, fixed, verified end to end
- skills: code-review high --fix
- Static reading of the dialog/backend showed nothing; `curl`ing `/media/3` on the films testbed
  showed every undecided provider row with `in_sync: true`, which the #364 `rowClass` clause
  turned into `matches`. Reproduced in the dialog (`Save 1 decision`), fixed, re-verified
  (`Write 1 field to file`, job landed, file carries the TMDB cover, witness `in_sync: true`).
- handoff: both fixes shipped with tests and three-skin geometry checks; Draft PR open for the
  owner's look at the tiles.
