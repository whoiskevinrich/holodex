# Spec: Video playlists — a container of videos with a sort, two producers, next-up playback (F69)

**Status**: Draft
**Phase**: Phase 3 (presentation / library organisation) — a new page and store, no new subsystem;
touches the resolver / enrichment / writeback seams **not at all**
**Owner**: Project owner
**Date**: 2026-09-20
**Feature block**: **F69** — a **playlist** is an owner-made, Holodex-only container of videos with a
**sort**. It is populated two ways in v1 — *Save as playlist* from the browse page (a **snapshot** of
the current result set) and *Add to playlist* from a video's detail page — is **private by default**,
and plays through: when one item ends, the player continues to the next in the playlist's order on
the **same `<video>` element**, so Picture-in-Picture survives the hand-off (Plex behaviour).
**Film playlists are out of scope.**

**Epic**: [HOLODEX-438](https://whoiskevinrich.atlassian.net/browse/HOLODEX-438) ·
stories proposed under *Timeline / routing* (S1 store + API · S2 pages + producers · S3 next-up)

**Depends on** (all shipped):
- Browse filters as a shareable URL (F4.7, [`web/src/lib/filters.ts`](../../web/src/lib/filters.ts)) and
  the server-side parse `videoFilterFromQuery` in `internal/api/handlers.go` — *Save as playlist* reuses
  both: the client sends the filter query string it already serialises, the server runs the same
  `ListVideos` it runs for browse, unpaged.
- The media sort vocabulary (F12.1, `MEDIA_SORTS`, seeded `random` per [ADR-045](../architecture/ADR-045-seeded-random-ordering.md)) —
  a playlist's `sort` **is** one of these values, plus `manual`.
- Owner mode (F29, `requireOwner`) — every mutation in this spec is owner-gated; the visitor/owner
  distinction is what `visibility` gates against.
- The entity reference handle ([ADR-096](../architecture/ADR-096-entity-identity-card.md) D1,
  `internal/model/ref.go`) — playlists mint `playlist:<id>` and accept it wherever an id is.
- The media detail page's reuse across `/media/[id]` navigations (SvelteKit keeps the component; the
  page's own load effect resets per-item state) — next-up rides that reuse and **fixes** the one thing
  it gets wrong today (P0-9).

**Related, not depended on**: tags (F5) — a tag is also a hand-built set of videos, but it is
**file metadata** (written back, lowercase by policy, extraction-queued). A playlist is a **library
fact**: never written to a file, freely named, ordered, private. The two coexist; nothing migrates.
Hotkeys (F62) — next/previous-in-playlist keys are a P1 here, wired into F62's map, not a new one.

**ADR**: [ADR-104](../architecture/ADR-104-video-playlists-container-and-persistent-player.md) — D1 container,
not entity · D2 playlist = membership + sort · D3 snapshot-only producers, server-side through the
extracted clause builder · D4 persistent player element · D5 visibility as a read gate.
**Design**: [video-playlists-handoff.md](../design/video-playlists-handoff.md) +
[mockup](../design/video-playlists-mockup.svg) — approved 2026-09-20; OQ3 → option A (toolbar).

**Spike (2026-09-20, HOLODEX-438 comment)**: the media page currently **destroys and recreates its
`<video>` on every item change** (the page-wide `{#if loading}` gate wraps the player). PiP is bound to
the element, so next-up must be a `src` swap on a persistent element — that finding is P0-9 below and
ADR-104 D4. The in-app (Electron) browser is autoplay-permissive and is **not** evidence for autoplay
policy; Chrome/Firefox are expected to pass on sticky per-document activation; **Safari is unverified**
(OQ2).

---

## Problem Statement

The owner has three recurring moments the library cannot serve today: *sit down and watch a run of
videos hands-off*, *build a named shelf over time and come back to it*, and *hand a visitor a curated set
without exposing the whole browse surface*. The only grouping primitive is a **tag**, which is the wrong
tool for all three: it is file metadata (every "playlist" would be written into the files and pass
through the extraction queue), it is a set with no order, it cannot be private, and the player stops
dead at the end of every item. The workaround is re-building the same filter by hand and clicking play
on each tile, which is exactly the friction a media library exists to remove.

## Goals

1. **A run of videos plays hands-off.** From a playlist, one press of *Play all* plays every item in
   the playlist's order with no further interaction; PiP started on item *n* is still open on item *n+1*.
2. **Building a playlist costs one action per intent.** The browse page's current result set becomes
   a playlist in one action; a video joins a playlist from its detail page in one action.
3. **Playlists are private until the owner says otherwise.** A visitor never sees, lists, or can
   guess the existence of a private playlist; a public playlist is a shareable page.
4. **Zero footprint on the file layer and the metadata model.** No writeback, no extraction, no new
   field, no resolver branch, no enrichment. Deleting a playlist touches no video.

## Non-Goals

- **Reorder UI.** `position` exists and `manual` sort honours it, but v1 has no drag handle or
  move-up/down — insertion order is the manual order. *Why*: the brainstorm ranked order below
  continuity and privacy; shipping the column without the UI keeps the model honest at no UX cost.
- **Tile context menus / multi-select "add".** Only the detail page adds. *Why*: the browse page
  already has a producer (*Save as playlist*); a second one there is surface without a second job.
- **Cover image.** An `entityimage` kind is reserved by ADR-104 D1, not shipped. *Why*: "curate a shelf"
  wants a face eventually; nothing in v1 needs it.
- **Live / refreshable playlists.** *Save as playlist* is a snapshot; no `frozen_query` column, no
  *Refresh* button. *Why*: "hand someone a set" needs the set to stay put; the column is one nullable
  migration away when *Refresh* is a real feature, and migrations are append-only anyway.
- **Film playlists.** Owner decision at scoping.
- **Nav-search inclusion, "appears in N playlists" chips on entity pages, MCP tools beyond what the
  SPA calls.** Later, if wanted; none of them changes the model.
- **Layout-level / mini player** (a playback singleton in `+layout` that survives leaving `/media/*`).
  *Why*: the owner's PiP use is "switch to another app", which P0-9's persistent element already covers;
  a hoisted player is a later move, not a rewrite, because the element-persistence constraint is the same.
- **Anything a tag already does better** — file-visible grouping, writeback, provider adoption.

## Resolved Decisions

- **RD1 — Container, not entity.** A playlist has no `BaselineSource`, no per-field decisions, no
  aliases, no completeness score, no enrichment. It borrows the page conventions (a `playlist:<id>` ref,
  a list page, a detail page, `PosterTile` rows) and none of the ADR-061/096 spine. Rationale: the spine
  resolves identity under *multiple sources*; a playlist has one source, the owner.
- **RD2 — Playlist = membership + a sort.** `playlist_videos` is a **set** (PK `(playlist_id,
  video_id)`, no duplicates) with a `position`; `playlists.sort` is any `MEDIA_SORTS` value or `manual`.
  Read order = the sort applied to the membership; `manual` orders by `position`. A snapshot playlist and
  a curated one are indistinguishable at read time.
- **RD3 — Snapshot, server-side.** *Save as playlist* posts the browse filter's query string; the server
  parses it with the same `videoFilterFromQuery`, runs `ListVideos` **unpaged** with the same
  `HideFullFilmVideos` posture as browse, and inserts every id with `position` = rank in that ordering.
  `sort` is set to the filter's sort (default `added_desc`). The client's loaded page is irrelevant.
- **RD4 — Visibility is a playlist property, two values.** `private` (default) · `public`. A visitor
  request for a private playlist is a **404**, never a 403 — existence must not leak. Per-video
  visitor/owner field gating is unchanged and still applies to every tile and page a playlist shows.
- **RD5 — Next-up is a `src` swap on a persistent `<video>`.** The element in `/media/[id]` moves
  above the page's loading gate and is never unmounted while the route stays within `/media/*`. On
  `ended` inside a playlist context, the page navigates to the next item and calls `play()` once the
  loaded `video.id` equals the route id. PiP and the browser's sticky user activation both ride the
  same element / same document.
- **RD6 — Playlist context is in the URL; autoplay intent is not.** `/media/{id}?playlist={pid}` (plus
  `&seed=` when the playlist sort is `random`) is a shareable deep link that renders the next-up surface.
  The *play immediately* intent is in-memory state set by the `ended` handler, so reloading or sharing a
  deep link never autoplays.
- **RD7 — A `random` playlist is shuffled once per play-through.** The client picks a seed when
  *Play all* is pressed (or when a playlist context is entered without one), asks the server for the
  order under that seed, and carries the seed in the URL so next/previous are stable for the session.
- **RD8 — Trash and delete cascade.** `playlist_videos.video_id` is `ON DELETE CASCADE`; a trashed
  video is excluded from playlist reads the same way browse excludes it, and returns on restore.
  Deleting a playlist deletes its rows and nothing else.
- **RD9 — Adding is idempotent.** Adding a video already in the playlist is a no-op that reports the
  existing membership; the UI shows the member state rather than an error.

## User Stories

**Owner — queue a session**
1. As the owner, I want to press *Play all* on a playlist and have every item play in order without
   touching anything, so an evening's viewing is one action.
2. As the owner, I want a PiP window I opened on one item to keep showing the next item, so I can
   switch to another app and let the playlist run.
3. As the owner, I want a shuffled playlist to stay shuffled the same way for the whole run, so
   *previous* goes back to what actually played.

**Owner — curate a shelf**
4. As the owner, I want to add the video I am looking at to a playlist — or to a new one I name on the
   spot — so a shelf grows as I browse.
5. As the owner, I want to turn the filter I just built on the browse page into a named playlist, so I
   stop rebuilding the same view.
6. As the owner, I want to rename a playlist, change its sort, remove an item, or delete the playlist,
   so a shelf stays what I mean it to be.
7. As the owner, I want playlists to leave my files alone, so grouping never triggers a write or a
   review-queue item.

**Owner — hand someone a set**
8. As the owner, I want playlists private by default and public per playlist, so sharing is a
   deliberate act per set.

**Visitor**
9. As a visitor, I want to open a public playlist by link and play through it, so a set I was handed
   works like the owner's.
10. As a visitor, I want private playlists to be invisible — not listed, not 403'd, not guessable —
    so I see only what was meant for me.

**Edge cases**
11. As the owner, I want an empty playlist to be a valid, playable-when-filled thing, so I can name a
    shelf before it has an item.
12. As the owner, I want a video I trash to drop out of its playlists and come back when I restore it,
    so playlists never show a dead tile.
13. As the owner, I want a *Save as playlist* of a large result set to finish in one request and tell
    me how many items landed, so I trust the snapshot.

## Requirements

### Must-Have (P0)

**P0-1 — Store.** Migration `NNNN_playlists` (number claimed at implementation — 0051 is next on
main as of 2026-09-20; check other branches):
`playlists(id INTEGER PK, name TEXT NOT NULL, sort TEXT NOT NULL DEFAULT 'added_desc',
visibility TEXT NOT NULL DEFAULT 'private', created_at, updated_at)` and
`playlist_videos(playlist_id REFERENCES playlists ON DELETE CASCADE, video_id REFERENCES videos ON
DELETE CASCADE, position INTEGER NOT NULL, PRIMARY KEY (playlist_id, video_id))` with an index on
`(playlist_id, position)`. Manual down drops both.
- [ ] `sort` accepts every `MEDIA_SORTS` value plus `manual`; anything else is rejected at the API (P0-3).
- [ ] `visibility` ∈ {`private`, `public`}; default `private`.
- [ ] Deleting a video row cascades; deleting a playlist cascades; neither touches the other table's parent.

**P0-2 — Ref.** `model.KindPlaylist = "playlist"`; every playlist payload carries `ref: "playlist:<id>"`;
`ParseRef` accepts it; a kind mismatch is a 400 (ADR-096 D1, unchanged rules).

**P0-3 — API (owner-gated mutations, visibility-gated reads).**
| Method | Path | Gate | Behaviour |
|---|---|---|---|
| `GET` | `/playlists` | any | Owner: all. Visitor: `visibility=public` only. Each row: `ref, id, name, sort, visibility, item_count, updated_at`. |
| `GET` | `/playlists/{id}` | any | `{playlist, items, total, seed?}` — `items` in the playlist's order (see P0-4); `seed` echoed for a `random` sort. Visitor + private → **404**. |
| `POST` | `/playlists` | owner | Body `{name, sort?, visibility?, from_query?}`. `from_query` = a browse filter query string → RD3 snapshot; absent → empty playlist. 201 `{playlist}` with `item_count`. |
| `PATCH` | `/playlists/{id}` | owner | Any of `name, sort, visibility`; 200 `{playlist}`. |
| `DELETE` | `/playlists/{id}` | owner | 204. |
| `PUT` | `/playlists/{id}/videos/{videoId}` | owner | Append at `max(position)+1`; idempotent (RD9). 200 `{playlist}` (count current). |
| `DELETE` | `/playlists/{id}/videos/{videoId}` | owner | Remove; 204; 404 if not a member. |
| `GET` | `/capabilities` | any | *(existing)* gains `public_playlists: int` — count of `visibility=public` playlists. |
| `GET` | `/media/{id}` | any | *(existing)* gains `playlists: Playlist[]` — the video's memberships, visibility-filtered (the rail's PLAYLISTS row, P0-7). |
- [ ] `name` is required, trimmed, non-empty, ≤ 200 chars; duplicates **allowed** (a playlist is not an identity).
- [ ] Unknown `sort` / `visibility` → 400 naming the field; no write.
- [ ] `from_query` containing an owner-only sort (`completeness_*`) is fine — the caller is the owner by construction; a visitor never reaches this handler.
- [ ] `from_query` ignores `limit`/`offset`; a `random` filter sort snapshots in the **seeded order of that request** (its `seed`, or a fresh one) and stores `sort='manual'` so the shuffle the owner saw is what the playlist is. An explicit body `sort` wins over the filter's.
- [ ] `sort` is validated against `repo.ValidSort` + `manual` here; browse itself stays permissive (an unknown key, including `manual`, falls to `added_desc` as today).
- [ ] Every mutation runs under the repo's single writer (`writeMu`), one transaction per request — the snapshot insert is one `INSERT … SELECT`, not N statements.

