# Duplicates lanes and pair evidence — one lane per entity type; person and studio pairs expand into a compare panel — design handoff

**Tickets:** [HOLODEX-451](https://whoiskevinrich.atlassian.net/browse/HOLODEX-451) (the person
panel, shipped) · [HOLODEX-598](https://whoiskevinrich.atlassian.net/browse/HOLODEX-598) (lanes,
the People rework, the Studios panel) · F70 ·
**Status:** Revised 2026-10-27 for 598, approved by the owner in chat the same day ·
**Spec:** [`duplicates-pair-evidence.md`](../specs/duplicates-pair-evidence.md) P0-1…P0-11,
RD1–RD18, OQ7 · Videos lane: [`duplicate-videos-handoff.md`](duplicate-videos-handoff.md) ·
**Architecture:** none. The epic's `architecture` gate is `[~]` because there's no technology fork.

## Decision

**The page becomes five lanes, and person and studio pairs use the Videos verdict grammar.** A
tab row picks one lane: Videos, People, Studios, Tags or Films. Each tab shows its count, and an
empty lane stays in place with a 0. Clicking anywhere on a person or studio row opens its panel.
The names open in a new tab, marked ↗. The collapsed row now offers only **Keep separate**. The
survivor is chosen inside the panel, with **Merge into this one** at the foot of each column,
behind a `ConfirmDialog`. Videos already works this way, and the owner calls it intuitive.

The **People** panel keeps the F68 hover card's anatomy laid flat (the 451 decision, still true).
It adds a **240 px lead headshot** above the strip, and choosing a strip frame swaps the lead. The
**Studios** panel is new. For each side it shows where the name came from, with a video count per
source, three sample posters and the aliases. Its confirmation chooses which spelling the
surviving studio shows, and says which name writeback will offer.

![Lanes and the People panel: the lane tab row with counts above the collapsed People lane, one row hovered to show the whole row is the toggle; an expanded person pair with a 240px lead headshot per side, a five-frame strip, new-tab links and Merge into this one under each column; the loading and failed-side states; the empty Films lane and the loading Studios lane](duplicates-pair-evidence-mockup.svg)

