# Spec: Filter and sort on provider links and field values (F90)

**Status**: Draft
**Phase**: F46 (queryable fields). Reads what the resolver shows; changes nothing in enrichment,
curation or writeback.
**Owner**: Project owner
**Date**: 2026-11-06
**Feature block**: **F90**. The owner can ask which videos, people and studios are linked to a
metadata provider, which have or lack a value for a field, and how much providers contributed. This
replaces the Completeness score, ring and sort.

**Epic**: [HOLODEX-626](https://whoiskevinrich.atlassian.net/browse/HOLODEX-626).

**Changes other specs** (edited in place in the same PR):
- F55/F65 [entity-completeness-score.md](entity-completeness-score.md): the score, the extras number,
  the ring and the Completeness sort are retired. The owner's queue and the per-entity list of
  missing fields stay, without a number.
- F73 [list-toolbar.md](list-toolbar.md): the Non-Goal on a Missing filter is reversed (RD1). People
  and Studios gain a Filters control.
- F87 [query-language.md](query-language.md): adds the `has` condition and pins what "matched" means.

**Depends on**: [F89](smart-playlist-rules.md) (smart playlist rules,
[HOLODEX-619](https://whoiskevinrich.atlassian.net/browse/HOLODEX-619),
[PR 580](https://github.com/whoiskevinrich/holodex/pull/580), merged). F89 makes the values a video
displays queryable, and this spec builds on that.

**Architecture**: [field-resolution.md](../architecture/field-resolution.md), "Field presence and
provider contribution are cached".
**Design handoff**: [provider-and-field-filters-handoff.md](../design/provider-and-field-filters-handoff.md). **Test plan**: to be written.

---

## Problem Statement

The owner wants to ask plain questions about the library's metadata:

1. Which videos are linked to a provider?
2. Which videos aren't?
3. Which people are missing a birthdate?
4. Which videos have a release date?
5. Which videos have no poster?

Only the first two can be asked today, and only on Media. The rest need a percentage, a ring on every
card and a sort, which say how filled-in something is rather than which things match. A score also
hides the facts it summarises, so 75% can mean several different gaps. The owner's page of missing
fields answers some of this, but it's a queue for working through gaps, not a way to look at a list.

## Goals

1. **All five questions work**, each returning exactly the entities it describes.
2. **One meaning of "linked"** across Media, People and Studios.
3. **"Has a value" agrees with the page.** A filter's answer matches what the entity's own page shows
   for that field.
4. **Less on screen.** No completeness percentage or ring remains anywhere in the product.
5. **Visitors see no difference.** Everything here is owner-only.

## Non-Goals

- **Bulk or single actions on a filtered list.** The feature ends at the list. *Why*: the owner may or
  may not act on what they find, and acting is a separate feature.
- **Recording search attempts.** "Never tried" and "searched, found nothing" are both *not linked*.
  *Why*: this would need new storage and refresh changes, and the owner's question is about links.
- **A movie type.** Videos have no kind, so "movies with a release date" means any video with a release
  date. *Why*: the owner chose not to introduce the concept here.
- **Filters on the Films list.** It's the one entity list left without them, deliberately. *Why*: it
  follows from having no movie concept, and it's easy to add later.
- **The full query language on People and Studios.** They get the Filters control only, so conditions
  narrow with AND. F87's Non-Goal on querying people or studios as results stands. *Why*: no question
  here needs OR or NOT on those lists.
- **"Which source supplied this field" conditions.** F87 and F89's Non-Goal stands. `has` asks whether
  a value exists, not where it came from.
- **Numeric conditions on the enriched count**, such as fewer than 3. It's a sort and a column for now.
  *Why*: no question here needs it.

## Concepts

- **Linked**: at least one metadata provider has supplied data for this entity itself, and the owner
  hasn't cleared it.
  - Reading a title from a filename isn't a provider, so that alone doesn't make an entity linked.
  - A video isn't linked through its Film. A video showing data from a linked Film, with no link of its
    own, reads as not linked (RD3).
  - **Not linked** is everything else, whether or not a search was ever tried.
- **Has a value**: the entity's page shows a value for that field, whichever source won (file, provider
  or the owner's curation).
  - A value the owner's source decision hid doesn't count, and neither does one a provider stored for a
    different field the owner doesn't map.
  - A field marked *not applicable* with no value counts as **missing** (RD4). Not applicable only keeps
    an item out of the owner's queue.
- **Poster, photo and branding image**: images count as fields here. A video **has a poster** when it has
  a real poster, from the file's cover art, a provider or the owner. An automatic thumbnail doesn't count,
  even though it fills the card.
- **Enriched count**: how many of an entity's mapped fields have data from a real provider, counted once
  each however many providers supplied them.
  - It counts stored data whether or not that value is the one displayed. It says what providers
    contributed, not what won.
  - It excludes internal provider data, filename parsing, images and fields not in the owner's mapping.
  - A low count doesn't mean an entity is thin: a video with full file tags and no provider has 0.

```
matched:any -has:poster
-matched:any studio:Paramount
has:released
has:director -has:actors
```

## User Stories

- As the owner, I want to find videos linked to a provider, and videos that aren't, so that I can see
  how far enrichment has reached.
- As the owner, I want to find people with no birthdate so that I can see where that gap is.
- As the owner, I want to find videos that have a release date, or have no poster, so that I can look at
  exactly that set.
- As the owner, I want to sort any of these lists by how much providers contributed so that the most and
  least enriched are easy to see.
- As the owner, I want a filter's answer to match the entity's page so that I can trust the list.
- As a visitor, I want none of this to appear, so that lists stay as they were.

## Requirements

### Must-Have (P0)

**P0-1 One meaning of linked.** Every provider link condition follows *Concepts*.
- [ ] A video whose only data comes from filename parsing reads as not linked.
- [ ] A video with data from provider-1 reads as linked, and matches `matched:provider-1`.
- [ ] Filename parsing is never offered as a provider in suggestions.
- [ ] After the owner clears a video's only provider, it reads as not linked.
- [ ] A person or studio with data from provider-1 reads as linked in the Filters control.

**P0-2 The `has` condition on Media.** `has:<field>` is true when the video has a value for that field;
`-has:<field>` when it doesn't. It accepts any field in the owner's mapping, plus `released` and
`poster`. It composes with every F87 condition under AND, OR and NOT.
- [ ] `has:director` matches a video whose director comes only from a provider, and one whose director
      comes only from its file tags.
- [ ] A video where the owner removed the only value of a field matches `-has:` for that field.
- [ ] `-has:poster` matches a video showing only an automatic thumbnail.
- [ ] Every video a `released:` condition matches also matches `has:released`.
- [ ] A value that displays on the page but can't be read as its field's kind, such as a malformed date,
      still counts as having a value, though `released:` can't match it.
- [ ] Computed fields and internal provider data aren't accepted. The poster's name in a query is `poster`
      only.
- [ ] An unknown field is refused with F87's message for an unknown field, and a field later removed
      from the mapping makes a saved playlist stale, as F89 P0-6 does.
- [ ] A `has` condition shows as a chip; `-has` as F87's NOT chip. Both read in words ("No poster").

**P0-3 Filters control on Media, People and Studios.** The owner's Filters panel gains a **Provider link**
control (any, linked, not linked) and a way to pick fields and require each to have a value or be missing.
The fields offered are the ones that apply to that entity, including its image. Everything narrows with
AND, and the state lives in the URL like other filters.
- [ ] People: choosing *Birthdate: missing* lists exactly the people whose page shows no birthdate.
- [ ] Studios: choosing *Not linked* lists exactly the studios with no provider link.
- [ ] Media: choices become the same chips as typing the query, and removing a chip clears the choice.
- [ ] Media: conditions the panel can't show, such as `has:director OR has:actors` or `matched:provider-1`,
      stay as chips, leave the panel's matching controls unset, and aren't rewritten by opening the panel.
- [ ] A filtered People or Studios URL reopens with the same filters and results.
- [ ] Every list's count line reflects the filters, and an empty result says so with a way to clear them.
- [ ] The Films list gains none of this.

**P0-4 Most enriched sort and Enriched column.** Media, People and Studios offer **Most enriched** and
**Least enriched** sorts, and an **Enriched** column in Table view, showing the count in *Concepts*.
- [ ] Two providers supplying the same field count as one.
- [ ] Filename parsing, internal data and images add nothing to the count.
- [ ] Ties fall back to the list's default order, and paging never repeats or skips an item.
- [ ] Sort position survives Back, as other owner-only sorts do.

**P0-5 Answers are current.** After the owner changes a value, source decision or link, the next read of
a list reflects it.
- [ ] Choosing a different source for a field changes which filters the entity matches on the next read.
- [ ] Clearing a provider changes the entity's linked state and enriched count on the next read.
- [ ] While the boot-time refresh is running, lists still load, and any lag ends when it finishes.

**P0-6 The score is retired.** The percentage, extras number, ring and Completeness sort are removed
everywhere: cards, list rows, the person hover card, film scene rows and detail pages.
- [ ] No completeness percentage or ring appears anywhere, for owner or visitor.
- [ ] A saved sort preference, shared link or saved playlist using the Completeness sort opens with the
      list's default sort, with no error.
- [ ] One-click refresh from a card goes with the ring. Refreshing from the entity's own page is unchanged.

**P0-7 The queue and detail list stay, without a number.** The owner's queue keeps its grouping by missing
field and its split between ready to apply and needs research. Detail pages keep the list of fields with
present or missing status, the not-applicable toggle, and the jump to an empty field.
- [ ] The queue's groups and row actions work as before, including its separate measure of which gaps
      already have a candidate ready.
- [ ] An entity's list of fields shows no score, and the jump to an empty field still opens that field.
- [ ] Marking a field not applicable removes the entity from that queue group, but not from `-has:` (RD4).

**P0-8 Owner-only.** All of this is owner-only, and enforced by the server as well as hidden.
- [ ] A visitor's query or shared link using `has` or `matched` is refused as F87 refuses `matched`.
- [ ] Visitors see no new filters, sorts or columns, and their list responses carry no count or link state.
- [ ] Making a smart playlist public is refused while it uses `has` or an enriched sort, as F89 refuses
      other owner-only conditions, and a frozen snapshot may still be published (F89 RD14).

**P0-9 Saved queries keep working.**
- [ ] `has` works in smart playlists and in the search tool, like any other condition.
- [ ] A saved `matched:any` playlist whose videos have only filename data loses them, since that's the
      corrected meaning. This is the one deliberate exception to F87 P0-10, and the release note says so.

### Nice-to-Have (P1)

- **P1-1 A has-a-value suggestion.** Suggestions after `has:` list the owner's fields by label.
- **P1-2 Linked providers in the Table view.** A Provider link column naming each provider, or *Not linked*.

### Future Considerations (P2)

- **P2-1 Actions on a filtered list.** Nothing here should stop a later feature acting on a result set.
- **P2-2 Tried but found nothing.** A third link state, once attempts are recorded.
- **P2-3 Filters on the Films list.**
- **P2-4 Enriched count conditions.**
- **P2-5 Query language on People and Studios** (F89's P2-3 covers the same ground).

## Success Metrics

There's no analytics in this product, so success is checked directly:

- **All five questions** in the Problem Statement return the right set on a real library, compared with
  each entity's page for a sample of results.
- **No disagreements:** in that sample, no entity is in a filter's result while its page says otherwise.
- **No ring left:** none on any list, card, hover card or detail page in Cinémathèque.
- **Visitor test:** every visitor path that reaches `has`, `matched`, the sorts or the column is refused.

## Open Questions

None. The filename problem with `matched:any` was confirmed by P0-1's first test on 2026-11-09.

## Decisions

- **RD1 Filtering on a missing field comes back to list pages** (2026-11-06), reversing F73's Non-Goal.
  *Why*: F73 removed it as a work queue, but the owner's need is to look at a list, not to work one. The
  queue stays for working through gaps. *Rejected*: pointing the owner at the queue, which can't combine
  conditions and only covers the fields the score counted.
- **RD2 Retire the score, keep the queue** (2026-11-06). *Why*: the score answered "how good?" when the
  owner's questions are "which ones?". The queue is still where acting on gaps lives. *Rejected*: removing
  both, which loses the ready-to-apply split; keeping everything, which leaves two answers to "what's
  missing?" on screen.
- **RD3 A video's link is its own** (2026-11-06). *Why*: one simple definition, so *not linked* keeps
  meaning "needs linking". *Rejected*: counting a linked Film's link.
- **RD4 Missing means empty** (2026-11-06). *Why*: otherwise a video with an empty field could appear in
  neither *has* nor *missing*. *Rejected*: hiding not-applicable fields from *missing*.
- **RD5 One state for not linked** (2026-11-06). *Why*: no new storage, and the question is about links.
  *Rejected*: tracking attempts now (P2-2).
- **RD6 The Filters control on every list, not the query language** (2026-11-06). *Why*: the same control
  everywhere, with F87's Non-Goal intact. *Rejected*: full queries on People and Studios, a much larger
  build; a panel on People and Studios only, which makes the owner type on Media and click elsewhere.
- **RD7 Every mapped field, not only the ones the score counted** (2026-11-06). *Why*: it answers
  questions nobody has listed yet. *Rejected*: only the score's fields, which leaves director and others
  out.
- **RD8 No movie concept** (2026-11-06). *Why*: the owner's call; a release date on any video is enough.
  *Rejected*: filters on Films; a condition for videos marked as the full film.

## Timeline Considerations

F89 has merged. Suggested build order, each usable on its own:

1. The definition of linked, its fix, and `has` on Media (P0-1, P0-2, P0-5, P0-8, P0-9).
2. The Filters control on all three lists (P0-3).
3. Enriched sort and column (P0-4).
4. Retire the score (P0-6, P0-7), last, so nothing is lost before its replacement exists.
