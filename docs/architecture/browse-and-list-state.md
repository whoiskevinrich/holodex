# Browse and list state

This doc owns how a set of videos is queried, ordered and searched, where a list page keeps its
state, and how playlists hold or re-run such a set. FTS5 as the engine is in [stack.md](stack.md);
the `active` / `deleted_at` read seam every query here flows through is in
[media-ingest.md](media-ingest.md); the playlist visibility read gate and the owner-only input checks
are in [security-perimeter.md](security-perimeter.md).

## One clause builder: `VideoFilter.build()` + `orderBy()`

Every video list reads through `repo.VideoFilter`: `build()` assembles the WHERE clause (starting
with the visibility seam), `orderBy()` the ORDER BY from a whitelisted sort key. Three readers share
them, so membership and order cannot drift: `ListVideos` (paged, capped at `maxListLimit`),
`ListAllVideos` (full rows, for scans) and `ListVideoIDs` (ids only, uncapped, in order). The query string reaches a `VideoFilter` only through `videoFilterFromQuery`, wrapped by
`mediaFilterFor`, which also sets `HideFullFilmVideos` and `MissingFacets` and refuses owner-only
inputs for a visitor. It is the one path from a query to a filter for `listMedia`, `/media/ids` and
smart playlist reads. Person, tag and studio grids load through `GET /media?person=|tag=|studio_id=`
with browse's paging, not a bespoke capped endpoint, so every query-backed grid's query is a plain
`/media` filter. `sort` is validated against a fixed per-endpoint set before `orderBy()`; no raw
string reaches the ORDER BY. The client's sort vocabulary is `MEDIA_SORTS` in `web/src/lib/filters.ts`.

**Rejected:** a flag on `ListVideos` for uncapped id reads — a sibling reader over the same builder
keeps the page reader's cap intact.

