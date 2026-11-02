# Spec: Smart playlist rules — dates, a cap, playlists in queries, resolved fields, watch history and an on-page editor (F89)

**Status**: Draft
**Phase**: F46 (queryable fields) and Playlists. Adds conditions to the F87 query and settings to the
F75 smart playlist. Reads the resolver's output; changes nothing in enrichment or writeback.
**Owner**: Project owner
**Date**: 2026-11-01
**Feature block**: **F89**. A smart playlist can say what it can't today: "added in the last 30 days",
"only the newest 25", "in this other playlist", "directed by this person" when only a provider knows
it, and "not watched yet". The owner edits all of it on the playlist page itself.

**Epic**: [HOLODEX-619](https://whoiskevinrich.atlassian.net/browse/HOLODEX-619). Related:
[HOLODEX-16](https://whoiskevinrich.atlassian.net/browse/HOLODEX-16) (Playlists),
[HOLODEX-178](https://whoiskevinrich.atlassian.net/browse/HOLODEX-178) (F46),
[HOLODEX-98](https://whoiskevinrich.atlassian.net/browse/HOLODEX-98) (filterable resolved fields,
delivered by P0-6 here).

**Changes other specs** (edited in place in the same change):
- F75 [smart-playlists.md](smart-playlists.md): **P0-8 Edit filter** (browse edit mode) is replaced
  by P0-4 here, and **RD8 No cap** now holds only until the owner sets a cap (P0-2).
- F87 [query-language.md](query-language.md): P0-10's *Edit filter opens browse* is replaced by P0-4;
  the Non-Goal on provenance conditions stays, and P0-6 here adds resolved-value conditions, not
  provenance ones.

**Depends on** (all shipped): the query language (F87), smart playlists and runs (F75), playlists and
their visibility (F69), the field resolver and the owner's field mapping
([field-resolution.md](../architecture/field-resolution.md)).

**Depends on, not decided yet** (architecture, before build):
- **How resolved values are queried** (P0-6). Today the query sees only the file layer, and a
  video's resolved values exist only when it is shown. The choice of how a query reaches them belongs in
  [field-resolution.md](../architecture/field-resolution.md) or
  [browse-and-list-state.md](../architecture/browse-and-list-state.md).
- **Where plays are kept** (P0-7). Nothing records a play today.

**Design handoff**: pending (the on-page editor, the new chips and states). **Security review**:
design phase, rules SEC-1 to SEC-11 in [security-perimeter.md, "Smart playlist
rules"](../architecture/security-perimeter.md), cited by ID below. **Test plan**: pending.

---

## Problem Statement

A smart playlist is a saved query, so it can only ask what the query can. Four common playlists
can't be built today:

1. **Recently added**: videos added in the last 30 days. The query knows a video's year, not when
   it arrived.
2. **The newest 25 tagged *noir***: every smart playlist returns every match.
3. **Noir, minus what's in *Watched with friends***: a query can't refer to another playlist.
4. **Not watched yet, directed by a given person**: the director often comes only from a provider,
   which the query can't see, and nothing records what has been watched.

Editing a smart playlist also means leaving it: *Edit filter* opens browse in an edit mode, away from
the playlist's name, sort and settings.

## Goals

1. **All four playlists above can be built**, each as one smart playlist that stays correct as the
   library changes.
2. **The query sees what the owner sees.** A field condition matches the value the field list shows
   for that video, wherever it came from.
3. **Visitors never learn more than they can browse.** No new condition, cap or reference lets a
   visitor see an owner-only value, a private playlist or the owner's watch history.
4. **A playlist is edited where it lives.** Its query, sort and cap change on its own page, with a
   live preview, and nothing about the query editor is new to learn.

## Non-Goals

- **Watch history for visitors, or per-person histories.** Only the owner's plays count. *Why*:
  Holodex has one owner and anonymous visitors; there is no one else to record.
- **Resume where I left off.** Recording plays doesn't add a resume position. *Why*: separate feature,
  separate UI.
- **Provenance conditions** (*this field came from provider-1*). F87's Non-Goal stands. *Why*:
  conditions here match a field's value, never its source.
- **Conditions on people or studios' own fields** (a person's birthplace). *Why*: results are videos;
  a person condition still means a credit (F87).
- **A visual query builder.** The editor is F87's query box and chips, mounted on the playlist page.
  *Why*: one editor everywhere; F87 already rejected a builder.
- **Clearing watch history.** *Why*: the owner chose not to need it (2026-11-01); a purged video's
  plays go with it.

## Concepts

- **Date condition**: `added`, `updated` or `released` compared with a date.
  - **added**: when the video entered the library.
  - **updated**: when anything about the video last changed in Holodex: a Tag, a decision, an
    enrichment, or a rescan that changed the file.
  - **released**: the video's resolved release date, from whichever source wins for that field. A
    video with no release date never matches a `released` condition, and does match its negation.
  - **Relative**: `<30d`, `<2w`, `<6mo`, `<1y` mean "within the last …"; `>30d` means "longer ago
    than …". `d` days, `w` weeks, `mo` months, `y` years (`mo`, not `m`, which is minutes in
    `duration`).
  - **Absolute**: `>2026-01-01`, `<=2025-12-31`, ranges `2026-01..2026-06`. A year or year-month
    stands for the whole period.
  - A relative date is stored as written, so a saved playlist moves with time.
- **Cap**: a playlist setting, the most videos it holds. The query decides what matches and the sort
  decides the order; the cap keeps the first *N*. It is not part of the query, so browse and links
  never carry it.
- **Playlist condition**: `playlist:<name>`. The video is in that playlist as the playlist is right
  now, cap included. For a smart playlist that means its current results. Stored by identity, like
  Tags, people and studios.
- **Field condition**: `<field>:<value>` for any video field in the owner's field mapping, matched
  against the field's **resolved** value.
  - Text: the value contains the words, ignoring case (`title:alien`).
  - Lists such as actors: one item equals the value (`actors:"Sigourney Weaver"`).
  - Dates: the date syntax above. Numbers: F87's comparisons.
  - A field whose name is already a query field (`tag`, `person`, `studio`, `category`,
    `resolution`, `year`, `duration`, `matched`, `added`, `updated`, `released`, `playlist`,
    `watched`, `plays`, `played`) keeps the built-in meaning.
- **Watch history**: the owner's plays. A play counts once the owner has watched about 90% of the
  video or playback reaches the end. Watching again counts again.
  - **watched**: `watched:yes` or `watched:no`, with the owner's manual mark taking precedence
    (P0-9).
  - **plays**: how many counted plays, with F87's comparisons (`plays:>3`).
  - **played**: when the last counted play happened, with the date syntax (`played:<7d`).
  - All three are owner-only, like `matched`.

The four playlists from the Problem Statement:

```
added:<30d                                       sort: Newest first
tag:noir                                         sort: Newest first · cap 25
tag:noir -playlist:"Watched with friends"
watched:no director:"Ridley Scott"
```

## User Stories

**Owner**
- As the owner, I want a "Recently added" playlist that rolls forward by itself so that I never edit
  it.
- As the owner, I want to cap a playlist at 25 so that "newest noir" stays short.
- As the owner, I want to exclude another playlist's videos so that I can build a list from what's
  left.
- As the owner, I want to filter on the director a provider supplied so that a video with no
  director in its file tags still matches.
- As the owner, I want "not watched yet" playlists so that I can find what I haven't seen.
- As the owner, I want to mark a video watched or unwatched by hand so that a video I saw elsewhere,
  or want to see again, is placed right.
- As the owner, I want to edit a playlist's query, sort and cap on its own page and see the result
  before saving, so that I don't lose my place.

**Visitor**
- As a visitor, I want date conditions on browse so that I can find what was added recently.

**Edge cases**
- As the owner, when a playlist I reference is deleted, I want the referencing playlist to say so
  rather than quietly change what it holds.
- As the owner, when a reference would make a loop, I want to be told which playlists form it.
- As a visitor, I never want a public playlist to show me videos chosen by a private one or by the
  owner's watch history.

## Requirements

Ordered so the smaller items can land first: P0-1 to P0-5 need no architecture decision; P0-6 and
P0-7 to P0-9 each wait on one.

### P0: must have

**P0-1 Date conditions.** `added`, `updated` and `released` accept the relative and absolute forms in
*Concepts*, in typed queries, chips, suggestions and errors, on browse and in playlists, for owner
and visitors.
- [ ] `added:<30d` on a playlist saved today returns, 31 days later, only videos added in the 30
      days before that day.
- [ ] `added:2026-01..2026-06` returns videos added from 1 January to 30 June 2026 inclusive.
- [ ] `released:<1y` excludes videos with no release date; `-released:<1y` includes them.
- [ ] `added:<30m` is an error that suggests `30mo` or `30d` (F87 P0-7).
- [ ] A chip reads in words: `added: last 30 days`, `released: 2026`.

**P0-2 Cap.** The owner can give a smart playlist a cap, or none (the default). The playlist page,
its count, Play all, Shuffle, Freeze, and anything referencing it all see the capped set.
- [ ] A playlist matching 31 videos with cap 25 shows 25 and says so ("31 match, showing 25").
- [ ] With the Random sort, each play-through takes a fresh random selection of 25 from all 31.
- [ ] Freeze keeps exactly the videos the playlist shows at that moment.
- [ ] A visitor sees the cap applied after their own visibility rules, so it never pulls in a video
      the visitor couldn't browse.
- [ ] Snapshot playlists have no cap setting.
- [ ] A cap is a whole number from 1 to 10,000, or none (SEC-7).

**P0-3 Playlist conditions.** `playlist:` and `-playlist:` name any playlist, smart or snapshot.
- [ ] `tag:noir -playlist:"Watched with friends"` returns noir videos not in that playlist.
- [ ] Saving a definition that makes a loop, directly or through other playlists, is refused with
      the playlists in the loop named. A loop that appears later through someone else's edit marks
      the playlist stale (SEC-6).
- [ ] References reach at most 4 deep, 16 distinct playlists and 256 conditions in all (SEC-5).
      Past a limit, saving is refused with the limit named (as F87 P0-9); a playlist pushed past it
      by another playlist's edit is stale.
- [ ] A smart playlist that has both the Random sort and a cap has no fixed contents, so a reference
      to it is refused with that reason.
- [ ] Deleting a referenced playlist flags every referencing playlist as stale (F75 RD6): nothing
      runs until the reference is fixed, and the query is never quietly widened. The delete
      confirmation names the playlists that reference it.
- [ ] Renaming a referenced playlist changes how the condition reads, not what it matches.

**P0-4 Edit the definition on the playlist page.** *Edit filter* opens an editor on the playlist page
itself, made of F87's query box with suggestions, its chips, the browse Filters facets, Sort, and Cap.
A live count and grid preview follow every change. **Save definition** applies the changes and
**Cancel** discards them. This replaces F75 P0-8's trip to browse.
- [ ] Every query F87 can express, and every condition this spec adds, can be built and saved here.
- [ ] Leaving the page with unsaved changes asks first.
- [ ] Saving a definition the playlist's visibility doesn't allow (P0-5) is refused with the
      offending condition named, and the editor stays open.
- [ ] A stale playlist's notice opens this editor with the stale condition marked.
- [ ] Browse no longer has a playlist edit mode; *Save as playlist…* from browse is unchanged.
- [ ] Saving goes through the same owner-only save as today; the editor adds no new way to change a
      playlist, and every save re-checks the whole definition (SEC-11).

**P0-5 Visitors and public playlists.** A visitor sees a public smart playlist's name, count and
sort, and that it updates automatically, but never its definition. Outside the editor, the owner
sees the definition as read-only chips. A public playlist may use only what a visitor may.
- [ ] A visitor sees no chips, query text or condition names for a public smart playlist.
- [ ] The owner sees the definition as chips without edit controls until opening the editor.
- [ ] Making a playlist public, or saving a public one, is refused while it uses an owner-only
      condition (`matched`, `watched`, `plays`, `played`) or references a private playlist, or a
      playlist that itself uses one, at any depth (SEC-2, SEC-11). The message names the condition.
- [ ] Making a playlist private always succeeds. A public playlist that references it becomes stale
      and shows visitors nothing until the owner fixes it (SEC-3).
- [ ] A visitor's `playlist:` condition accepts public playlists only; a private one behaves as an
      unknown name (F87 P0-8, SEC-1).

**P0-6 Field conditions on resolved values.** Any video field in the owner's mapping can be queried
by name (*Concepts*), and matches what the video's field list shows.
- [ ] `director:"Ridley Scott"` matches a video whose director comes only from provider-1, and one
      whose director comes only from its file tags.
- [ ] A video where the owner chose the file's value over provider-1's matches on the file's value
      only; the losing provider value never matches.
- [ ] Changing a field's source decision changes which playlists the video is in, the next time
      they're read.
- [ ] Suggestions after `director:` offer existing values. A field hidden from visitors is owner-only
      in queries: a visitor is never suggested it or matched on it (SEC-8). `released` follows the
      same rule.
- [ ] A field later removed from the mapping makes a saved playlist stale (F75 RD6), naming the
      field.

**P0-7 Record the owner's plays.** While signed in as the owner, a play counts when about 90% of the
video has played or playback reaches the end. Visitors' playback records nothing.
- [ ] Sampling the first minutes, or skipping straight to the end, does not count. Watching
      through and seeking within the last 10% does.
- [ ] Watching twice counts two plays.
- [ ] Plays are kept for as long as the video exists, and removed when it is purged (SEC-10).
- [ ] Only the owner can record a play; a visitor's request to record one is refused (SEC-9).

**P0-8 Watch conditions.** `watched`, `plays` and `played` work as in *Concepts*, for the owner only.
- [ ] `watched:no director:"Ridley Scott"` returns that director's videos with no counted play and
      no manual watched mark.
- [ ] `played:<7d` returns videos last played in the past 7 days.
- [ ] A visitor query or link containing any of the three is refused as owner-only (F87 P0-8,
      SEC-4); nothing runs. Any later sort by plays is owner-only too.

**P0-9 Watched mark and manual toggle.** The owner sees which videos are watched, and can set it.
- [ ] An owner-only watched mark shows on video cards and the media page; visitors never see it.
- [ ] On the media page the owner can mark a video watched or unwatched. The mark decides
      `watched:` until the next counted play, which clears it. Marking adds or removes no plays.
- [ ] A smart playlist using `watched:no` drops a video on its next read after it is marked watched.

### P1: nice to have

- **P1-1 Cap in the save form.** *Save as playlist…* can set the cap when saving as smart.
- **P1-2 "Used by" on a playlist.** The playlist page lists the playlists that reference it.
- **P1-3 Syntax help** (F87 P1-1) covers the new conditions and the four playlists above.

### P2: future (design so these stay possible)

- **P2-1 Resume position** from the same play record, without changing what counts as a play.
- **P2-2 Natural-language queries** (F87 P2-1) produce every condition here, including relative dates.
- **P2-3 Person and studio field conditions**, once a query can return people or studios.

## Success Metrics

A personal library has no analytics, so success is checked, not measured:

- **Leading (at launch):** the four playlists from the Problem Statement return the right videos;
  every existing smart playlist returns the same set before and after; no visitor test reaches an
  owner-only condition, a private playlist or watch history (P0-5, P0-8).
- **Lagging (a month on):** the owner keeps a "Recently added" and a "not watched" playlist and
  edits playlists without going to browse. If the owner still rebuilds playlists from browse, the
  editor (P0-4) is missing something.

## Open Questions

None open for the owner. Before build: the two architecture choices under *Depends on*.

## Decisions

- **RD1 Dates take relative and absolute forms** (2026-11-01). *Why*: rolling playlists need
  relative dates; fixed ranges cost little more. `mo` for months avoids `duration`'s `m`.
  *Rejected*: relative only.
- **RD2 `updated` is the last change in Holodex** (2026-11-01). *Why*: what the owner means by
  "updated" is "something about it changed here". *Rejected*: the file's modified time.
- **RD3 The cap is a playlist setting, not a query term** (2026-11-01). *Why*: the query stays pure
  filtering, so browse, links and tools never meet a cap. *Rejected*: `limit:` in the query.
- **RD4 Cap with Random picks afresh each play-through** (2026-11-01). *Why*: matches how Random
  already reseeds per pass (F69 RD7). *Rejected*: fixed until rerolled.
- **RD5 Any playlist can be referenced, with a loop check and a depth limit** (2026-11-01). *Why*:
  smart-on-smart is the useful case. *Rejected*: snapshot playlists only.
- **RD6 A public playlist can't reference a private one; a referenced playlist going private makes
  its public referrers stale** (2026-11-01). *Why*: a visitor would otherwise learn about a private
  playlist's contents through the results. Going private stays one click, and the cost lands on the
  public playlist that depends on it (SEC-3). *Rejected*: allow and hide the name; refusing to make a
  referenced playlist private.
- **RD7 Fields are queried by their own name; built-ins win a clash** (2026-11-01). *Why*: reads
  naturally in text and chips. *Rejected*: a `field:` prefix.
- **RD8 Only the owner's plays count; a play is ~90% or the end; kept forever** (2026-11-01).
  *Why*: one owner, so "watched" means the owner watched it. *Rejected*: anonymous all-visitor
  counts; any play over N seconds; manual marks only; a rolling retention window.
- **RD9 Watched mark and manual toggle, no clear-history** (2026-11-01). *Rejected*: clearing
  history.
- **RD10 Visitors never see a smart playlist's definition** (2026-11-01, confirmed with the design
  handoff). *Why*: the name and contents say enough, and nothing about how a list was built can
  reach a visitor. *Rejected*: read-only chips for visitors.
- **RD11 Edit on the playlist page** (2026-11-01). *Why*: the playlist's sort, cap and settings live
  there. *Rejected*: extending browse's edit mode (F75 P0-8).

## Timeline Considerations

- **No hard deadline.** All P0, one epic (owner, 2026-11-01).
- **Gates before build:** architecture (resolved-value querying; play storage), design handoff (the
  editor, chips and states at 375px), testing strategy. The design-phase security review is done
  (SEC-1 to SEC-11); re-review at build.
- **Suggested slicing within the epic:** (1) dates, cap and playlist references with the visibility
  rules (P0-1 to P0-3, P0-5); (2) the on-page editor (P0-4); (3) resolved-field conditions (P0-6);
  (4) plays, watch conditions and the watched mark (P0-7 to P0-9). Slices 3 and 4 each start once
  their architecture choice is made.
