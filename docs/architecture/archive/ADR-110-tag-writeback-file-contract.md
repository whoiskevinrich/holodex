# ADR-110: A genres writeback makes the file's tag keys match the UI — filter the extra keys, ignored tags are UI-only

> **Archived — not current truth.** This numbered ADR was retired by HOLODEX-528. Its architecture now lives in [writeback.md](../writeback.md). Read those; this file is kept only as history.


**Status:** Proposed
**Date:** 2026-09-26
**Deciders:** Project owner

**Amends:** [ADR-077](ADR-077-tag-writeback-exclusion.md) D1 (what "writeback disabled" *means*: from
"not added to Genre" to "removed from the file, kept in the UI") · [ADR-041](ADR-041-metadata-writeback.md)
(a write batch now names keys beyond its fields' own `formatMap` targets; the copy→write→rename file-safety
model is unchanged). **Relates to:** [ADR-075](ADR-075-tag-governance-and-video-enrichment.md) D3 (only
`source='file'` links are rescan-managed — D4 leans on it) · [ADR-073](ADR-073-post-write-baseline-resync.md)
(post-write re-extract) · [ADR-096](ADR-096-entity-identity-card.md) RD8 (probe-chosen write
keys — D5's precedent). **Spec:** [tag-writeback-exclusion.md § Amendment](../../specs/tag-writeback-exclusion.md#amendment--the-file-tag-contract-holodex-465)
(HOLODEX-465, also HOLODEX-466).

---

## Context

Tags go to the file through one canonical field, `genres`, which `formatMap` sends to one key: Genre
(`QuickTime:Genre` on MP4). The scanner reads tags from five keys: `Genre, Genres, Keywords, Category,
Categories`. Because of that mismatch, nothing Holodex writes can remove a tag that sits in the other four,
and a tag removed in the UI comes back on rescan. The owner's rule (2026-09-26): after a writeback, the file's
tags are the UI's tags, minus ignored tags, which are UI-only.

The write pipeline can't express that today. Several things are missing:

- **One key per field:** `ResolveForContainer` maps each canonical field to exactly one key.
- **Snapshots and audit are keyed by field:** `ReadCurrentValues` returns `map[canonical]value`, and there
  is one audit row per mapped field.
- **Revert goes back through the field:** it re-enqueues by canonical field, so it can only restore Genre.
- **Nothing can remove a key:** `WriteBatch` rejects a field with no values.

MP4 adds one more problem: `tagline` is written to `QuickTime:Keywords`, which is one of the five tag keys.

## Decision

**D1 — The written set.** The genres value is computed as today: `TagNamesForVideo` (ancestor-expanded,
`writeback_enabled = 1`) unioned with the deny-filtered raw resolved genres. One change: **the raw side also
drops the names of ignored tags.** Without that, an ignored tag reaches Genre again through `file:Genre`.

**D2 — Extra tag keys are written as derived fields of the genres write, in the same batch.** When a batch
carries `genres`, the writer does four things:

1. It reads the file's current values for the *extra tag keys* (`writeback.ExtraTagKeys`: `Genres, Keywords,
   Category, Categories`) with `exiftool -G1 -a`, and writes each back under the name exiftool reported. A
   probe on generated clips (2026-09-26) showed why no fixed per-container table works. On MP4, exiftool
   files Keywords under the **`Keys`** group and Category under `ItemList`, so `-ItemList:Keywords=` misses
   the Keywords tag entirely. So on exiftool containers the write name is the reported `Group:Name`. On
   Matroska/WebM it is the bare name, since ffmpeg and mkvpropedit address tags by name, and ffmpeg's
   `-metadata Keywords=` matches regardless of case.
2. For each key present on the file, it keeps only the values that match the written set.
3. It adds one `FieldWrite` per key whose value changed. A key with nothing left gets a **delete**
   `FieldWrite`:
   - exiftool: `-TAG=`
   - ffmpeg: `-metadata key=`
   - mkvpropedit: drop the Simple
4. It never adds values and never creates keys.

Matching resolves each file value through the same name-identity spine the scanner uses
(`resolveOrCreateByName`'s lookup half: case and whitespace folding, aliases, merged-away names). So `sci fi`
in Keywords survives when the written tag is `Sci-Fi`.

It all stays **one tool invocation**, so ADR-041's rule that a failed write leaves the file untouched still
holds. The filter is computed **when the job runs**, not when it is enqueued, because the file can change
while the job waits in the queue.

*Why not new canonical fields* (`keywords`, `category`, …): they would put four fake fields in the resolver,
the mapping file and the writeback dialog, for keys the owner never edits directly. They exist only as a side
effect of `genres`, so they're modelled as one.

A derived write travels as a job field named `tagkey:<name>`. A job payload is data (security C2), so the
worker accepts that shape only when `ValidTagKeyName` allows it: an extra tag key, grouped only on
exiftool containers, with a plain group name. The HTTP API refuses a client-sent `tagkey:` field outright.

**D3 — Snapshot, audit and revert are keyed by write key for the derived fields.**
- `ReadCurrentValues` also returns the prior value of each extra tag key it's about to change, keyed by
  **tag name**.
- Each changed key writes its own audit row: `field = genres`, `tag_name = <key>`.
- Revert restores those rows **by tag name**. They skip the `formatMap` lookup, which only knows Genre.

The rule "a prior value of `""` isn't reverted" can't bite here, because D2 never writes a key that
wasn't already on the file. A revert job skips the D2 filter: it restores the tag keys it snapshotted as
fields of its own, and filtering them against the restored Genre would undo the undo.

**D4 — Ignored tags leave the file but stay on the video.** When a genres write succeeds, the writer updates
the job's video: every link with `source='file'` to a tag whose `writeback_enabled = 0` becomes
`source='manual'`. This runs in the same place and transaction as the success audit rows. `replaceAssociations`
only deletes and re-adds `file` links (ADR-075 D3), so the next rescan and the post-write re-extract
(ADR-073) keep the tag.

It runs **after** the write succeeds, so a failed write leaves the links as they were. Changing the source is
one-way on purpose: the tag has become a Holodex-owned tag and is no longer a fact about the file.

**D5 — The MP4 tagline moves out of Keywords (HOLODEX-466).** `formatMap["MP4"]["tagline"]` moves from
`QuickTime:Keywords` to a key outside `tagKeys`, chosen the way RD8 chose edition's key: by writing and
reading back generated clips. That probe settled on `QuickTime:Description`: exiftool files it as
`ItemList:Description` and reads it back as `Description`, which is not a tag key. `UnreadableWriteTargets["tagline"]`
stays, because tagline has no `file:` source to read back (a separate gap, not this ADR's). Only its wording
changes.

**D6 — An empty written set clears the file.** A genres job with no values deletes Genre instead of being
dropped as "nothing to write". Both enqueuers used to skip an empty union: the writeback dialog dropped the
field, and the tag sync skipped the video. But an empty union is exactly the video whose only tags were
just turned off, and skipping it left the ignored tags on the file for good. The legacy synchronous handler
path (unused in production, where the queue is always wired) keeps dropping it, because it has no filter.

A Genre delete on a file that has no Genre, with nothing else to filter, is not written at all. On
Matroska it would otherwise be a full remux for nothing.

**D7 — The scanner reads a list-valued tag key as separate tags.** exiftool returns Matroska `KEYWORDS` as
a JSON **array**, and `metadata.ToString` rendered it with `%v`. So Keywords `Drama, Heist` became a single
tag named `[Drama Heist]`. The round-trip test for D2 found this. It predates this ADR, but rule 2 ("tags
from the file appear in the UI") needs it fixed. `ToString` now joins an array with `", "`, the form
writeback stores a multi-value field in and `SplitMulti` undoes.

## Consequences

- **The file matches the UI after any genres writeback.** Two paths write genres: the per-video writeback
  dialog and the tag sync. Merge propagation and the film cascade write only people and studios. This is
  also what makes HOLODEX-401's per-tag "on file" markers possible to build.
- **MKV files with Keywords correct themselves on the next rescan.** A bracketed tag link like
  `[Drama Heist]` came from the file, so the rescan replaces it with `Drama` and `Heist`. The bracketed
  tag itself is left with no videos, for the owner to delete.
- **Holodex now edits keys other tools own**, but only by removing values the owner removed from the UI. It
  never adds to them and never clears a key outright unless that key holds nothing canonical. A value another
  tool put in Keywords that the owner also has as a tag is left in place.
- **Detach still doesn't write the file** (owner decision). The known gap: a rescan that runs before the next
  writeback brings a detached tag back.
- **Every genres write now reads the file first.** One extra exiftool read per job, with no new binaries.
- **Earlier leaks aren't cleaned up automatically.** Tagline pieces that an old rescan turned into tags are
  real tags now, so the filter keeps them until the owner removes them.

## Rejected

- **Clear the extra keys** (Genre becomes the only home): this destroys other tools' data even when it's
  canonical.
- **Mirror the full set into every key**: tags get duplicated across keys, and keys get created that the file
  never had.
- **Read only Genre**: tags that libraries legitimately keep in Keywords would silently disappear.
- **Suppress detached tags per video instead of writing** (an `entity_alias_suppressions`-style table): that
  fixes rescans but leaves the file wrong, and the owner's rule is about the file.
- **Leave ignored tags wherever they already are**: the owner chose UI-only, and this option also keeps
  today's silent Genre loss.
