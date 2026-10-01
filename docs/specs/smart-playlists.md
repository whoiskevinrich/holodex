# Spec: Smart playlists and Play all — any query-backed video grid becomes a live playlist or a run (F75)

**Status**: Draft
**Phase**: Phase 3 (presentation / library organisation) — extends the F69 playlist container; touches
the resolver / enrichment / writeback seams **not at all**
**Owner**: Project owner
**Date**: 2026-09-30
**Feature block**: **F75** — every video grid whose cards come from a `/media` query (browse, person,
tag, studio, film) gets two grid-header actions: **Play all** plays the grid's current result set in
order through the F69 next-up player without saving anything, and **Save as smart playlist** stores the
grid's **query** (not its ids) as a playlist that re-runs whenever it is opened. **Shuffle** sits in
*Play all*'s split-button menu and plays the same set in a fresh random order; a run can be shuffled or un-shuffled
mid-play, can repeat endlessly, and a playlist can be set to always play shuffled. The stored query **is**
the canonical `filters.ts` query string, so whatever the browse filters can express — today's facets,
and later AND/NOT ([HOLODEX-181](https://whoiskevinrich.atlassian.net/browse/HOLODEX-181)) — smart
playlists can express with no playlist change.

**Epic**: [HOLODEX-16](https://whoiskevinrich.atlassian.net/browse/HOLODEX-16) (Playlists) ·
stories [HOLODEX-58](https://whoiskevinrich.atlassian.net/browse/HOLODEX-58) (smart playlists) ·
[HOLODEX-500](https://whoiskevinrich.atlassian.net/browse/HOLODEX-500) (Play all) · prerequisite
[HOLODEX-501](https://whoiskevinrich.atlassian.net/browse/HOLODEX-501) (entity grids through `/media`,
no 500 cap)

**Design**: [smart-playlists-handoff.md](../design/smart-playlists-handoff.md) +
[mockup](../design/smart-playlists-mockup.svg) — approved 2026-09-30: one *Save as playlist…* with a
Smart | Snapshot toggle, a `smart` chip, Play all / Shuffle as one split button.

**Supersedes**: F69 [video-playlists.md](video-playlists.md) **P2-2** (*Live playlists — a nullable
`frozen_query` + Refresh from filter*). F69's Non-Goal "Live / refreshable playlists" stays true *of
F69*; this spec is the follow-up it anticipated, with one change of shape: the query is **live**
(re-run on read), not refreshed by hand.

**Depends on** (all shipped unless noted):
- F69 playlists ([video-playlists.md](video-playlists.md), [ADR-104](../architecture/ADR-104-video-playlists-container-and-persistent-player.md)) —
  the container, `visibility` (RD4: private ⇒ 404), seeded `random` per play-through (RD7), the
  `?playlist=` next-up context (RD6) and the persistent `<video>` element (RD5 / ADR-104 D4).
- Browse filters as a URL ([`web/src/lib/filters.ts`](../../web/src/lib/filters.ts)) and the server
  parse `videoFilterFromQuery` (`internal/api/handlers.go`). Entity facets are already **numeric ids**
  (`person`, `tag`, `studio_id`), so a stored query survives renames by construction.
- The list-state model ([ADR-114](../architecture/ADR-114-list-state-model.md)) — filters live in the URL,
  the random seed lives in the session.
- **HOLODEX-501 (not shipped, blocking):** person / tag / studio detail grids load through
  `GET /media?person=|tag=|studio_id=` with paging. Today they use bespoke endpoints capped at 500.

**Related, not depended on**: HOLODEX-181 / HOLODEX-178 (rule-based queries over the F46 substrate) —
the AND/NOT phase. Saved / named views (rejected in [list-toolbar.md](list-toolbar.md) as "bookmark the
URL") — a smart playlist is a saved view that can play, and does not reopen that decision.

---

## Problem Statement

The owner's most common "watch a run" moment starts on an entity page — *everything with this person,
this studio, this tag* — and today that page is a dead end: each card links to a bare `/media/{id}`,
the player stops at the end of every item, and the only way to get next-up playback is to go to browse,
re-build the same filter by hand, *Save as playlist*, and press *Play all*. That snapshot is then
**stale the moment a new video is tagged**, so a "playlist of everything by X" has to be rebuilt to stay
true. Separately, the entity grids silently truncate at 500 — one real tag already exceeds it — so the
page under-reports the very set the owner wants to watch.

## Goals

1. **Any query-backed grid plays hands-off in one action.** From browse, a person, tag, studio or film
   page, one press of *Play all* plays the grid's whole result set in its current order, through the
   F69 next-up player, with no further interaction.
2. **"Everything matching X" stays true without upkeep.** A smart playlist saved from a grid shows a
   newly matching video the next time it is opened, with no refresh action.
3. **What you see is what plays and what saves.** The set played or saved equals the grid's set exactly
   — same filters, same title-box text, same sort, **no cap**.
4. **Rules arrive free.** When browse filters gain new operators (AND/NOT, HOLODEX-181), smart
   playlists can store them with zero playlist-side schema or code change.
5. **Random playback without re-sorting.** Any set that can be played can be played shuffled in one
   action, without changing how the grid is sorted, and a shuffled run can go on indefinitely.
6. **Zero footprint on the file layer and the metadata model** (as F69): no writeback, no extraction,
   no resolver branch.

## Non-Goals

- **A rule editor / query builder.** The only editor is the browse page itself (*Edit filter*, P0-8).
  *Why*: the rule language belongs to filters (Goal 4); a second editor would fork it.
- **AND/NOT and cross-entity rules.** *Why*: filter work, tracked as HOLODEX-181 / 178; this spec only
  guarantees they will be storable.
- **Smart playlists with exceptions** (pinned or excluded videos on top of a query, iTunes-style).
  *Why*: two sources of membership is a different model; *Freeze* (P0-7) covers "I want to hand-edit
  this now". Revisit only if Freeze-then-edit proves to be a common path.
- **Play all / Save on search results and on algorithmic shelves** (Related, Recently Added). *Why*:
  search runs on `repo.Search`, not a `MediaFilters` query; shelves are chosen by algorithm or recency,
  and "recently added" as a live rule is a trap. The rule is *the action exists only where a query
  does*.
- **Per-card playlist actions from the grid.** *Why*: a card is one video; the collection action
  belongs to the collection. F69's *Add to playlist* stays on the detail page.
- **Film playlists** (F69 owner decision stands).

## Resolved Decisions

- **RD1 — One primitive: the grid's list query.** A grid is *query-backed* when its cards come from a
  `MediaFilters` query run through `/media` (or the film-videos route, which reuses the same parser).
  Play all and Save as smart playlist read **that** query; neither knows which page it is on.
- **RD2 — The stored query is the canonical `/media` query string**, server-canonicalised (ADR-121
  D2): unknown keys rejected, re-serialised in a stable key order, with fetch mechanics (`limit`,
  `offset`, `seed`) and `sort` stripped — sort lives only in `playlists.sort`. It carries a `query_version` (starts at `1`) so a future change to the filter
  grammar can migrate stored queries rather than reinterpret them.
- **RD3 — Live means re-run on read.** A smart playlist has no `playlist_videos` rows. Every read
  evaluates the stored query with the same `ListVideos` call and the same `HideFullFilmVideos` posture as
  browse, unpaged for the id list and paged for the tile list.
- **RD4 — Sort.** The playlist's `sort` is taken from the grid's sort at save time and stays editable
  afterwards, over `MEDIA_SORTS` only — **`manual` is refused** for a smart playlist (there is no
  `position` to honour). `random` mints a fresh seed per play-through (F69 RD7).
- **RD5 — Play all fixes the id list at the press.** Pressing *Play all* evaluates the query once and
  holds the resulting ordered ids for the run. A set that changes mid-watch (a video tagged in another
  tab) does not reorder or extend the run; the next press picks it up. Same rule for *Play all* on a
  smart playlist.
- **RD6 — Merged entities are followed; deleted ones are flagged.** When a person / studio (and tag, if
  and when it gains merge) referenced by a stored query is merged, the merge rewrites the id in every
  stored query to the survivor in the same transaction. When a referenced entity is deleted, or a stored
  mapped-field key no longer exists, nothing is rewritten or dropped: the playlist reports the stale
  reference and its page shows a notice with *Edit filter* and *Delete*. A query is never silently
  broadened.
- **RD7 — The title filter is part of the query.** On entity pages the title filter (the nav search
  box filtering in place, `navSearch.inPlace`) folds into the query's `q`, so the grid is filtered server-side and Play all / Save see exactly the visible set.
- **RD8 — No cap.** Neither Play all nor a smart playlist truncates. The entity grids lose their 500 cap
  in HOLODEX-501 (paged, like browse).
- **RD9 — Visibility is F69's.** `private` (default) ⇒ a visitor gets 404; `public` ⇒ the query is
  evaluated under the **visitor's** posture, exactly as `/media` would evaluate it for that visitor.
- **RD10 — Shuffle is a playback mode, not a sort.** A run has a *mode* (`in order` | `shuffled`)
  separate from the grid's sort. *Shuffle* fetches the same id list as *Play all* and permutes it on
  the client under a seed minted at the press (ADR-121 D7); the grid on screen keeps its sort. Shuffle appears
  wherever *Play all* does — query-backed grids **and** F69 playlist pages. A playlist whose `sort` is
  already `random` behaves as today (F69 RD7); its *Play all* and *Shuffle* are the same action.
- **RD11 — Toggling shuffle mid-run reorders only what hasn't played.** Played items stay as history
  (*Previous* walks them in the order they actually played). Toggling on shuffles the remaining items
  under a new seed; toggling off restores the remaining items to the run's source order. The current
  item never changes on a toggle.
- **RD12 — Repeat starts a new pass; it never loops a stale list.** With repeat on, the end of a run
  starts a new pass: the query is re-evaluated (so a smart playlist or grid picks up new videos), and a
  shuffled run gets a new seed. A new pass never starts with the item that just finished. Repeat off is
  the default; F69's "stop at the last item" stays the default behaviour.
- **RD13 — "Always shuffle" is a playlist property.** A playlist (smart or F69) can be set to play
  shuffled: its primary play action becomes *Shuffle*, and entering a run from one of its tiles starts
  shuffled. Its page still displays in its `sort`. The owner sets it; visitors inherit it.

## User Stories

**Owner, watching**
- As the owner on a person page, I want to press *Play all* so that every video with that person plays
  in order without me choosing each next one.
- As the owner on browse with filters set, I want *Play all* to play exactly the filtered set so that I
  don't have to save a playlist just to watch a run once.
- As the owner, I want a run to keep its order even if the library changes mid-watch so that *Next* and
  *Previous* stay predictable.
- As the owner on a studio page sorted by date, I want to press *Shuffle* so that I get a random run
  without losing the page's date order.
- As the owner halfway through an in-order run, I want to switch to shuffle (and back) so that I can
  change my mind without restarting.
- As the owner, I want a shuffled run to keep going when it ends so that I can leave it playing like a
  channel.
- As the owner, I want a "random favourites" playlist to always play shuffled so that I don't have to
  remember to press *Shuffle*.

**Owner, curating**
- As the owner on a tag page, I want to *Save as smart playlist* so that the playlist always contains
  everything with that tag, including videos tagged next month.
- As the owner, I want to open a smart playlist's filter in browse and update it so that I can narrow or
  widen it without rebuilding it.
- As the owner, I want to *Freeze* a smart playlist into a normal one so that I can hand-order it or
  hand a visitor a set that stays put.
- As the owner, I want to see when a smart playlist refers to something I deleted so that a playlist
  never quietly turns into something else.

**Visitor**
- As a visitor given a public smart playlist, I want to play it so that I can watch the curated run, and
  see only what I could see on browse anyway.

**Edge cases**
- A query matching nothing: the smart playlist page shows the empty state; *Play all* is disabled.
- A merged person: the playlist keeps working, now pointing at the survivor.
- A deleted tag: the playlist shows the stale-reference notice.
- A grid with no query behind it (search, shelves): no actions shown.

## Requirements

### Must-Have (P0)

**P0-1 — Entity grids are query-backed (HOLODEX-501).** Person, tag and studio detail grids load
through `GET /media` with the entity facet and paging, matching browse; the bespoke 500-capped
endpoints stop feeding the grid. The title box sends `q` (RD7).
- [ ] A tag with > 500 videos shows all of them across pages; `total` equals `/media?tag=<id>`'s.
- [ ] Same set and order as before for ≤ 500 (the owner's parity check, now a test).
- [ ] Typing in the title box narrows the grid server-side and updates the query the actions read.

**P0-2 — Grid-header actions where a query exists.** Browse, person, tag, studio and film grids show
a count line with the *Play all* split button (everyone who can see the grid) and *Save as playlist…*
(owner only). Browse's ⋯ *Save as playlist…* entry moves here. Search and shelves show neither.
- [ ] Visitor sees *Play all*, never *Save as playlist…*.
- [ ] *Play all* is disabled when the result set is empty.
- [ ] No action on `routes/search`, `RelatedShelf`, `RecentlyAddedShelf`.

**P0-3 — Play all.** Evaluates the grid's query (RD5), navigates to the first item with a run context
on `/media/{id}`, and plays through via the F69 next-up surface (Next / Previous / ended ⇒ next). In-app
press autoplays; a deep link or reload renders the context but does not autoplay (F69 RD6). `random`
gets a seed (F69 RD7).
- [ ] A 3+ item person grid plays to the last item with zero clicks after the first.
- [ ] Tagging a new matching video mid-run does not change the run's order or length.
- [ ] PiP opened on item 1 is still open on item 2 (F69 P0-9 element, unchanged).

**P0-4 — Save as smart playlist.** One *Save as playlist…* form with a **Smart | Snapshot** toggle,
Smart by default; Snapshot is F69's `from_query`. For Smart, the owner names it; the server canonicalises the grid's query (RD2) and
stores it with `sort` from the grid (RD4) and `visibility = private`. Toast reports the current match
count with a link.
- [ ] Saved query round-trips: opening *Edit filter* reproduces the same grid.
- [ ] `limit` / `offset` / `seed` never appear in the stored query.
- [ ] A query from a page with a title-box value stores it as `q`.

**P0-5 — Live reads.** `GET /playlists/{id}` for a smart playlist evaluates the query (RD3); tile list
paged, ordered id list (for next-up) unpaged. The playlists list marks smart playlists distinctly.
- [ ] A video tagged after saving appears on the next open, with no refresh action.
- [ ] Trashed videos are excluded (as browse).
- [ ] `sort = manual` on a smart playlist is refused with 400.

**P0-6 — Smart playlists take no manual membership.** *Add to playlist* (F69, detail page) does not
offer smart playlists as targets; the membership endpoints refuse a smart playlist id.
- [ ] Adding to a smart playlist via the API returns 400; the picker does not list it.

**P0-7 — Freeze.** Owner action that evaluates the query once, writes the ids into `playlist_videos`
with `position` = rank, and clears the query — the playlist becomes an F69 snapshot playlist.
- [ ] After Freeze, a newly matching video does **not** appear; `manual` sort becomes available.

**P0-8 — Edit filter.** From a smart playlist, *Edit filter* opens browse with the stored query loaded
and an *Update ‹name›* action in place of *Save as playlist…*; updating replaces the stored query
(re-canonicalised).
- [ ] Update changes membership on next read; cancelling leaves the playlist unchanged.

**P0-9 — Merge follows, delete flags (RD6).** Every entity merge path (person, studio) rewrites stored
query ids to the survivor in the merge transaction. A read whose query references a missing entity or
an unknown mapped-field key returns `stale_refs`; the page shows a notice with *Edit filter* / *Delete*.
- [ ] Merge person 12 → 40: the stored query now names 40; membership equals `/media?person=40`.
- [ ] Delete a referenced tag: the playlist reports it; nothing is rewritten; no silent broadening.

**P0-10 — Visibility (RD9).** Private ⇒ visitor 404. Public ⇒ evaluated with the visitor's posture.
- [ ] Handler test: visitor cannot open a private smart playlist (404, not 403).
- [ ] A public smart playlist never returns a video the same visitor could not get from `/media`.

**P0-11 — Shuffle (RD10).** *Shuffle* is the second item of every *Play all* split button
(query-backed grids and F69 playlist pages); with *Always shuffle* on, the two swap places. Same autoplay rules as *Play all*; the run's seed is held with the run, not put in the
grid's URL.
- [ ] *Shuffle* on a date-sorted grid plays a random order; the grid stays date-sorted.
- [ ] Two presses give two different orders (different seeds); *Previous* / *Next* within one run are
  stable.
- [ ] The run contains exactly the grid's set — shuffle never adds, drops, or repeats an item within a
  pass.

**P0-12 — Always shuffle (RD13).** An owner toggle on a playlist's page (smart or F69). When on, the
primary play action is *Shuffle* and runs entered from the playlist's tiles start shuffled.
- [ ] Visitor on a public always-shuffle playlist gets a shuffled run.
- [ ] The playlist page's display order is unchanged by the toggle.

### Nice-to-Have (P1)

- **P1-4 — Shuffle toggle mid-run (RD11)** on the next-up surface. *Acceptance:* toggling never changes
  the current item; already-played items are not replayed before the pass ends; toggling off then on
  yields a new order.
- **P1-5 — Repeat (RD12)** on the next-up surface, off by default. *Acceptance:* with repeat on, the
  last item's `ended` starts a new pass with zero clicks; a video added to the set mid-pass is in the
  next pass; the new pass never starts with the item that just ended.

- **P1-1 — Live count on the playlists list** ("~N videos") without loading tiles.
- **P1-2 — "Play all" hotkey** in the F62 map on query-backed pages.
- **P1-3 — Describe the query** in words on the smart playlist page ("Tag *noir* · 2010–2019 · newest
  first"), reusing the active-filter chips from the list toolbar.

### Future Considerations (P2)

- **P2-1 — AND/NOT** (HOLODEX-181). Lands in the filter grammar; smart playlists store it as-is. Bump
  `query_version` only if an existing key's meaning changes.
- **P2-2 — Exceptions** (pin / exclude on top of a query). Would need a second membership source;
  nothing here precludes adding it beside `query`.
- **P2-3 — Play all from search**, once search results can be expressed as a `MediaFilters` query.
- **P2-4 — A run continues across pages** (layout-level player, F69 P2-4).

## Behavior detail

- **Run context lifetime.** The run's ordered ids live in session storage keyed by a run id carried in
  the URL (`/media/{id}?run=<rid>`), alongside the query it came from. Reload keeps the run. If the
  stored run is gone (new tab, cleared storage), the page re-evaluates the query once from the URL and
  starts a new run from the current item — no autoplay.
- **Current item not in the run** (stale link): the next-up surface offers *Play from start*; `ended`
  does nothing (F69 behaviour).
- **Huge runs.** The id list for the whole library is a few hundred KB of ints at worst; the snapshot
  is ids only, never tile payloads.
- **Run state.** A run is `{source query or playlist, mode, seed, ordered ids, played history,
  repeat}`, all held under the run id. Shuffle, the mid-run toggle and repeat only change this state;
  none of them writes to the server.
- **Saving from a filter with no clauses** (bare browse) is allowed — "everything, newest first" is a
  legitimate smart playlist.

## Data model

One append-only migration on `playlists` (number claimed at implementation, see
`.claude/rules/migrations.md`):

```sql
ALTER TABLE playlists ADD COLUMN query TEXT;              -- NULL ⇒ F69 snapshot playlist
ALTER TABLE playlists ADD COLUMN query_version INTEGER;   -- NULL iff query IS NULL
ALTER TABLE playlists ADD COLUMN play_shuffled INTEGER NOT NULL DEFAULT 0;  -- RD13, any playlist
```

A playlist is **smart iff `query IS NOT NULL`**; a smart playlist has no `playlist_videos` rows (P0-6).
Freeze writes rows and nulls both columns. The ADR decides whether merge rewriting parses the stored
string or a normalised side table of referenced ids.

## API

Base `/api/v1`; envelope as F69. Mutations under `requireOwner`; reads visibility-gated.

- `POST /playlists {name, query}` — creates a smart playlist (F69's `{from_query}` keeps its snapshot
  meaning).
- `GET /playlists/{id}` — adds `query`, `stale_refs`, and for smart playlists evaluates live.
- `PATCH /playlists/{id} {query}` — Edit filter's *Update*; `{play_shuffled}` — always shuffle.
- `POST /playlists/{id}/freeze`.
- `GET /media/ids?<filter>&sort=` — the ordered id list for a grid run, gated like `GET /media`
  (ADR-121 D7).

## UI

Two affordances and three changes:
1. **Grid-header actions** — the *Play all ▾* split button (Shuffle in its menu) and *Save as
   playlist…* (Smart | Snapshot) on query-backed grids; the split button on F69 playlist pages.
2. **Run context on `/media/[id]`** — the F69 next-up surface, labelled with the run's source
   ("Person · ‹name›") instead of a playlist name, plus the shuffle and repeat toggles (P1-4, P1-5).
3. **Smart playlist page** — F69 detail page plus a smart marker, *Edit filter*, *Freeze*, the
   stale-reference notice, and the *Always shuffle* toggle (also on F69 playlist pages).
4. **Playlists list** — smart marker.
5. **Browse in edit mode** — *Update ‹name›* replacing *Save*.

The design handoff fixes placement and treatment (Cinémathèque only).

## Success Metrics

Single-owner app, no analytics — **verification outcomes**:

*Leading (at ship):*
- *Play all* from a person, tag, studio, film and filtered browse page each reach the last item with
  zero clicks after the first.
- The > 500 tag shows its full count on its page, in its Play all run, and in a smart playlist saved
  from it — all three equal `/media?tag=<id>`'s `total`.
- A video tagged after saving appears in the smart playlist on next open.
- *Shuffle* on a 20-item grid, run twice, gives two different orders, each containing all 20 once.
- With repeat on, a 3-item shuffled run plays 7 items with zero clicks and no back-to-back repeat.
- Merge and delete cases (P0-9) pass as handler tests.

*Lagging (first month):*
- The owner starts at least one run from an entity page rather than from `/playlists`.
- At least one smart playlist is reopened after the library changed. If every smart playlist gets
  Frozen, live membership was the wrong default.

## Open Questions

1. ~~**[security, blocking for P0-10]** Can a stored query filter on owner-only facets (e.g.
   `missing_facet`) such that a public smart playlist leaks owner-only knowledge to a visitor?~~
   **Resolved in the model by ADR-121 D4:** yes, so a query or sort using owner-only inputs can't be
   public (400), and the read path re-checks. `/security-review` still signs off.
2. **[engineering, non-blocking]** Is `q` a title-only match or broader? If broader, the entity title
   box adopts `q`'s semantics (RD7 still holds — the grid and the query stay identical).
3. **[engineering, non-blocking]** Does tag merge exist? If a tag merge path is added later, it must
   join P0-9's rewrite.
4. **[engineering, blocking for S2]** Migration number — claim at implementation.

## Timeline / routing

Three stories under HOLODEX-16, in order:

1. **HOLODEX-501 — entity grids via `/media`** (P0-1). Landable alone; removes the cap.
2. **HOLODEX-500 — Play all + Shuffle** (P0-2 *Play all* half, P0-3, P0-11, P1-4, P1-5). Needs only
   P0-1 and the run context.
3. **HOLODEX-58 — smart playlists** (P0-2 *Save* half, P0-4..P0-10, P0-12). Needs the ADR and
   migration.

Gates: **spec** (this document) · **ADR**
([ADR-121](../architecture/ADR-121-smart-playlists-stored-query-and-runs.md): stored-query format and
versioning, live evaluation, merge rewriting, runs — supersedes ADR-104 D3's "no `frozen_query`") · **design handoff** (grid-header actions,
run context label, smart playlist page, browse edit mode; SVG committed) · **testing strategy** ·
**security review** (visitor evaluation of a public live query, OQ1).
