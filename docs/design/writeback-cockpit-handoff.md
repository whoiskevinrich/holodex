# Design Handoff: Writeback dialog as cockpit — applied vs. on file, chooser on differing rows

**Spec**: [Per-field source-of-truth (F36)](../specs/field-source-of-truth.md) §Writeback ·
**ADRs**: [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md) ·
[ADR-091](../architecture/ADR-091-fire-and-forget-writeback-status.md) ·
[ADR-093](../architecture/ADR-093-writeback-readback-and-tristate-in-sync.md) ·
[ADR-090](../architecture/ADR-090-two-layer-entity-metadata-management.md) (precedence layer only)
**Builds on**: [writeback-selection-handoff.md](writeback-selection-handoff.md) (HOLODEX-213 —
the decided/undecided split and the three gutter tiers are ground truth here) ·
[writeback-poster-and-decision-legibility-handoff.md](writeback-poster-and-decision-legibility-handoff.md)
(HOLODEX-245 — the poster comparison row is untouched) ·
[two-tier-field-editing-handoff.md](two-tier-field-editing-handoff.md) (the chip row this
dialog now reuses).
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[theming.md](theming.md) — **tokens only, QA all three skins.**
**Surface**: `web/src/lib/components/writeback/WritebackFormDialog.svelte` (the only file whose
behaviour changes) · reuses `curation/CurationChip.svelte` (radio mode), `f36.ts`
(`sourceChips`, `resolveSelection`), and the stacked-row idiom of `curation/SourceEditModal.svelte`.
**Issue**: [HOLODEX-400](https://whoiskevinrich.atlassian.net/browse/HOLODEX-400) (parent epic
HOLODEX-167). Frontend-only; no endpoint, no ADR.

![Left: today's dialog — every differing row is a seeded free-text input with a "was:" line, checkboxes and a Select all. Right: the cockpit — a differing row renders the field's candidate chooser (chip row for short fields, stacked radio rows for long text); no checkboxes and no Select all: a decided row shows a will-write arrow, an undecided row a hollow circle until a chip is picked, a row that matches the file collapses to the "=" tier with a quiet "change" toggle, an unverifiable row says so, an unwritable row is unchanged](writeback-cockpit-mockup.svg)

---

## Overview

The writeback dialog is the moment of highest intent — the owner is about to burn a value into
the file — and today it is the surface with the *least* information. A differing row shows the
winning value in a text input plus one muted `was: <file value>` line
(`WritebackFormDialog.svelte` `fieldRow` snippet). The other candidates are invisible; the only
way to change what gets written is to type over the winner, which commits as `manual`.

The page already has the chooser: `SourceBadge`'s [chip row](../reference/ui-vocabulary.md#control-patterns-by-name)
(Tier-2) and the pencil + modal for `long_text`. This handoff moves that chooser **into the
dialog row** and lets the Write button be its Confirm.

### The model the owner asked for: applied vs. on file

> "The only time I care about a disagreement is when the file doesn't match the selected choice."
> "I don't care where the tags came from." — 2026-09-16 brainstorm

