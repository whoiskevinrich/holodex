# ADR-041: Metadata Writeback — explicit per-field write-back to media files

**Status**: Proposed
**Date**: 2026-06-20
**Deciders**: Project owner
**Relates to**: [ADR-004](ADR-004-metadata-extraction.md) (exiftool extraction pipeline), [ADR-013](ADR-013-metadata-field-mapping.md) (configurable field mapping), [ADR-033](ADR-033-metadata-source-plugins.md) (metadata source plugins), spec [Metadata Source Plugins / F27](../specs/metadata-plugins.md) (unified field resolution).

---

## Context

F27 (unified field resolution) merges file-metadata and enrichment shadow-store data into a
precedence-ordered winning value per canonical field, and displays it in both the detail page
and browse cards. The winning value is **display-only**: it is never written back to the media
file. As a result, the enrichment shadow store must be queried on every read — a batch
`EnrichmentForVideos` query on every list page, and a per-file `EnrichmentForEntity` query on
every detail-page load — so that browse cards and detail panels show the enriched title,
overview, and so on.

The long-term clean state is for the *file itself* to carry the curated value. Once a TMDB
title is embedded in the file's own tags, `file:title` resolves correctly with no shadow-store
involvement; the enrichment infrastructure recedes to its correct role (an import tool, not a
permanent read-path fixture). Writing back also makes the value **portable** — it survives
database resets, shows up in other media tools, and does not silently disappear if the sidecar
is unavailable.

Three modes exist for how writeback could be triggered:

| Mode | Trigger | Status |
|---|---|---|
| **Explicit** | Operator clicks "Write to file" per field in the UI | This ADR |
| **Rule-based** | `writeback: overwrite\|if_empty` flag in mapping YAML triggers on enrich | Backlog |
| **Batch** | Admin action writes all resolved fields for all videos at once | Backlog |

### Constraints

- **Files are precious.** An in-place `exiftool` write that fails mid-stream can corrupt a file.
  The write path must be atomic and leave the original untouched on failure.
- **Explicit only in v1.** The operator sees the resolved value before committing. No silent
  overwrites, no scheduled jobs; operator action is the sole trigger.
- **No prior-value capture in v1.** Undo via a stored original is deferred to the backlog. The
  operator reviews the winning value in the UI before writing.
- **Auth parity with enrich.** Writeback modifies library files; it is gated behind
  `requireOwner` (same as enrichment endpoints — ADR-030).

---

## Decision

Implement explicit per-field metadata writeback using **copy → exiftool-write → atomic rename**,
with a `file_writebacks` audit table and a canonical-to-tag-name format mapping table.

### File-safety model: copy → write → rename

```
1. copy  original → <file>.holodex-tmp (same directory, same FS)
2. write exiftool writes the target tag on the .holodex-tmp copy
3. if OK  rename .holodex-tmp → original  (atomic on same FS)
   if ERR  delete .holodex-tmp, return error — original untouched
```

Using a same-directory temp copy ensures the rename is a single `rename(2)` syscall — atomic on
any POSIX filesystem (Linux/macOS). On Windows (WSL and native), `os.Rename` over the same
volume is similarly atomic. `exiftool`'s own `-overwrite_original` flag performs the same
pattern internally, but we own the outer temp copy so that an exiftool crash or OOM during the
write step never leaves a half-written original.

### Format-mapping table

A canonical field name maps to one or more tag targets per format family. The table lives in
`internal/writeback/tags.go` and is the single place where "canonical → wire tag name" is
authoritative.

| Canonical field | ID3 (mp3/flac) | XMP | QuickTime atoms (mp4/mov/m4v) | MKV / Matroska |
|---|---|---|---|---|
| `title` | `Title` | `XMP:Title` | `QuickTime:Title` | `Title` |
| `original_title` | `Grouping` | `XMP:OriginalDocumentID`¹ | — | `OriginalMediaType`¹ |
| `overview` | `Comment` | `XMP:Description` | `QuickTime:Comment` | `Summary` |
| `tagline` | — | `XMP:Headline` | — | — |
| `release_date` | `Year` | `XMP:CreateDate` | `QuickTime:ContentCreateDate` | `DATE_RELEASED` |
| `genres` | `Genre` | `XMP:Genre` | `QuickTime:Genre` | `GENRE` |
| `original_language` | — | `XMP:Language` | — | `LANGUAGE` |

¹ Best available; no standard slot exists. Operators can extend the table via config in a future
  iteration (see Consequences).