**P0-4 — Ordered read.** `GET /playlists/{id}` returns `items` as the same tile payload the browse list
returns (so `VideoCard` / `PosterTile` render unchanged), ordered by the playlist's `sort`; `manual` →
`position ASC`; `random` → the ADR-045 seeded shuffle under `?seed=`. Trashed videos are excluded
(RD8). `total` is the un-trashed member count.
- [ ] Order for every non-`manual` sort matches what browse would produce for the same set.
- [ ] `random` with the same `seed` returns the same order across requests.

**P0-5 — Playlists page (`/playlists`).** Nav item alongside People / Studios / Tags. Lists
playlists as rows/cards (name, item count, visibility badge for the owner, sort). Owner: *New
playlist* (name → empty playlist). Visitor: public playlists only; empty state when none.
- [ ] Visitor with zero public playlists: the **nav item is hidden** (OQ1 resolved 2026-09-20); the
      ungated `/capabilities` payload gains `public_playlists: <count>` so the layout decides at
      bootstrap without a second fetch. Owner: always shown. A direct visit to `/playlists` with
      nothing public still renders an empty state, never a 404.
- [ ] `public_playlists` is identical for owner and visitor (it counts public ones only) — the owner's
      nav shows regardless.

**P0-6 — Playlist page (`/playlists/[id]`).** Title, item count, sort control (owner), visibility
toggle (owner), *Play all*, tiles in the playlist's order using the existing tile component, per-tile
*Remove* (owner). Rename inline (owner). Delete with confirm (owner).
- [ ] *Play all* navigates to the first item as `/media/{id}?playlist={pid}[&seed=]` and starts playback
      from that user gesture.