Two columns, not N sources: **applied** (what Holodex resolves — the winner, decided or not)
and **on file** (the file's own tag value, the `·file` candidate). Sources are only the
*candidates* the owner picks from when changing what's applied. A row is interesting exactly when
applied ≠ on file. Provenance stays as the existing `·tmdb` label suffix; it is never a reason to
open a row.

This is a **precedence** control in ADR-090's terms — every chip pins which stored namespace wins.
No adoption verdict is made here, and no competing provider value is ever put in an adoption row.

### The write is atomic over everything decided (owner, 2026-09-18)

> "The writeback should be atomic for all values." · "There is no reason to write back to the
> file with an undecided entry; deciding should be the check action."

So the dialog has **no per-row checkbox and no Select all** on cockpit rows:

- A **standing decision that lags the file is always written** — every one, together, in the
  one atomic job (HOLODEX-109 / ADR-091). There is no "skip this one": don't want it written?
  Re-point it (a decision) or Cancel.
- An **undecided row is never written on its own.** Picking a chip in the dialog — the already
  highlighted RD6 pending chip included — *is* the confirm: the row becomes decided-in-dialog
  and is written. Nobody touched it → not written, not decided.
- **Select all is gone.** With the checkbox now a decision, it had become "decide `provider:x`
  for 13 fields without reading them" — a bulk adoption verdict, which ADR-090 sends to the
  review queues, not here.
- **No checkbox anywhere.** `image_url` (poster) and merge rows have nothing to decide here —
  no chooser yet (HOLODEX-403 / 401) and, for merge fields, no decision model at all (RD1) — so
  they are listed read-only (`⊖`, "Nothing to decide here yet — not written from this dialog")
  and never written. Consequence, accepted by the owner (2026-09-18, "you only eliminated some
  of the checkboxes, not all of them"): **the poster cannot be written from this dialog until
  HOLODEX-403 gives it a chooser**, and merge fields until HOLODEX-401.

### What does *not* change

- The decided / undecided split and the disclosure line (HOLODEX-213). The lead group is now
  "writes on open" — `needsWriteback()` plus decided rows whose sync state is unknown (below) —
  so the header's "· {n} out of sync" is a lower bound on it, never more.
- The `=` and `⊖` gutter tiers and their glyphs (R4.3). The checkbox tier is replaced by two
  static glyphs on cockpit rows (§1); the `=` tier gains one affordance (below).
- `image_url` rows (HOLODEX-245's read-only comparison) — the poster chooser is
  [HOLODEX-403](https://whoiskevinrich.atlassian.net/browse/HOLODEX-403).
- Merge / multi rows — they stay listed, unchecked, without a decision (RD1); tag writeback is
  [HOLODEX-401](https://whoiskevinrich.atlassian.net/browse/HOLODEX-401).
- The footer, busy/enqueue-error states, fire-and-forget close (ADR-091), focus trap + return.
- The row already names the file tag (`Overview → Comment`), so the "Comments vs Overview"
  vocabulary gap from the brainstorm is a *page* issue, not a dialog one. No change here.

---

## Decided visual spec

### 1. Row classes

Replace-field rows fall into one of four classes. The class is derived from three things the
dialog already computes: `isWritable(field)`, `rowMatchesFile(row)` (live staged value vs. the
`·file` candidate), and `field.in_sync`.

| Class | Condition | Gutter | Body | Written? |
|---|---|---|---|---|
| **W · will write** | writable ∧ staged value ≠ on-file value ∧ (standing decision ∨ owner picked a chip here) | `↧` arrow-into-bar glyph, `text-accent` | header + **chooser (expanded)** | yes — always, atomically |
| **U · undecided** | writable ∧ staged value ≠ on-file value ∧ no standing decision ∧ untouched | `○` hollow circle, `text-muted` | header + **chooser (expanded)**, pending chip dashed | no — picking a chip turns it into **W** |
| **M · matches file** | writable ∧ staged value = on-file value | `=` glyph | header + value + "— matches the file" + quiet **change** toggle | no (a re-point to file is a decision-only save, §3) |
| **? · unverifiable** | writable ∧ `in_sync === undefined` | as **W** or **U** | as **W**/**U**, plus an amber note under the chooser | as **W**/**U** |
| **⊖ · unwritable** | `!isWritable(field)` | `⊖` glyph | unchanged (value + "no file tag for this container") | no |
| `image_url` / merge | non-cockpit — nothing to decide here | `⊖` glyph, title "Nothing to decide here — not written from this dialog" | HOLODEX-245 comparison (read-only, "Enriched") / value + `on file:` line, plus the note | no — HOLODEX-403 / 401 |

`?` is a **W**/**U** row with a note, not a sixth gutter glyph: it *can* be written; what is
missing is the read-back that would let the row report `=` later (ADR-093). A *decided* `?` row
is **W** and leads the dialog — and, because its file candidate is always empty, it will read
as "will write" on every open until the mapping gains a read-back source (the server WARNs
which key to add at startup). Copy: `Can't verify — {write_target} isn't read back from this
file.` in `text-warn`, `text-xs`.

**M → W promotion.** The `=` tier is the collapsed state, not a dead end (owner's call, 2026-09-18:
"expandable on demand"). A quiet `change` toggle (`.btn-quiet`, `text-xs`, `aria-expanded`,
`aria-controls` → the chooser) sits at the row's trailing edge. Expanding shows the same chooser
as **W**. The moment the staged pick ≠ on-file value the row **becomes W**: the `=` glyph is
replaced by the `↧` glyph (the owner just chose it, so it is decided-in-dialog) and the footer
count includes it. Picking the `·file` chip again returns the row to **M** (glyph back, count
drops; the pick is kept as a decision-only save if the field was decided elsewhere, §3).
`rowMatchesFile()` already reads the *live* value, so this is the existing tier logic applied to
the staged chip value instead of the text input.

### 2. The chooser

One shape per `display`, mirroring the page's own split so the owner meets the same control in
both places:

| `display` | Chooser | Source of the idiom |
|---|---|---|
| everything but `long_text` | **chip row** — `role="radiogroup"` of `CurationChip` in `radio` mode built from `sourceChips(field)`, trailing **Custom** chip (inline `<input>` opener) | `SourceBadge.svelte` expanded state, minus its Confirm/Cancel |
| `long_text` (Overview, tagline-length prose) | **stacked rows** — one full-width `role="radio"` row per chip: source label in a fixed left column (`text-[0.65rem]`, `text-muted`; `text-accent` when checked), value wrapped at full width; trailing **Custom** row is a `textarea` (`use:autoResize`, `max-height: 10rem`) | `SourceEditModal.svelte` |

Both:

- **Seed** the staged key from `resolveSelection(field, chips).key`; `selection.pending` (RD6
  implicit winner, no standing decision) renders the checked chip with the existing dashed-ring
  + hollow-dot treatment (`CurationChip radio.pending`).
- **Folded chips** are the `sourceChips()` model as-is: a provider whose value equals the file
  value folds into the `·file + tmdb` chip. A row whose every provider folds has two chips
  (`·file + …`, Custom) — and is **M** unless a Custom value is staged.
- **Empty baseline** (`·file` chip with `value === ''`) shows the existing `—` placeholder chip
  from `SourceBadge`; picking it stages the baseline (a write of nothing is filtered by
  `checkedRows` exactly as an empty edited input is today).
- **Custom**: typing then blur/Enter stages `custom` with the typed value; an empty draft leaves
  the previous staged pick untouched (`SourceBadge` rule). While a manual decision is standing,
  the Custom chip shows the frozen literal (`sourceChips` already does this).
- **No `on file:` / `was:` line.** The `·file` chip/row *is* the on-file value — a separate line
  would say it twice (owner's call, 2026-09-18). The text `<input>` / `<textarea>` seeded with
  the winner goes away for replace rows; Custom is the only free-text path.

The stacked-row renderer should be lifted out of `SourceEditModal.svelte` into a shared
`curation/` component (name it for the mechanism — e.g. `SourceRadioList.svelte`) rather than
duplicated; add it to `curation/CLAUDE.md` in the same change. If the modal's rows are not yet
separable, duplicating ≤ 40 lines is acceptable for this story, with the extraction filed.

### 3. Commit semantics — the Write button is Confirm

Nothing in the dialog hits the network until `submit()`. Chip clicks and arrow keys **stage**
locally (as `SourceBadge` does before Confirm); Cancel/Escape/backdrop discard every staged pick.

On submit, for each row that will write (`willWrite`: cockpit ∧ writable ∧ differs ∧ carries a
value ∧ (standing decision ∨ touched)):

1. **Decide** when needed — generalize `ensureDecision(row)` from "create if none standing" to:
   `decide(canonical, chip.decisionSource, customValue?)` **iff** no standing decision **or** the
   staged key ≠ `resolveSelection(field, chips).key`. An untouched, already-decided row still
   makes no call. `image_url` and merge rows stay excluded, as today.
2. **Write** the staged chip's value: `values = [chip.value]` (Custom → the typed value). The
   comma-split of a free-text input goes away with the input; a replace field is one value.
   `source` stays `winning_source` (the server re-resolves after the decision lands — ADR-091 —
   so the enqueued source is informational, as it is today).

The footer count is recomputed from the live staged picks — a row whose staged pick equals the
file is never promised.

Two rules added at implementation (2026-09-18, from `/code-review`):

- **A blank Custom pick is "nothing chosen yet."** The stacked rows stage `custom` the moment
  the textarea takes focus (the `SourceEditModal` idiom), so a row can sit on an empty literal.
  It is excluded from the count, the write and the decision — never `values: []` or
  `manual:''`. (The modal refuses the same state at Save.)
- **Re-pointing a row at the `·file` chip is still a decision.** That row matches the file, so
  there is nothing to write and no checkbox — but the pick pins the baseline, and dropping it
  would leave the old provider/manual decision standing with the page still reading "out of
  sync". Such rows ride along with a write; when nothing is written the button reads
  **`Save N decision(s)`**, stays enabled, and submit records the decisions without enqueuing
  an empty job. An unchecked, differing row is still left alone: the checkbox means "act on
  this one".

This closes [HOLODEX-219](https://whoiskevinrich.atlassian.net/browse/HOLODEX-219) as a side
effect: an undecided provider value can no longer be written without recording the decision,
because the checkbox + staged chip *is* the decision.

### 4. Layout and tokens

| Element | Treatment |
|---|---|
| Dialog | unchanged — `max-w-xl`, `max-h-[60vh]` body scroll, `rounded-theme border border-rule bg-surface shadow-lg` |
| Row | unchanged `flex items-start gap-3`; gutter `h-4 w-4` |
| Row header | unchanged: `label` `text-xs font-medium text-muted` · `·{source}` `text-[0.65rem]` (`text-accent` provider / `text-muted` baseline) · `→ {write_target}` `text-[0.65rem] text-muted` |
| Chip row | `mt-1 flex flex-wrap gap-1.5` — `CurationChip` radio styling as shipped (accent border + filled dot when checked; dashed ring + hollow dot when pending) |
| Stacked row | `mt-1 space-y-1`; each row `flex items-start gap-2 rounded-theme border border-rule px-2 py-1 text-sm text-ink`; checked → `border-accent bg-accent/10`; label column `w-12 shrink-0 text-[0.65rem] text-muted` |
| Custom textarea | the dialog's existing `textarea` classes (`bg-bg border-rule … focus:ring-accent`) |
| **M** row body | `text-xs text-muted` with the value in `text-ink`, then `— matches the file`; the `change` toggle is `.btn-quiet text-xs` and sits `ml-auto` on the header line |
| `?` note | `mt-1 text-xs text-warn` |
| Busy | every chip, radio row, textarea, toggle and checkbox `disabled` (chips already withdraw their affordance rather than dimming — never `disabled:opacity` on `text-muted`) |

No new tokens. No hardcoded values.

### 5. Copy

| Where | Text |
|---|---|
| **M** row | `{value} — matches the file` |
| **M** toggle | `change` (collapsed) / `close` (expanded) — sentence case, no punctuation |
| Footer, nothing to write but decisions staged | `Save {n} decision{s}` (busy: `Saving…`) |
| `?` note | `Can't verify — {write_target} isn't read back from this file.` |
| Empty `·file` chip | `—` (existing placeholder) |
| Custom opener | `Custom…` (chip) / `Write your own…` (textarea placeholder) |
| Everything else | unchanged |

### 6. Accessibility

- **No checkbox on any row.** The gutter glyphs are `role="img"` with a `<title>` each
  (`Will be written to the file` / `Undecided — pick a source to write it` / `Nothing to decide
  here — not written from this dialog` / the existing `=` and `⊖` titles); the row label is a
  plain `span`. The chooser radiogroup is a row's only control and carries its own
  `aria-label`; a non-cockpit row has no control at all.
- **Nested radiogroups inside a dialog.** Each chooser is its own `role="radiogroup"` with roving
  tabindex (one chip at `0`, the rest `-1`), `aria-labelledby` → the row's label element. Arrow
  keys move focus and stage within the group (`SourceBadge` handler, verbatim); they must not
  reach the dialog's `onKeydown` (it only handles Escape/Tab, so no conflict — but do not add
  arrow handling at dialog level).
- **`trapTab` must exclude `[tabindex="-1"]`.** Today's selector is
  `'input, textarea, button, [tabindex="0"]'`; `CurationChip` radio renders as a `<button>`, so
  every unchecked chip would become a Tab stop. Filter `el.tabIndex !== -1` (or select
  `button:not([tabindex="-1"])`). The same fix applies to the `onMount` first-focus selector.
- **Tab order per W/U row:** checked chip → (Custom input when open) → next row. An M row:
  `change` toggle only. First focus on open: the first control inside the lead group (the
  checked chip of a chip row, the checked radio of a stacked list).
- **Promotion / demotion never moves focus.** When a pick flips a row between M and W, focus
  stays on the chip that caused it; the `checkedCount` change is reflected in the footer button's
  visible label, which is sufficient — no `aria-live` region for the count. *(Implementation
  note, 2026-09-18: this requires two things — the chooser stays open once the owner has
  interacted with it, even when the pick lands back on the file value, and the chooser has a
  single mount point across the `=` and will-write branches. Either the row body switching
  `{#if}` branches or the chooser collapsing on demotion unmounts the radiogroup mid-arrow-key
  and drops focus to `<body>`; both were caught live in §9.2.)* After an Enter-commit of the
  Custom input, focus returns to the Custom chip (the input unmounts); a blur-commit never pulls
  focus back.
- **Pending chip**: `aria-checked="true"` plus the dashed ring; selection is never colour-only
  (dot + border + `aria-checked`).
- **Escape** while a Custom textarea has focus: first Escape closes the custom editor (discarding
  the draft), second closes the dialog — as `SourceBadge`'s custom editor behaves.

### 7. States

| Element | State | Behaviour |
|---|---|---|
| W row | standing decision, untouched | `↧`; submit writes, no decide |
| W row | staged ≠ current selection | `↧`; submit decides then writes |
| U row | RD6 pending winner, untouched | `○`, dashed ring; **not** written, **not** decided |
| U → W | any chip picked, the pending one included | `↧`; submit decides (`provider:<x>` / `file` / `manual`) then writes (HOLODEX-219) |
| W row | staged Custom, empty draft (chip row) | previous staged pick stays; nothing changes |
| M row | collapsed | `=` glyph, value line, `change` |
| M row | expanded, staged = file | chooser open, still `=`; toggle reads `close` — also the state a W row lands in when demoted, so the chooser never disappears under the owner. If the field was decided elsewhere, this is a decision-only row: counted in `Save N decisions`, decided on submit, not written |
| W row | staged Custom, empty literal (stacked rows) | not counted, not written, not decided — "nothing chosen yet" |
| M row | expanded, staged ≠ file | **promoted to W**: checkbox (checked), counted |
| ? row | any | as W + warn note; never pre-checked |
| ⊖ row | any | unchanged |
| Undecided disclosure | any | one line + chevron; **no Select all** |
| Dialog | busy | all controls disabled; Escape/backdrop ignored (unchanged) |
| Dialog | enqueue error | staged picks and check state preserved for retry (unchanged rule) |

### 8. Edge cases

- **Many differing rows.** A short-field chooser adds one wrapped chip line (~1.5rem); a
  `long_text` chooser adds ~3 rows plus a textarea (~7rem). Typical media has ≤ 2 `long_text`
  fields (overview, tagline), so the body scroll (`max-h-[60vh]`) absorbs it. No accordion —
  collapsing a W row would hide the very thing the dialog exists to show.
- **Long chip values** in a short field (e.g. a 60-char title): chips wrap via `flex-wrap`; the
  chip's own `truncate`/`max-w` rules from `CurationChip` apply. If a short field's candidate
  exceeds ~80 chars, treat it like `long_text` (stacked) — implement as `display === 'long_text'
  || value.length > 80` on any candidate; document the threshold next to the code.
- **Single candidate** (no providers, undecided): chips = `·file`, `Custom…`. The row is M unless
  Custom is staged. Nothing to choose, nothing to show beyond today's `=` line + `change`.
- **Provider value present, file value absent** (`·file` chip is `—`): W, RD6 pending when
  undecided. Picking `—` stages the baseline; the write filters the empty value as today.
- **Decision standing on a provider that no longer supplies a value**: `sourceChips` omits it;
  `resolveSelection` falls back to the baseline key and reports `pending: false`. The row reads
  as M or W by the file comparison alone. (Pre-existing behaviour; HOLODEX-339 covers the
  no-op-write half.)
- **Container changed since open** (unwritable at write time): unchanged — shows as a failed badge
  on the page, not in the dialog (ADR-091).

### 9. Verification checklist

Numbered `section.item`; tagged by verifier; grouped by tag.

#### Setup
- 9.0 `[smoke]` `cd web && npm run check && npm run test` green; `f36.test.ts` unchanged (no
  helper semantics change).

#### Agent
- 9.1 `[agent]` Open the dialog on a video with a decided, out-of-sync `year` and an undecided
  `overview` where tmdb ≠ file: `year` leads with the `↧` glyph and a chip row; `overview` is U
  (`○`) in the undecided disclosure, with **stacked** rows. No checkbox and no text input
  anywhere in the dialog; no Select all; Genres / Actors / Poster read "Nothing to decide here
  yet — not written from this dialog" behind a `⊖` glyph.
- 9.2 `[agent]` Pick the `·file` chip on `year`: row flips to M (`=` glyph, "matches the file",
  `change`); footer count drops by one. Pick `·tmdb` again: back to W (`↧`).
- 9.3 `[agent]` On an M row click `change`, pick a provider chip: `↧` appears, count rises. Click
  `close` with the pick staged: chooser hides, row stays W.
- 9.3a `[agent]` On a U row click the already highlighted pending chip: `○` → `↧`, count +1. Same
  on a stacked-rows U row by clicking its already-checked radio. Press Write: a `PUT …/decision`
  for exactly those rows precedes the writeback; an untouched U row is neither written nor
  decided.
- 9.4 `[agent]` Custom: type a value, blur → staged Custom; clear it and blur → previous pick
  restored. Submit → `PUT …/decision` with `manual` + the value, then `POST …/writeback` with
  `values: [<value>]` (single element, no comma split).
- 9.5 `[agent]` Open with only untouched U rows and no decided-lagging rows: footer disabled
  (`Write 0 fields to file`) — nothing is ever written by default.
- 9.6 `[agent]` Already-decided row, untouched, submit → **no** `decision` call.
- 9.7 `[agent]` Field with `in_sync === undefined` and a differing provider value: checkbox,
  not pre-checked, chooser open, `text-warn` note naming the `write_target`.
- 9.8 `[agent]` Keyboard: first focus lands on the lead row's checked chip; Tab from it lands on
  the next row's checked chip, **not** on an unchecked chip; Shift+Tab from the first focusable wraps to the footer's last button; arrow
  keys inside a radiogroup stage without leaving the group; Escape in a Custom textarea closes
  the editor first, the dialog second.
- 9.9 `[agent]` `rg 'zinc-|sky-|rounded-(lg|md|sm|xl)' web/src/lib/components/writeback` and
  `rg 'text-muted[^"]*disabled:opacity'` both empty.
- 9.10 `[agent]` Three skins via `javascript_tool` computed styles: checked chip border vs.
  `bg-surface` ≥ 3:1; `text-warn` note ≥ 4.5:1; stacked-row checked background does not swallow
  `text-ink` (see `reference-holodex-skin-qa-without-screenshots`).

#### Human
- 9.11 `[human]` Open a media page you own (owner mode), press **Write decisions to file**. In
  each skin (header picker: Cinémathèque, Broadcast, Brutalist) the differing rows should read as
  "here are the candidates, this one is picked" without reading any labels — the picked chip is
  the only one with a filled dot and a coloured border. Rows with the `↧` arrow are the ones the
  Write button will touch; a hollow `○` row should read as "nothing happens here unless I pick".
  The `=` rows should feel *quieter* than the W rows, and `change` should be findable but not
  shouting.
- 9.12 `[human]` With Overview differing: the three stacked rows should be fully readable at the
  dialog's width — no truncated prose. If you can't tell which text is on the file vs. from tmdb
  without hovering, the left label column is too faint on that skin.

---

## Not in scope (filed)

- Tags as a set with per-tag on-file markers — HOLODEX-401.
- Film link as a `film:` source (Album / Track) — HOLODEX-402.
- Poster chooser in the dialog — HOLODEX-403.
- The page-side "Overview vs. Comment" naming — no ticket; raise with HOLODEX-220 if it recurs.
