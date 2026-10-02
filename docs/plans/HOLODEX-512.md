---
key: HOLODEX-512
status: in-progress
profile: ui                  # UX change, behavior unchanged — the media title's rename field
depends-on: []
approved:
  design:
    on: 2026-10-02
    at: 300ef43c
release_note: Renaming a media item with a long title now shows the whole title while you edit it. The field spans the header, wraps onto more lines as needed, and uses the title's own type.
---

# HOLODEX-512 · Media title rename input is clipped for long titles

This is done when the media page's title rename field shows the whole title. It spans the
header row, wraps, and uses the heading's type. Enter saves, Esc cancels, and newlines are never
saved. Every other `NameEditControl` mount (Person/Studio/Tag headings, Film year) is unchanged.

**Root cause:** the page's title wrapper is a shrink-to-fit flex item, so in the edit state it sizes
to the `<input>`'s ~20ch intrinsic width. Measured: a 246px input in a 700px row for an 802px title.

**Design package:** bug fix, no spec or ADR (rename behavior is unchanged) ·
[design handoff](../design/title-edit-in-place-handoff.md) (option B from the 2026-10-02 critique)

## Gates — definition of done

- [x] design `design-handoff` → `docs/design/**`. `title-edit-in-place-handoff.md` + committed SVG mockup; owner picked B in the critique.
- [ ] frontend → `web/src/**`. `NameEditControl` `multiline` prop; media title wrapper `has-[form]:basis-full`.
- [ ] testing `testing-strategy`. Browser QA per the handoff's checklist, plus a `docs/testing-strategy.md` row.

## Up next — ordered (position = priority)

1. [x] [—] `/implement`: owner signs off on the committed handoff, then build.
2. [ ] [—] Build the `multiline` field and the wrapper class; run `npm run check`; QA the handoff checklist in Cinémathèque.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-02 · critiqued the title edit, filed the bug, wrote the handoff
- skills: design-critique, design-handoff, implement
- handoff: Crossed into build. Design signed off at 300ef43c; draft PR open. Start at Up next item 2 (the build).

## Dropped — newest first (the reason is the point)
