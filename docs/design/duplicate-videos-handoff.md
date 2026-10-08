# Handoff: Duplicate videos — Videos group, fact-table compare, keep one / keep both / label (F76)

**Spec**: [duplicate-videos.md](../specs/duplicate-videos.md) (F76) ·
**Jira**: [HOLODEX-521](https://whoiskevinrich.atlassian.net/browse/HOLODEX-521) ·
**Date**: 2026-10-08 · **Skin**: Cinémathèque (the only look)

## Decision

Video pairs get their own **Videos** group on the owner Duplicates page. A row is collapsed by
default, and only one panel is open at a time, the same as the person queue (F70). The expanded
panel is **one fact table with a column per file** (option B), not one card per file. Every
difference sits on a single row, a missing value leaves a blank cell instead of shifting the
layout, and the comparison stays side by side at 375 px. The owner chose both on 2026-10-08,
against option A (F70's per-side cards) and an auto-opening queue.

![Seven panels in the Cinémathèque skin. 1: the Videos group collapsed, three pair rows each showing the shared title once, a resolution-bucket and duration summary per side, "· same provider match" and a bordered Keep both button. 2: an expanded pair as one fact table — two 16:9 posters with resolution and duration badges, then rows File, Folder, Resolution, Duration, File size, Video codec, Bitrate, Container and Your work, a Keep this one pill and an Open link under each column, and a footer with Label as editions…, Label as parts… and Keep both. 3: the "Move the other copy to Trash?" confirm naming the 4K file and listing 2 playlist places, a film link and 3 field edits that move to the kept copy, with Cancel and a warn-filled Move to Trash. 4: the Edition row turned into two pill inputs with a "Give at least one file an edition." error and Save editions. 5: two numeric Part inputs both set to 1 with "The two files need different part numbers." and Save parts. 6: the table at 375 px, still side by side. 7: the panel's Loading… and "Couldn't load these files. Retry" states.](duplicate-videos-mockup.svg)

### Design calls made here (not in the spec)

1. **The row shows the title once.** Both files resolve the same provider title, so writing it twice
   is noise. Each side is summarized as `{resolutionBucket} · {formatDuration}`, joined by `↔`.
2. **Keep both is the only verdict in the row**, as `btn-row btn-ghost px-2` (an immediate resolve).
   Keeping one copy needs the facts, so **Keep this one** lives only in the panel, one per column.
   This differs from F70, where Keep separate was the dominant answer and took the accent. Here 19
   of 21 pairs are copies, so the dominant answer is *keep one*, and that answer carries the accent.
3. **Keep this one is an accent pill** (`btn-row btn-pill btn-accent`) because it is the
   affirmative action. It is the step before the warn-filled Move to Trash in the confirm, which is
   where the destructive styling belongs.
4. **Fact rows are per row, not per side.** A row renders when either side has a value. A side
   without one leaves an empty cell, never `—` or "unknown" (the F68 absent-is-absent rule applied
   per cell). Edition and Part rows appear only when either side has one set.
5. **The confirm lists only what will actually move.** An edit the kept copy already has stays its
   own, so it isn't counted. With nothing to move, the list is replaced by one sentence.
6. **Open ↗ opens the media page in a new tab**, so the queue keeps its place and its open panel
   (spec P0-3).
7. **The Videos group gets its own one-line intro** under its heading, because the page intro
   describes name pairs and doesn't fit files.

## Layout

**The group.** It reuses the page's group markup: `section.rounded-theme.border.border-rule.bg-surface`,
with the heading `h2.px-3.pb-2.pt-3.text-xs.uppercase.tracking-wide.text-muted` reading
`Videos · {n}`. The new intro `p.px-3.pb-2.text-xs.text-muted` reads "Files matched to the same
provider item. Keep one, keep both, or label them as editions or parts." Order: tag, studio,
person, film, **video**. `?type=video` filters to it.

**The row** keeps the existing row shell: `border-t border-rule px-3 py-2.5 text-sm`, one 44 px line
on desktop. In order:

1. The disclosure, exactly as F70: `btn-quiet h-7 w-7`, a chevron that rotates when open,
   `aria-expanded`, `aria-controls`.
2. The title, `text-ink truncate`. It is the only element that shrinks.
3. The facts, `text-xs text-muted shrink-0`, e.g. `4K · 34:14 ↔ FHD · 34:12`.
4. The reason, `text-xs text-muted shrink-0`, reading `· same provider match`.
5. The verdicts, pushed right: `Keep both`.

**The panel** is the F70 well, `border-t border-rule bg-surface-2 px-3 py-3`, holding a single
`<table>` with `table-layout: fixed`:

| Column | Width | Content |
|---|---|---|
| Label | `w-26` (104 px) desktop · 76 px under `sm` | `text-xs text-muted` |
| File A | half the rest | values `text-xs text-ink`; File name `text-sm` |
| File B | half the rest | same |

- **Rows**, in order:
  - Posters (header row)
  - File: the file name, with `title` = the full path
  - Folder: the parent folder, `text-muted`
  - Resolution: `{bucket} · {w}×{h}`
  - Duration
  - File size
  - Video codec
  - Bitrate
  - Container
  - Edition and Part, only when either side is set
  - Your work: `2 playlists · film link · 3 edits`, absent when the side carries none
  - The action row: `Keep this one` plus an `Open ↗` link
- **Row and cell styling.** Each row has a `border-t border-rule` divider. Cells are padded
  `px-2 py-1.5`, and every value cell truncates with `title` holding the full value.
- **Posters.** These are the `.video-frame` 16:9 thumbnail (`api.thumbnailURL(id)`), with the
  VideoCard resolution badge (top-left) and duration badge (bottom-right) reused as-is.
- **Footer.** `mt-3 flex flex-wrap justify-end gap-2 border-t border-rule pt-3` holds three items:
  - `Label as editions…` (`btn-row btn-quiet`)
  - `Label as parts…` (`btn-row btn-quiet`)
  - `Keep both` (the row's own button, repeated)

## Design tokens used

| Token / class | Where | Note |
|---|---|---|
| `bg-surface`, `border-rule`, `rounded-theme` | group, rows, dialog | existing group and row shell |
| `bg-surface-2` | panel well | F70 well |
| `text-ink` / `text-muted` | values / labels, facts, reason | |
| `text-accent` | Open ↗ | link, `hover:text-ink` |
| `text-warn` | label-editor errors, panel load error | |
| `btn-row btn-ghost px-2` | Keep both | immediate resolve |
| `btn-row btn-pill btn-accent` | Keep this one, Save editions, Save parts | affirmative |
| `btn-row btn-quiet` | Label as editions…, Label as parts…, Cancel, Retry | UI-only toggles, no side effect |
| `bg-warn text-warn-ink` | Move to Trash in the confirm | the existing ConfirmDialog destructive action |
| `bg-accent text-accent-ink` | resolution badge on the poster | VideoCard's badge |
| `font-display` (`skin-title`) | confirm title | ConfirmDialog default |

There are no new tokens and no hex values.

## Components

| Component | Status | Notes |
|---|---|---|
| `routes/owner/duplicates/+page.svelte` | edit | Add `video` to the type list, label and order; add the group intro; extend `?type=` |
| `duplicates/VideoPairRow.svelte` | **new** | The row in Layout. **Owns every verdict and the `busy` state**: keep-one(side), keep-both, label-editions, label-parts. It passes them to the panel as snippets and callbacks. This is the F70 rule that the panel never owns a verdict; extend `verdictOwnership.test.ts` to cover it |
| `duplicates/VideoComparePanel.svelte` | **new** | The fact table and the editing states. It renders the verdicts it is given and decides none of them |
| `duplicates/queue.ts` | edit | Video pair ids and labels go through it, never spelled by hand |
| `shared/ConfirmDialog.svelte` | reuse | Title, body and carry-over list go in its body; destructive variant |
| `format.ts` | reuse | `formatDuration`, `resolutionBucket`, `formatBytes`, `formatBitrate` |
| `duplicates/CLAUDE.md` | edit | Add rows for the two new components and the per-cell absent rule |

## States and interactions

| Element | State | Behaviour |
|---|---|---|
| Row | collapsed (default) | Opening another row's panel collapses the open one (F70 RD12) |
| Panel | loading | The well shows `Loading…` (`text-sm text-muted`, centred). Keep both stays usable in the row |
| Panel | load failed | `Couldn't load these files.` (`text-xs text-warn`) plus `Retry` (`btn-quiet`). The failure is not cached |
| Keep this one | press | Opens ConfirmDialog (panel 3). Two-step: no single click trashes a file |
| Confirm | carries work | Body: "**{file name}** ({bucket}) will be hidden from your library and permanently deleted in {n} days. You can restore it from Trash until then." Then "Moving to the copy you keep:" and one line each for playlist places, the film link (film name, plus scene when numbered), field edits and tags you added. Each line is omitted when zero |
| Confirm | carries nothing | The list is replaced by "It carries no playlists, film link or edits of its own." |
| Confirm | no grace period | "…will be hidden from your library. You can restore it from Trash." (ConfirmDialog's existing wording) |
| Confirm | busy / error | `Move to Trash` reads `Working…`. An error appears under the buttons, the dialog stays open and nothing changes |
| Confirm | success | The dialog closes and the pair row is removed. Focus moves per *Keyboard and focus* |
| Label as editions… | press | The Edition row appears (if hidden), with an input per column (the media page's edition pill input: `w-40 rounded-full border border-accent bg-bg px-2 py-0.5 text-xs`, placeholder `Edition`), prefilled. The footer becomes `[error] Cancel · Save editions` |
| Label as editions… | invalid | Both inputs empty: `Give at least one file an edition.` |
| Label as editions… | a prefilled side emptied | Saving removes that file's edition (spec RD9). No extra confirm: the empty input is the visible intent. A side that had no edition and stays empty is left as it was |
| Label as parts… | press | Same pattern on the Part row: `w-28`, `inputmode="numeric"`, placeholder `Part number` |
| Label as parts… | invalid | Either input empty: `Give both files a part number.` Equal numbers: `The two files need different part numbers.` |
| Label save | success | Sets the values and resolves the pair as keep both, so the row is removed |
| Label as editions… | edition not settable | Not rendered |
| Open ↗ | press | The media page opens in a new tab (`target="_blank" rel="noopener"`) |

## Keyboard and focus

- The disclosure works with Enter or Space. Escape collapses the open panel and returns focus to its
  disclosure, as F70 does. Escape inside a label editor cancels the editor first; a second Escape
  collapses the panel.
- **Tab order inside the panel:**
  1. Open ↗ (A)
  2. Keep this one (A)
  3. Open ↗ (B)
  4. Keep this one (B)
  5. Label as editions…
  6. Label as parts…
  7. Keep both
- **In a label editor:**
  - Focus starts in the first input, and Enter in either input saves.
  - After Cancel, focus returns to the button that opened the editor.
- **After a pair resolves** (trash, keep both or label), focus moves to the next row's disclosure,
  or to the Videos heading if it was the last row. This is the existing `focusLandingIds` ladder.
- The confirm traps focus as ConfirmDialog already does, and starts on Cancel.

## Responsive behaviour

| Breakpoint | Changes |
|---|---|
| ≥ `sm` | As in panels 1–5 |
| < `sm` (375 px, panel 6) | The label column narrows to 76 px and the cell padding to `px-1 py-1`. The two file columns stay side by side, and the posters shrink with their column. The row wraps the way the person row does: the facts and reason go under the title, and Keep both drops to its own line below them |

## Edge cases

- **Long file names and folders** truncate in their cell. `title` holds the full path.
- **Same file name in two folders.** The Folder row is what tells them apart, which is why it is
  always shown when set.
- **A side with no poster** shows the `.video-frame` placeholder. The table doesn't collapse.
- **Three or more files on one item.** Each two-file combination is its own row (spec P0-1). Trashing
  one copy removes every row it appears in.
- **A pair left the queue while the panel was open** (another tab trashed a file). The next action
  fails with the row error, and the row is removed on the next list refresh.
- **A label editor is open and Keep both is pressed.** The editor is discarded and the pair resolves.

## Accessibility

- **The row** has `role="group"`, aria-label "Possible duplicate files: {title}".
- **The table** has a `<caption class="sr-only">` reading "Compare the two files of {title}", plus
  `<th scope="row">` on labels and `<th scope="col">` on the poster cells. Each poster cell's `alt`
  is "{bucket} copy".
- **Keep this one** gets aria-label "Keep the {bucket} copy, {file name}". Two identical visible
  labels need distinct names.
- **Open ↗** gets aria-label "Open {file name} in a new tab". The ↗ is `aria-hidden`.
- **Errors** in the label editors and the panel use `role="alert"`.
- **The Videos group heading** is the focus fallback and keeps `tabindex="-1"`.

## Cinémathèque QA

Tokens only: no hex values, palette classes, named fonts or `rounded-lg`. Check computed styles
for each of the following:

- the row
- the well
- table borders (`--rule`)
- the accent pills
- the warn-filled confirm action
- the poster badges

Check them on the expanded panel at 1280 px and 375 px. Never QA or mock up another skin.

## Build checklist

- [ ] Videos group, intro and `?type=video` on the Duplicates page
- [ ] `VideoPairRow` owns every verdict and `busy`; ownership test extended
- [ ] `VideoComparePanel` fact table: per-row presence, per-cell absent, File/Folder `title`s
- [ ] Keep this one → ConfirmDialog with the carry-over list; carries-nothing and no-grace variants
- [ ] Label as editions / parts editors with their validation copy; editions hidden when not settable
- [ ] Focus ladder after every resolution, plus Escape order (editor, then panel)
- [ ] 375 px layout as in panel 6
- [ ] `duplicates/CLAUDE.md` rows for both components
- [ ] Cinémathèque computed-style QA

**P1 is not in this handoff:** frame strips, the Media-list banner, and difference emphasis
(spec P1-1 to P1-3).
