# ADR-104: Video playlists — a container, not an entity; membership + sort; server-side snapshot; a persistent player element; visibility as a read gate

**Status:** Proposed
**Date:** 2026-09-20
**Deciders:** Project owner

**Extends:** [ADR-096](ADR-096-entity-identity-card.md) D1 (the `kind:id` reference handle gains a
`playlist` kind — and *only* D1: none of D2–D5 apply) · [ADR-045](ADR-045-seeded-random-ordering.md)
(the seeded shuffle is reused as a playlist sort, seed carried by the play-through) ·
[ADR-037](ADR-037-soft-delete-and-purge.md) (the `deleted_at IS NULL` read seam is the one place trash
is decided; playlists inherit it, never restate it) · [ADR-021](ADR-021-frontend-theming-and-skins.md)
(new surfaces are tokens-only, three skins).
**Deliberately does not extend:** [ADR-061](ADR-061-unified-entity-name-identity.md) /
[ADR-096](ADR-096-entity-identity-card.md) D2–D5 (the identity spine) ·
[ADR-051](ADR-051-per-field-source-of-truth-decisions.md) / [ADR-090](ADR-090-two-layer-entity-metadata-management.md)
(no field is resolved, adopted, or decided on a playlist) · [ADR-081](ADR-081-entity-completeness-score.md)
(nothing to score).
**Issue:** [HOLODEX-438](https://whoiskevinrich.atlassian.net/browse/HOLODEX-438) (spec
[F69](../specs/video-playlists.md)).

---

## Context

