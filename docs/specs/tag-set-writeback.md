# Spec: Tag set writeback — write tags from the dialog, and show which are on the file (F72)

**Status**: Draft
**Story**: [HOLODEX-401](https://whoiskevinrich.atlassian.net/browse/HOLODEX-401) (epic HOLODEX-167 Writeback)
**Owner**: Project owner
**Date**: 2026-09-26
**Architecture**: [ADR-111](../architecture/archive/ADR-111-recorded-file-tag-set.md) · **Design**:
[tag-set-writeback-handoff.md](../design/tag-set-writeback-handoff.md)

**Depends on (all shipped)**: the genres write path, which writes the full applied set and ignores the
client's values (ADR-075 RD9, `internal/api/genre_writeback.go`) · one comma-joined genre string
(HOLODEX-464) · the file tag contract, under which a genres write filters every tag key to the written
set (ADR-110, HOLODEX-465) · the writeback cockpit: no checkbox, a gutter glyph per row
(HOLODEX-400, [handoff](../design/writeback-cockpit-handoff.md)).

## Problem

Tags added or removed in Holodex never reach the file from the writeback dialog. `needsWriteback()` and
`willWrite()` are replace-only, and the Tags row reads "Nothing to decide here yet". The only way to get
tags onto a file today is the Tags page's sync button. The owner also cannot tell which applied tags are
actually on the file:

> "I don't care where the tags came from. I care about what tags are applied … and if they exist on the
> media file." — owner, 2026-09-16

## Goals

1. The writeback dialog writes the video's tags as one set, in the same atomic job as the other rows.
2. The dialog shows the set diff, *applied ∪ file*, one chip per tag: **on file**, **will add**, **will
   drop**.
3. On the media page, owner tag chips carry a glyph saying whether each tag is on the file.

## Non-goals

- **Per-tag provenance.** No source affordance (ADR-090: tags have no layer 2). The existing `·file` /
  `·provider` suffix on owner chips goes away (ADR-111 Consequences).
- **Editing tags in the dialog.** Tags are curated in the page's Tags section. The dialog shows the
  consequence and writes it; it has no chooser, no per-chip toggle and no checkbox (HOLODEX-400).
- **Other merge fields** (people, studios). They keep the read-only merge row (RD1).
- **Film pages.** The film dialog has no genres set row, so nothing changes there.
- **Attach/detach writing the file.** Still no (ADR-110 owner decision). The file changes only on Write.
- **Backfilling file tags at boot.** Unknown until the file is next re-extracted (owner,
  2026-09-26). That happens when the file changes on disk, on a Refresh, on a write, or on the
  next background scan: the scanner treats `file_tags IS NULL` as changed and re-extracts that
  file once (HOLODEX-468), so the fill is spread over a normal scan, never done at boot.

## Definitions

- **W (written set)**: what a genres write puts on the file. It is the existing
  `genreWritebackItems` union: attached tags, ancestor-expanded, minus writeback-off tags; plus the raw
  resolved genres, minus denied and writeback-off names.
- **F (file set)**: the tag names the extractor last read from the file's
  Genre/Genres/Keywords/Category/Categories. It is recorded at every scan and re-extract
  (`videos.file_tags`). Unknown (`NULL`) until the video's first scan after this ships.
- **Same tag**: the two names share a tag name key, or they resolve to one tag entity through
  aliases and merges. This is the rule the writer's filter already uses (ADR-111 D2).

## Requirements

### P0-1 · Record the file's tag set
Every `UpsertVideo` stores `file_tags` = the extracted tag names (JSON array) in the same transaction as
the tag links. Denied, oversized and category-colliding names are recorded even though they are not
linked. A file with no tags stores `[]`. Existing rows stay `NULL`. *(ADR-111 D1, migration 0054.)*

### P0-2 · The genres row carries the diff
On `GET /media/{id}`, the `genres` entry in `resolved[]`:
- **P0-2a** `items[i].on_file`: `true` if the value is in F, otherwise `false`.
- **P0-2b** `file_only: string[]`: the members of F that are not in W, in the file's spelling, in file
  order.
- **P0-2c** `in_sync`: `true` iff W and F are the same set, `false` if they differ, omitted when F is
  unknown. When F is unknown, P0-2a and P0-2b are omitted too.
- **P0-2d** The row is present when W or F is non-empty. Today it is dropped when W is empty, which
  would hide the write that deletes Genre (ADR-110 D6).

### P0-3 · Each attached tag carries membership
`video.tags[i]` gains:
- `written: bool`: the tag is in W, i.e. its writeback flag is on.
- `on_file: bool`: the tag is in F, omitted when F is unknown.

The API returns both to every viewer. The glyph is owner-only (P1-1).

### P0-4 · The dialog's Tags row
A `genres` row with a `write_target` is a **tag-set row**, not a read-only merge row.
- **P0-4a** When `in_sync === true`: `=` gutter, and the line reads "{W joined} — matches the file".
- **P0-4b** When `in_sync === false`: `↧` gutter, and a chip per tag in three groups, in this order:
  on file (W ∩ F), will add (W − F), will drop (`file_only`). Below the chips is a one-line
  summary: "Writes the {n} applied tag{s}." When there are drops, it adds "{names} {is/are} on the
  file but won't be written." When W is empty the summary reads "Clears the file's tags."
- **P0-4c** When `in_sync` is omitted (unknown): `↧` gutter, the applied chips without markers, and
  "Not read from this file yet — writes the {n} applied tag{s}." This is ADR-093's unknown rule, and
  the owner confirmed write on open (2026-09-26).
- **P0-4d** A tag-set row with no `write_target` keeps the existing "No file tag for this container"
  read-only rendering.
- **P0-4e** The row writes whenever `in_sync !== true` and it is writable. It counts toward the Write
  button's field count and the header's out-of-sync count, and it sits in the lead ("writes on open")
  group. There is no control on the row itself. Cancel is the opt-out (HOLODEX-400).
- **P0-4f** The payload entry is `{field: "genres", values: W, source: winning_source}`. The server
  keeps recomputing W (unchanged).

### P0-5 · Round trip
After a genres write completes, the post-write re-extract records F. The next `GET /media/{id}` then
reports `in_sync: true`, and every chip reads *on file*, unless the file refused a value. The job
still makes exactly one `WriteBatch` per file (HOLODEX-109).

### P1-1 · On-file glyph on owner tag chips
On the media page, when the owner view is on (`onremove` passed), each tag chip starts with one glyph
from P0-3. This is the 2B treatment (owner, 2026-09-26):

| `written` | `on_file` | Glyph | Tooltip / accessible name |
|---|---|---|---|
| true | true | file-check (muted) | On the file |
| true | false | file-plus (accent) | Not on the file yet — added on the next writeback |
| false | true | file-minus (warn) | On the file — writeback is off, so the next writeback removes it |
| false | false | file-off (muted) | Kept in Holodex only — writeback is off for this tag |
| any | omitted | *(none)* | — |

Visitors see the plain link chip, unchanged.

## Acceptance criteria

1. Scan an MP4 whose Genre is `Drama, Noir` and whose Keywords is `heist`. `file_tags` is
   `["Drama","Noir","heist"]` (extractor order, deduped).
2. Attach `crime` and set `noir` to writeback-off. The genres row reports `drama` and `heist` with
   `on_file: true`, `crime` with `on_file: false`, `file_only: ["Noir"]` and `in_sync: false`. The
   `crime` chip has file-plus, and the `noir` chip has file-minus.
3. The dialog shows the tag-set row with `↧` and "Writes the 3 applied tags. Noir is on the file but
   won't be written." Write enqueues one job whose genres entry is W.
4. After the job and re-extract, the row reads "drama, heist, crime — matches the file". The noir chip
   shows file-off.
5. A video scanned before 0054 shows no glyphs, and the dialog row reads "Not read from this file
   yet". Writing it populates F. So does the next background scan, even when the file is unchanged
   (HOLODEX-468); the scan after that skips the file again.
6. A file whose tags are all writeback-off shows a tag-set row that reads "Clears the file's tags",
   even though W is empty.
7. An alias on the file (`Sci-Fi` → tag `science fiction`) counts as on file for `science fiction`.
   It does not appear in `file_only`.

## Open questions

None. The four design choices were settled by the owner on 2026-09-26: write on open, 2B glyph on
every chip, record at every scan, and unknown until rescan.