- [ ] Changing `sort` re-renders the tiles without reload; a change to/from `random` mints/clears the seed.
- [ ] Visitor sees no owner controls and no private playlist (404 page, same as an unknown id).

**P0-7 — *Add to playlist* on the video detail page.** An owner-only affordance in the existing
action row (same idiom as *+ Add tag*): a picker listing playlists with member state, plus *New
playlist…*. Selecting adds (RD9) and the row reflects membership immediately.
- [ ] Adding to a playlist the video is already in shows the member state, no error.
- [ ] *New playlist…* creates and adds in one flow (two requests, one interaction).
- [ ] Visitor: the affordance does not render (owner/visitor rule — the field gating stays per-field;
      this is an owner *action*, not content).

**P0-8 — *Save as playlist* on the browse page.** An owner-only action next to the sort/filters:
prompts for a name, posts `from_query` = the current shareable filter string (the same one F4.7 puts in
the URL), then navigates to the new playlist page. Reports the item count.
- [ ] With no active filter, saves the whole (un-trashed, browse-visible) library in the current sort.
- [ ] The snapshot ignores how many tiles the client had loaded.

**P0-9 — Persistent player element + next-up.** In `/media/[id]`:
- The `<video>` is rendered **outside** the page's `{#if loading}` gate and outside any per-item
  `{#if}` that would unmount it; per-item state that used to unmount it (the codec-failure branch, the
  poster) becomes sibling/overlay state. The element identity is stable across `/media/A → /media/B`.
