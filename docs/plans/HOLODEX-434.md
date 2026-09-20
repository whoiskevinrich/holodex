---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-434
status: in-progress
release_note: The "Write metadata to file" dialog now shows every field's source chooser up front — no more "provider values you haven't decided on" fold to open and no per-row "change" link — and is wider on desktop screens. Phones are unchanged.
---

# HOLODEX-434 · Writeback dialog: always-open choosers, no undecided disclosure, wider on desktop

Owner's ask (2026-09-19): the Media Detail writeback modal should always include the change
dialog — remove the "…provider values you haven't decided on" chevron, make each row behave as if
`change` were clicked, widen the modal on wide screens, keep mobile width the same.

Three subtractive changes in `WritebackFormDialog.svelte` plus one class: the HOLODEX-213
disclosure is replaced by a static `Not yet decided` caption; the `=`-tier `change`/`close`
toggle and the `chooserOpen` row state are deleted so every cockpit row renders its chooser;
`max-w-xl` → `max-w-3xl` (the overlay's `px-4` gutter binds first on a phone, so 375px is 343px
either way).

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/field-source-of-truth.md` §Writeback paragraph amended
- [~] architecture `architecture` — n/a: frontend-only, no seam touched
- [x] design `design-handoff` — `docs/design/writeback-dialog-always-open-handoff.md` + SVG; supersession note in the cockpit handoff
- [x] frontend — dialog edited in place; `web/src/lib/components/writeback/CLAUDE.md` updated
- [~] testing `testing-strategy` — n/a: no test referenced the removed controls; the pure predicates in `writebackCockpit.ts` are untouched (380/380 green)
- [~] security `security-review` — n/a
- [x] `code-review high --fix` — one finding (stale focus comment), applied
- [x] three-skin QA — Cinémathèque / Broadcast / Brutalist: caption contrast 6.0 / 4.7 / 5.6 : 1, 768px desktop, 343px phone, no overflow; M → W promotion round-trips on the `=` Poster row with focus kept

## Up next — ordered (position = priority)

1. [ ] [—] Owner's look on the prod skin, then `gh pr ready`; on merge HOLODEX-434 → Done via CI (branch-keyed)
2. [ ] [—] If a real film's row count makes the `max-h-[60vh]` body scroll uncomfortably, raise it — flagged in the critique, not changed

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-19 · critique → mockup → implemented and QA'd
- skills: design-critique, code-review high --fix
- Mockup (current vs. proposed desktop vs. proposed phone) approved in-session; caption kept,
  `max-w-3xl` chosen over `2xl` for parity with `FilmBulkAttachDialog`.
- Verified live on the films testbed (`/media/3`): no chevron, no `change`, caption present,
  `=` rows (Title/Studio/Poster) carry their choosers, promotion flips glyph + footer and back.
- handoff: everything shipped and verified; Draft PR open for the owner's skin look.
