---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-215
status: in-progress
profile: ui                  # design-system refactor; no product rule changes
depends-on: []
release_note: Every dialog now opens in one of two consistent positions and four widths, Escape closes only the topmost dialog, and dialogs that had no Escape now get one.
---

# HOLODEX-215 · Consolidate hand-rolled modals into one shared Modal primitive

Eleven dialogs each build their own modal shell. They use five vertical paddings, seven widths,
two z-indices and seven focus-trap selectors. Escape is bound in three places, so one keypress
can close several dialogs at once. The ticket counted nine dialogs. Since then `PickerShell`
already wraps seven pickers, and `HotkeySheet` and `WritebackBatchDialog` were added.

**Design package:** [shared-modal-handoff.md](../design/shared-modal-handoff.md) ·
![mockup](../design/shared-modal-mockup.svg)

## Gates — definition of done

- [x] design `design-handoff` → `docs/design/**` — two positions, four widths, stack-aware Escape and layer, Modal owns the surface; the owner approved all four plus the stacked `/85` backdrop on 2026-10-14
- [ ] frontend → `web/src/**` — `shared/Modal.svelte` + 11 call sites migrated; `rg 'trapTab' web/src --glob '*.svelte'` hits only Modal
- [ ] testing `testing-strategy` — browser-mode `Modal.svelte.test.ts` (trap, return, busy, stack); HOLODEX-215 entry in `docs/testing-strategy.md`

## Up next — ordered (position = priority)

1. [ ] `/implement` — open the Draft PR
2. [ ] Build `shared/Modal.svelte` and correct the selector in `lib/focusTrap.ts`
3. [ ] Migrate the 11 call sites; remove PersonGallery's Escape routing
4. [ ] Add Modal component tests and a testing-strategy entry
5. [ ] Live QA in Cinémathèque: every dialog, Gallery → Viewer stack, busy states

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-14 · session
- skills: design-handoff
- handoff: The design is approved and committed (handoff and SVG). Next: `/implement`, then build Modal and migrate the 11 dialogs.

## Dropped — newest first (the reason is the point)
