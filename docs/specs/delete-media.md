# Spec: Delete a Media Item (F24)

**Status**: Accepted
**Phase**: Media management (user-facing)
**Depends on**: the owner gate (ADR-030, `requireOwner`); scanner reconciliation (ADR-018);
the activity read-model / `job_runs` history (ADR-028).
**Related**: distinct from the scanner's `active=0` reconciliation (ADR-018) — that hides files
that *vanished from disk*; this hides files the *owner chose* to remove. Interacts with the
reactivation fast-path ([issue #26](https://github.com/whoiskevinrich/holodex/issues/26)).
**Architecture**: [ADR-037](../architecture/ADR-037-soft-delete-and-purge.md) — `deleted_at` axis
orthogonal to `active` + dedicated purge job.
**Design handoff**: [`docs/design/delete-media-handoff.md`](../design/delete-media-handoff.md).

---

## Objective

Let the **owner** remove a media item they no longer want in the library — with a safety net.
A delete is a **soft-delete** first: the item disappears from every view immediately but its file
and row survive a **configurable grace period** (default **7 days**), during which the delete can be
undone from a Trash view. After the grace period a background **purge job** hard-deletes the row and
the file from disk. The owner can also **purge now**, bypassing the grace period.

> **Why a separate delete state, not `active`.** The scanner already owns the `active` flag:
> `active=0` means "the file vanished from disk," and — as of [issue #26](https://github.com/whoiskevinrich/holodex/issues/26) — the
> change-detection fast-path **reactivates** an `active=0` row the moment its file reappears
> unchanged. A user-initiated delete must be orthogonal to disk presence: the file is usually
> *still on disk* when the owner deletes it, so storing the delete in `active` would be undone by
> the very next scan. F24 introduces a distinct `deleted_at` timestamp; a row is library-visible
> only when `active = 1 AND deleted_at IS NULL`, and the scanner treats a soft-deleted row as
> untouchable (never reactivates, re-extracts, or clears it).

> **Why owner-gated.** Deletion is destructive (eventually removes files from disk). It mounts
> inside the same `requireOwner` choke point (ADR-030) as enrichment, aliases/merge, and admin
> rescan. Non-owners never see delete controls and the endpoints reject them (`401`).

---

## Scope

### In scope
- **Soft-delete** a media item (owner-gated), behind a confirm dialog. The item is hidden from
  **every** read surface immediately: browse/list, detail, global search, related shelves, a
  person's/tag's media list, facets/metadata-keys counts, and the MCP search tools.
- **Soft-delete survives re-scans.** The scanner resolves a soft-deleted row to "leave exactly as
  is" — it is never reactivated, re-extracted, or re-surfaced while its file remains on disk.
- **Grace period + purge job.** A configurable window (default 7 days) after which a periodic
  background job **hard-deletes** expired items: the row (cascading its people/tags/metadata
  junctions and FTS rows) and, when enabled, the file on disk. Each purge pass is recorded in the
  activity history (`job_runs`, ADR-028) like a scan.
- **Purge now (owner override).** Skip the grace period and hard-delete a single item immediately.
- **Restore within grace.** Undo a soft-delete before it is purged: clear `deleted_at`, the item
  returns to all views. Reachable via a **Trash** view of soft-deleted items (owner-only).
- **Graceful disk-delete failure.** If a file can't be removed (read-only mount, permission), the
  purge logs it, leaves the row marked for retry, and surfaces it in activity — the library is
  never left in a half-deleted state where the row is gone but the file lingers.
- **Capabilities-gated UI.** `GET /capabilities` already reports `owner`; delete/restore/purge
  controls render only for the owner (consistent with enrich/merge controls).

### Out of scope (tracked follow-ups, not gaps)
- **Bulk delete / multi-select delete** from the browse grid. v1 deletes one item at a time;
  the people-list multi-select (F23) is the pattern to mirror later.
- **Auto-delete rules** (e.g. "purge anything not played in N months"). There is no play/engagement
  signal in a personal library; manual curation only.
- **Trashing whole people/tags/folders.** F24 deletes *media items*; entity cleanup is separate.
- **Two-tier "Trash then Recycle Bin"** or per-item custom grace windows. One global grace window.
- **Undo after purge.** Once hard-deleted (file + row gone), it is gone; recovery is from the
  user's own backups. The grace period *is* the safety net.
- **Audit log of who deleted what.** Single-owner model today (ADR-030); revisit with multi-user.

---

## Functional Requirements

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F24.1 | The owner can **soft-delete** a media item. | `DELETE /media/{id}` (owner-gated) sets `deleted_at` to now and returns `204`. A second delete of the same id is idempotent (`204`, `deleted_at` unchanged). `404` for an unknown id. |
| F24.2 | A soft-deleted item is **hidden from every read surface**. | After delete, the item is absent from `GET /media`, `/media/{id}` (→ `404`), `/search`, `/media/{id}/related` (as subject *and* as a candidate), `/people/{id}` and `/tags/{id}` media lists, `/facets`, `/metadata-keys`, and the MCP `search_media` tool. Counts/totals exclude it. |
| F24.3 | **Soft-delete survives re-scans.** The scanner leaves a soft-deleted row untouched. | With a soft-deleted row whose file is still on disk, a full re-scan does **not** clear `deleted_at`, reactivate it, re-extract it, or re-surface it — and does not count it as `added`/`updated`/`skipped` noise beyond a single "seen, ignored" path. (Guards against the #26 reactivation fast-path resurrecting a deleted item.) |
| F24.4 | **Grace-period purge.** A background job hard-deletes items whose `deleted_at` is older than the configured grace period. | With grace = `168h` (7d), an item soft-deleted 8 days ago is, on the next purge tick: its file removed from disk (when file-removal is enabled), its `videos` row deleted (cascading `video_people`/`video_tags`/`video_metadata` and FTS mirrors), and the pass recorded as a `job_runs` row (`kind=purge`) with a count. An item deleted 1 day ago is untouched. |
| F24.5 | The owner can **purge now**, bypassing the grace period. | `DELETE /media/{id}?purge=true` (owner-gated) hard-deletes immediately (file + row), returns `204`. Works whether or not the item was already soft-deleted. |
| F24.6 | The owner can **restore** a soft-deleted item before it is purged. | `POST /media/{id}/restore` (owner-gated) clears `deleted_at`; the item returns to all views. `404` if the id isn't soft-deleted (nothing to restore). |
| F24.7 | The owner can **view the Trash** (soft-deleted, not-yet-purged items). | `GET /admin/trash` (owner-gated) lists soft-deleted items with `deleted_at` and the computed `purge_at`. Non-owners get `401`. |
| F24.8 | **Disk-delete failures degrade gracefully.** | If file removal fails (read-only FS, permission, missing file): a *missing* file is treated as success (already gone → finish the row delete); a *permission/read-only* failure leaves the row soft-deleted, logs a warning, records the purge pass with an error count, and is retried on the next tick. The library is never left with a deleted row but a surviving file, or vice-versa, silently. |
| F24.9 | **Owner-gated end to end.** | All mutating endpoints sit inside the `requireOwner` group; with a token configured, missing/invalid token → `401`. `GET /capabilities` drives the UI so non-owners never see delete/restore/purge controls. |
| F24.10 | **Confirm before delete.** | The UI requires an explicit confirm step before issuing a soft-delete; "purge now" requires a distinct, stronger confirm naming the irreversibility. |

### Validation & semantics
- `deleted_at` is an ISO-8601 UTC timestamp (same format as `indexed_at`/`file_mtime`).
- Visibility predicate is `active = 1 AND deleted_at IS NULL`, applied wherever `active = 1` is
  today (the read path is the single seam — see Non-functional).
- `purge_at = deleted_at + grace_period` is computed, not stored, so changing the config reflows
  pending purges without a migration.
- Grace period `0` (or unset) disables **automatic** purge — items stay in Trash until the owner
  purges them manually (a conservative default worth weighing in Open Questions).

---

## Data model

```
videos
  + deleted_at  TEXT NULL          -- ISO-8601 UTC; NULL = live, set = soft-deleted
  INDEX idx_videos_deleted_at ON videos(deleted_at)   -- backs the purge sweep + Trash list

-- Visibility everywhere becomes:  active = 1 AND deleted_at IS NULL
-- (active = disk presence, ADR-018; deleted_at = owner intent, F24 — orthogonal axes)
```

Migration **0010** (next free after `0009_person_images`; the F25 person-images work merged `0009`
after this spec was drafted, so what the spec first sketched as "0008" is now `0010` — golang-migrate
only applies versions *above* the current one, so a retroactive lower number would never run). See
[ADR-037](../architecture/ADR-037-soft-delete-and-purge.md). `deleted_at` is additive and nullable — existing rows
default to live (`NULL`). Hard-delete relies on the existing `ON DELETE CASCADE` foreign keys
(`video_people`/`video_tags`/`video_metadata`) and the `videos_ad` FTS trigger, so purging a row
cleans up its junctions and search index automatically.

**Scanner seam (F24.3).** `VideoStat`/`StatByPath` — already extended with `Active` for #26 — gain
`Deleted bool` (`deleted_at IS NOT NULL`). The scanner's `index()` short-circuits a soft-deleted
row *before* the change-detection fast-path: record it as seen (so reconciliation doesn't touch it)
and return, never reactivating or re-extracting. `UpsertVideo`'s `ON CONFLICT` branch must **not**
clear `deleted_at` (it stays soft-deleted even if the file changes on disk).

---

## API

All under `/api/v1`. Reads ungated (but soft-deleted items are invisible to them); mutations and the
Trash list inside the `requireOwner` group (ADR-030).

| Method | Path | Gating | Body | Returns |
|--------|------|--------|------|---------|
| DELETE | `/media/{id}` | owner | — | `204` — soft-delete (idempotent) |
| DELETE | `/media/{id}?purge=true` | owner | — | `204` — hard-delete now (file + row) |
| POST | `/media/{id}/restore` | owner | — | `200 {media}` — clears `deleted_at` |
| GET | `/admin/trash` | owner | — | `{items:[{…, deleted_at, purge_at}], total}` |

Errors: `404` unknown id (or `/restore` on a live item); `401` unauthorized (gated); `409`/`500`
surfaced if a `purge=true` disk removal fails on a permission/read-only error (the row stays
soft-deleted, message explains).

**Purge job** is internal (no endpoint): a periodic sweep mirroring the scanner's lifecycle
(`internal/purge` or a scanner-adjacent ticker), interval/grace from config, server-lifetime
context, recorded via the `JobRecorder` seam (ADR-028, `kind=purge`).

---

## Configuration

| Key | Default | Meaning |
|-----|---------|---------|
| `DELETE_GRACE_PERIOD` | `168h` (7d) | How long a soft-deleted item lingers before auto-purge. `0` disables auto-purge (manual only). |
| `DELETE_REMOVE_FILES` | `true` | Whether purge removes the file from disk. `false` = DB-only purge, leaves files (for read-only/externally-managed libraries). |
| `PURGE_INTERVAL` | `1h` | How often the purge sweep runs. |

Documented in `holodex.yaml.example`. On a read-only media mount, set `DELETE_REMOVE_FILES=false`
so a delete is an app-level hide + DB purge without a doomed `unlink`.

---

## UX

- **Detail page** (`/media/[id]`): owner-only "Delete" action (token-styled, `--warn`) → confirm
  dialog ("Move to Trash? You can restore it within {N} days.") → soft-delete → toast + navigate
  back to the grid (the item is gone from it). A secondary "Delete permanently" requires a stronger
  confirm naming the irreversibility and the file path.
- **Trash view** (owner-only, e.g. `/trash` or an admin section): lists soft-deleted items with
  "deleted {when} · purges {when}", each with **Restore** and **Delete permanently**. Empty state
  themed. Linked from the owner's header/admin affordance, not shown to non-owners.
- All-tokens styling (`--warn` for destructive affordances, never a hardcoded red); QA in all three
  skins. Full layout, states, a11y (confirm focus-trap, destructive-button labeling) in the design
  handoff.

---

## Non-functional

- **Single read-path seam.** Visibility (`active = 1 AND deleted_at IS NULL`) must be applied in one
  place in the repo's query builder, not sprinkled per-handler, so no read surface can forget it.
  The PR's burden of proof is that *every* listed surface (F24.2) excludes soft-deleted rows —
  covered by tests, not just the common list path.
- **Scan cost**: one extra nullable column on the existing `StatByPath` read (negligible) plus an
  early skip for soft-deleted rows — strictly less work than indexing them.
- **Purge cost**: one indexed range scan (`deleted_at < cutoff`) per tick, then per-item unlink + a
  cascading `DELETE`. Bounded by what the owner deleted; idle when Trash is empty.
- **Durability**: soft-delete is a single timestamp write; it survives restarts and re-scans by
  construction. Hard-delete is irreversible by design — the grace period is the safety margin.
- **Safety**: disk removal is gated behind both the grace period (or an explicit purge-now confirm)
  and `DELETE_REMOVE_FILES`; a failed unlink never desynchronizes row and file.

---

## Decisions (settled 2026-06-16, Kevin)
- **Grace = 7 days, auto-purge ON.** Soft-deleted items hard-delete automatically 7 days after
  deletion; Trash stays bounded. (`DELETE_GRACE_PERIOD=168h`.)
- **Remove files by default** (`DELETE_REMOVE_FILES=true`) — "delete means delete." Read-only
  mounts degrade gracefully (F24.8) and can opt into DB-only purge via the flag.
- **Trash view + restore are in v1 (P0).** The grace window only has value if the owner can see and
  recover items during it.

## Open questions
1. **Purge job home** — a dedicated `internal/purge` ticker vs. folding the sweep into the scanner's
   periodic pass. Reusing the scanner's lifecycle is less machinery; a separate job keeps concerns
   clean. *(engineering — settle in ADR-037)*
2. **Stream/thumbnail during grace** — a soft-deleted item is hidden from listings, but should a
   direct `/media/{id}/stream` or `/thumbnail` by id also 404? (Spec says yes via the shared
   predicate; confirm no internal caller needs the bytes mid-grace.) *(engineering)*