For multi-valued fields (e.g. `genres`), exiftool supports repeated flag invocations
(`-Genre="Drama" -Genre="Thriller"`); the write layer iterates the values array.

### Audit table

```sql
CREATE TABLE file_writebacks (
    id          INTEGER PRIMARY KEY,
    video_id    INTEGER NOT NULL REFERENCES videos(id),
    field_key   TEXT    NOT NULL,  -- canonical field name
    tag_name    TEXT    NOT NULL,  -- the actual tag written, e.g. "Title"
    value       TEXT    NOT NULL,  -- newline-joined for multi-value
    source      TEXT    NOT NULL,  -- winning source, e.g. "tmdb:title"
    written_at  TEXT    NOT NULL   -- ISO-8601 UTC
);
CREATE INDEX idx_file_writebacks_video ON file_writebacks(video_id);
```

Prior-value capture (recording the value that existed before the write) is **not included in
v1**. The operator reviews the winning value before writing; the audit log records what was
written and when, which is sufficient for diagnosing unexpected changes.

### API endpoint

```
POST /api/media/{id}/writeback
Authorization: requireOwner
Body: { "field": "title", "value": "Fight Club", "source": "tmdb:title" }

Response 204 No Content on success.
Response 422 if the field has no format mapping for this file's container.
Response 500 (wrapped) if the temp-copy or rename fails.
```

The `value` and `source` in the request body are the resolver's output — the SPA passes them
directly from the `resolved[]` array it already has. The handler performs no re-resolution; it
trusts the caller's winning value.

### Auth

All writeback endpoints are behind `requireOwner` (ADR-030 / `ADMIN_TOKEN`). A writeback
modifies library files; it is never available to unauthenticated guests.

---

## Options Considered

### A — Explicit (chosen)

Operator sees the resolved winner in the detail-page UI and clicks "Write to file" per field.
The handler copies the file, writes the tag on the copy, and renames atomically. A per-video
audit log records what was written.

**Pros:** Safe, intentional, no silent mutations. Operator has full visibility. Implementable
without a rules DSL or scheduler.

**Cons:** Manual — does not scale to bulk libraries without the batch mode (backlog).

### B — Rule-based (backlog)

A `writeback: overwrite|if_empty` flag in the mapping YAML triggers writeback automatically
whenever an enrichment run completes. Powerful, but every re-enrich writes files, and a
misconfigured source could overwrite operator-curated values silently.

### C — Batch (backlog)

An admin endpoint writes all resolved winners to all files in one pass. High blast radius;
requires a preview/diff step before commit to be safe. Suitable once explicit mode is mature
and trusted.

---

## Consequences

**What becomes easier**

- Once a field is written back, `file:title` in the F27 mapping resolves correctly from the
  file itself; the shadow-store enrichment query is no longer needed for that field on every
  list page. Over time this reduces the runtime cost of the `browse:true` enrichment mechanism
  and potentially eliminates the need for it entirely once libraries are curated.
- The written value is present in the file for any external tool (Plex, Kodi, mpv) that reads
  tags — portability without Holodex running.

**What becomes harder**

- A format-mapping gap (a canonical field with no known tag slot for a given container) must
  return a clear 422 rather than silently skipping.
- Operators working on Windows with files on a different drive letter from the OS temp dir
  cannot use `os.Rename` for atomic cross-volume replacement; the copy must land on the same
  volume as the file (handled by the same-directory temp-copy rule above).

**What we will need to revisit**

- **Prior-value capture / undo** (backlog) — once writeback is used at scale, the lack of a
  stored original becomes friction for operators who want to roll back a bad value.
- **Rule-based writeback** (backlog) — the `writeback: if_empty|overwrite` flag in the mapping
  YAML, triggered on enrich completion, for automation at library scale.
- **Batch writeback** (backlog) — admin bulk-apply with preview/diff before commit.
  *(Update: [ADR-048](ADR-048-metadata-curation-and-write-queue.md) partially realizes Option C —
  the "all curated fields at once" batch write via a durable, throttled queue — while keeping
  it owner-triggered; rule-based/automatic writeback remains deferred.)*
- **Extended format-mapping config** — operators may need to map canonical fields to
  non-standard tags for their container/tool combination; a config override table in
  `metadata-sources.yaml` is the natural home.
- **`browse:true` / shadow-store read-path review** — once writeback is widely adopted,
  evaluate whether the F27 browse-title enrichment-query mechanism is still earning its keep
  or can be simplified.