Decided in [`8ea55a0e`](https://github.com/whoiskevinrich/holodex/commit/8ea55a0e).

## Seeded shuffle: `holo_shuffle(id, seed)` for paged lists, client PRNG for unpaged

A paged random sort must tile across `LIMIT`/`OFFSET` windows, which `ORDER BY RANDOM()` cannot.
`internal/db/shuffle.go` registers `holo_shuffle(id, seed)` (a splitmix64-style mix in Go) as a
deterministic scalar function at DB open, so every connection has it. `sort=random` orders by
`holo_shuffle(v.id, :seed), v.id`; the trailing `v.id` makes the order total. `seed` is an `int64`
bound parameter; the client sends its session seed (`sortPreference.svelte.ts`, `sessionStorage`)
on the API call, never in the page URL. Unpaged lists (People, Studios, Films, Tags) return canonical
order and shuffle in the client with `seededShuffle` (`web/src/lib/shuffle.ts`) under the same seed.

**Rejected:** inline arithmetic hash in SQL — lumpy distribution, untestable. **Rejected:** a
materialized per-seed ordering table — state and eviction for a cosmetic sort.

Decided in [`f483a0b9`](https://github.com/whoiskevinrich/holodex/commit/f483a0b9).

## FTS prefix queries for the filter `q` and the global search

Title search in the clause builder is `v.id IN (SELECT rowid FROM videos_fts WHERE videos_fts MATCH ?)`.
Global search (`GET /api/v1/search`, `Repo.Search`) is a separate, grouped mixed-entity reader over the
name FTS tables (`people_fts`, `tags_fts` and siblings), capped per group. Both build the MATCH
argument with `ftsPrefixQuery`: each whitespace token has `"` stripped and becomes `"token"*`, so
user input can never inject FTS5 operators and results update as the user types. The tables use
`unicode61` with `remove_diacritics=2` and are kept in sync by triggers in migrations.

**Rejected:** `LIKE '%term%'` — no tokenization, no diacritic folding, slow at library scale.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## Related media: one endpoint, seedless `ORDER BY RANDOM()` kept out of the builder

`GET /media/{id}/related` (`Repo.Related`) returns a person block and a tag block in one response.
The tag key is the item's tag maximizing `c · (1 − c / N)` (global count `c`, active total `N`);
the person key is the highest global count. Each block's items are a `... AND v.id != ? ORDER BY
RANDOM() LIMIT ?` select. This is the project's only `ORDER BY RANDOM()`, kept local to `Related`
so the general list path stays deterministic. Rule: **seedless for single-draw fetch-and-hold,
seeded for paginated.** Stability per page view is the client holding the response.

**Rejected:** a seed for related — fetch-and-hold already gives stability.

Decided in [`296dbe1b`](https://github.com/whoiskevinrich/holodex/commit/296dbe1b).

## Resolution buckets computed at read time from width

The resolution bucket is never stored. `internal/metadata/resolution.go` holds the thresholds:
`ClassifyResolution(width)` labels a video, and `ResolutionWidthRange(bucket)` turns a
`?resolution=` filter into an index-friendly `[min, max)` width predicate. Buckets, by frame width
with a 10% downward tolerance off each nominal tier (1280 / 1920 / 3840): **SD** `< 1152`,
**HD** `1152–1727`, **FHD** `1728–3455`, **4K** `≥ 3456`. Changing a boundary is a constant edit with
no reindex. The `resolution_*` sorts order by width to match.

**Rejected:** bucketing by height — a 2.39:1 4K master (3840×1606) would read as FHD.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## List state: the URL holds the view, storage holds only preferences

`web/src/lib/listState.ts` is the one contract for every list page: each page declares a schema, and
the module provides `parse(url)`, canonical `serialize` (defaults omitted, keys sorted), `commit` and
`exitAfterRemoval`.

- **Query** keys (filters, entity-scope chips, Tags `type`, `q`) live in the URL only. `sort` lives
  in the URL and localStorage; view and density in localStorage only; the seed in the session.
- State derives **reactively from `page.url`**, not once in `onMount`, because SvelteKit reuses the
  component on a same-route navigation. `sort` precedence: URL, then storage, then default; a
  non-default resolved sort is written back into the URL.
- Storage is written only from a control's change handler, never by an effect over derived state, so
  a shared link never overwrites a preference. `commit` is the single SvelteKit `replaceState` (never
  raw `history.replaceState`).
- After a destructive action, `exitAfterRemoval(listHref)` is `history.back()` for an in-app visit,
  otherwise `goto` the bare list.

**Rejected:** storage as source of truth with the URL as mirror — shared links fight stored state,
filters reappear hidden, tabs clobber each other. **Rejected:** a `?returnTo=` param or a
sessionStorage "last list" for the removal exit — open-redirect surface or a stale second copy of
history.

Decided in [`de97c861`](https://github.com/whoiskevinrich/holodex/commit/de97c861).

## Snapshots keyed by the canonical query string; scroll survives reload

Two snapshot stores restore a list on Back, both keyed by the list's canonical query string, so a
snapshot restores only into exactly the same view:

- **`browseCache`** (`web/src/lib/browse.svelte.ts`) — the Media grid's loaded pages and scroll in a
  module-scoped store, seeded **synchronously** on mount so page height exists before the scroll
  restore (keep it synchronous). Memory only: a full reload rebuilds from the URL.
- **`listScroll`** (`createNavSnapshotRegistry`, `navSnapshot.svelte.ts`) — scroll-only slots for the
  other lists, Search and entity video grids, mirrored to `sessionStorage` under `${namespace}:${id}`.
  `take` reads memory then storage and clears both. Storage access is best-effort `try/catch`.

**Rejected:** SvelteKit `snapshot` for the Media grid — serializing hundreds of videos per navigation.
**Rejected:** localStorage for scroll — shared across tabs, outlives the history it restores into.

Decided in [`8d8ce859`](https://github.com/whoiskevinrich/holodex/commit/8d8ce859).

## Playlists are containers: `playlists` + `playlist_videos`, membership plus a sort

A playlist is not an entity: own tables, repo methods and handlers, no baseline, alias, external-id,
enrichment, resolver or completeness row. It borrows only the `kind:id` handle (`model.KindPlaylist`).

- `playlist_videos` is a set (`PRIMARY KEY (playlist_id, video_id)`) with a `position`.
- `playlists.sort` is any browse sort or `manual` (validated at the API against `repo.ValidSort`
  plus `manual`; no `CHECK`). Read order is `orderBy()` over the membership join; `manual` is
  `position ASC`.
- **Snapshot producer:** `POST /playlists` with `from_query` goes through the browse parser into
  `ListVideoIDs`, then one chunked `INSERT OR IGNORE` under `writeMu`, `position` = rank. A `random`
  source stores `manual` with the order on screen. `from_query` reaches SQL only as a `VideoFilter`.
- Purge removes membership rows by `ON DELETE CASCADE`; trash is decided by the read seam.

**Rejected:** an identity-spine entity — the spine reconciles several sources; a playlist has one.
**Rejected:** a tag with a flag — tags are file metadata and get written back.

Decided in [`8ea55a0e`](https://github.com/whoiskevinrich/holodex/commit/8ea55a0e).

## Smart playlists store the canonical browse query and re-run it on read

A playlist is smart iff `playlists.query IS NOT NULL`; it then has no `playlist_videos` rows, and
membership writes and `sort = manual` are refused. *Freeze* evaluates once, writes membership and
nulls `query` in one transaction.

- `canonicalPlaylistQuery` is the only writer of `query`: it rejects unknown keys, strips `sort`,
  `limit`, `offset`, `seed`, and normalizes (keys sorted, ids de-duplicated and sorted). Sort lives in
  `playlists.sort`.
- The filter grammar is persisted: `query_version` is bumped, with a migration rewriting stored
  queries, only when an existing key changes meaning. New keys and operators need no bump.
- Reads evaluate live through `mediaFilterFor`, `ListVideos` (tiles) and `ListVideoIDs` (ordered ids).
  No cache. Visibility and owner-only posture are in [security-perimeter.md](security-perimeter.md).
- Merges call `rewriteSmartPlaylistRefs` inside their transaction. At read, a dead id facet or a
  vanished mapped key yields `stale_refs` and **no evaluation**, never a silently broadened set.

**Rejected:** a structured JSON rule tree — a second grammar to keep in step. **Rejected:** a side
table of referenced ids — a second copy of the query that can drift.

Decided in [`8ea55a0e`](https://github.com/whoiskevinrich/holodex/commit/8ea55a0e).

## Runs are client-held state over an id list fetched once

Play all and Shuffle are a run (`web/src/lib/run.ts`, `runContext.ts`): a source (a query, a
playlist or a film), the source-order ids fetched once (`GET /media/ids` or the playlist's ordered
ids), and a play order. Shuffle is a seeded Fisher–Yates (`mulberry32`) in the client, so a mid-run
toggle needs no request. The run lives in `sessionStorage`; the URL carries the run id plus its
source so a lost run is rebuilt. Runs never write to the server. `playlists.play_shuffled` only tells
the client which mode to start in.

**Rejected:** server `sort=random&seed` for shuffle mode — two orders from two requests could
disagree. **Rejected:** server-side runs — a table and a write per play for per-tab state.

Decided in [`8ea55a0e`](https://github.com/whoiskevinrich/holodex/commit/8ea55a0e).
