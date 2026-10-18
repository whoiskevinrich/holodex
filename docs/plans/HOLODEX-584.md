---
key: HOLODEX-584
status: in-progress
profile: ui                  # UX change, behavior unchanged — which writeback rows stack on a phone
depends-on: []
approved:
  design:
    on: 2026-10-18
    at: 00ca4df5
release_note: On a phone, the writeback dialog now stacks a field's candidates whenever a chip would be too wide for the screen, so no value is cut off.
---

# HOLODEX-584 · Writeback chip can still clip in a column narrower than its 14rem value span

This is done when no writeback chip truncates its value at any dialog width (HOLODEX-434's rule).
Chips that fit stay chips.

**Root cause:** HOLODEX-483 measured the value against the chip's 14rem span, but the chip button is
`max-w-full` beside a shrink-0 `·source` suffix. At 375px the chooser column is 282px, which leaves
the value roughly 170–218px depending on the suffix, so values under 224px still clipped.

**Fix:** option B from the 2026-10-17 mockup (over wrapping, or always stacking on phones).
`chipFits` measures the whole chip (value, suffix, `, pending`, chrome) against the chooser's
bound width.

**Design package:** bug fix, no spec or architecture fork ·
[design handoff](../design/writeback-dialog-always-open-handoff.md) (the stacking rule, edited in
place)

## Gates — definition of done

- [x] design `design-handoff` → `docs/design/**`. Handoff stacking rule now measures the whole chip against the column; owner approved 2026-10-18.
- [x] frontend → `web/src/**`. `chipFits` in `writebackCockpit.ts`; `WritebackFormDialog` binds the chooser width and measures an empty chip as its placeholder.
- [x] testing `testing-strategy`. `chipFits` unit tests (value cap, column, suffix, pending); `svelte-check` clean. In the live phone-width dialog (282px column), `chipFits` matched real truncation in all 21 cases.

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge the PR, then confirm CI moves HOLODEX-584 to Done.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-18 · measured the whole chip against the chooser column
- skills: code-review, handoff
- handoff: Built, verified at 375px, and approved; every gate is settled and the PR is ready. Next: review and squash-merge.
