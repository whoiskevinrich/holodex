---
key: HOLODEX-483
status: in-progress
profile: ui                  # UX change, behavior unchanged — which writeback rows stack
depends-on: []
approved:
  design:
    on: 2026-10-17
    at: 66a4546e
release_note: The writeback dialog now stacks a field's candidates whenever one is too wide to show whole in a chip, so all-caps and CJK values are no longer cut off, and ordinary titles up to about 37 characters stay as chips.
---

# HOLODEX-483 · Retune CHIP_VALUE_MAX_CHARS for Archivo

This is done when a writeback chip never truncates a candidate in Archivo (HOLODEX-434's rule), and
values that really fit stay chips.

**Finding:** the ticket assumed Archivo is narrower than the retired mono, so the cap could rise. It
can't. Archivo is proportional, and at 12px in the 224px value span it fits ~37 title-case
characters but only 27 all-caps, 20 W's, 18 CJK and 15 Œ. So the old cap of 32 already clipped. The
owner compared four options in a mockup (cap 32 / measured width / cap 18 / cap 15) and chose
**measured width**.

**Design package:** bug fix, no spec or architecture fork ·
[design handoff](../design/writeback-dialog-always-open-handoff.md) (the stacking rule, edited in
place)

## Gates — definition of done

- [x] design `design-handoff` → `docs/design/**`. Handoff doc's stacking rule now says measured width; owner approved 2026-10-17.
- [x] frontend → `web/src/**`. `chipValueFits` in `writebackCockpit.ts` measures with canvas in the `--font-ui` face against 224px; falls back to a 15-character cap with no canvas or before the font loads. `stacksCandidates` takes it as a `fits` seam.
- [x] testing `testing-strategy`. Unit tests for measured stacking and the fallback; `svelte-check` clean; in the dev server `chipValueFits` matched real DOM clipping on 10 samples (title case, all caps, CJK, W and Œ runs at the boundary). No testing-strategy row: the existing writeback unit-test coverage covers it.

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge the PR, then confirm CI moves HOLODEX-483 to Done.
2. [ ] [—] Narrow containers can still clip a chip below 224px → HOLODEX-584

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-17 · measured Archivo and replaced the char cap with measured width
- skills: code-review, handoff
- handoff: Built, tested and approved; every gate is settled and the PR is marked ready. Next: review and squash-merge. The narrow-container gap is HOLODEX-584.