- When `?playlist=` is present, the page holds the playlist's ordered ids (fetched once per playlist +
  seed, cached in a store) and renders a **next-up surface**: the next item's tile + *Next* / *Previous*
  controls, and the playlist name linking back.
- On `ended` with a next item: navigate to `/media/{next}?playlist=…` and set an in-memory *play on
  load* flag; when the loaded `video.id === route id`, call `play()` **once**; clear the flag on any
  outcome. The stale-element `AbortError` seen in the spike must be impossible by construction (the
  flag is checked against the route id, not the element).
- Media Session: `navigator.mediaSession.metadata` = the current title; `nexttrack` / `previoustrack`
  action handlers set while a playlist context is active, cleared when it is not — this is what gives
  Chrome's PiP window its Next/Previous buttons.
- [ ] `document.querySelector('video')` is the same node before and after next-up (test asserts identity).
- [ ] A PiP window opened on item *n* is still open and showing item *n+1* after next-up (manual QA, Chrome).
- [ ] Reloading `/media/{id}?playlist=…` does **not** autoplay (RD6).
- [ ] Last item ends → playback stops; the next-up surface says so; no navigation.
- [ ] Leaving `/media/*` still tears the player down (no change — the mini-player is a non-goal).
- [ ] Behaviour outside a playlist context is byte-for-byte today's (no `?playlist=` → no surface, no
      handlers, `ended` only clears the atmosphere class).

