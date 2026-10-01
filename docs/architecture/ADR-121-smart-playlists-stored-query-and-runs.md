# ADR-121: Smart playlists store the canonical browse query and re-run it on read; Play all and Shuffle are client-held runs over an id list

**Status:** Proposed — supersedes **ADR-104 D3's "snapshot-only, no `frozen_query`"** clause only;
ADR-104 D1, D2, D4, D5 and D3's snapshot producer (`from_query`) stand
**Date:** 2026-09-30
**Deciders:** Project owner
**Spec:** [F75 smart-playlists.md](../specs/smart-playlists.md) · **Jira:** HOLODEX-58 (smart playlists),
HOLODEX-500 (Play all + Shuffle), HOLODEX-501 (entity grids through `/media`)

## Context

F69 (ADR-104) shipped playlists as a set of video ids plus a sort, filled by a server-side snapshot of a
browse filter. It explicitly rejected live playlists and left one nullable column for later. F75 asks
for three things the snapshot model can't do:

1. **A playlist that stays true** — "everything with this tag" must include next month's videos.
2. **Play any grid without saving** — person, tag, studio, film and browse grids become runs through
   the F69 next-up player (ADR-104 D4), in order or shuffled, toggleable mid-run, optionally repeating.
3. **Rules arrive free** — when browse filters gain AND/NOT (HOLODEX-181), smart playlists must store
   them without a playlist-side change.

Forces found in the code:

- **The filter grammar already has one server parser**, `videoFilterFromQuery`
  (`internal/api/handlers.go:545`), shared by browse, completeness facets and film-video candidates.
  Entity facets (`person`, `tag`, `studio_id`, `category_id`) are **numeric ids**. Filterable mapped
  fields (`?studio=Acme`, F20.5) are **values**, keyed by a canonical name that comes from
  `metadata-mappings.yaml` and can disappear on a mapping reload.
- **The parser ignores keys it doesn't know.** A stored query whose mapped key was removed from the
  mappings would be silently *broadened* on the next read.
- **Some filter inputs are owner-only.** `listMedia` gates `missing_facet` and the completeness sorts
  with `requireOwnerInline` (`handlers.go:505`), and `redactFileMetadataForVisitors` strips owner-only
  fields from tiles.
- **An uncapped id-only reader exists**: `repo.ListVideoIDs` (ADR-104 D3), same `build()`/`orderBy()`
  as `ListVideos`, so a set and its order can't disagree with browse.
- **Merges repoint ids inside one transaction**: `MergePersonsWithAffectedVideos`
  (`internal/repo/aliases.go:113`) and `MergeEntitiesWithAffectedVideos` (`identity_ops.go:347`).
- **Person, tag and studio grids don't use `/media`.** `getPerson` / `getTag` / `getStudio` call
  `ListVideos` with a hardcoded facet capped at 500. The owner's parity check (2026-09-30) found the
  same set and order as `/media` apart from the cap, and one real tag exceeds 500.
- **Smart playlists will number in the tens**, not thousands. Membership can be thousands of videos.

## Decision

### D1 — A smart playlist is an F69 playlist row with a stored query; membership is never stored

Migration (number claimed at implementation):

```sql
ALTER TABLE playlists ADD COLUMN query TEXT;                                -- NULL ⇒ F69 snapshot playlist
ALTER TABLE playlists ADD COLUMN query_version INTEGER;                     -- NULL iff query IS NULL
ALTER TABLE playlists ADD COLUMN play_shuffled INTEGER NOT NULL DEFAULT 0;  -- D8, any playlist
```

A playlist is **smart iff `query IS NOT NULL`**, and a smart playlist has **no `playlist_videos`
rows**. The playlist API refuses membership writes to it (400) and `sort = manual` (400; there is no
`position`). *Freeze* runs the query once, writes `playlist_videos` with `position` = rank, and nulls
`query` / `query_version` in one transaction under `writeMu`, which turns it into exactly an ADR-104
snapshot playlist. Still a container (ADR-104 D1): no entity spine, no resolver branch.

### D2 — The stored query is the canonical `/media` filter string, minus sort and fetch mechanics

