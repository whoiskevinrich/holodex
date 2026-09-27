# ADR-111: Record the file's tag set at every scan; tags get a set-valued in_sync

**Status:** Proposed
**Date:** 2026-09-26
**Deciders:** Project owner

**Extends:** [ADR-093](ADR-093-writeback-readback-and-tristate-in-sync.md) D1 (the tri-state `in_sync`,
until now replace-fields only, gains a set-valued meaning for the `genres` row) ·
[ADR-110](ADR-110-tag-writeback-file-contract.md) (which made "the file's tags equal the UI's written set"
true after every genres write — this ADR makes it *observable*). **Relates to:**
[ADR-075](ADR-075-tag-governance-and-video-enrichment.md) D3 (`video_tags.source`) ·
[ADR-090](ADR-090-two-layer-entity-metadata-management.md) (tags have no layer 2 — no per-tag provenance) ·
[ADR-091](ADR-091-fire-and-forget-writeback.md) (one atomic job). **Spec:**
[tag-set-writeback.md](../specs/tag-set-writeback.md) (F72, HOLODEX-401).

---

## Context

The owner's question about tags is set membership: *which tags are applied, and does each one exist on the
file?* ("I don't care where the tags came from", 2026-09-16.) Holodex cannot answer the second half today:

- `video_tags` holds **one row per (video, tag)**, and a manual or provider attach overwrites
  `source='file'` (`attachTagTx`'s `ON CONFLICT … WHERE source='file'`). A manual tag that is also on the
  file reads `manual`. `source` is provenance, not membership.
- The extractor sends `Genre/Genres/Keywords/Category/Categories` to `ex.Tags`, never to
  `video_metadata`, so no raw copy of the file's tags is kept. Denied, oversized or
  category-colliding names are dropped silently at link time and leave no trace at all.
- The `file_writebacks` ledger knows only what Holodex last *wrote*, so it has no answer for a file
  Holodex never wrote. The owner rejected ledger-only for that reason (2026-09-26).

Separately, the writeback dialog never writes tags. `needsWriteback()` and `willWrite()` are replace-only
(RD1), and the merge row reads "Nothing to decide here yet". The server has written the full applied set
for any `genres` entry since ADR-075 RD9 / ADR-110. What was missing is a way to say when the file is
behind.

## Decision

**D1 — `videos.file_tags`: the file's tag names as last read.** A nullable `TEXT` column holding a JSON
array of the raw tag names the extractor read (`ex.Tags`, already split and de-duplicated). It records
every name, including ones the link step skips: a denied name is still *on the file*, and it is exactly
the name a write will drop. `UpsertVideo` sets it in the same transaction as `replaceAssociations`, so the
scanner, the refresh path and the post-write re-extract (ADR-073) all keep it current with no new writer.
`NULL` means *never read since this shipped* (unknown). `[]` means *read, and the file has no tags*.
Migration `0054` adds the column with no backfill. Re-reading every file at boot was rejected for cost,
so a row fills in the next time its file is re-extracted: a change on disk, a Refresh, or a write
(owner, 2026-09-26: "unknown until rescan"). A routine scan skips unchanged files (ADR-018 change
detection), so it does not fill them. A library-wide fill is follow-up HOLODEX-468.

**D2 — Membership is identity, not spelling.** A file name and a written name are the same tag when they share
the tag name key (lowercased, trimmed, spaces removed) or resolve to the same tag entity through the
name-identity spine (`LookupEntityIDByName`: alias- and merge-aware, ADR-061). A denied term resolves to
nothing, so only its name key can match. This is exactly the rule the ADR-110 writer's `tagKeeper` uses to
filter the extra keys, and the two share one implementation, so "on file" and "kept on write" can never
disagree.

**D3 — The `genres` row carries a set diff.** `applyGenreWriteback` stamps the written set **W** (the
existing union: ancestor-expanded, writeback-flag-filtered tags plus the deny/ignore-filtered raw genres)
against the recorded file set **F**:

- each `items[i].on_file` is `true` (in F) or `false` (not in F — *will add*);
- a new `file_only: []string` lists F − W in file spelling — *will drop*;
- `in_sync` is `true` when W and F are equal under D2, `false` when they differ, and **nil when F is
  unknown**. This is ADR-093's tri-state carried over to a set, and nil means unknown here too, never
  "differs".

The row is now also kept when **W is empty but F is not**. That is the ADR-110 D6 case (every tag
ignored, so the write deletes Genre). Dropping the row there would hide the one write that clears the
file. When F is unknown the per-item flags and `file_only` are omitted, and only `in_sync: nil` remains.

**D4 — Each attached tag carries `on_file` and `written`.** On `GET /media/{id}`, each `video.tags[i]`
gains two fields. `written` means the tag is in W, i.e. its writeback flag is on. `on_file` is
tri-state: true, false, or omitted when F is unknown. Together they give the owner's four chip states:
on file, *not on file yet*, *on file but will be dropped*, and *UI only*. They are computed at read time
from `file_tags`, so nothing is stored per tag. Visitors receive the same fields, since a file's tags
are no secret; the chip glyph is an owner affordance, and that is gated in the SPA, not the API.

**D5 — The dialog writes the tags row whenever `in_sync !== true`.** The applied set *is* the standing
decision (owner, 2026-09-26: "write on open"), so a differing or unknown row joins the one atomic job
like any lagging replace-field decision (HOLODEX-400: no checkbox, no per-row skip; Cancel is the
opt-out). The request shape is unchanged. The client sends `{field: "genres", values: W}`, and the server
still recomputes W and ignores the client's values (ADR-075 RD9), so the dialog can never write a stale
set. One job still means one `WriteBatch` per file (HOLODEX-109).

## Consequences

- The header count (`outOfSyncCount`) and the dialog's lead group now include the tags row when it lags.
  The "Nothing to decide here yet" merge-row copy stays for every *other* multi field (people, studios,
  which are still RD1).
- Per-tag `source` stops being shown on owner chips. The on-file glyph replaces the `·file` /
  `·provider` suffix, because provenance is noise to the owner (ADR-090: tags have no layer 2).
  `source` stays in the API and still drives rescan management (ADR-075 D3).
- A video last scanned before 0054 shows no glyphs and a *not read yet* Tags row, which writes on open.
  After that write, the re-extract records F and the row reads `=`.
- Cost: one JSON column per video (a few dozen bytes), and a per-read lookup of |F| + |W| names, usually
  under 20, on the media detail endpoint only. No list endpoint pays it.

## Alternatives rejected

- **Ledger-only.** The last `file_writebacks` genres value has no answer for never-written files, which is
  most of a library (owner, 2026-09-26).
- **An `on_file` flag on `video_tags`.** It cannot represent file names that are not links (denied terms,
  or tags detached in the UI but still on the file), and those are exactly the *will drop* names.
- **A `video_file_tags(video_id, name)` table.** It holds the same information as D1 with a join and a
  second delete+insert per scan. Nothing queries file tags across videos.
- **Backfill at boot** by re-reading every file with exiftool. It is slow on a large library, and
  unknown-until-rescan is honest (owner, 2026-09-26).
- **Per-value (per-tag) writes or checkboxes in the dialog.** The file's genre is one comma string
  (HOLODEX-464), and the cockpit has no per-row skip (HOLODEX-400).
