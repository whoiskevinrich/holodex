# Design Handoff: Writeback hides the target file tag (HOLODEX-216)

**Epic**: HOLODEX-167 (Writeback)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[theming.md](theming.md) — **tokens only, QA all three skins.**
**Prior art (same dialog)**: [writeback-selection-handoff.md](writeback-selection-handoff.md) —
the "no dimming" rule and `.btn-*` disabled treatment this handoff follows.
**Surface**: `WritebackFormDialog.svelte`'s `fieldRow` snippet only. No new component, no new
route.

---

## Problem

A field could be checked and submitted in the writeback dialog even when the video's container
had no destination file tag for it (`writeback.TagForField`/`ImageTagForField` returns nothing
for that field+container pair). The write silently dropped it — the row still animated to "done."
Bug, not new scope: the fix is to make the existing unwritable case visible and unselectable,
never to redesign the dialog.

## Design-system fit (the `/design-system` check)

**No new tokens, no new component — three additions inside the existing row, all composed from
idioms already in this file:**

- **Destination-tag chip** — same `text-[0.65rem] text-muted` treatment as the existing
  `·{provenance}` suffix beside the label, just a second inline `<span>` reading `→ {write_target}`
  (e.g. `→ QuickTime:Title`). Muted, not accent — it's informational, not another affordance to
  press.
- **Unwritable status icon** — reuses the row's existing icon-slot pattern (`isDone` → check,
  `isWriting` → spinner, `isError` → X), adding one more state, `!writable` → a muted circle-slash
  glyph with a `<title>` tooltip. Same `h-4 w-4` sizing, same slot, no new icon set.
- **Explanatory copy** — reuses the row's existing secondary-line slot (where "was: {value}" /
  "already in file, nothing to write" already render), swapped for "No file tag for this
  container — can't be written to file." at `text-xs text-muted` — the row's standard secondary-text
  treatment, not a new warning style (this isn't an error, it's a structural fact about the file).

Audit output: **zero new tokens, zero new components, zero new CSS.**

## The "no dimming" rule, applied

Per `.claude/rules/frontend-theming.md` and this dialog's own precedent
(`writeback-selection-handoff.md`): a `disabled:opacity-60` checkbox for the unwritable case would
land at ~2.4–2.9:1 contrast, unreadable in at least one skin. So the row doesn't render a disabled
checkbox at all — it withdraws the checkbox and replaces it with the status icon, and the label
switches from a `<label for=...>` (implying a control it's paired with) to a bare `<span>` at the
same `text-xs font-medium text-muted` — full contrast preserved, no opacity trick, no control that
looks clickable but silently isn't.

## Row states (unchanged rows omitted — only the new branch)

| Condition | Icon slot | Label | Secondary line |
|---|---|---|---|
| `isWritable(field)` (existing rows, all prior states unchanged) | checkbox / done / writing / error | `<label>` | `was:` / `already in file` / image compare, as before |
| `!isWritable(field)` (new) | muted circle-slash, `title="No file tag for this container — can't be written"` | `<span>` (not a label — no control to pair with) | current value (or `—`) + "No file tag for this container — can't be written to file." |

```
┌ Write metadata to file ─────────────────────────────────┐
│ /media/clip.mp4                                          │
│                                                            │
│ ☑ Title              ·tmdb  → QuickTime:Title             │
│   was: Old Title                                          │
│                                                            │
│ ⊘ Director           ·tmdb                                │
│   Chloé Zhao                                               │
│   No file tag for this container — can't be written to file. │
│                                                            │
│                                     Cancel   [Write 1 field to file] │
└────────────────────────────────────────────────────────────┘
```

(The `⊘` row is unchecked and uncheckable regardless of `busy`/decision state — `isWritable` gates
both the seed in `needsWriteback` and the checkbox branch, so it can never become checked through
Select-all-undecided either, since that helper only flips rows already present in the `undecided`
group, and an unwritable row's icon slot never renders an `<input>` to flip.)

## Non-goals (explicitly out of this change)

- **Async (durable-queue) path's per-field outcome.** The queued (202) path still can't report
  `written`/`skipped` after the job completes — no `job_runs` schema change here. The UI-side fix
  (never offering an unwritable field) closes the user-visible defect on both paths regardless;
  the async job-status gap is tracked as a separate follow-up, not built here.
- **Any change to which fields are mapped per container.** `writeback.TagForField`/
  `ImageTagForField` (`internal/writeback/tags.go`) are untouched — this change only surfaces what
  they already decide.

## QA checklist

1.1 [smoke] Open the writeback dialog on a video whose container has at least one field with no
tag mapping (e.g. an MP4 with a `director`-only-mapped-on-MKV field enriched from a provider) —
that row shows the circle-slash icon, a `<span>` label (not focusable as a form control), and the
"No file tag..." line; it is not among the checked/submitted rows.
1.2 [smoke] A writable, out-of-sync row still shows its destination-tag chip (`→ {tag}`) next to
the provenance tag and behaves exactly as before (checkbox, was:/matches-file lines, submit).
1.3 [agent] Submit a batch mixing a writable and an unwritable field (only the writable one is
checkable, so this exercises the dialog's own gating rather than a crafted request) — the
unwritable row never enters `writing`/`done`.
2.1 [human] Switch to Cinémathèque, Broadcast, and Brutalist in turn; confirm the circle-slash icon
and the destination-tag chip read at full contrast against `bg-surface` in all three (muted-token
color, no hardcoded value) and that the unwritable row's label doesn't look like a disabled
checkbox — it should read as informational, not "broken."