F69 adds the first **grouping of videos that is not file metadata**. Every grouping the product has
today is one of two things: an *entity* on the ADR-061/096 spine (Person, Studio, Film, Tag — each
with a file-layer baseline, provider ids, aliases, per-field decisions, a completeness score), or a
*filter* (F4.7's shareable URL — stateless, live, unordered beyond its sort). A playlist is neither: it
is owner-authored, single-source, ordered, private-able, and must never touch a file. Five facts about
the current code shape the decisions:

1. **The spine is identity machinery.** `entity_aliases`, `entity_external_ids`, `SetDecisionChecked`,
   the name-key collision gates and the completeness bands all exist to reconcile *several sources'
   claims about one thing*. A playlist has one source (the owner) and no external counterpart. Every
   spine feature applied to it would be a code path with nothing to do. ADR-078 already set the
   precedent of a *deliberately reduced* grouping (`categories`), and ADR-090's Scope excludes
   entity-link fields from the two-layer model — a playlist is a bag of entity links seen from the
   other side.
2. **Ordering is already a vocabulary.** `MEDIA_SORTS` (F12.1) is the single source of truth for how a
   set of videos is ordered, server-validated, and includes ADR-045's seeded `random`. A playlist
   needs exactly this plus one value the browse page cannot have: an owner-authored order.
3. **The browse filter is already parsed server-side.** `videoFilterFromQuery(url.Values)` turns the
   F4.7 query string into a `repo.VideoFilter`; `ListVideos` applies the visibility seam
   (`v.active = 1 AND v.deleted_at IS NULL`, ADR-037), the film-hiding posture and `orderBy()`. But it
   **caps at `maxListLimit`** — it is a page reader, not a set reader. A snapshot needs the same
   clauses and order with no cap.
4. **The media page destroys its `<video>` on every item change.** SvelteKit reuses the
   `/media/[id]` component across ids, but the page-wide `{#if loading}` gate (~L1291) wraps the
   player, and the load effect flips `loading = true` per item. The 2026-09-20 spike verified it by
   element identity (`sameElement: false`, old element gone from the document; the first `play()`
   after navigation fails with `AbortError: media was removed from the document`). Picture-in-Picture
   is bound to the element, so any next-up built on today's page would close the PiP window at every
   hand-off. The owner's stated requirement is Plex behaviour: continue to the next item **with PiP
   intact**.
5. **Owner/visitor is a request property, not a row property.** `requireOwner` gates handler
   groups; per-field visitor gating is decided at resolve time. Nothing stored today says "visitors
   may not see this row". A playlist is the first thing that needs that.

## Decision

**D1 — A playlist is a container, not an entity.** `playlists` + `playlist_videos` are their own
tables with their own repo methods and handlers. There is **no** `BaselineSource`, no row in
`entity_aliases` / `entity_external_ids` / `entity_enrichment`, no `resolver` branch, no completeness
row, no `entityimage` kind (reserved, not shipped), no name-key. The *only* spine convention borrowed is
ADR-096 D1's reference handle: `model.KindPlaylist = "playlist"`, `ref: "playlist:<id>"` on every
payload, `ParseRef` accepts it, kind mismatch is a 400. The ADR index and `internal/model/ref.go`
comment say "container, not entity" so a future reader does not reach for the spine. Rename is a plain
`UPDATE` — it is not a name-identity edit (ADR-061 §rename), because nothing routes on the name.

**D2 — Playlist = membership + a sort.** `playlist_videos` is a **set** — `PRIMARY KEY (playlist_id,
video_id)` — with a `position`. `playlists.sort` is any `MEDIA_SORTS` value or **`manual`**, validated at
the API against the same list the browse handler validates against (one source of truth; `manual` is
appended for playlists only and rejected by browse). Read order = `orderBy()` for the stored sort
applied to the membership join; `manual` → `position ASC`; `random` → ADR-045's shuffle under a seed
the client carries. **A snapshot playlist and a curated playlist are the same row shape** — the
producer is not stored. Reorder is therefore a future `UPDATE position`, never a schema change.

**D3 — Producers are snapshot-only and the snapshot is server-side, through the browse clause
builder.** `POST /playlists` with `from_query` parses it with `videoFilterFromQuery` (unchanged),
strips `limit`/`offset`/`seed`, sets `HideFullFilmVideos` as browse does, and calls a new
**id-only, uncapped** repo read — `ListVideoIDs(ctx, VideoFilter) ([]int64, error)` — built from the
**same** WHERE/ORDER builder `ListVideos` uses, extracted so the two cannot drift. The insert is one
`INSERT INTO playlist_videos … SELECT` (or one multi-row insert) under `writeMu`, `position` = rank.
`sort` is set to the filter's sort — **except `random`, which stores `manual`** with the seeded order
of that request, because "save this shuffle" means the one on screen. **No `frozen_query` column.** The
membership is the whole record; a *Refresh from filter* would be one nullable column and a button
later, and migrations are append-only, so deferring it costs nothing now.

**D4 — Next-up is a `src` swap on a persistent `<video>`; the element is lifted above the media page's
loading gate.** In `/media/[id]` the `<video>` renders **outside** `{#if loading}` and outside every
per-item `{#if}` that would unmount it (the codec-failure branch and the poster become sibling / overlay
state). Its identity is stable across `/media/A → /media/B`; a test asserts it. Playlist context is
`?playlist=<id>[&seed=]` on the URL (shareable, deep-linkable); *play immediately* is **in-memory
state set only by the `ended` handler** and is consumed once the loaded `video.id === route id` —
never a URL param, so a reload or a shared link never autoplays and the stale-element race from the
spike is structurally impossible. Media Session `metadata` + `nexttrack` / `previoustrack` action
handlers are registered while a playlist context is active and cleared when it is not; that is what
gives the PiP window its Next/Previous. **The layout-level player (a playback singleton in `+layout`,
the full Plex model) is explicitly deferred**: the owner's PiP use is "switch to another app", which a
route-scoped persistent element already satisfies; leaving `/media/*` still tears the player down. D4's
constraint — one element, `src` swapped — is the same constraint a hoisted player would have, so the
hoist is a later move of the same element, not a redesign.

**D5 — Visibility is a playlist property and a read gate; trash is inherited, not restated.**
`playlists.visibility ∈ {private, public}`, default `private`. Reads are ungated at the router and
**filtered by the request's owner-ness**: owner sees all; visitor sees `public` only, and a visitor's
request for a private id is a **404 identical to an unknown id** (existence must not leak — 403 would
confirm the id). Mutations mount inside the `requireOwner` group. The ungated `/capabilities` payload
gains `public_playlists: <count>` so the layout can hide the nav item for a visitor with nothing to see
(spec OQ1). Playlist reads join through the same `v.active = 1 AND v.deleted_at IS NULL` seam every
list surface uses — a trashed video vanishes from its playlists and returns on restore with no playlist
code knowing about trash; purge removes the rows via `ON DELETE CASCADE`. Per-video visitor field
gating is untouched and still applies to every tile a playlist renders.

## Consequences

- **The spine stays the spine.** Nothing in `internal/resolver`, `internal/enrich`, the alias tables
  or the completeness trigger set is touched. The ADR-090 review queues never see a playlist. This is
  the property the index row exists to protect.
- **`ListVideos` gets a sibling, not a flag.** Extracting the clause builder is a behaviour-neutral
  refactor of the central read seam (Context 3); it ships as its own commit with the existing
  list/count tests green before any playlist code lands on top.
- **The media page's player refactor is behaviour-neutral and precedes the feature.** Lifting the
  element above the loading gate (D4) changes nothing a user sees today, so it lands as its own
  commit with an element-identity test — the same discipline as the clause-builder extraction.
- **A new sort value exists that browse must reject.** `manual` is meaningful only where a
  `position` exists; the validator is shared and the browse handler filters it, tested.
- **Autoplay policy is a per-browser fact this ADR cannot decide.** D4 gives every browser its most
  favourable case (same document, same element the user clicked play on). Chrome/Firefox are expected
  to pass on sticky activation; Safari is unverified (spec OQ2). If a browser refuses, the fallback is
  the spec's P1-3 *Continue automatically* toggle surfacing *Next* as a button — the design does not
  change.
- **`from_query` is never SQL text.** It reaches the database only as a `VideoFilter` built by the
  existing parser, which is the `/security-review` line for this feature.
- **Revisit** when a second container kind appears (a "queue", a "collection"): D1's tables are
  playlist-shaped, not generic; a second kind should be its own tables or a `kind` column, decided then.

## Alternatives considered

| Alternative | Why not |
|---|---|
| **Full spine entity** (aliases, external ids, decisions, completeness, `entityimage`) | Every spine feature exists to reconcile several sources; a playlist has one. It would be a set of no-op code paths, a place in every entity sweep, and a completeness score with nothing to score. ADR-078's reduced `categories` is the precedent for saying no |
| **A tag with a flag** (`tags.kind = 'playlist'`) | Tags are file metadata: written back, lowercase by policy (0034), extraction-queued, unordered. Every one of those is a property a playlist must *not* have; a flag would be four `if` branches in the writeback/extraction seams |
| **Live playlists** (store the query, materialise on read) | "Hand someone a set" needs the set to stay put; a live view is a bookmark of a filter, which F4.7's URL already is. Deferred as one nullable column (D3) |
| **Client-side snapshot** (post the ids the browse page has loaded) | Wrong by construction — the page holds one `maxListLimit` page. The server has the clause builder; the client has a URL |
| **Store `sort='random'` on a random snapshot** | Re-shuffles on every open; the owner saved the order on screen, not the fact that it was random |
| **Duplicates allowed in `playlist_videos`** | A playlist is "membership + sort" (D2); a multiset would make *Add* non-idempotent and *Remove* ambiguous for no v1 job |
| **Layout-level player now** (playback singleton in `+layout`) | Moves ~50 lines of player/poster/atmosphere logic out of a 2,100-line page and adds a queue store, to serve "keep browsing while it plays" — a use the owner does not have. D4 keeps the same element constraint, so the hoist stays available |
| **Autoplay intent in the URL** (`?autoplay=1`) | Shared/reloaded links would try to start sound; browsers may refuse on a fresh load; the spike's stale-element race lived exactly here |
| **403 for a private playlist** | Confirms the id exists; 404 is indistinguishable from unknown |
| **A `CHECK` on `sort` / `visibility`** | Precedent (`film_images.role`) validates at the API so a new value is a code change, not a migration |
| **`ON DELETE CASCADE` only, no read-seam join** | Soft delete (ADR-037) does not fire cascades; the read seam is the only place trash is decided, so playlist reads must go through it |

## Action items

1. [ ] Claimed: `ADR-104` via `adr-claims.mjs --reserve video-playlists`; index row in
   `docs/architecture/README.md`; `internal/model/ref.go` comment "container, not entity" (D1)
2. [ ] Extract the `ListVideos` WHERE/ORDER builder; `ListVideoIDs(ctx, VideoFilter)` uncapped,
   ordered; existing list/count tests green — own commit (D3)
3. [ ] Migration `NNNN_playlists` (number claimed at implementation; 0051 next on main 2026-09-20),
   repo CRUD + membership under `writeMu`, `KindPlaylist`, `/playlists*` handlers, shared sort
   validator with `manual` rejected by browse, `/capabilities.public_playlists` (D1, D2, D5)
4. [ ] `POST /playlists` `from_query` → `videoFilterFromQuery` → `ListVideoIDs` → one insert;
   `random` → `manual` (D3)
5. [ ] `/media/[id]`: lift `<video>` above `{#if loading}`; codec-failure + poster as overlays;
   element-identity test — own behaviour-neutral commit (D4)
6. [ ] Playlist context store keyed `(id, seed)`, next-up surface, `ended` → in-memory play flag →
   `play()` on `video.id === route id`; Media Session handlers set/cleared (D4)
7. [ ] Visitor-private = 404 handler test; order-parity-with-browse test; `docs/testing-strategy.md`
   rows incl. manual PiP + Safari (D5, D4)
8. [ ] Spec F69 "ADR-104 (to claim)" → link; worklog gate `[x]`