`canonicalPlaylistQuery(url.Values) (string, error)` lives beside `videoFilterFromQuery` and is the
only writer of `playlists.query`:

- **Keeps only filter keys**: the fixed set `videoFilterFromQuery` reads, `missing_facet`, and the
  *current* filterable mapped canonical keys. **Rejects** (400) any other key, so a typo or a stale
  client can't save a query that means less than it looks.
- **Strips** `limit`, `offset`, `seed` (fetch mechanics) and **`sort`**. Sort belongs to
  `playlists.sort` (ADR-104 D2), copied from the grid at save and editable afterwards. One home for
  order means *Edit filter* can't fight the sort control.
- **Normalises**: keys sorted, repeated id values de-duplicated and numerically sorted, empty values
  dropped, encoded with `url.Values.Encode`. Two grids showing the same set store the same string.
- `query_version = 1`. Bump it only when an **existing** key changes meaning, and ship a migration
  that rewrites stored queries. New keys and new operators (AND/NOT) don't bump it. This is the point:
  the playlist stores whatever the filter grammar can say.

Reads parse with the same `videoFilterFromQuery` browse uses, then set `HideFullFilmVideos` as
browse does. A smart playlist can never mean something browse wouldn't.

### D3 — Live evaluation on read, under the reader's own posture

`GET /playlists/{id}` for a smart playlist, in this order:

- **Visibility first** (ADR-104 D5): visitor + private = 404, byte-identical to an unknown id, **before**
  the D5 stale check and the D4 owner-only re-check. Either of those would otherwise answer a visitor
  with `stale_refs`, confirming that a private playlist exists and echoing the ids it references
  (security review, 2026-09-30).
- **Stale check (D5)**, then evaluate.
- **Tiles**: `ListVideos` with the stored filter, `playlists.sort`, and the request's paging.
  `redactFileMetadataForVisitors` and browse-title resolution apply exactly as in `listMedia`, through
  one shared helper so the two can't drift.
- **Ordered ids** (for runs): `ListVideoIDs`, uncapped.

**One filter builder.** `videoFilterFromQuery` deliberately leaves `HideFullFilmVideos` unset and doesn't
read `missing_facet`; `listMedia` adds both, then gates owner-only inputs. A reader that calls only the
parser would show full-film videos `/media` hides and would skip the owner-only check on
`missing_facet`. So the filter-and-gate step moves into one helper, `mediaFilterFor(r, query)`, which
parses, sets `HideFullFilmVideos` and `MissingFacets`, and refuses owner-only inputs for a visitor. It
is the only path from a query to a `repo.VideoFilter` for `listMedia`, `GET /media/ids` (D7) and this
smart playlist read (security review, 2026-09-30).

No cache. A `ListVideoIDs` over the stress fixture's largest filter is a single indexed query; tens of
smart playlists don't justify invalidation machinery. Revisit only with a measured slow read.

### D4 — A query that uses owner-only inputs can't be public

A stored query with `missing_facet` can't be combined with `visibility = public`. The refusal (400)
happens at save, at *Edit filter → Update* and at the visibility PATCH, whichever comes second. The
completeness sorts are refused on `playlists.sort` for public playlists by the same rule. The read
path also re-checks: an owner-only input on a public playlist is evaluated as a visitor `/media` call
would be — refused — and the playlist reports `stale_refs: [{kind: "owner_only"}]` instead of
returning anything. This closes the spec's security question in the model; `/security-review` still
signs it off.

### D5 — Merges follow; deletions and vanished keys flag; nothing is dropped silently

- **Merge**: `MergePersonsWithAffectedVideos` and `MergeEntitiesWithAffectedVideos` call
  `rewriteSmartPlaylistRefs(tx, key, mergedID, canonicalID)` **inside their existing transaction**.
  Key is `person` for people, `studio_id` for studios, and the matching facet key for any entity type
  `MergeEntities` handles that has one. It selects `id, query FROM playlists WHERE query IS NOT NULL`,
  parses each with `url.ParseQuery`, replaces the id, re-normalises (D2) and writes back changed rows
  only. A scan of tens of rows inside a merge is negligible. A side table of referenced ids was
  rejected for that reason: it is a second copy of the query that can drift.