**P0-10 — Three skins.** Every new surface (P0-5..P0-9) is tokens-only and QA'd in Cinémathèque,
Broadcast, Brutalist per `.claude/rules/frontend-theming.md`.

### Nice-to-Have (P1)

**P1-1 — Hotkeys.** `N` / `P` (or the F62 equivalents) for next/previous while in a playlist context,
registered in F62's map, shown in its cheat-sheet.
**P1-2 — *Play all* from the playlists list.** A play affordance per row so a session starts from
`/playlists` without opening the page.
**P1-3 — Autoplay-next opt-out.** A per-browser toggle on the next-up surface ("Continue automatically")
persisted in `localStorage`, default on. *(Per-viewer convenience — the right use of local storage.)*
**P1-4 — Snapshot count in the toast.** *Save as playlist* reports "Saved N items to ‹name›" with a link.

### Future Considerations (P2)

**P2-1 — Reorder.** Drag handle / move controls writing `position`; `manual` sort already honours it.
**P2-2 — Live playlists.** A nullable `frozen_query` + *Refresh from filter*; the store needs one
added column, nothing restructured.
**P2-3 — Cover image.** `entityimage` kind `playlist`; the playlists list gets a face.
**P2-4 — Layout-level player.** A playback singleton in `+layout` (the Plex model): PiP + queue survive
leaving `/media/*`, mini-player, "queue this next". Requires nothing in v1 to change shape — RD5's
persistent element is the same element, hoisted.
**P2-5 — Search & chips.** Playlists in nav search; "in N playlists" on a video / person page.
**P2-6 — MCP.** `list_playlists` / `get_playlist` for the MCP server, gated like the REST reads.

## Behavior detail

- **Playlist context lifetime.** Entering `/media/{id}?playlist=` loads the ordered ids for `(pid,
  seed)` into a page-level store keyed on both; navigating between items in the same playlist reuses
  it; a different `pid`/`seed` refetches. Membership changes made *during* a play-through (the owner
  removes an item from another tab) are not reflected until the store is refetched — acceptable, noted.
- **Current item not in the playlist** (deep link with a stale id, or the item was removed): the
  next-up surface shows the playlist name and *Play from start*; `ended` does nothing.
- **Visitor in a playlist context** whose playlist is private: the media page still renders (the video
  itself is visitor-visible per existing rules); the playlist fetch 404s; the next-up surface renders
  nothing and the `?playlist=` param is simply inert. No error toast — the visitor was handed a link
  that does not include the playlist, and the video works.
- **`random` and next-up.** The seed is minted client-side (same helper browse uses), carried in the
  URL, and sent to `GET /playlists/{id}?seed=`; *Previous* walks the same order.
- **Trash during a run.** If the next item is trashed between the store load and `ended`, the
  navigation lands on the media page's existing not-found state and the run stops; the next store
  fetch drops it. No special handling.

## Data model

