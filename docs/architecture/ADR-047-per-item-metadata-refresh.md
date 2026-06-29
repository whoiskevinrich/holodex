# ADR-047: Per-item Metadata Refresh — forced re-extract + re-enrich as one owner action

**Status**: Proposed
**Date**: 2026-06-28
**Deciders**: Project owner
**Relates to**: [ADR-004](ADR-004-metadata-extraction.md) (exiftool/ffprobe extraction), [ADR-018](ADR-018-scanner-change-detection.md) (scanner `(size, mtime)` change-detection — this forces past it), [ADR-013](ADR-013-metadata-field-mapping.md) (configurable field mapping / precedence), [ADR-033](ADR-033-metadata-source-plugins.md) (enrichment sidecars + shadow store + on-demand ethos), [ADR-028](ADR-028-activity-surface-and-job-history.md) (`job_runs` activity history), [ADR-030](ADR-030-access-control-gating-seam.md) (owner gating), [ADR-037](ADR-037-soft-delete-and-purge.md) (soft-delete / #26 reactivation guard), [ADR-041](ADR-041-metadata-writeback.md) (the complementary *write* path). Spec: [Refresh Metadata / F31](../specs/metadata-refresh.md).

---

## Context

Holodex indexes each file once and re-reads it only when the periodic scan sees its `(size, mtime)`
change ([ADR-018](ADR-018-scanner-change-detection.md)). The owner routinely edits the same files
from **other systems** (a desktop tagger, another media manager, a script), which breaks two ways:
(1) some taggers rewrite tags **in place without bumping mtime**, so change-detection never re-reads
them; (2) even when mtime changes, there is no way to sync **one** file *now* short of a whole-library
pass. Separately, provider enrichment ([ADR-033](ADR-033-metadata-source-plugins.md)) is fetched once
and has **no re-fetch UI** (the F22 re-enrich follow-up was deferred).

F31 introduces a single owner action — **"Refresh metadata"** on one media item — that re-reads the
file **and** re-pulls the providers it is matched to. This ADR decides *how* that action is built:
its orchestration shape, the forced-extract seam, how a per-item job is recorded, its concurrency
story, and — critically — the seams that keep a **future batch** version (with conflict resolution)
implementable without rework, **without speccing batch now**.

Refresh is a **read** action (file + providers → Holodex's stores). Writing values *into* files is
the separate, separately-gated F28 writeback ([ADR-041](ADR-041-metadata-writeback.md)).

### Constraints

- **Forced, not opportunistic.** The headline value is catching external edits *including those that
  preserve mtime*, so refresh must re-extract **unconditionally**, bypassing the change-detection
  fast-path. (Resolved with owner; see F31 Resolved Decisions.)
- **Non-destructive layering is load-bearing.** File-extracted and provider-enriched data are
  separate layers merged at display time by the resolver ([ADR-013](ADR-013-metadata-field-mapping.md)
  precedence, F27). Refresh must never flatten them into one stored value — this invariant is what
  lets any future conflict-resolution policy exist without re-extraction.
- **Owner-gated.** Re-reading files and calling providers is owner-only
  ([ADR-030](ADR-030-access-control-gating-seam.md)), parity with rescan/enrich/writeback.
- **Soft-delete is untouchable.** A refresh must never reactivate or re-read a soft-deleted row
  ([ADR-037](ADR-037-soft-delete-and-purge.md), the #26 guard).
- **Single item in v1.** No bulk/library-wide forced re-read; but the design must not foreclose it.
- **Best-effort, resilient.** A single provider failure must not fail the file re-extract; a file
  failure must not corrupt the row or flip its `active` state.

---

## Decision

Build a dedicated **`refresh` service** (a new `internal/refresh` package) that orchestrates a
**forced single-file extract seam** and the existing **`enrich.Apply`** into **one** operation
returning a typed `RefreshReport`, exposed behind one owner endpoint. The service is internally split
into a side-effect-free **`plan`** phase and a committing **`apply`** phase.

### Orchestration: one unified operation, built on reusable seams

`Refresh(ctx, videoID) → RefreshReport` is the single public entry point. Internally:

```
plan(videoID)  → RefreshPlan        // no DB writes, no file writes
  1. load video; refuse if missing (404) or soft-deleted (409)
  2. force re-extract: exiftool + ffprobe on the file (bypass (size,mtime) fast-path)
  3. for each provider the item is matched to: enrich.Fetch using the persisted external_id
  4. diff incoming vs current per layer; compute per-field sources_disagree (file vs provider)
apply(plan)    → RefreshReport      // commit
  5. UpsertVideo(file layer) + video_metadata extras + cover-art flag   [file: layer only]
  6. enrich store upsert per provider                                   [{provider}: layer only]
  7. record one job_runs row (kind=refresh); return the report
```

F31 calls `plan` then `apply` back-to-back, so the split is invisible for the single-item flow. A
future batch (F31.11) runs `plan` across N items, interposes conflict resolution, then `apply`s —
which is the entire reason the phases are separable. **No public `plan` endpoint** ships in v1.

This is deliberately **not** a thin handler that calls two subsystems and lets each record its own
activity row (Option B): one owner click is one operation with one combined status and one activity
entry, and a batch driver wraps **one** `Refresh(id)` function rather than re-orchestrating two
subsystems and re-stitching two result types.

### Forced single-file extract seam

A new scanner method re-extracts **one** file unconditionally:

```go
// RefreshOne re-extracts a single file regardless of (size, mtime), updating the
// file: layer. It does NOT acquire scanMu (a one-file op must not wait behind a
// full-library scan) and never reactivates a soft-deleted row (ADR-037).
func (s *Scanner) RefreshOne(ctx context.Context, videoID int64) (FileExtractResult, error)
```

It reuses the existing pure `Extractor.Extract(ctx, path)` ([ADR-004](ADR-004-metadata-extraction.md))
and `repo.UpsertVideo`, bypassing only the change-detection check in the normal index path. The same
soft-delete guard the scanner already enforces applies. This seam is what a future bulk forced
re-extract (F31.11) iterates.

### Non-destructive layering (the invariant)

Re-extract writes **only** `videos.*` + `video_metadata` (the `file:` layer). Re-enrich writes
**only** the enrichment shadow store (the `{provider}:` layer). **No refresh code path writes a
resolved/merged value into either store as the stored truth.** The resolver (F27) remains the sole
merge point and re-runs at display time. This guarantees both raw layers always survive, so any
future conflict policy (operator picks file-vs-provider per field) is implementable without
re-fetching anything.

### Re-enrich reuses the persisted match

For each provider the item is matched to, refresh calls the existing per-item enrich apply with the
**stored `external_id`** — never the identity picker. An item with **no** match skips the provider
step cleanly (no error, no picker). A refresh never clears or changes a match. (Reuses the F22
`POST /media/{id}/enrich` machinery and its `entity_enrichment` external-id persistence.)

### `RefreshReport` and conflict detection

`apply` returns a typed result, per source and per field: `previous`, `incoming`, `winner`,
`changed`, and a derived **`sources_disagree`** flag (the file value and a provider value differ for
that field). Because the resolver discards losing candidates (`resolveField` breaks on first non-empty
source), `sources_disagree` is computed in the **refresh layer**, which already holds both inputs from
`plan` — **no resolver change, no extra I/O**. The report feeds (a) the HTTP response, (b) the
activity `detail` string, and (c) — later — a batch conflict-triage queue.

### Activity recording: flat `job_runs` row, no FK

Refresh appends **one** `job_runs` row, following the established per-entity pattern
([F22.6b](../specs/metadata-plugins.md), migration `0006_job_detail`): a new
`model.JobKindRefresh = "refresh"` constant, `trigger="manual"`, scan-count columns `0`, and a
free-text `detail` summarizing both halves, referencing the item as `#<id>`:

```
status=success  detail="#42 — file: 3 fields; tmdb: 5 fields"
status=error    detail="#42 — file: ok; tmdb: failed"     error_message="enrichment failed"
```

**No `video_id` column is added** — `job_runs` stays a denormalized event log; the entity lives in
`detail` as `#id`. The [ADR-028](ADR-028-activity-surface-and-job-history.md) **no-secrets invariant**
holds: detail carries provider name + entity id + counts only — never a filesystem path, env value,
or token (provider errors are flattened to a generic message). Recording is best-effort on a detached
context (like `recordEnrichJob`), so a failed/cancelled refresh still records.

### Partial-success contract

The operation is **202 Accepted**; the response body and the row status both derive from the one
`RefreshReport`:

- Row `status` is `success` only if the file re-extract **and** every attempted provider succeed;
  otherwise `error`, with `detail` naming the failed half.
- The response carries a per-source breakdown (`file: ok`, `tmdb: failed`) plus the changed-field
  summary — no 207-style multi-status; one combined status mirrored in both places.

A file-read failure (missing/locked file) fails the refresh **without** mutating the row's data or
`active` state. A provider failure fails only that provider's step; the file re-extract still commits.

### Concurrency

The single-file refresh **does not** acquire the global `scanMu` — a one-file op must not block
behind a 10k-file scan. Row safety rides on the existing single-statement `UpsertVideo` under
`repo.writeMu`. A small **per-item in-flight guard** (a `map[int64]struct{}` under a mutex, or
`singleflight`) de-dupes a double-click server-side, returning "already running" (mirroring
`TriggerRescan`'s `TryLock` semantics). A refresh racing a full scan is safe: both read the same file
and write the same derived data, so a race is redundant work, not corruption.

### API & auth

```
POST /api/v1/media/{id}/refresh              (behind requireOwner — ADR-030/046)

202 Accepted   { report summary: per-source status + changed-field counts }
401/403        no/valid owner auth (X-Admin-Token header or ADR-046 session cookie)
404            unknown id
409            id is soft-deleted (row exists, action disallowed — truer than 404)
```

The per-item path mirrors the existing `/media/{id}/enrich` and `/media/{id}/writeback` owner routes
(`mountEnrich`/`mountWriteback`); the `/admin/` group is reserved for library-wide operations
(`adminRescan`, status, activity). Mounted in the same `requireOwner` group as those siblings.

---

## Options Considered

### A — Unified refresh service over reusable seams, with plan/apply split (chosen)

A dedicated `refresh` service orchestrates a forced single-file extract seam and `enrich.Apply`,
returns one `RefreshReport`, records one `kind=refresh` activity row, and is internally split into
`plan` (no writes) and `apply` (commit).

**Pros:** One owner click = one operation, one combined status, one activity entry. The
`plan`/`apply` split + `RefreshReport` are exactly the seams a future batch-with-conflict-resolution
needs, banked at near-zero cost. Maximal reuse (existing extractor, `UpsertVideo`, `enrich.Apply`,
`job_runs`).

**Cons:** One new package and a `JobKindRefresh` constant + a small combined-status helper; the
sub-operations' own activity recording must be suppressed in the refresh path to avoid double-logging.

### B — Composed in the handler (rejected)

The handler calls the extract seam, then loops `enrich.Apply` per provider; each subsystem records
its **own** `job_runs` row.

**Pros:** Least new code; partial-success falls out (each row has its own status).

**Cons:** One click scatters 1 + N rows into the 30-day activity history; a partial failure has no
single place to live; "what changed" must be re-stitched from two result types; a future batch must
re-orchestrate two subsystems itself. The batch consideration is what tipped the decision to A.

### C — Reuse the full-library scanner (force a whole rescan) + re-enrich (rejected)

Trigger the existing `TriggerRescan` (or a forced variant) and a separate re-enrich.

**Pros:** No new single-file extract seam.

**Cons:** Re-walks the entire library to sync one file (slow, wasteful at scale), and `TriggerRescan`
respects change-detection so it would not even re-read an mtime-preserved edit without a deeper
change. Wrong granularity for a per-item action; provides no per-item report or conflict seam.

---

## Consequences

**What becomes easier**

- The owner syncs one externally-edited file on demand — including silent (mtime-preserved) edits —
  without a full rescan, and re-pulls its providers in the same click (closing the deferred F22
  re-enrich gap).
- A future **bulk forced re-extract with conflict resolution** (F31.11) layers on by driving the
  existing `plan`/`apply` seams across many items and filtering `RefreshReport`s for
  `sources_disagree` — no rework of the per-item path, because the non-destructive invariant keeps
  both raw layers intact for any policy.

**What becomes harder**

- A second activity `kind` (`refresh`) with count-columns-zero semantics joins `enrich`/`purge`; the
  activity UI must render it from `detail` (already true for enrichment).
- The "forced" path bypasses change-detection, so its tests must explicitly assert the mtime-preserved
  case (the one a normal scan can't catch).

**What we will need to revisit**

- **Bulk forced re-extract + conflict triage** (F31.11) — the batch driver and any throttling.
- **Per-item / per-field precedence override** — the most likely *new* store batch conflict
  resolution would add (operator pins "file wins" / "provider wins" for an item, overriding the
  global [ADR-013](ADR-013-metadata-field-mapping.md) precedence). Deliberately **not** built now;
  the non-destructive invariant is what lets it be added cleanly later.
- **`last_refreshed_at`** (F31.9) — a per-item timestamp distinct from `indexed_at`, if staleness
  visibility proves useful.
- **Thumbnail coupling** — refresh updates the cover-art flag but not the thumbnail image; if owners
  routinely run refresh then "Regenerate thumbnail" back-to-back, consider an opt-in chain.
