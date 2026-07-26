# Design Handoff: Writeback dialog selection + undecided grouping (F36 / F28)

**Spec**: [Per-field source-of-truth (F36)](../specs/field-source-of-truth.md) §Writeback ·
**ADRs**: [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md) ·
[ADR-041](../architecture/ADR-041-metadata-writeback.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[theming.md](theming.md) — **tokens only, QA all three skins.**
**Issue**: HOLODEX-213 · **Surface**: `WritebackFormDialog.svelte`, opened from `/media/[id]`.

---

## Overview

The dialog behind **Write decisions to file** had two problems that turned out to be one problem.

1. Its initial selection was seeded from `winning_source` — so every provider value winning by
   *mapping precedence* arrived pre-checked, even though the owner had decided nothing. The header
   said "· 3 out of sync"; the dialog opened primed to write more than 3. `poster_url`, which has
   no file candidate, was always in that surplus, arming an image download and a cover-art embed
   (a full remux on MKV) the owner never asked for.
2. Fixing (1) alone would leave a dialog whose default selection is empty and whose rows are
   almost all `opacity-50` — where a `text-muted` label measures **2.0–2.4:1** against
   `--surface` on every skin, below AA.

Spec §Writeback already states the write action collects "all of the item's decided + out-of-sync
fields." This handoff records the surface that makes that true and legible.

### Design-system fit (the `/design-system` check)

**No new tokens, no new component.** Everything is an existing idiom:

- **Disclosure row** — `.btn-quiet` (borderless neutral, the UI-only-toggle role in `app.css`)
  with the chevron path already used elsewhere, rotated 90° when open.
- **Select all** — `.btn-row .btn-accent .btn-pill`, the same compact affirmative action the owner
  queue rows use (`ExtractionQueueRow`, `EnrichQueueRow`, `DuplicatePairRow`).
- **Section rule** — `border-t border-rule` + `pt-3`, as elsewhere in the dialog chrome.
- **Row markup** — unchanged; extracted verbatim into a `{#snippet fieldRow(row)}` so both groups
  render the identical row.

Audit output: **one disclosure row, one Select all button, one empty-state line, and a snippet
extraction. No new primitive, no new token.**

---

## The selection rule (one predicate, both surfaces)

`needsWriteback(field)` in `web/src/lib/f36.ts` is the single predicate:

```
needsWriteback = isReplaceField(field) && field.in_sync === false
```

- The header's `outOfSyncCount()` **is** `fields.filter(needsWriteback).length`.
- The dialog seeds each row's `checked` from the same call, and splits its two groups on it.

They therefore cannot disagree: **the decided group is exactly the checked set on open, and its
size is exactly the number the header reported.** Per the resolver (`replaceMarkers`), `in_sync` is
`true` by construction unless a **standing** decision exists — so "undecided" and "not pre-checked"
are the same set by definition, not by coincidence.

Not narrowed: an undecided value — `poster_url` included — is still fully writable. Only its
default selection changed.

## Layout (option A of three mocked)

```
┌ Write metadata to file ─────────────────────────────┐
│ E:\…\Dune (1984) - {edition-Extended Edition}.mkv   │
│                                                     │
│ [x] Title    ·tmdb                                  │   ← decisions lead
│     was: Dune (1984) - {edition-Extended Edition}   │
│     [ Dune                                       ]  │
│ [x] Tagline  ·tmdb                                  │
│ ─────────────────────────────────────────────────── │
│ ▸ 12 provider values you haven't decided on  [Select all] │
│                                    Cancel  [Write 2 fields to file] │
└─────────────────────────────────────────────────────┘
```

| Option | What | Verdict |
|---|---|---|
| **A — collapsed group (chosen)** | Decisions listed; undecided values behind one disclosure + Select all | Default state reads as "your decisions"; nothing hidden, nothing dimmed, so the contrast floor holds without a special case. |
| B — always-open sections | Two labelled sections, both fully rendered | Everything visible, but the dialog runs long and unchecked rows still need a non-`opacity` treatment. |
| C — no grouping, per-row Decide | Flat list; each undecided row gains a `Decide` action | Smallest structural change, but duplicates the source-chip affordance already on the page behind the modal. |

### States

- **Some decisions** — decided rows render first, checked. Disclosure reads
  `{n} provider values you haven't decided on`, collapsed.
- **No decisions** (the common case on a freshly enriched item) — the decided group is replaced by
  `No decisions to write — nothing in this file lags a source you picked.` The disclosure still
  carries every row, so the dialog is never a dead end.
- **Expanded** — rows render at full contrast, unchecked. Checking one includes it in the write;
  it stays in the group (the grouping describes decision state, not selection state).
- **Select all** — expands the group and checks every row in it. The footer count is the one
  running total; collapsing the group does not clear the selection.

### Behaviour notes

- **No dimming.** An unchecked row is no longer `opacity-50`. The group heading carries the meaning
  the opacity was standing in for, and `opacity` on `text-muted` is a contrast bug per the theming
  rules. The checkbox is the state signal.
- **Disclosure a11y** — a real `<button>` with `aria-expanded` and `aria-controls="wb-undecided"`;
  the group is always in the DOM and toggled with `hidden`, so the reference always resolves.
- **Focus trap** — on open, focus goes to the first *visible* interactive element. The collapsed
  group's inputs still match the selector but are `display:none`, so they are filtered on
  `offsetParent` (as `trapTab` already does); with nothing to write, focus falls back to the
  dialog element itself, which carries `tabindex="-1"`. Focus must start inside the dialog or the
  trap does not hold.

---

## Measured contrast (all three skins, dialog surface)

| | Cinémathèque | Broadcast | Brutalist |
|---|---|---|---|
| Row label (`text-muted`), checked or not | 6.00 | 4.67 | 5.59 |
| Disclosure label (`.btn-quiet`) | 6.00 | 4.67 | 5.59 |
| Select all (`.btn-accent`) | 8.71 | 11.51 | 16.76 |

Elements below `opacity: 1` inside the dialog: **0**. Before this change the dimmed row label
measured 2.39 / 2.04 / 2.25.