```sql
CREATE TABLE playlists (
  id          INTEGER PRIMARY KEY,
  name        TEXT    NOT NULL,
  sort        TEXT    NOT NULL DEFAULT 'added_desc',   -- MEDIA_SORTS value | 'manual'
  visibility  TEXT    NOT NULL DEFAULT 'private',      -- 'private' | 'public'
  created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE TABLE playlist_videos (
  playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
  video_id    INTEGER NOT NULL REFERENCES videos(id)    ON DELETE CASCADE,
  position    INTEGER NOT NULL,
  PRIMARY KEY (playlist_id, video_id)
);
CREATE INDEX playlist_videos_order ON playlist_videos(playlist_id, position);
```

No `CHECK` on `sort` / `visibility` — validated at the API like other enum-ish columns here (the
`film_images.role` precedent), so a future value is a code change, not a migration.

## API

Base `/api/v1`; shapes in P0-3. `items` rows are the browse tile payload. Errors follow the existing
`{"error": "…"}` envelope. Reads are ungated but visibility-filtered (RD4); mutations mount inside the
`requireOwner` group.

## UI

Three surfaces plus one affordance:
1. **`/playlists`** — list; nav item.
2. **`/playlists/[id]`** — detail; *Play all*; owner controls.
3. **Next-up surface on `/media/[id]`** — only with `?playlist=`; next tile + Next/Previous + playlist link.
4. **Producers** — *Save as playlist* (browse, owner) · *Add to playlist* (detail action row, owner).

Design handoff decides the exact placement and the three-skin treatment; this spec fixes only what
each surface must contain and who sees it.

## Success Metrics

Holodex is a single-owner application with no analytics; these are **verification outcomes**, not
funnel metrics.

*Leading (at ship, QA'd):*
- *Play all* on a 3+ item playlist reaches the last item with **zero** clicks after the first, in Chrome
  and Firefox; Safari result recorded (OQ2).
- PiP opened on item 1 is still open on item 2 (Chrome, manual).
- A visitor session cannot enumerate or open a private playlist by id (404, not 403) — asserted in a
  handler test and checked by `/security-review`.
- *Save as playlist* on the largest testbed filter (whole AMV library) completes in one request; item
  count equals browse's `total` for the same filter.

*Lagging (first month of use):*
- The owner has ≥1 playlist they return to; at least one session is run end-to-end via next-up
  rather than tile-by-tile. If neither happens, the "queue a session" job was wrong and P2-4 should
  not be pursued.

## Open Questions

1. ~~**[design]** Does the *Playlists* nav item hide for a visitor when there is no public playlist,
   or show an empty state?~~ **Resolved 2026-09-20 (owner):** hidden — `/capabilities.public_playlists`
   (P0-5).
2. **[engineering, non-blocking, manual]** Safari: does an unmuted `play()` after a `src` swap on the
   element the user originally clicked succeed? Recipe in the HOLODEX-438 spike comment. If it fails,
   P1-3's toggle becomes the Safari fallback (surface *Next* as a button), not a redesign.
3. ~~**[design]** Where *Save as playlist* lives on the browse page?~~ **Resolved 2026-09-20 (design
   handoff, option A):** in the toolbar group beside *Clear filters*, expanding in place into the
   tag-add inline form.
4. **[engineering, blocking for S1]** Migration number — claim at implementation (`0051` on main as of
   this writing; verify against in-flight branches, cf. the 0048→0050 renumbering on F67).

## Timeline / routing

Three stories, one Draft PR on `HOLODEX-438-video-playlists` (ADR-069), opened once this spec and
ADR-104 land:

1. **S1 — store + API** (P0-1..P0-4, RD3, RD8, RD9): migration, repo, handlers, ref kind, tests.
   Landable alone (API-only).
2. **S2 — pages + producers** (P0-5..P0-8, P0-10): `/playlists`, `/playlists/[id]`, *Add to
   playlist*, *Save as playlist*. Needs the design handoff.
3. **S3 — next-up** (P0-9, P1-1): the persistent element refactor of `/media/[id]` first (its own
   commit, behaviour-neutral, element-identity test), then the playlist context + Media Session.

Gates per the change-routing table: **spec** (this document) · **ADR** (ADR-104: container-not-entity,
membership + sort, snapshot-only, persistent player element, visibility) · **design handoff** (S2 + the
next-up surface, SVG committed) · **testing strategy** (P0-3 handler tests incl. the 404-not-403
visitor case, P0-4 order parity with browse, P0-9 element-identity test, the three-skin QA matrix,
manual PiP + Safari rows) · **security review** (new owner-gated writes; visibility as a read gate;
`from_query` is parsed by the existing browse parser and never reaches SQL as text).
