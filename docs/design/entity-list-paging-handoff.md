# Design handoff: paged entity lists (F80, HOLODEX-354)

**Approved by the owner 2026-10-18** (mockup below). Rules: [entity-list-paging.md](../specs/entity-list-paging.md)
(EL1–EL10). Sibling handoffs: [bounded picker search](bounded-picker-search-handoff.md) (HOLODEX-586).

![Approved states](entity-list-paging-mockup.svg)

People, Studios, Tags and Films now show one page and a Load more control, instead of every entry at once.
The look is unchanged except for the states below. Every new piece reuses a control the app already has.
Tokens only, in Cinémathèque ([theming.md](theming.md)).

## Load more (all four pages)

The control is the same one the Media grid and the entity video grids use (`entity/EntityVideos.svelte`):
- A centred row, `flex justify-center pt-4`, below the last entry.
- A `btn-ghost px-4 py-2 text-sm` button labelled **"Load more (N left)"**. N is the total minus the loaded
  count, with thousands separators.
- While a page is in flight the button reads **"Loading…"** and is `disabled`. The loaded entries stay put.
- Once everything is loaded the row is gone. A list that fits in one page never shows it.

| Page | Under |
|---|---|
| People (List) and Studios | the last row |
| People (Poster) and Films | the poster grid, full width |
| Tags | the pill cloud, full width |

The same control everywhere is deliberate: the owner learns one paging gesture.

**Failure.** A failed Load more keeps the loaded entries and shows the `text-sm text-warn` line above the
list, the same as `EntityVideos`. The button comes back so it can be retried.

## A–Z restart (People, Studios)

`entity/AlphaIndex.svelte` gains one state: **active**. Both layouts show it, the sticky bar from `sm` up
and the right-edge rail below `sm`.

- The active letter is `text-accent font-medium` with a 2px `--accent` underline (`border-b-2 border-accent`),
  and carries `aria-current="true"`.
- The active letter stays clickable, and clicking it again reloads from that letter.
- Letters with no names keep today's dimmed, `aria-hidden` placeholder.

**Count line.** While a letter is active, the count line (`{n} people` / `{n} studios`, `text-sm text-muted`)
adds `· from M · Back to start`:
- "from M" is in `text-ink`.
- "Back to start" is a text button, `text-accent underline`. It clears the letter and restarts at the top,
  the same as clicking A or `#`, whichever has names first.

This answers why the list begins at M, and gives one way back without hunting in the index.

**On click** (a letter or Back to start):
1. The list is replaced by the first page from that point.
2. The page scrolls so the top of the list sits under the sticky bar, using smooth scrolling unless
   `prefers-reduced-motion` is set.
3. Focus moves to the first row's link.

Changing the sort, the view or the search clears the active letter, and the count line drops its suffix.
The index shows under the same conditions as today: name sort, no search, List view.

## Tags All tab

Categories follow the last tag in the **same** pill cloud: no divider, no heading, no second Load more.
Category pills keep their accent border and glyph, which is enough to tell them apart. N in "Load more
(N left)" counts tags and categories together on the All tab, and only its own kind on the Tags or
Categories tab.

The owner's "+ New" pill stays first in the cloud and is never paged.

## Search and Random

Nothing visible changes.
- A search shows the first page of matches with its own Load more.
- "No … match “q”." is unchanged.
- The result count is the match total. On Tags it reads "N results for “q”".
- Random looks the same as it does today. Re-roll replaces the list with a fresh first page and scrolls to
  the top of the list.

## Back

The page comes back with the same entries loaded, plus the same letter and count-line suffix, and then
scrolls. It never shows a short list first and then jumps; it draws once at the restored size.

## Accessibility

- **Load more.** It's a real `<button>`. Each finished page is announced in a polite live region:
  "Loaded 50 more. 1,164 left." When the last page lands and the button disappears, focus moves to the
  first newly added entry so it isn't stranded on `<body>`.
- **A–Z.** Active is `aria-current="true"` on the letter's button. "Back to start" is a button with that
  visible name.
- **Focus order.** Count line (Back to start), then the A–Z index, then the rows, then Load more.

## Edge cases

| Case | Shows |
|---|---|
| Fewer entries than one page | no Load more |
| A letter with one name | one row and no Load more; the count line still says "from X" |
| A–Z on the last letters | the list ends at Z, and Load more disappears |
| Owner merge select mode (People) | selection checkboxes persist across Load more; the merge bar counts every selected person |
| A merge, delete or create while pages are loaded | the loaded entries redraw in place at the same count; scroll is kept |
