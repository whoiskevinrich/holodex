# ADR-037: Soft-delete media — a `deleted_at` axis orthogonal to `active`, swept by a dedicated purge job

**Status**: Accepted
**Date**: 2026-06-17
**Deciders**: Project owner
**Extends**: ADR-018 (scanner change-detection / write path), ADR-028 (activity surface & `job_runs` history), ADR-030 (owner gate), ADR-016 (embedded migrations), ADR-014 (configuration)
**Spec**: [Delete a Media Item (F24)](../specs/delete-media.md)

---

## Context

The owner wants to remove a media item they no longer want in the library, with a safety net: a
delete should be undoable for a grace period, then permanently remove the row **and the file**. The
spec (F24) settled the product shape — soft-delete first, 7-day grace, auto-purge on, files removed
by default, a Trash view with restore. This ADR settles the **technical** questions the spec left
open.

The library already has a `videos.active` flag owned by the scanner (ADR-018): `active = 0` means
"the file vanished from disk." As of [issue #26](https://github.com/whoiskevinrich/holodex/issues/26)
the change-detection fast-path **reactivates** an `active = 0` row the moment its file reappears
unchanged. So `active` tracks *disk presence*, and a sweep at the end of each scan
(`DeactivateExcept`) reconciles it.

Three questions follow:

1. **Where does a user-initiated delete live in the data model?** It cannot live in `active`: the
   file is usually *still on disk* when the owner deletes it, so the very next scan's reactivation
   fast-path (#26) would resurrect it. Delete intent and disk presence are independent axes.
2. **Where does the purge sweep run?** A periodic job must hard-delete rows whose grace has expired.
   Fold it into the scanner's existing periodic pass, or stand up a dedicated ticker?
3. **How is a soft-deleted item hidden from *every* read surface** without sprinkling the predicate
   across handlers — including direct-by-id surfaces (`GET /media/{id}`, stream, thumbnail) that
   today bypass the `active = 1` filter?

## Decision

### 1. A new `deleted_at` timestamp column — orthogonal to `active`

Add `videos.deleted_at TEXT NULL` (ISO-8601 UTC, the same format as `indexed_at` / `file_mtime`).
`NULL` = live; a set timestamp = soft-deleted. A row is **library-visible** iff
`active = 1 AND deleted_at IS NULL`. The two axes are independent:

| | `deleted_at IS NULL` | `deleted_at` set |
|---|---|---|
| **`active = 1`** (on disk) | live — visible everywhere | soft-deleted, file still present (the common case) |
| **`active = 0`** (vanished) | scanner-deactivated (#26 can reactivate) | soft-deleted *and* file gone |

```sql
ALTER TABLE videos ADD COLUMN deleted_at TEXT;            -- NULL = live
CREATE INDEX idx_videos_deleted_at ON videos(deleted_at); -- backs purge sweep + Trash list
```

`purge_at = deleted_at + grace_period` is **computed, never stored**, so changing the grace config
reflows every pending purge without a migration or a backfill.

**Migration `0010`** (next free after `0009_person_images`). The spec drafted this as "0008," but
person-images (F25) merged `0009` in the interim; golang-migrate's `m.Up()` only applies versions
**greater** than the current schema version, so a retroactive `0008` below an already-applied `0009`
would silently never run. `0010` is the correct next ordinal. The column is additive and nullable —
existing rows default to live.

### 2. The scanner treats a soft-deleted row as untouchable

`StatByPath` (already extended with `Active` for #26) gains `deleted_at`, surfaced to the scanner as
`VideoStat.Deleted bool`. The scanner's `index()` **short-circuits a soft-deleted row before the
change-detection fast-path**: record it as seen (so end-of-scan reconciliation never deactivates it)
and return immediately — never reactivate, never re-extract, never re-surface. `UpsertVideo`'s
`ON CONFLICT` branch must **not** clear `deleted_at` (a soft-deleted file that changes on disk stays
soft-deleted). This is the load-bearing invariant: **a soft-delete survives every re-scan while its
file is on disk**, which is exactly what storing the intent in `active` could not guarantee.

The scanner never *creates* a `deleted_at` and never *clears* one. Soft-delete is owner-driven
(API) and the purge job is the only thing that removes the row. The scanner's sole responsibility is
to leave soft-deleted rows alone.

### 3. A dedicated `internal/purge` ticker — not folded into the scanner

Stand up a small `internal/purge.Purger` with its own ticker, mirroring the scanner's lifecycle
(constructed in `main`, `Run(ctx)` on a goroutine, `JobRecorder` seam from ADR-028). Resolving spec
open question 1 **in favour of a separate job**:

- **Independent cadence and independent of scanning.** Purge runs on `PURGE_INTERVAL` (default 1h),
  unrelated to the scan interval, and must run **even when scanning is disabled** (`MEDIA_PATH`
  empty) or when `DELETE_REMOVE_FILES=false` on a read-only mount. Folding it into the scan tick
  would couple two unrelated clocks and skip purges whenever scanning is off.
- **Clean separation of concerns.** The scanner reconciles disk→DB (presence). The purge job
  reconciles grace-expiry→hard-delete (intent). They share nothing but the `JobRecorder`.
- **Cost is trivial.** One more goroutine and one indexed range scan
  (`WHERE deleted_at < cutoff`) per tick; idle when Trash is empty.

Each pass is recorded as a `job_runs` row with a new `kind = "purge"` (ADR-028, `model.JobKindPurge`),
counting items purged and disk-removal errors — so a purge shows up in the activity history exactly
like a scan.

### 4. One read-path visibility seam; direct-by-id surfaces join it

The list/count seam already funnels through `VideoFilter.build()`; its single `v.active = 1` clause
becomes `v.active = 1 AND v.deleted_at IS NULL`. Every surface built on `ListVideos` /
`namedCountQuery` (browse, search, people/tag media, facets, metadata-keys, related shelves, the MCP
`search_media` tool) inherits the predicate for free.

The **direct-by-id** surfaces that bypass `build()` today —`GetVideo` (`/media/{id}`),
`PathByID` (stream), the thumbnail-by-id read, and `Related` as subject — must each apply the same
`deleted_at IS NULL` guard so a soft-deleted item 404s when fetched, streamed, or thumbnailed by id
(resolving spec open question 2: **yes**, hide the bytes during grace; the purge job reads the path
through its own un-gated query, so no internal caller is starved). A dedicated `PurgeCandidates` /
path read inside `internal/repo` is the *only* code allowed to see soft-deleted rows.

The PR's burden of proof (per the spec's non-functional seam requirement) is a test that **every**
listed surface excludes a soft-deleted row — not just the common list path.

### 5. Owner-gated mutations; reads stay public but blind

`DELETE /media/{id}` (soft-delete), `DELETE /media/{id}?purge=true` (hard-delete now),
`POST /media/{id}/restore`, and `GET /admin/trash` all mount inside the existing `requireOwner`
group (ADR-030) — no access-model change. Reads remain ungated but cannot see soft-deleted rows by
construction (§4). `GET /capabilities` already reports `owner`, so delete/restore/purge controls
render only for the owner, exactly like the enrich/alias/merge controls.

### 6. Hard-delete relies on existing cascades; disk removal degrades gracefully

Purging a row is a single `DELETE FROM videos WHERE id = ?`. The existing `ON DELETE CASCADE` foreign
keys (`video_people` / `video_tags` / `video_metadata`) and the `videos_ad` FTS trigger clean up the
junctions and search index automatically — no bespoke teardown. Order per item: **remove the file
first** (when `DELETE_REMOVE_FILES=true`), then delete the row, so the library is never left with a
deleted row but a surviving file. Disk-removal outcomes:

- **File already missing** → treated as success; finish the row delete (the desired end state is
  "gone").
- **Permission / read-only failure** → leave the row soft-deleted, log a warning, count it as an
  error in the `job_runs` row, and retry on the next tick. Never delete the row while the file
  survives.

## Rationale

- **Why a column, not a `deleted` lifecycle table.** A single nullable timestamp answers all three
  questions F24 asks of the data — *is it deleted?* (`IS NOT NULL`), *when?* (the value), *should it
  purge yet?* (`< cutoff`) — with one indexed predicate and zero joins on the read hot path. It
  composes with the existing `active` column as a plain `AND`. A separate table would add a join to
  every read for no expressive gain in a single-owner model with no per-item delete metadata
  (audit-of-who is explicitly out of scope until multi-user).
- **Why `deleted_at` and `active` must stay separate** is the cardinal lesson of #26: the
  reactivation fast-path keys on disk presence. Any delete stored in `active` is undone by the next
  scan of a still-present file. Orthogonal axes make the soft-delete re-scan-safe *by construction*,
  not by a guard that a future scanner change could forget.
- **Why a dedicated purge job over folding into the scanner.** The two jobs answer to different
  clocks and different preconditions (purge must run with scanning off). The spec leaned toward
  "separate keeps concerns clean"; the only cost is one goroutine, which the scanner's own lifecycle
  shows is cheap and well-understood. Reusing the `JobRecorder` keeps the activity surface uniform.
- **Why route every direct-by-id surface through the predicate.** A soft-deleted item that still
  streams or thumbnails by guessed id would leak the very bytes the owner asked to hide, and would
  desync "it's in the Trash" from "I can still play it." Hiding by id during grace keeps the model
  coherent; the grace window's safety value is the *Trash UI restore*, not a back-door byte fetch.
- **Why `purge_at` is computed.** Storing it would freeze the grace window at delete time; computing
  it lets the owner shorten or lengthen `DELETE_GRACE_PERIOD` and have every pending item reflow,
  with no migration.
- **Configuration in seconds, not duration strings.** The spec wrote `168h` / `1h` for readability,
  but the entire config surface (ADR-014) uses `*_SECONDS` ints with the `envInt` parser
  (`SCAN_INTERVAL_SECONDS`, `SCAN_MIN_AGE_SECONDS`, …). To stay consistent rather than introduce a
  one-off duration parser, the keys are `DELETE_GRACE_PERIOD_SECONDS` (default `604800` = 7d),
  `PURGE_INTERVAL_SECONDS` (default `3600` = 1h), and the bool `DELETE_REMOVE_FILES` (default
  `true`). `DELETE_GRACE_PERIOD_SECONDS = 0` disables **automatic** purge (Trash holds until the
  owner purges manually).

## Consequences

- **Migration `0010` is additive**: one nullable column + one index; the `down` drops both. No change
  to any existing row, trigger, or FTS table. Existing rows are live (`NULL`).
- **The visibility predicate now lives in two shapes**: the shared `build()` clause (covers the bulk
  of surfaces) **and** an explicit `deleted_at IS NULL` guard on each direct-by-id read. The latter is
  the deliberate cost of SQLite not having row-level views; tests enumerate every surface so none is
  forgotten. If the by-id reads ever multiply, a repo-level `liveVideo(id)` helper is the place to
  centralize them.
- **One new background goroutine** (`Purger.Run`) on server lifetime, alongside the scanner's. It is
  idle when Trash is empty and bounded by what the owner deleted when not. A `purge` row joins
  `scan`/`enrich` in the activity history (ADR-028) — the UI's job-kind rendering gains one label.
- **Hard-delete is irreversible by design.** Once a row + file are gone the grace window was the only
  safety net; recovery is from the owner's own backups (spec: undo-after-purge is out of scope).
- **Disk and DB never desync silently.** A failed unlink keeps the row soft-deleted and surfaces the
  error in activity; the item is retried, never half-deleted.
- **Read-only / externally-managed libraries** set `DELETE_REMOVE_FILES=false`: delete becomes an
  app-level hide + DB purge, leaving files on disk for the external manager. The grace/Trash UX is
  unchanged.
- **MCP `get_video` gains the guard.** It calls `GetVideo`, which now refuses soft-deleted rows, so
  the MCP tool surface matches the web surface — a soft-deleted item is invisible to agents too.
- **Covered by tests** following the existing repo/handler/scanner patterns: soft-delete idempotency
  + 404; the *every-surface* invisibility sweep (F24.2); **the soft-delete surviving a re-scan with
  the file still on disk** (the cardinal invariant, guarding the #26 fast-path); the grace-cutoff
  purge with cascade cleanup + a `job_runs` row; purge-now; restore + the 404-on-live-restore; the
  graceful disk-removal failure paths (missing → success, permission → retry); and owner-gating
  (401) across all four endpoints.
