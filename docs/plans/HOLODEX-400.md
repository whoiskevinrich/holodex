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
- [ ] frontend — `WritebackFormDialog.svelte`: chooser per row class (W / M / ? / ⊖), staged picks,
  generalized `ensureDecision`, `trapTab` excludes `[tabindex="-1"]`; lift `SourceEditModal`'s
  stacked rows into `curation/SourceRadioList.svelte` (+ `curation/CLAUDE.md` row)
- [ ] testing `testing-strategy` — dialog row-class + submit-payload tests (decide-before-write,
  single-value payload, no decide on untouched decided row); `trapTab` exclusion
- [~] security `security-review` — n/a: no auth/access/infra change; same owner-gated endpoints

## Up next — ordered (position = priority)

1. [ ] [frontend] Implement per handoff §1–§3; keep HOLODEX-213's decided/undecided split intact
2. [ ] [frontend] `trapTab` / first-focus selectors exclude `tabindex="-1"` (handoff §6) — do this
   first, it is the one place the reuse breaks the dialog's a11y
3. [ ] [testing] Component tests for §9.4–9.6 payload shapes; three-skin QA §9.10 via javascript_tool
4. [ ] [—] Handoff §9.11–9.12 `[human]` — Kevin's look in all three skins, then `gh pr ready`
5. [ ] [—] Read HOLODEX-337 / 339 before QA — both make `in_sync` lie, and the cockpit keys on it

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-18 · design gate
- skills: product-brainstorming (2026-09-16), design-handoff
- handoff: branch renamed `HOLODEX-400-writeback-cockpit`, Jira In Progress; design handoff +
  SVG committed, Draft PR open; decisions locked — option 2 chooser (chip row / stacked for
  long_text), `=` rows expandable on demand (owner overrode the display-only recommendation),
  file chip is the on-file value (no `was:` line). Next session starts at Up next 2 then 1.
