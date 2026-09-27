# ADR-110: A genres writeback makes the file's tag keys match the UI — filter the extra keys, ignored tags are UI-only

**Status:** Proposed
**Date:** 2026-09-26
**Deciders:** Project owner

**Amends:** [ADR-077](ADR-077-tag-writeback-exclusion.md) D1 (what "writeback disabled" *means*: from
"not added to Genre" to "removed from the file, kept in the UI") · [ADR-041](ADR-041-metadata-writeback.md)
(a write batch now names keys beyond its fields' own `formatMap` targets; the copy→write→rename file-safety
model is unchanged). **Relates to:** [ADR-075](ADR-075-tag-governance-and-video-enrichment.md) D3 (only
`source='file'` links are rescan-managed — D4 leans on it) · [ADR-073](ADR-073-post-write-baseline-resync.md)
(post-write re-extract) · [ADR-096](ADR-096-entity-identity-card.md) RD8 (probe-chosen write
keys — D5's precedent). **Spec:** [tag-writeback-exclusion.md § Amendment](../specs/tag-writeback-exclusion.md#amendment--the-file-tag-contract-holodex-465)
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

1. It reads the file's current values for the container's *extra tag keys*. That is a per-container table
   kept next to `formatMap`: `Genres, Keywords, Category, Categories` under their writable names, which
   are verified by an integration probe.
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

**D3 — Snapshot, audit and revert are keyed by write key for the derived fields.**
- `ReadCurrentValues` also returns the prior value of each extra tag key it's about to change, keyed by
  **tag name**.
- Each changed key writes its own audit row: `field = genres`, `tag_name = <key>`.
- Revert restores those rows **by tag name**. They skip the `formatMap` lookup, which only knows Genre.

The rule "a prior value of `""` isn't reverted" can't bite here, because D2 never writes a key that
wasn't already on the file.

**D4 — Ignored tags leave the file but stay on the video.** When a genres write succeeds, the writer updates
the job's video: every link with `source='file'` to a tag whose `writeback_enabled = 0` becomes
`source='manual'`. This runs in the same place and transaction as the success audit rows. `replaceAssociations`
only deletes and re-adds `file` links (ADR-075 D3), so the next rescan and the post-write re-extract
(ADR-073) keep the tag.

It runs **after** the write succeeds, so a failed write leaves the links as they were. Changing the source is
one-way on purpose: the tag has become a Holodex-owned tag and is no longer a fact about the file.

**D5 — The MP4 tagline moves out of Keywords (HOLODEX-466).** `formatMap["MP4"]["tagline"]` moves from
`QuickTime:Keywords` to a key outside `tagKeys`, chosen the way RD8 chose edition's key: by writing and
reading back generated clips. The candidate is `QuickTime:Description`. `UnreadableWriteTargets["tagline"]`
stays, because tagline has no `file:` source to read back (a separate gap, not this ADR's). Only its wording
changes.

## Consequences

- **The file matches the UI after any writeback.** This applies to every path that writes genres: the
  per-video dialog, the tag sync, merge propagation and the studio cascade. It is also what makes HOLODEX-401's
  per-tag "on file" markers possible to build.
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