![Studios panel and confirmations: an expanded studio pair listing each side's name sources with video counts, three sample posters, aliases and Merge into this one; the person merge dialog; the studio merge dialog with its Show the studio as choice, at its default and with the other spelling picked](duplicates-studios-compare-mockup.svg)

| Mockup | Panel | Surface |
|---|---|---|
| lanes + People | 1 | lane tabs and collapsed People rows, one of them hovered |
| | 2 | People panel, expanded |
| | 3 | People states: loading, and a side that failed to load |
| | 4 | an empty lane and a loading lane |
| Studios | 1 | Studios panel, expanded |
| | 2 | the person and studio merge confirmations, including Show as |

### Choices the owner made (2026-10-27)

The mockups were rendered with the options side by side, and the owner picked these.

| # | Question | Answer | Why |
|---|---|---|---|
| C1 | How are lanes picked? | **A tab row**, using `SearchResultsPanel`'s tablist idiom (`px-2 py-1 text-xs`, active `bg-surface-2 text-ink`) | It sits one level and one size below the Owner tabs (`text-sm`), so it reads as a sub-nav. A row of stacked headings would put every lane's chrome back on screen, which is what overloaded the page. |
| C2 | Which verdict carries the accent on People and Studios? | **Match Videos.** Merge into this one is `.btn-row .btn-pill .btn-accent`, and Keep separate is `.btn-row .btn-ghost px-2`, in the row and in the footer. | In the Videos lane, Keep this one is the pill and Keep both the ghost. One grammar across lanes is Goal 7. Tag and film rows keep RD7 unchanged (see "Spec deltas"). |
| C3 | Sample posters (OQ7) | **Up to 3: one per name source first, in the order the sources are listed, then the most recently added** | Each spelling then shows a video it actually came from, which makes the source list checkable at a glance. A random sample would reshuffle on every expand, and posters from the newest videos alone could all come from one source. |
| C4 | How is Show as picked? | **A radio choice inside the confirm dialog**, defaulting to the survivor's own name | The writeback line and the confirm label update with the pick. Two buttons would hide the consequence until after the click. |
| C5 | The column action's label | **"Merge into this one"**, not "Keep this one" | Owner's note: "Keep this one" suggests the other side is dropped, when it's actually merged in, and its name survives as an alias. The Videos lane keeps "Keep this one", because there the other copy really does go to Trash. |

## Lanes (P0-8, RD13)

```
OwnerLayout nav            Status · Duplicates · …        ← unchanged, text-sm
div role="tablist" aria-label="Duplicate type"            ← new, on the Duplicates page
├── button role="tab"  Videos  2                         one per lane, in this fixed order:
├── button role="tab"  People  31                        Videos, People, Studios, Tags, Films
├── …                                                    label text-xs; count text-xs text-muted tabular-nums
div role="tabpanel"  aria-labelledby={tab id}
├── p  text-sm text-muted        the lane's own intro line (below)
└── section rounded-theme border border-rule bg-surface   ← the existing group shell, one lane's pairs
    └── rows · LoadMoreButton
```

- **Reuse the tablist from `entity/SearchResultsPanel.svelte`.** Copy its markup, roving
  `tabindex`, and its ←/→/Home/End handler. Lift a shared one only if a third caller appears.
- **The open lane is in the URL** as `?type=`, the existing deep link from the entity lists'
  Duplicates banner. Switching tabs calls `replaceState`, so the banner link, reload and a new tab
  all land on the same lane. No `?type=` → the first lane in tab order that has pairs. If every
  lane is 0 → Videos.
- **Counts render on every tab, always.** A 0 is a 0 (`text-muted`, still clickable), never
  hidden, so tabs never shift (F76 P0-2). On first load the tabs render **without numbers**
  until the page's first read returns them. Don't draw a spinner per tab.
- **The group heading goes.** Inside a lane, the old `h2` heading ("People · 31") just repeats
  the tab. The focus ladder's group rung (`groupId`) moves to the tabpanel, which keeps
  `tabindex="-1"`.
- **Intro copy is per lane.** The page's single intro paragraph describes name pairs only. Each
  lane gets its own line. People and Studios: *"Keep separate remembers your choice; merging
  keeps the other name as an alias."* Videos: its current line. Tags and Films: the current
  paragraph's wording.
- **After a verdict, re-read only that lane and the counts.** The tab's number drops with the
  row, using the same optimistic decrement as today.

| Lane state | Renders |
|---|---|
| loading (first open, or switching) | the tabpanel shows `py-16 text-center text-sm text-muted` "Loading…", and every count stays put |
| empty | "No possible duplicate {lane}." in the same `py-16` slot, inside the bordered shell |
| error | the existing `text-warn role="alert"` line, inside the tabpanel only. The tabs stay usable. |

## Rows (P0-1, P0-4, P0-9, RD15–RD17)

**The whole row toggles its panel (RD16).** On a person or studio row the root `div` gets
`onclick` → `onexpand(!open)`, plus `cursor-pointer` and `hover:bg-surface-2`. Ignore the click
when `e.target.closest('a, button')` is anything other than the disclosure itself, so names and
verdicts never toggle. **The chevron `<button>` stays** as the keyboard and screen-reader
control, with `aria-expanded`/`aria-controls` as today. The row's click is a pointer convenience
on top of it. Don't give the row root a role or a tabindex, and nothing gets nested in a control
(P0-1).

**Names are links (RD17, P0-9).** Each name `<span>` becomes
`<a href={profile} target="_blank" rel="noopener" class="truncate text-ink hover:text-accent">`
followed by `<span aria-hidden="true" class="text-xs text-muted">↗</span>`. The link's
`aria-label` is `` `Open ${name} in a new tab` ``. The mark is the Videos lane's `Open ↗`
treatment. Profile paths are `/people/{id}` and `/studios/{id}`. Tag and film rows keep plain
names.

**The verdicts (C2, C5).** On a person or studio row:

| Where | Control | Class |
|---|---|---|
| collapsed row | Keep separate (the only verdict) | `btn-row btn-ghost px-2` |
| panel footer | Keep separate | `btn-row btn-ghost px-2` |
| foot of each column | **Merge into this one** | `btn-row btn-pill btn-accent` |

On a tag or film row nothing changes: Keep separate is accent, Merge is ghost, and Merge opens the
inline `Keep: A / B / Cancel` step (RD7). The row's `verdicts` snippet branches on
`showsComparePanel(pair.entity_type)`. The row owns every verdict and `busy`, as before. It hands
the panel a `mergeInto(keep, other)` snippet, the way `VideoPairRow` hands over `keepThis`.

The `aria-label` for Merge into this one is `` `Merge ${other} into ${keep}` ``. Without the
name, "this one" is ambiguous to a screen reader.

## People panel (P0-2, P0-3, RD18)

The column, top to bottom. Rows 3–6 are the 451 column unchanged:

| # | Element | Markup | Rule |
|---|---|---|---|
| 1 | **Lead** | `PersonImageFrame` with `frameClass="portrait-frame--1x1 w-60"`, `sizes="240px"`, `eager` | Starts as the headshot. **One per side, both always visible.** No hover zoom, no lightbox (P2-4). |
| 2 | Strip | `<div class="mt-1 flex gap-1">` of up to 5 `PersonImageFrame`s in `<button type="button">`, each `w-11` | Slot order as before: headshot, then up to 4 non-headshot gallery images in `sort_order`. **Clicking a frame makes it that side's lead.** The other side doesn't change. The frame in the lead carries `aria-pressed="true"` and a 2 px `ring-accent` inset. `5 × 44 + 4 × 4 = 236 px`, so the strip spans the 240 px lead. **No strip at all** when the side has one image or none. The lead is enough. |
| 3 | Name + flags | as 451, with the name now an `<a>` + ↗ (see Rows) | |
| 4 | Meta line | as 451 | absent segments never pushed |
| 5 | Aliases | as 451. A side with none says **"No other names"** (`text-xs text-muted`, not italic). | The spec's "says so in words" (P0-10) applies to both panels, so the columns line up. |
| 6 | Link row | `Videos ↗` · `Films ↗` (only when `film_count > 0`) · `ProviderLinkBadge`s | **All of them open in a new tab** with the ↗ mark (P0-9). The badges already open externally, so add only the mark. |
| 7 | Verdict | `mt-2 border-t border-rule pt-2` holding Merge into this one | |

The lead state is per side: `lead: { a: imageKey, b: imageKey }`. It lives in the panel, never on
the row. It resets when the panel closes, so a reopened panel always starts on the headshots.

**Column width.** `field-grid` is still the grid, `repeat(auto-fit, minmax(min(320px,100%),1fr))`.
The lead needs 240 + 2 × 12 padding = 264 px, inside the 320 px minimum, so neither the lead nor
the strip ever reflows. Below two columns' width the sides stack, A above B, as before.

## Studios panel (P0-10, RD10)

`showsComparePanel` returns true for `'studio'` too. A studio pair gets the disclosure, the
whole-row toggle and the panel. Its match-kind label moves into the panel as a person pair's
does (`labelPlacement` → `'panel'` for both). This needs a new sibling,
**`DuplicateStudioPanel.svelte`**, because the data and the column differ from the person card.
It reuses the panel well, the `field-grid`, the column shell and the footer markup.

| # | Element | Markup | Rule |
|---|---|---|---|
| 1 | Name | `<a>` + ↗ (`skin-title font-display text-sm font-semibold`) | links `/studios/{id}` in a new tab |
| 2 | Count | `text-xs text-muted` "40 videos" | |
| 3 | **Name came from** | `<p class="mt-3 text-xs text-muted">Name came from</p>` + `<dl class="mt-1 grid grid-cols-[1fr_auto] gap-y-0.5 text-xs">` with a `<dt class="text-ink">` source and a `<dd class="text-muted tabular-nums">N videos</dd>` per row | One row per source that gave this studio its name on some video: **"File tags"** or the provider's own label (`provider-1`). Order: most videos first. One source → one row, with no empty slots. **Never a verdict word** ("wrong", "typo", "likely"): Goal 3. |
| 4 | Posters | `<div class="mt-3 grid grid-cols-3 gap-1">` of `.video-frame` | **Up to 3, per C3.** Plain images with `alt=""`, not links: the column's `Videos ↗` is the path. No videos → no grid, and no reserved space. A broken image keeps the themed empty frame, as `VideoComparePanel`'s poster does. |
| 5 | Aliases | `text-xs italic text-muted` "also known as …" (first 3) or **"No other names"** | |
| 6 | Link row | `Videos ↗` → `/studios/{id}#videos` | new tab |
| 7 | Verdict | Merge into this one | as People |

Use **"Name came from"** exactly. It describes provenance and judges nothing. It also avoids
"source", which owners read as the per-field source decision (the `SourceBadge` chip row),
and this panel is not that layer. There's no precedence control here, no provider-value
comparison, and no adoption row.

**Loading.** Same as the person panel. Each column shows its name and count from the pair row
right away. The sources list and posters are absent until data lands, and the poster grid is
held as three empty `.video-frame`s so nothing shifts. **Failure** is per side, with the same
"Couldn't load this side." + Retry as the person panel. It's never cached and never disables a
verdict.

## Confirmations (P0-4 RD15, P0-11 RD14)

Both use `shared/ConfirmDialog.svelte`, `variant` left at its default, `destructive` (`bg-warn`).
A merge can't be undone, and the Videos lane already gives Move to Trash the same weight. The row
owns the dialog, as `VideoPairRow` does.

**Person.** Title: `Merge into {keep}?` Body:

> **{other}**'s {N videos}, films and links move to {keep}. "{other}" stays as an alias, so it
> still finds this person.
>
> This can't be undone. *(`text-muted`)*

Confirm `Merge` · Cancel. This drops the "films" clause when the other side has no films, and the
video count comes from the pair row.

**Studio.** Title: `Merge into {keep}?` Body:

> **{other}**'s {N videos} and links move to {keep}.
>
> Show the studio as *(`text-muted`)*
> ◉ {keep} *its name now* · ○ {other}
>
> *(Default pick)* The other spelling stays as an alias. Writeback will offer **{keep}** for these
> files.
> *(Other spelling picked)* This renames the studio, as renaming it on its own page would.
> "{keep}" stays as an alias. Writeback will offer **{other}** for these files.
>
> This can't be undone.

- **The radio group** is `<fieldset>` + `<legend class="text-muted">Show the studio as</legend>` +
  two native `<input type="radio" class="accent-accent">` with their labels. The default is the
  survivor's own name. "its name now" is a `text-xs text-muted` hint on that option only.
- **The confirm label follows the pick:** `Merge` by default, and `Merge and rename` with the other
  spelling. `busy` shows the dialog's own `Working…`.
- **The writeback line always names the spelling that will be written** (P0-11). It's the
  sentence that makes Show as consequential, so it changes with the radio rather than staying
  generic.
- Cancel, Escape or a backdrop click merges nothing (P0-11).

## States and interactions

| Element | State | Behaviour |
|---|---|---|
| Lane tab | default / active / focus | `text-muted hover:text-ink` / `bg-surface-2 text-ink` / the global focus ring. `aria-selected` follows the active state. |
| Row (person, studio) | hover | `bg-surface-2` and `cursor-pointer` across the whole row. Links and buttons keep their own hover. |
| Row | expanded | chevron `rotate-180`, and the panel mounts below it (no height animation, as 451) |
| Strip frame | chosen | `aria-pressed="true"` + `ring-2 ring-inset ring-accent`. The lead swaps instantly, with no crossfade. |
| Strip frame | hover / focus | the frame gets `ring-1 ring-rule`. **No enlargement** (P0-3). |
| Merge into this one | pressed | opens the confirm. The panel stays open behind it. |
| Confirm | busy / error | `ConfirmDialog`'s own `Working…` and its inline `text-warn` error. The dialog stays open on error. |
| Row | resolved | removed instantly (no fade). Focus follows the existing ladder: next row's disclosure → the tabpanel → `QUEUE_ID`. |

### Keyboard and focus

| Key | Result |
|---|---|
| ←/→, Home/End on the tablist | moves between lanes (roving `tabindex`) and opens the lane |
| `Enter`/`Space` on the disclosure | toggles (the row's click is pointer-only) |
| `Escape` with a panel open | collapses it, and focus returns to its disclosure. **An open confirm owns Escape first** (as `VideoPairRow`). |
| Tab through a person column | name link → strip frames → Videos → Films → badges → Merge into this one, then the other column, then the footer's Keep separate |

Opening a second panel still collapses the first (RD12). The page's single `openKey` already
spans every lane, and switching lanes closes any open panel.

## Design tokens used

All the 451 tokens still apply. New uses:

| Token / utility | Where |
|---|---|
| `bg-surface-2` | active lane tab; row hover |
| `text-xs text-muted tabular-nums` | lane counts; the source counts |
| `portrait-frame--1x1 w-60` | the People lead |
| `ring-accent` / `ring-rule` | chosen / hovered strip frame |
| `.video-frame` | studio posters (`VideoComparePanel`'s poster shell, without its badges) |
| `.btn-row .btn-pill .btn-accent` | Merge into this one |
| `.btn-row .btn-ghost px-2` | Keep separate on person and studio pairs |
| `bg-warn text-warn-ink` | the confirm button (ConfirmDialog's destructive variant) |
| `accent-accent` | the Show as radios |

Nothing is hardcoded. The mockups' literal hexes are the Cinémathèque tokens at `app.css:101–123`.
The image placeholders' `#221c18` is a stand-in for real images, not a token.

## Edge cases

- **A lane whose count is 0 but is named in `?type=`.** It opens and shows the empty message.
  It never redirects to another lane, because the banner link must land where it says.
- **A pair whose two names differ only in what the row truncates.** The full names are in the
  confirm dialog's title and body, which never truncate.
- **A studio with one name source on each side, from the same source** (both file tags). The
  counts are still the evidence, so show both columns as normal.
- **A studio with no videos.** It shows a count of "0 videos", no sources list, no posters, and
  its aliases or "No other names".
- **Show as when the survivor's name and the other spelling differ only in case.** Out of scope:
  case is already auto-merged before a pair is ever proposed.
- **A person with a single image.** It gets the lead and no strip (rule 2), not a strip of one.

## Accessibility

- The tablist follows the ARIA tabs pattern, copied from `SearchResultsPanel`: `role="tablist"`
  with an `aria-label`, then `role="tab"`, `aria-selected`, `aria-controls`, and `role="tabpanel"`
  with `aria-labelledby`. Each tab's accessible name includes its count ("People, 31").
- New-tab links carry `aria-label="Open {name} in a new tab"`. The ↗ is `aria-hidden`.
- Strip frames are buttons labelled `Show image {n} of {name}`, with `aria-pressed`. The lead's
  `alt` is the person's name. Strip images are `alt=""`, since the button carries the label.
- Merge into this one carries `aria-label="Merge {other} into {keep}"`.
- The Show as radios are a native `fieldset`/`legend` group. The writeback sentence sits in the
  dialog body beside them, so it is read with the choice, and its text changes with the pick.
- The row's whole-row click is pointer-only and duplicates the disclosure button, so no
  keyboard path depends on it.

## Cinémathèque QA

Per `.claude/rules/frontend-theming.md`, QA Cinémathèque, the only look.

1. The active lane tab: `bg-surface-2` against the page `bg-bg`. Check it reads, since the two are
   close.
2. Row hover `bg-surface-2` against `bg-surface`. It's about 2 % apart, so the hover is a
   convenience, not a signal. The cursor carries it.
3. The chosen-frame `ring-accent` on a dark headshot and on the themed placeholder.
4. The ↗ mark at `text-xs text-muted` beside a `text-ink` name: legible, and subordinate to it.

Greps that must stay empty:
`rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` ·
`rg 'text-muted[^"]*disabled:opacity' web/src --glob '*.svelte'`

## Build checklist

- [ ] `routes/owner/duplicates/+page.svelte`: the lane tablist and tabpanel, `?type=` via
      `replaceState`, the default lane, a per-lane intro line, per-lane read and re-read, and
      counts on every tab. Remove the group `h2`s.
- [ ] `queue.ts`: `showsComparePanel` → person **and** studio. `labelPlacement` follows it. The
      focus ladder's group rung becomes the tabpanel id. Update `queue.test.ts`.
- [ ] `DuplicatePairRow.svelte`: whole-row toggle, name links + ↗, and verdicts branched by
      `showsComparePanel` (ghost Keep separate only; the `mergeInto` snippet; the merge
      `ConfirmDialog` with Show as for studios). Tag and film rows unchanged.
- [ ] `DuplicateComparePanel.svelte`: the lead, the strip buttons, per-side lead state, new-tab
      links, "No other names", and Merge into this one per column. Drop the footer's Merge.
- [ ] `DuplicateStudioPanel.svelte`: new. **Add its row to `duplicates/CLAUDE.md`** in the same
      commit, and update the `DuplicateComparePanel`/`DuplicatePairRow`/`queue.ts` rows.
- [ ] `verdictOwnership.test.ts`: extend it to the studio panel and the `mergeInto` snippet.
- [ ] `app.css` `.btn-ghost`/`.btn-accent` doc comments: on person and studio pairs, Keep separate
      is now the ghost.
- [ ] `/testing-strategy`, `/security-review` (the new lane, studio-evidence and merge-rename
      reads are owner-gated), `/code-review high --fix`.

## Spec deltas this handoff introduced — **applied 2026-10-27**

| Spec | Was | Now |
|---|---|---|
| OQ7 | open | **answered** (C3): up to 3 posters, one per name source first, then the most recently added |
| intro, Goal 7, RD15, P0-4 (598 criteria), P0-11 | "Keep this one" | **"Merge into this one"** (C5). The Videos lane (F76) keeps "Keep this one". |
| Goal 2, P0-4 intro and first two criteria | Keep separate is the accent on any pair row | applies to **tag and film** pairs. On person and studio pairs Keep separate is the bordered neutral and Merge into this one the accent (C2, Goal 7). |
| P0-10, criterion 4 | "up to a small fixed number" of posters | "up to 3, one per name source first, then the most recently added" |
