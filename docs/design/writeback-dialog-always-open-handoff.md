# Design Handoff: Writeback dialog — every chooser open, no disclosure, wider on desktop

**Builds on**: [writeback-cockpit-handoff.md](writeback-cockpit-handoff.md) (HOLODEX-400 — the
row classes, gutter glyphs, chooser shapes and commit semantics are ground truth; this handoff
changes only *when* a chooser is visible and how wide the dialog is) ·
[writeback-selection-handoff.md](writeback-selection-handoff.md) (HOLODEX-213 option A — the
decided/undecided disclosure this handoff **retires**).
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[theming.md](theming.md) — **tokens only, QA all three skins.**
**Surface**: `web/src/lib/components/writeback/WritebackFormDialog.svelte` (the only file whose
behaviour changes). No endpoint, no ADR, no new component.
**Issue**: [HOLODEX-434](https://whoiskevinrich.atlassian.net/browse/HOLODEX-434).

![Left: the HOLODEX-400 cockpit — decided rows lead, the undecided rows sit behind a "4 provider values you haven't decided on" chevron, and a row that matches the file shows a quiet "change" toggle. Right: the same dialog with the chevron gone — undecided rows render under a static "Not yet decided" caption — every row's chooser open, and the dialog widened to max-w-3xl. Bottom: the phone rendering, identical in width to today](writeback-dialog-always-open-mockup.svg)

---

## Overview

The owner's ask (2026-09-19): *"The Media Detail writeback modal should always include the
change dialog. Remove the '…provider values you haven't decided on' chevron. Each item should
behave as if 'change' were clicked. Increase the modal width for wider screens; mobile width
should stay the same."*

Three changes, all subtractive except the width:

1. **No disclosure.** The HOLODEX-213 chevron defended "the dialog's default weight matches the
   header count". It cost every undecided row a click before the owner could act on it. The
   undecided group now renders inline under a static caption, `Not yet decided` — information,
   not a control (no chevron, not clickable, no `aria-expanded`).
2. **No `change` toggle.** Every cockpit row renders its chooser unconditionally — the chip row,
   the stacked radio list for `long_text`, the image tiles — exactly as if `change` had been
   clicked. The `=` tier keeps its gutter glyph and its `value — matches the file` line above
   the chooser; the `·file` chip is the checked one, so the row reads "this is on file, here are
   the alternatives". M → W promotion is unchanged: stage a non-file chip and the glyph flips to
   `↧` and the footer count includes the row.
3. **Wider on desktop.** `max-w-xl` (576px) → `max-w-3xl` (768px). The overlay is `px-4`, so on
   a 375px phone the dialog is 343px either way — mobile is unchanged for free. `max-w-3xl` is
   the size `FilmBulkAttachDialog` already uses.

### What does *not* change

Gutter glyphs and their titles, the three chooser shapes, `onStaged` → `touched`, the footer's
`Write n fields / Save n decisions` copy, focus trap + return, Escape, the `max-h-[60vh]` body
scroll, the read-only merge rows. `chooserOpen` is deleted from `Row` — with the chooser always
mounted there is nothing left for it to gate, and the "one mount point so the radiogroup never
unmounts under the keyboard" guarantee holds trivially.

## Decided visual spec

| Element | Before | After |
|---|---|---|
| Dialog | `w-full max-w-xl …` | `w-full max-w-3xl …` (rest unchanged) |
| Undecided group heading | `btn-quiet` chevron button, `aria-expanded`/`aria-controls`, text `{n} provider value(s) you haven't decided on` | `<p class="mt-3 border-t border-rule pt-3 text-xs text-muted">Not yet decided</p>` |
| Undecided rows | `hidden={!showUndecided}` | always rendered, `mt-3 space-y-3` |
| `=` row header | `change` / `close` `.btn-quiet ml-auto text-xs` toggle | no toggle |
| `=` row body | value line; chooser only when `row.chooserOpen` | value line, then the chooser |
| Chooser ids | `id="wb-chooser-{canonical}"` (target of `aria-controls`) | dropped — nothing points at them |

No new tokens. No hardcoded values.

## Accessibility

- Removing the two toggles removes their `aria-expanded` / `aria-controls` pairs; no dangling
  references remain.
- Initial focus still lands on the first decided row's checked chip (unchanged).
- The caption is a `<p>`, read once in document order; it is not a heading (the dialog has one
  `<h2>`) and not a landmark.

## Verification checklist

1. `[smoke]` Open the dialog on a video with ≥1 standing decision the file lags and ≥1
   undecided provider value: no chevron, both groups visible, caption `Not yet decided` between
   them.
2. `[smoke]` A row whose staged value matches the file shows `value — matches the file` and its
   chooser directly beneath; no `change` link anywhere.
3. `[agent]` Stage a non-file chip on an `=` row: gutter flips to `↧`, footer count +1; stage
   `·file` again: glyph and count revert.
4. `[agent]` Desktop viewport ≥ 900px: dialog `getBoundingClientRect().width` = 768. 375px
   viewport: width = 343 (unchanged from main).
5. `[human]` Three skins (Cinémathèque, Broadcast, Brutalist): caption contrast, chip rows wrap
   cleanly at 768 and at 343, the body scrolls rather than the page when the row count is high.