- **Delete / vanish**: at read, the evaluator checks every id facet (`person`, `tag`, `studio_id`,
  `category_id`) against its table, and every mapped key against the **current** filterable set. Any
  miss is listed in `stale_refs: [{key, value}]`, and **the playlist is not evaluated**. The response
  carries the stale list and an empty item list, so the page shows the notice with *Edit filter* /
  *Delete*. Evaluating with the dead clause dropped would broaden the set, and evaluating with it kept
  usually returns nothing for a reason the owner can't see. Both are wrong.
- Mapped-field **values** (`studio=Acme`) aren't rewritten by merges. They match text, not identity,
  and the owner chose them as text. Only a vanished **key** is stale.

### D6 — Entity grids load through `/media` with paging (HOLODEX-501)

Person, tag and studio pages fetch their grid from `GET /media?person=|tag=|studio_id=` with browse's
paging. `EntityVideos.svelte`'s title box sends `q`, so the grid is filtered server-side and the
query a run or a save reads is exactly what's shown. The 500 cap goes away with the bespoke path. The
entity endpoints stop embedding the video list once no caller needs it; whether the MCP server or any
other caller reads it is an action item, not an assumption. This makes **every query-backed grid's
query a plain `MediaFilters`** that `filters.ts` already serialises. Grids without one (search, Related,
Recently Added) don't get the actions.

### D7 — A run is client-held state over an id list fetched once; shuffle is client-side and seeded

A **run** is `{id, source, order, mode, seed, ids, history, repeat}`:

- `source`: `{kind: "query", query}`, `{kind: "playlist", id}`, or `{kind: "film", id}`. A film's grid
  is its scenes from `GET /films/{id}`, not a `/media` query, so a film is its own source: it plays the
  scenes in scene order (one `sortScenes`, shared with the film page) and carries `film=<id>` in the
  URL. It can be played and shuffled but not saved as a smart playlist (owner decision, 2026-10-01).
- `ids`: the **source-order** id list, fetched once at the press.
- `order`: the play order derived from `ids`.

