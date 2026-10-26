# Spec: List views — a shared Table view, and Posters for Tags (F82)

**Status**: Draft
**Epic**: HOLODEX-32 (Media view toggle) · with HOLODEX-42 (People Table) and HOLODEX-46 (Tags
views). HOLODEX-57 (add to a playlist from the grid by multi-select) was split out on 2026-10-24.
**Owner**: Project owner
**Date**: 2026-10-24

**Builds on**: [`list-toolbar.md`](list-toolbar.md) (F73). This spec fills the toolbar's View slot
on Media and Tags and adds a third view to People. It does not change F73's slot order, its
list-state rules (R7) or how sort works (R2).
**Builds on**: [`people-poster-view.md`](people-poster-view.md) (F55). People's List and Poster
views and the remembered People choice are unchanged.
**Reuses**: [`studio-images.md`](studio-images.md) (F51). A tag's poster image follows the owner
upload rules that studio and person images already have.

**Design handoff**: [`list-views-handoff.md`](../design/list-views-handoff.md), with committed SVG
mockups. **Test plan**: [testing-strategy §33](../testing-strategy.md#33-list-views--the-shared-table-view-and-tags-posters-f82-holodex-32--42--46).

---

## Problem Statement

Three list pages show their items one way only, and two of those ways hide what an owner is
scanning for.
- **Media** shows thumbnails only. Comparing duration, resolution or date added across 40 videos
  means reading the badge on each card. There is no compact view (HOLODEX-32).
- **People** has List and Poster. Neither lines up video counts in a column you can scan or sort
  from (HOLODEX-42).
- **Tags** shows pills only. A tag has no picture, so a library with 300 tags is a wall of words,
  and there is no row view to compare counts (HOLODEX-46).

Each page that does have a second view invented its own. A third, unshared table on each page
would repeat that.

## Goals

1. **One Table view, the same on every page that has one.** Media, People and Tags show the
   same table: same row height, header behaviour, narrow-width rules and empty state. Only the
   columns differ.
2. **Scan and re-sort from the table.** Clicking a column header sorts by it, through the same
   single sort the toolbar already has. There is never a second sort state.
3. **Tags get a visual view.** The owner can give any tag a poster image from its page. Tags
   without one show a shared placeholder, so the Posters view is never broken.
4. **No one's current view changes.** Existing owners and visitors land in the view they had
   before this ships.

## Non-Goals

- **Multi-select in any view.** Adding videos to a playlist from the grid is HOLODEX-57's own
  epic. People's merge selection stays in List view only (F55 RD2), and Tags' bulk select stays in
  Chips.
- **New sort orders.** A header is clickable only when the toolbar's Sort already has a matching
  entry. This spec adds no entries.
- **New columns that need data the list doesn't already carry.** Studio, actors and director on
  Media rows, and aliases on People rows, are P2. The lists don't return them today.
- **Chosen or resizable columns, and column reordering.** The column set is fixed per page.
- **Images for categories, and images for tags from a provider.** Tag images are owner uploads
  only. Tags aren't in the per-field source model (see `field-resolution.md`).
- **View toggles on Studios and Films.** Neither page asked for one. The Table pattern is built
  so either can adopt it later.

## Resolved Decisions

- **RD1 — The Table view has fixed columns per page.** It is called Table, not Details, so it
  never collides with the Media Details page. Decided 2026-10-24.
- **RD2 — Media's columns are file facts only:** title, duration, resolution, date added, plus file
  size and completeness for the owner. Decided 2026-10-24. Anything else needs a list change (P2).
- **RD3 — A column header sets the toolbar sort.** Clicking it picks the matching entry in Sort.
  Clicking it again flips the direction, where Sort has both directions. Decided 2026-10-24.
- **RD4 — At narrow widths, low-priority columns drop.** Each column has a priority. The table never
  scrolls sideways, and the name column is always kept. Decided 2026-10-24.
- **RD5 — Tag posters are owner uploads, with a placeholder otherwise.** No thumbnail is borrowed
  from a tag's videos. Decided 2026-10-24.
- **RD6 — Categories in Posters view show the placeholder card.** The view toggle offers the same
  choices on every Tags tab, so switching tabs never changes the view.

## User Stories

**Owner**
- As the owner, I want to see my videos as rows with duration, resolution and date added, so that
  I can spot short clips, low-resolution files and recent imports without opening each one.
- As the owner, I want to click "Duration" to sort by it, so that I don't have to find the same
  choice in the Sort menu.
- As the owner, I want a sortable table of people with their video counts and completeness, so
  that I can find the people who need work.
- As the owner, I want to give a tag a poster image from its page, so that the Tags page can show
  my library as pictures.
- As the owner, I want to replace or remove a tag's poster, so that a bad upload isn't permanent.

**Visitor**
- As a visitor, I want the same Table view without owner-only columns, so that I can browse
  densely without seeing columns I can't act on.
- As a visitor, I want the Posters view of Tags to look complete even when few tags have images,
  so that the page doesn't look broken.

**Edge cases**
- As anyone on a phone, I want the Table view to fit the screen, so that I never scroll
  sideways to read a row.
- As anyone whose filters match nothing, I want the Table view to show the same "no matches"
  message the grid does.

## Requirements

### Must-have (P0)

**P0-1 — The View slot offers these choices.** The first is the default.
- Media: **Grid**, **Table**.
- People: **List**, **Poster**, **Table**.
- Tags: **Chips**, **Posters**, **Table**, on every tab (All, Tags, Categories).

*Acceptance:*
- [ ] Each page's View slot shows exactly its choices, in this order.
- [ ] Studios and Films show no View slot (unchanged from F73).
- [ ] A visitor and the owner see the same choices.

**P0-2 — The choice is remembered per page, on this device.** As F73 R7 says, the view is not in
the URL and only an explicit choice saves it. A first visit gets the page's default.
*Acceptance:*
- [ ] Choosing Table on Media and reloading shows Table. People is unaffected.
- [ ] An owner who chose Poster on People before this ships still lands in Poster.
- [ ] A shared link never carries or forces a view.

**P0-3 — One shared Table view.** Every Table view has the same row height, header style,
hover and focus states, and empty and loading states. Only the columns differ.
- Each row opens its item, as a card or chip does.
- Rows load in the same pages as the other views.
- The density slider is hidden in Table, as it is in People's List view.

*Acceptance:*
- [ ] Clicking or pressing Enter on a row opens that video, person or tag.
- [ ] Load more adds the next page under the last row, with no gap or duplicate row.
- [ ] A filter that matches nothing shows the page's existing empty message.

**P0-4 — Columns per page.** Columns marked (owner) are hidden from visitors. The name or title
column is always kept. The others are listed from the highest priority down.
- **Media:** title (with a small thumbnail) · duration · resolution · date added · size (owner) ·
  completeness (owner).
- **People:** name (with a small headshot) · videos · completeness (owner).
- **Tags, tag rows:** name · videos · parent tag.
- **Tags, category rows:** name · tags.
- **Tags, the All tab:** shows tag rows, then category rows, as Chips does today.

*Acceptance:*
- [ ] A visitor never sees size or completeness, and the table has no empty gap where they were.
- [ ] A tag with no parent shows an empty parent cell, not "None" or a dash.

**P0-5 — Headers sort through the toolbar (RD3).**
- A header is clickable only when Sort has a matching entry. Otherwise it is plain text.
- Clicking a header selects that entry in Sort.
- Clicking the header of the active sort flips its direction, when Sort offers both directions.
- The active column shows its direction.

*Acceptance:*
- [ ] After clicking a header, the Sort select shows the same choice, and the URL `sort` value
  matches (F73 R7).
- [ ] Changing Sort in the toolbar moves the header's indicator.
- [ ] A screen reader announces the sorted column and its direction.

**P0-6 — Narrow widths drop columns, never scroll (RD4).** As the page narrows, the
lowest-priority columns drop first. At 375px a row shows the name plus at least one other column.
*Acceptance:*
- [ ] At 375px the page has no horizontal scroll in Table on Media, People or Tags.
- [ ] Media at 375px keeps title and duration. People keeps name and videos. Tags keeps name and
  videos.

**P0-7 — The A–Z rail follows People's rule.** It shows in Table under the same conditions as in
List: name sort, no search query.
*Acceptance:*
- [ ] The rail appears in People Table when sorted by name, and jumps to that letter's first row.

**P0-8 — Owner tag actions work in every Tags view.** The per-tag owner menu (rename, alias, set
parent, merge) is reachable from a chip, a poster card and a Table row. Bulk select stays in
Chips.
*Acceptance:*
- [ ] Each of the four actions can be started from a Posters card and from a Table row.

**P0-9 — Tags Posters view.** Each tag is a poster card with its image, or the shared placeholder
when it has none, plus its name and video count. Categories show as placeholder cards with their
name and tag count (RD6).
- The placeholder is the same for every tag. It is never a thumbnail from the tag's videos.
- Density works in Posters, using the shared density value as People's Poster does.

*Acceptance:*
- [ ] A library where no tag has an image shows a full, even grid of placeholders.
- [ ] Uploading an image to a tag shows it on that tag's card on the next visit to the list.

**P0-10 — The owner can upload, replace and remove a tag's poster from the tag's page.**
- The upload follows the rules that person and studio image uploads already have: the same
  accepted formats and size limit, and the image is cleaned before it is stored.
- Visitors see the poster but no upload controls.
- Removing it returns the tag to the placeholder.
- Renaming or merging a tag keeps its poster. In a merge, the surviving tag keeps its own poster,
  or takes the merged tag's if it had none.
- A tag with a poster counts as one you've worked on, so it is never removed automatically as an
  unused tag. It goes only when you merge it away, and its poster goes with it unless the merge
  carried it over.

*Acceptance:*
- [ ] An upload larger than the person-image limit is refused with the same message.
- [ ] A visitor's tag page shows the poster, if one exists, and no controls.
- [ ] Merging a tag that has a poster into one with no poster carries the image over.

### Should-have (P1)

- **P1-1 — Table on Studios and Films.** Both pages adopt the shared table, with their own
  columns, once the pattern has shipped.
- **P1-2 — Tag poster on the tag's own page as a header image**, not only on the list.

### Future considerations (P2)

- **P2-1 — Richer Media columns:** studio, actors, director. These need the list to return them.
- **P2-2 — People aliases and Tags aliases as columns.** These need the lists to return them;
  aliases were left out of the tags list on purpose (HOLODEX-589).
- **P2-3 — Category images**, using the same upload rule as tags.
- **P2-4 — Multi-select in Table**, which HOLODEX-57's epic may want.

## Success Metrics

This is a single-owner app with no analytics, so the measures are QA checks and owner use.
- **Leading:** every P0 acceptance box passes in live QA on the stress fixture at desktop width
  and at 375px, as owner and as visitor.
- **Leading:** zero horizontal scroll at 375px on all three Table views (geometry check).
- **Lagging:** the owner keeps choosing Table on Media or People a month after shipping. If not,
  revisit the column set.
- **Lagging:** the owner uploads posters to at least their 20 most-used tags within a month, which
  shows Posters is worth having.

## Open Questions

- **[engineering]** Whether every Media column already has a matching Sort entry. Any that doesn't
  stays a plain header under P0-5. Not blocking.

## Timeline Considerations

- No deadline.
- Phase 1, frontend only: the shared Table view on Media, People and Tags.
- Phase 2, needs backend work: tag poster upload and the Posters view. It reuses the studio image
  pattern, so it needs `/security-review` as a new upload surface before merge.
- Both phases can ship in one PR or two. The design handoff covers both.

## Gate status

- [x] `/write-spec` — this document
- [x] `/design-handoff` — [`list-views-handoff.md`](../design/list-views-handoff.md); owner
  sign-off at `/implement`
- [x] `/testing-strategy` — §33
- [ ] `/security-review` — the new tag image upload
- [ ] `/architecture` — expected `[~]`, no technology fork: tag images reuse the studio image store
