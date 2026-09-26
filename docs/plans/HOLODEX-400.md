---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-400
status: in-review
profile: ui
release_note: The "Write metadata to file" dialog now shows every candidate value for a field that differs from the file and lets you pick which one to write — the chooser from the media page, inside the dialog, with the Write button as its confirm.
---

# HOLODEX-400 · Writeback dialog as cockpit — applied vs. on file

The dialog is the moment of highest intent and had the least information: a differing row was a
seeded text input plus `was: <file>`. The owner's model is **applied vs. on file** — sources are
only the candidates. So a differing row now renders the field's chooser (chip row; stacked rows for
`long_text`), the `=` tier gains a quiet `change` toggle, an `in_sync === undefined` row says it
can't be verified, and the Write button commits the staged pick as the decision. Frontend-only;
scalar replace fields only. Parent epic HOLODEX-167. Siblings deferred: 401 (tags as a set),
402 (film → Album/Track), 403 (poster radio).

## Gates — definition of done

- [x] design `design-handoff` — `docs/design/writeback-cockpit-handoff.md` + `writeback-cockpit-mockup.svg`
  (2026-09-18); vocabulary term **applied vs. on file** + two translations added to
  `docs/reference/ui-vocabulary.md`
- [x] frontend — `WritebackFormDialog.svelte` cockpit rows (F / S / U / M / D / ⊖), **no checkbox anywhere**, no Select all (`willWrite` / `savesDecisionOnly`; image_url/merge read-only), two destinations in the gutter (`↧` file / cylinder Holodex), unmapped rows decidable, `not read back` file chip, staged picks, Write =
  Confirm via `needsDecision`, `focusables()` excludes `tabIndex === -1`; lifted
  `curation/SourceChipRow.svelte` (from `SourceBadge`, which still keeps its own copy) and
  `curation/SourceRadioList.svelte` (from `SourceEditModal`, now rewired to it); pure helpers in
  `web/src/lib/writebackCockpit.ts`; both component `CLAUDE.md` tables updated (2026-09-18)
- [x] testing `testing-strategy` — `writebackCockpit.test.ts` (25 cases) + live `[agent]` pass on
  `backend-films` (Dune, TMDB): 2 decision PUTs + 1 writeback POST for 3 checked rows, none for the
  untouched decided row; Tab lands on the checked chip; three-skin contrast all ≥ 4.5:1 (row in
  `docs/testing-strategy.md`)

## Up next — ordered (position = priority)

1. [ ] [—] On merge: sweep HOLODEX-400 to Done is CI's job (branch carries the key); nothing by hand
2. [ ] [—] Fold `SourceBadge`'s expanded row onto `curation/SourceChipRow` (it still carries its own
   copy of the radiogroup + Custom-draft logic) → file as a HOLODEX task when picked up
3. [ ] [—] HOLODEX-403 (poster chooser) is now the only path to poster writeback from the dialog —
   consider pulling it forward
4. [ ] [—] HOLODEX-337 / 339 make `in_sync` lie in places; the cockpit keys on it

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-19 (b) · merged main, marked ready
- skills: —
- handoff: owner asked to mark ready (§9.11–9.12 human look = owner's call). Merged `origin/main`
  (#355 touched the same modal body: `SourceValueClamp` + `rows=5` ported into `SourceRadioList`,
  so the page modal and the dialog's stacked rows share it); curation `CLAUDE.md` + testing
  strategy unioned. `check` clean, 362 tests. `gh pr ready` → CI fires In Review.

### 2026-09-19 · golden record, two destinations
- skills: —
- handoff: Kevin — "golden record" = system source of truth, mapped subset reaches the file; "if
  it isn't written to file the icon shouldn't be a down arrow". Cylinder gutter glyph = saved in
  Holodex only; unmapped fields get the chooser; header says `→ tag` / `no file tag for this
  container`; red "Can't verify" line replaced by `not read back` on the file chip. Pure
  `savesDecisionOnly` (+5 tests, 319 total); live-verified (one runtime decision PUT, writeback
  with the 4 mapped fields). Vocab: **golden record** + two translations. Remaining: Kevin's skin
  look (§9.11–9.12) → `gh pr ready`.

### 2026-09-18 (c) · atomic write, no checkboxes
- skills: code-review high --fix (prior)
- handoff: Kevin — "atomic for all values; deciding should be the check action". Removed Select all
  and every cockpit-row checkbox: `willWrite` gate (standing ∨ touched), `↧`/`○` gutter glyphs,
  lead group = `leadRow` (standing ∧ lags file, incl. unverifiable). Then "you only eliminated
  some of the checkboxes" → image_url/merge rows are read-only too (`⊖`, "Nothing to decide
  here yet"); poster unwritable from the dialog until 403 — flagged on the ticket. Live-verified
  (0 checkboxes, 0 text inputs); mockup/handoff/vocab/testing row updated.
  Remaining: Kevin's skin look (§9.11–9.12) → `gh pr ready`.

### 2026-09-18 (b) · frontend + tests
- skills: design-handoff (prior), code-review high --fix, code-review
- handoff: cockpit implemented and live-verified end to end (decisions + writeback enqueued on the
  Dune fixture); two focus bugs found live and fixed (chooser stays open; single mount point);
  code-review found two more (blank Custom counted as a write; file-chip pick on a decided row
  dropped) — fixed, `Save N decisions` path added and verified. Remaining: Kevin's three-skin
  look (§9.11–9.12), then mark ready.

### 2026-09-18 · design gate
- skills: product-brainstorming (2026-09-16), design-handoff
- handoff: branch renamed `HOLODEX-400-writeback-cockpit`, Jira In Progress; design handoff +
  SVG committed, Draft PR open; decisions locked — option 2 chooser (chip row / stacked for
  long_text), `=` rows expandable on demand (owner overrode the display-only recommendation),
  file chip is the on-file value (no `was:` line). Next session starts at Up next 2 then 1.