It lives in `sessionStorage` under the run id; the URL carries `run=<id>` plus the source (`from=<canonical
query>` or F69's existing `playlist=<pid>`). That is enough to rebuild a run when storage is gone: a
reload in a new tab re-fetches and starts a new run at the current item, without autoplay (ADR-104 D4's
rule that autoplay intent is never in the URL is unchanged). F69's `?playlist=&seed=` context becomes
a run with a playlist source, so there is **one next-up model**, not two.

- **Ids**: grid runs use a new `GET /media/ids?<filter>&sort=` → `ListVideoIDs`, built and gated by
  D3's `mediaFilterFor` (the owner-only check and full-film hiding included) because it is the same
  query. Playlist runs use D3's
  ordered ids.
- **Shuffle is a mode, done on the client.** `order` is `ids` permuted by a seeded Fisher–Yates
  (`mulberry32(seed)`), a pure function in `web/src/lib/` with unit tests. The server's ADR-045 seeded
  order is **not** used for shuffle mode, because the mid-run toggle needs both the source order and a
  shuffled order of the *remaining* items without another request. The grid's `random` **sort** is
  unchanged: a playlist or grid sorted `random` gets its server-seeded order as `ids`, and its *Play
  all* and *Shuffle* coincide (spec RD10, owner decision to keep the sort as is).
- **Toggle** (spec RD11): `history` is never reordered. On, the remaining items are permuted under a
  fresh seed; off, the remaining items are re-sorted by their index in `ids`. The current item never
  moves.
- **Repeat** (spec RD12): at the end of a pass, re-fetch `ids` (the source may have grown), re-permute
  with a fresh seed if shuffled, and if the first item equals the one that just ended, swap it with
  the second.
- **The run never writes to the server.** Shuffle, toggle and repeat are view state.

### D8 — "Always shuffle" is a playlist column read by the client

`play_shuffled` (D1) makes a playlist's primary action *Shuffle*, and a run entered from its tiles
starts in shuffle mode. Visitors inherit it. It doesn't change the page's display sort. It applies to
F69 snapshot playlists too, because it is about playback, not membership.

## Options Considered

### Storing the query: canonical string (chosen) vs structured JSON vs side table of refs

| Dimension | Canonical `/media` string | Structured JSON rule tree | String + `playlist_query_refs` table |
|---|---|---|---|
| Complexity | Low: one normaliser beside the parser | High: second grammar | Medium |
| AND/NOT later | Free, whatever `filters.ts` can say | Must be designed twice | Free |
| Merge rewrite | Parse tens of rows in the merge tx | Walk the tree | Indexed update, but two copies to keep in sync |
| Drift risk | None: one grammar | High | Refs vs string can disagree |

**Chosen: canonical string.** It is the only option where Goal 4 (rules arrive free) holds by
construction.

### Shuffle: client seeded permutation (chosen) vs server `sort=random&seed`

The server path reuses ADR-045 but needs a request per toggle and still needs the source order kept
client-side to toggle back. That makes two orders from two requests that could disagree if the set
changed in between. The client path fetches once and is a pure, testable function. Cost: two shuffle
implementations exist (ADR-045 for the `random` sort, D7 for the mode). They answer different
questions — "sort the page randomly" vs "play this set in a random order" — so they are kept apart on
purpose.

### Run state: sessionStorage + URL source (chosen) vs URL-only vs server-side runs

URL-only can't hold a multi-thousand-id list. Server-side runs add a table, expiry and a write per
play for state that is per-tab and disposable. sessionStorage is per-tab, survives reload, and the URL
source makes it rebuildable (ADR-118 is the same pattern for list scroll).

## Consequences

- **Easier:** any future filter operator becomes a smart-playlist capability for free; every
  query-backed grid gets runs with no per-page code; F69 playlist runs and grid runs share one model.
- **Easier:** the 500 cap and the bespoke entity video path disappear (D6).
- **Harder:** the filter grammar is now **persisted**. A change to an existing key's meaning needs a
  `query_version` bump and a migration of stored queries (D2). Reviewers of `filters.ts` /
  `videoFilterFromQuery` changes have to ask "does this change a stored meaning?"
- **Harder:** every merge path must call the rewrite (D5). A new merge path that forgets to will leave
  stale ids, though D5's read-time check still flags them rather than broadening.
- **Accepted gap:** a run doesn't see membership changes until its next pass or press (spec RD5); a
  run is per-tab.
- **Revisit:** cache live evaluation only on a measured slow read; tag merge joins D5 if tags gain a
  merge path.

## Action Items

1. [ ] Migration: `query`, `query_version`, `play_shuffled` on `playlists` (claim the number; see
   `.claude/rules/migrations.md`).
2. [ ] `canonicalPlaylistQuery` + tests: rejects unknown keys, strips sort / fetch keys, normalisation
   is idempotent, and equal sets give equal strings.
3. [ ] Smart playlist read path (D3) with a shared tile-hydration helper used by `listMedia`, the
   visibility 404 ahead of the stale and owner-only checks (handler test: a visitor on a private
   playlist with stale refs gets the unknown-id 404), and `mediaFilterFor` as the one filter builder.
4. [ ] Owner-only refusal (D4) at save, update, visibility and sort PATCH, plus the read-time
   re-check; handler tests in visitor mode.
5. [ ] `rewriteSmartPlaylistRefs` called from both merge transactions; a test per merge path; a test
   that a stale id or vanished mapped key returns `stale_refs` and no items.
6. [ ] HOLODEX-501: entity grids via `/media` with paging, title box → `q`. Check MCP and other
   readers of the embedded video lists before removing them.
7. [ ] `GET /media/ids` (D7) through `mediaFilterFor` (test: a visitor's `missing_facet` gets 401, and
   a full-film video is absent, as on `/media`).
8. [ ] `web/src/lib` run module: seeded permutation, toggle and repeat as pure functions with unit
   tests. `playlistContext.ts` becomes a playlist-source run.
9. [x] `/security-review` on D3/D4, design stage (2026-09-30): no vulnerability. D4 covers every
   owner-only `/media` input (`missing_facet`, the completeness sorts). Two build notes were folded in:
   visibility first (D3) and `mediaFilterFor` (D3, D7). An implementation review is still due before
   merge.
10. [ ] Update the ADR index; mark ADR-104 D3's "no `frozen_query`" as superseded here.
