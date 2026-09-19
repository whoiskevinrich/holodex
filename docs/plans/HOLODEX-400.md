---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-400
status: in-progress
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

- [~] spec `write-spec` — n/a: no new capability; F36 spec §Writeback already covers decisions
  written from the dialog. Revisit if the M→W promotion needs a spec line.
- [~] architecture `architecture` — n/a: no new seam; reuses `PUT …/decision` + ADR-091 enqueue
- [x] design `design-handoff` — `docs/design/writeback-cockpit-handoff.md` + `writeback-cockpit-mockup.svg`
  (2026-09-18); vocabulary term **applied vs. on file** + two translations added to
  `docs/reference/ui-vocabulary.md`
- [~] backend — n/a
- [x] frontend — `WritebackFormDialog.svelte` cockpit rows (W / M / ? / ⊖), staged picks, Write =
  Confirm via `needsDecision`, `focusables()` excludes `tabIndex === -1`; lifted
  `curation/SourceChipRow.svelte` (from `SourceBadge`, which still keeps its own copy) and
  `curation/SourceRadioList.svelte` (from `SourceEditModal`, now rewired to it); pure helpers in
  `web/src/lib/writebackCockpit.ts`; both component `CLAUDE.md` tables updated (2026-09-18)
- [x] testing `testing-strategy` — `writebackCockpit.test.ts` (14 cases) + live `[agent]` pass on
  `backend-films` (Dune, TMDB): 2 decision PUTs + 1 writeback POST for 3 checked rows, none for the
  untouched decided row; Tab lands on the checked chip; three-skin contrast all ≥ 4.5:1 (row in
  `docs/testing-strategy.md`)
- [~] security `security-review` — n/a: no auth/access/infra change; same owner-gated endpoints

## Up next — ordered (position = priority)

1. [ ] [—] Handoff §9.11–9.12 `[human]` — Kevin's look in all three skins, then `gh pr ready`
2. [ ] [—] Fold `SourceBadge`'s expanded row onto `curation/SourceChipRow` (it still carries its own
   copy of the radiogroup + Custom-draft logic) → file as a HOLODEX task when picked up
3. [ ] [—] HOLODEX-337 / 339 make `in_sync` lie in places; the cockpit keys on it — re-read both
   before the human QA so a wrong `=` row is recognised as theirs, not this PR's

## Session log — append-only (cap: last 8 sessions; older → archive/)

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
