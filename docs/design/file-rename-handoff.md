# Handoff: File names — template editor, rename plan, out-of-date queue, name lock and undo (F86)

**Spec**: [file-rename.md](../specs/file-rename.md) (F86) ·
**Architecture**: [file-rename.md](../architecture/file-rename.md) ·
**Jira**: [HOLODEX-602](https://whoiskevinrich.atlassian.net/browse/HOLODEX-602) ·
**Date**: 2026-10-31 · **Skin**: Cinémathèque (the only look)

## Decision

Renaming gets one new owner tab, **File names**, which goes between Extraction and Completeness. It
holds two things, one above the other:

1. **The Renaming card**: the mode, the settle window and the ordered template editor.
2. **The plan**: one list of every video's current name and new name, with a reason filter row.

The spec's drift review queue appears in the UI as **Out of date**. This doc says "out of date"
too, because *drift* already means two diverging renderings in
[ui-vocabulary.md](../reference/ui-vocabulary.md). The **out-of-date queue is the plan's `Out of
date` filter**, not a surface of its own. The tab opens on
that filter whenever it isn't empty. On the video page, a **File name** row in the Maintenance
section carries the single-video rename, the name lock and undo. Visitors see none of this, because
both the File names tab and Maintenance are owner-only.

The owner chose, on 2026-10-31:

- **Out of date is a filter on the plan (A), not its own Owner tab (B).** This was decided after
  both were mocked up side by side. A keeps one row and one Rename bar for every reason, so an
  out-of-date name sits next to the reasons it might not rename.
- **The saved template card folds to one line**: `Manual · 3 templates · first: …`, with
  **Edit templates ▾**. It is always open while a draft is unsaved or a template has an error. The
  alternative was an editor that is always open.
- **Templates preview with a Preview button.** Preview recomputes the whole plan below from the
  unsaved templates. The alternative was a live sample of five videos as you type. The button lets
  the owner tune against the whole library: per-template counts, every refusal reason, and the
  spec's read-back success metric.
- **Manual rename and undo go in a Maintenance row**, next to Flip, Re-encode and the held-original
  banner. The alternative was the read-only File block.
- **Rename always asks once, inline in the bar.** The bar shows `Rename N files? You can undo each
  one from its video page. [Yes, rename N] [Cancel]` for any selection, not only for Select all. It
  is the confirm pattern System activity already uses.

![File names tab, panel 1: the Owner header and tab row with File names active; the Renaming card with a five-way mode control (Off, Plan, Manual, On writeback, Job) on Manual and its one-line description; the Templates list of four ordered rows, each a monospace input with per-template counts ("names 2,890 · 15 won't read back"), move up, move down and remove buttons, the first row's up and the last row's down withdrawn; row 4 "{studio} {year}" in a warn border with "Add {title} or an identity token. Without one, unrelated videos could get the same name."; + Add template; token insert chips {title} {studio} {people} {year} {edition} {part} {provider-1-id} {provider-2-id}; a footer reading "Unsaved changes. The plan shows these templates after Preview." with "Fix template 4 to save.", Discard, Preview and a withdrawn Save templates. Panel 2: Job selected with a Settle window input of 7 days. Panel 3: Off selected, with no plan.](file-rename-settings-mockup.svg)

![Rename plan, panel 1: the PLAN heading and a wrapping filter row with counts (All, Will rename, Out of date, Already named, No template, Won't read back, Clash, Bad name, Locked, Held); a select-all row "3 selected of the 1,241 you can rename" with Select all 1,241; nine rows: a junk name struck through to "[Acme] Ada Lovelace (2023).mkv"; an out-of-date name with "Since Holodex named it, title changed: Nothern Lights → Northern Lights"; a clash settled as "Ada Lovelace (2023) {hx-412}.mkv" because video 398 holds the plain name; then rows without checkboxes for won't read back, clash with an untracked file, too long, locked (with Unlock), held and no template, each with its reason; a sticky bar with 3 selected, Clear and Rename 3. Panel 2: the inline confirm. Panel 3: rows after Rename with renamed, renaming and failed (Retry, Dismiss) and a link to System activity. Panel 4: the Plan-mode bar and the draft-preview bar with Save templates. Panel 5: the list at 375 px with filters clipped to scroll sideways and names stacked. Panel 6: the landing view, with the card folded to one line and the Out of date filter active, two out-of-date rows naming the changed field, and Rename 37.](file-rename-plan-mockup.svg)

![Video page File name row in Maintenance, ten states: up to date with "Matches template 1." and Lock name; out of date with "Would be [Acme] Northern Lights (2019).mkv", the changed field, Rename and Lock name; the accent renaming pill with "Started 14:02. You can leave this page."; renamed, with a surface box "Renamed Oct 30 from Ada Lovelace (2023).mkv", Undo rename and History (2); the history list of names it has carried, oldest to newest (arrived, then a job rename, then a manual rename), with the trigger of each; locked after a hand edit, with a padlock after the name and Unlock; blocked by a held original; refused because the name wouldn't read back; undo refused because the old name is taken; a failed rename with Retry and Dismiss.](file-rename-video-mockup.svg)

### Design calls made here (not in the spec)

1. **Every name is shown whole.** A name is a filename the owner must verify, so it is never
   truncated. The owner's rule is the same as for writeback candidates, HOLODEX-434. Old → new sits
   on one line when both fit. Otherwise the new name stacks under the old one, behind a leading `→`.
   At 375 px, names always stack.
2. **The old name is struck through in `--warn`.** It is `text-muted line-through decoration-warn`,
   then `→`, then the new name in `text-ink`. This is the old→new idiom of
   `ExtractionPreviewDialog`, reused rather than invented.
3. **An out-of-date row names the field that changed**: `Since Holodex named it, title changed:
   Nothern Lights → Northern Lights`. A name goes out of date because metadata changed, and this
   line says which change. It shows the value in the name Holodex wrote on the left and the value
   now on the right (the spec's P0-10 record). When several fields changed, the row names each
   one, comma-separated.
4. **A row that can't rename has no checkbox, and its 16 px slot stays empty.** No disabled
   checkbox is shown. The reason sits on the title line as a chip and is spelled out under the
   name. Refusals use a `--warn` border: won't read back, clash, bad name, too long. States that
   are fine use a `--muted` border: locked, held, no template.
5. **A clash settled by the id is not a refusal.** The row is selectable. Its explanation line names
   the video that keeps the plain name: `Video 398 already has … so this one carries its id.`
6. **Lock is a per-row padlock button at the row's end**, and on the video page it is **Lock name**
   or **Unlock** with the padlock. It reuses `CoreImageEditButton`'s 12 px stroke padlock.
   - A **labelled** button's glyph shows what the click does: **Lock name** has a closed padlock,
     and **Unlock** has an open one.
   - The plan row's **icon-only** toggle shows the current state instead (open means unlocked),
     with `aria-pressed`. On a locked row it is replaced by the labelled **Unlock**.
   - The locked state itself reads from the `locked` chip in the plan, and from a closed `text-ink`
     padlock after the name on the video page.
7. **The lock line says why it is locked**, in one of three phrasings:
   - "Locked when you renamed it outside Holodex on Oct 12." (an automatic lock)
   - "Locked by you."
   - "Locked when you undid a rename on Oct 30."
8. **Undo mirrors the held-original banner.** After a rename, while the file still has that name, a
   `bg-surface` box reads "Renamed {date} from `{old}`", followed by a quiet **Undo rename**. The
   undo is reachable only from the video page. The plan's confirm says so, which is how bulk
   renaming stays reversible without a run-level undo (a P2 item in the spec).
9. **Mode is a segmented control** (Off · Plan · Manual · On writeback · Job). It is styled like the
   owner tab row: the active segment is `bg-surface-2 text-ink`, never accent. One `text-xs` line
   below describes the chosen mode. The settle window input appears only when Job is chosen.
10. **Templates reorder with ↑ / ↓ buttons, not drag.** There is no drag-reorder list in the app.
    `PersonGallery`'s keyboard move buttons are the precedent. The first row's ↑ and the last row's
    ↓ are withdrawn (drawn in `--rule`, with `aria-disabled`).
11. **Per-template counts after Preview**: `names N` counts every video that template names, both
    renamed already and still to rename. It is followed by ` · N won't read back` in `--warn`
    when it isn't zero. This shows the owner the effect of ordering, which the spec's "first
    template that fits" rule makes non-obvious.
12. **Token insert chips** insert at the cursor of the template input that last had focus. They are
    not a picker, so typing a template by hand still works. One `{provider-N-id}` chip appears per
    configured provider.
13. **Filter labels.** `Clash` is the refusal for a name an untracked file holds. A clash with
    another video renames with the id, so it stays under Will rename or Out of date. `Bad name`
    groups the names refused as reserved, empty or too long (spec P0-4), and its row chip is the
    specific one (`too long`, `reserved name`). The other labels follow the spec's P1-1 list.
14. **The single-video Rename on the video page doesn't confirm.** It is one file, undone from the
    same row. The owner's "always confirm" choice is for the plan's bar, where a selection can be
    any size.
15. **Two input bounds the UI sets.** The settle window takes 1–365 days. The last template can't be
    removed while the mode isn't Off, so × is withdrawn there. Neither is a spec rule, so the server
    needn't share them. If the spec ever states a bound, the spec's number wins.

## Layout

### Owner → File names (`/owner/file-names`)

- **Tab.** It is added to `routes/owner/+layout.svelte`'s tab list after Extraction, labelled
  **File names**. It is shown whatever the mode is.
- **Intro.** `p.max-w-2xl text-sm text-muted`, two sentences, as on the extraction page.
- **Renaming card.** `section.rounded-theme border border-rule bg-surface`:
  - **RENAMING block.** The section label `text-xs uppercase tracking-wide text-muted`, then the
    segmented control, then the mode line `text-xs text-muted`. The settle window sits to the right
    of the control in Job mode: a label, a 3-digit numeric input and `days`. It wraps under the
    control below `md`.
  - **Rule.** `border-t border-rule`.
  - **TEMPLATES block.** The label and a one-line rule of thumb, then one row per template:
    - the index (`font-mono text-xs text-muted`)
    - the input (`rounded-theme border border-rule bg-bg px-2.5 py-1.5 font-mono text-sm
      text-ink`, `border-warn` on error, about 420 px wide on desktop and full width below `md`)
    - the counts (`text-xs`)
    - ↑ ↓ × as `btn-row btn-quiet` icon buttons
  - Under a row with an error, its message is shown in `text-xs text-warn`.
  - Then `+ Add template` (the text CTA), the token chips row, and its hint.
  - **Footer.** `border-t border-rule`. It appears only while a draft differs from the saved list:
    - the left side reads `Unsaved changes. The plan shows these templates after Preview.`
    - the right side holds the error hint (`text-xs text-warn`), **Discard** (`btn-quiet`),
      **Preview** (`btn-ghost`) and **Save templates** (`btn-accent`, withdrawn while any template
      has an error).
- **Folded card.** Once the templates are saved and valid, the card is folded to one line:
  `{Mode} · {N} templates · first: {template 1}` and **Edit templates ▾** (`btn-quiet`) at the
  right. Folded is the default when saved. It is open while a draft exists or a template has an
  error, and it stays open on reload until saved.
- **Plan.** The `PLAN` label and its count line:
  - `{N} videos · names from your saved templates`, or `… from your unsaved templates` after
    Preview.
  - Then the filter row: `rounded-theme px-2 py-1 text-xs`, the active filter `bg-surface-2
    text-ink`, each label followed by its count in `text-muted`. It wraps on desktop and scrolls in
    one row below `sm`.
  - Then the list card, the same `rounded-theme border border-rule bg-surface` as an extraction
    group, with rows separated by `border-t border-rule`.
- **Row.**
  - **Title line**: the checkbox slot (16 px), the resolved title `text-sm text-ink`, then
    `template N` and any status (`· out of date`, `· part 2`) in `text-xs text-muted`, or a reason
    chip. The lock button sits at the far right.
  - **Name line(s)**: `font-mono text-xs wrap-anywhere`.
  - **Explanation line**: `text-xs text-muted`, with any quoted value or name in `font-mono
    text-ink`.
- **Sticky bar.** It is the extraction commit bar: `sticky bottom-0 … border-t border-rule
  bg-surface px-3 py-2.5`. The left side reads `N selected`. The right side holds **Clear** and
  **Rename N**, the page's one solid `bg-accent` primary. The confirm replaces the bar's content in
  place.
- **Paging.** `shared/LoadMoreButton` under the list. Select all selects every video the active
  filter can rename, not only the loaded page, and its label carries that count.

### Video page, Maintenance (`routes/media/[id]/+page.svelte`)

- A new **File name** row comes after the existing Maintenance rows. It uses the same anatomy: the
  `w-28 shrink-0 pt-1.5 text-sm text-muted` label, then the value column.
- **Value column.** The current base name is shown in `font-mono text-sm text-ink wrap-anywhere`,
  followed by a closed padlock when the video is locked.
- **Action.** At the right is **Lock name** or **Unlock** (`btn-quiet` with the padlock). It moves
  under the status lines below `sm`.
- **Status lines**, one per state below, are `text-xs`. Actions sit under them in `mt-2 flex gap-2`.
- The row is hidden when the mode is Off.

## Design tokens used

| Token | Usage |
|---|---|
| `bg-bg` | page; template inputs; settle input |
| `bg-surface` / `border-rule` | Renaming card, plan list card, sticky bar, "Renamed from" box |
| `bg-surface-2` | active mode segment, active filter, active tab |
| `text-ink` | titles, new names, quoted values, active labels |
| `text-muted` | labels, counts, old names, explanation lines, `btn-quiet` |
| `text-warn` / `border-warn` | refusal chips, the old name's strike (`decoration-warn`), template errors, failed rows, undo refused |
| `bg-accent` / `text-accent-ink` | Rename N and Yes, rename N (primary); the renaming pill; the checked checkbox |
| `text-accent` | Select all, Open System activity, Retry (inline links); the done ✓ |
| `font-mono` | every file name, template text and token chip |
| `rounded-theme` | every box and button; chips that are pills use `rounded-full` |

There are no new tokens and no hard-coded colours.

## Components

| Component | Where | Notes |
|---|---|---|
| `rename/RenamingCard.svelte` | File names tab | Mode, settle window and templates, with the draft state and fold. Composes the owner section chrome. |
| `rename/TemplateList.svelte` | inside the card | Ordered rows, ↑/↓/×, + Add template, token chips, per-row error and counts. |
| `rename/RenamePlan.svelte` | File names tab | Filter row, select-all, rows, `LoadMoreButton`, sticky bar and confirm. |
| `rename/RenamePlanRow.svelte` | plan | Title line, name lines, explanation and lock button. One row shape for every reason. |
| `rename/FileNameRow.svelte` | video page Maintenance | Every video-page state below, including History (P1). |
| `rename/NameLockButton.svelte` | plan row and video page | The padlock toggle. Takes its glyph from `CoreImageEditButton`'s padlock (extract it to `shared/` if a third caller appears). |
| `sort/segmentedToggle.ts` | mode control | The existing segmented-toggle helper; no new control. |

A new `rename/` folder follows the folder rule in `lib/components/CLAUDE.md`, because the product
mechanism is renaming. Add its `CLAUDE.md` table and the folder's row in the same change.

## States and interactions

### Renaming card

| Element | State | Behaviour |
|---|---|---|
| Mode | change | Saves immediately, like an owner setting, with no Save button. The mode line updates. Choosing Off hides the plan. Choosing Plan removes the checkboxes and the bar's Rename. |
| Settle window | edit | Numeric, 1–365. It saves on blur or Enter. An invalid value shows `text-xs text-warn` "Enter 1 to 365 days." under it and keeps the saved value. |
| Template input | edit | Marks the draft dirty and shows the footer. Errors appear on Preview or Save, not per keystroke. |
| ↑ / ↓ | click | Moves the row and keeps focus on the moved row's same button. It is withdrawn at the ends. |
| × | click | Removes the row. With one template left, × is withdrawn (call 15). |
| + Add template | click | Appends an empty row and focuses its input. |
| Token chip | click | Inserts at the last-focused input's cursor. With no input focused, it goes into the last row. |
| Preview | click | The plan recomputes from the draft. The plan count line says `unsaved templates`, and the bar shows "Previewing unsaved templates. Save them to rename." with **Save templates** (`btn-accent`). Rename is not offered while previewing. |
| Save templates | click | Saves the list. On a server refusal, each refused row gets its message and the draft is kept. On success the footer goes and the card folds. |
| Discard | click | Reverts the draft. The plan reloads from the saved templates if a preview was showing. |
| Leaving with a draft | navigate | The app's standard unsaved-changes prompt. |

### Plan

| Element | State | Behaviour |
|---|---|---|
| Filter | click | Narrows the list, the count line and Select all. It lives in the URL (`?reason=`) like every list filter, so it survives Back. The default is `out-of-date` when its count is above 0, otherwise `all`. |
| Checkbox | toggle | Adds or removes the row from the selection. The selection survives a filter change. |
| Select all N | click | Selects every renameable video in the filter. It toggles to **Clear selection**. |
| Lock button | click | Locks or unlocks immediately (no confirm). The row moves to the Locked reason. With a filter active, the row stays in place until the next reload, so it doesn't jump under the cursor. |
| Rename N | click | The bar turns into the inline confirm. |
| Yes, rename N | click | Queues the renames. Each row's checkbox is replaced by its job state: an accent spinner while renaming, an accent ✓ when renamed (the new name turns `text-ink`), or a warn triangle when failed. A failed row has a `text-xs text-warn` reason plus **Retry** and **Dismiss**. The bar reads `{n} renamed · {n} renaming · {n} failed.` with **Open System activity**. |
| Cancel | click | Restores the bar. Focus returns to Rename N. |
| Empty filter | — | `py-16 text-center text-sm text-muted`. Out of date reads "No names are out of date." All reads "Every file has its name." |
| Loading | — | "Loading…" in the same centred style. |
| Error | — | "Couldn't load the plan. Retry" in `text-warn`, with Retry as an inline link. |

### Video page File name row

| State | Shows | Actions |
|---|---|---|
| Up to date | "Matches template N." | Lock name |
| Will rename or out of date | "Would be `{new}`", then when out of date "{Field} changed since Holodex named it." | **Rename** (`btn-ghost`, Manual and above, with no confirm because it's one file and undoable), Lock name |
| Plan mode | The same lines | Lock name only |
| Renaming | The accent pill `renaming` with a spinner, and "Started hh:mm. You can leave this page." | — |
| Renamed (latest rename, file still has that name) | The `bg-surface` box "Renamed {date} from `{old}`" | **Undo rename** (`btn-quiet`), **History (N) ▾** |
| History open (P1) | The names it has carried, oldest to newest: date, trigger (`manual`, `writeback`, `job`, `undo`), name. The current one is in `text-ink`. | History ▴ |
| Locked | The lock reason line (call 7), and a closed padlock after the name | **Unlock** |
| Held original | "Can't rename while the original is held (until {date})." | — (the banner above carries Keep or Restore) |
| Refused | A `text-warn` reason, the same wording as the plan row, then "Edit the template or the {field} in File names." | — |
| No template fits | "No template fits: it has no {token}." | Lock name |
| Undo refused | "Can't undo: `{old}` is taken." in `text-warn` | **Dismiss** |
| Rename failed | "Rename failed: {reason}." in `text-warn` | **Retry** (accent link), **Dismiss** (muted link), the Maintenance pattern |

After Undo rename succeeds, the row shows the Locked state with the undo phrasing.

## Keyboard and focus

- **Mode control.** A `radiogroup` with roving tabindex: ←/→ move the choice, and Space or Enter
  confirms it. This is `segmentedToggle`'s existing behaviour.
- **Template rows.** Tab order is input → ↑ → ↓ → × → the next row. **Alt+↑ / Alt+↓** inside an
  input move that row, and focus stays in the input.
- **Plan rows.** Tab order is checkbox → lock button → the next row. Space toggles the focused
  checkbox. The sticky bar is last in tab order and reachable with Shift+Tab from the top.
- **Confirm.** Focus moves to **Yes, rename N**, and Escape cancels.

## Responsive behaviour

| Width | Changes |
|---|---|
| ≥ `md` | As drawn: the settle window is beside the mode control, and old → new is on one line when it fits. |
| `sm`–`md` | The template input is full width; counts and ↑ ↓ × wrap to a second line under it. The settle window goes under the mode control. |
| < `sm` (375 px) | Filters are one horizontally scrolling row. Names always stack, old over `→ new`. The video-page lock action moves under the status lines. The bar keeps `N selected` and Rename N, with no Clear; the select-all row's **Clear selection** toggle clears. |

## Edge cases

- **Long names.** They are never truncated (call 1). A title longer than one line wraps, and the
  chip follows it.
- **A thousand-plus rows.** The list pages with `LoadMoreButton`. The counts and Select all are
  server totals, never the loaded page.
- **The plan changes under the owner.** A rename whose new name changed since the plan was shown
  fails with "The name changed since you looked." It is a failed row whose action is **Refresh
  plan** (accent link), not Retry: a retry would ask for the stale name again. After the refresh,
  the row shows its current new name and can be selected again.
- **Two rows swap names.** Both show plainly, with no disambiguator and no warning (spec Edge
  Cases).
- **Mode drops to Plan or Off mid-run.** Queued renames finish. The rows keep reporting.
- **No providers configured.** No `{provider-N-id}` chips appear.

## Accessibility

- Name lines are real text, so a screen reader reads both. The `→` has `aria-label="becomes"`.
- Reason chips are text, never colour alone: the chip word plus the explanation line.
- The lock button: `aria-pressed`, and `aria-label` "Lock name for {title}" or "Unlock name for
  {title}".
- Job-state changes in the plan announce through the list's existing `aria-live="polite"` count
  line, as `{n} renamed, {n} failed`.
- The `text-muted` labels stay at full contrast. Withdrawn controls drop their border rather than
  dimming (`frontend-theming.md`).

## Cinémathèque QA

- Accent appears on Rename N, Yes, Select all, Retry, the done ✓, the spinner and checked boxes,
  and nowhere else.
- The warn strike on old names is legible at 12 px mono on `--surface`.
- The mode segment and filter actives use `bg-surface-2`, never accent.
- Loading, empty and error states are themed. Fonts load offline (Archivo, the mono stack).

## Build checklist

- [ ] File names owner tab, with intro, Renaming card (mode, settle window, templates, draft footer,
      fold) and plan
- [ ] Plan: filter row in the URL, select-all across pages, rows for every reason, lock button,
      sticky bar, inline confirm, per-row job states, empty, loading and error states
- [ ] Out of date is the default filter when not empty; its rows name the changed field
- [ ] Video page Maintenance File name row with every state in the table, including undo and
      History (P1)
- [ ] `rename/` folder `CLAUDE.md` table; `lib/components/CLAUDE.md` folder row
- [ ] Keyboard: mode radiogroup, Alt+↑/↓ reorder, confirm focus and Escape
- [ ] 375 px pass; Cinémathèque QA
