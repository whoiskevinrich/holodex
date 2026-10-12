# Spec: Keep a Video's Identity When Its File Moves (F79)

**Status**: Draft
**Phase**: Library scanning (owner-facing, no new screen)
**Jira**: [HOLODEX-568](https://whoiskevinrich.atlassian.net/browse/HOLODEX-568)
**Related**: [HOLODEX-567](https://whoiskevinrich.atlassian.net/browse/HOLODEX-567) separates
"Date added" from "Recently updated" (F12.1, F12.7 and F12.8 in
[`phase-2-mcp-polish.md`](phase-2-mcp-polish.md)). This spec is what lets a move keep its place in
both sorts. [Delete a Media Item](delete-media.md) owns the trash. The
[file-maintenance](video-file-maintenance.md) swap replaces a file at the **same** path and is not
a move.
**Architecture**: to be decided under the `needs-adr` gate. How two files are recognised as the
same content, and where that check runs, is a technology fork (see "Open questions").

---

## Problem Statement

Holodex knows a video by the path of its file. If the owner moves or renames a file on disk, the
next scan treats the new path as a brand-new video and hides the old one as missing. The moved file
jumps to the top of "Date added". Everything the owner built on the old video stays behind on the
hidden copy: field edits, source choices, people, studio, film and tag links, playlist places, and
provider data.

Moving a single folder re-adds every video in it as new and strands its curation. On production,
**232 of 3,921 visible videos (about 6%)** have an earlier hidden copy with the same size and
duration, which is likely the trace of exactly this.

## Goals

1. **A move is invisible to the library.** A file that is moved or renamed keeps its identity: the
   same video, the same place in "Date added" and in "Recently updated", and all of its curation,
   links, playlist places and provider data.
2. **Past moves are healed.** Every past move that can be recognised without ambiguity is folded
   back into the original video automatically, with no owner action.
3. **Never merge two different videos.** A false match destroys a video's separate identity. That
   is worse than a missed move, so every rule below prefers "arrive as new" over "guess".
4. **The owner can see that it happened.** A scan reports how many moves it recognised and how many
   it declined as ambiguous.

## Non-Goals

- **Renaming or moving files from inside Holodex.** This spec only recognises moves made outside
  it. An in-app move is a separate feature.
- **Recognising a file that was moved *and* re-encoded.** Changed content is a new file. That is
  acceptable and expected.
- **Merging two copies that are both present.** Two files with the same content that both exist on
  disk are legitimate duplicates. The composite-key check in
  [video-composite-key-collision.md](video-composite-key-collision.md) is the place to flag those.
- **An owner review queue for matches.** Recovery is automatic (decided 2026-10-12). A review list
  would add a screen, and the matching rule (P0-1) is required to be strict enough not to need one.
- **Undoing a fold.** Once two copies are folded, there is no "split them back". This is why P0-1
  sets a high bar for matching.
- **Trashed videos.** A video the owner sent to the trash is never a match target, even if its file
  turns up at a new path. See "Edge cases".

## User Stories

- As the **owner**, when I reorganise my library folders on disk, I want every moved video to stay
  exactly as I left it, so that reorganising never costs me curation.
- As the **owner**, I want "Date added" to keep showing what actually arrived recently, not what I
  happened to move this week.
- As the **owner**, I want the videos that got duplicated by earlier moves to quietly heal
  themselves, so that I don't have to find and re-curate around 232 of them by hand.
- As the **owner**, I want a scan to tell me it recognised moves, so that I can trust nothing was
  silently lost or wrongly merged.
- As the **owner**, when a drive comes back after months away at a different mount path, I want its
  videos recognised as the ones I already had.

## Requirements

### Must-Have (P0)

**P0-1. What counts as "the same video".** A file at a new path is the same video as a missing one
only when its **content** matches what the library last recorded for the missing video. Matching
on size and duration alone is **not enough**. The architecture doc chooses the evidence.
- Given two different files with identical size and duration, when one disappears and the other
  appears, then they are **not** matched.
- Given a file Holodex earlier wrote metadata into (writeback), when it is later moved unchanged,
  then it **is** matched. The library's record of a file's content stays current through writeback
  and any other in-place change.

**P0-2. Who can be matched.** Only a video whose file is **missing** can be matched: either it went
missing in this scan, or it is already hidden as missing from any earlier scan, however long ago
(decided 2026-10-12). A video whose file is still present is never a match target. A trashed video
is never a match target.
- Given a copy (old path still present, new path added), when the scan runs, then the new path
  arrives as a new video and the original is untouched.
- Given a video hidden as missing for six months, when its file appears at a new path, then it is
  recognised and shown again.

**P0-3. A move found during a scan.** When a new path matches exactly one missing video, the scan
points that video at the new path and shows it again. No new video is created.
- The video keeps its identity, its place in "Date added", and all of its curation, links,
  playlist places and provider data.
- A move alone is **not an update**: its place in "Recently updated" doesn't change (HOLODEX-567,
  F12.7).

**P0-4. Ambiguity is declined.** If a new path matches **more than one** missing video, or more than
one new path matches the same missing video, nothing is matched (decided 2026-10-12). The new paths
arrive as new videos, and the scan counts them as ambiguous (P0-7).

**P0-5. Healing a move that already happened.** When a hidden older video and a visible newer video
match under P0-1, and the match is unambiguous under P0-4, the newer one is **folded into the older
one**:
- The **older** video survives, with its identity and its place in "Date added". Its file path
  becomes the newer one's, and it is shown again.
- The newer video is removed. Nothing attached to it is lost: everything moves onto the survivor
  according to P0-6.
- This runs automatically on the first scan after release, and on every later scan, so a pair
  created by any path is healed (decided 2026-10-12).
- Given the ~232 production candidates, after the first scan every unambiguous pair is one video.

**P0-6. Folding rules.** When both copies carry something for the same slot:

| What | Survives on the folded video |
|---|---|
| An owner edit or source choice for a field, including tags, people and studio | The **most recent** owner edit wins (decided 2026-10-12). Where only one copy has one, it is kept. |
| A field with no owner edit on either copy | Resolves normally from the file and provider data. |
| Playlist places and film memberships | The union of both. If both copies are in the same playlist or film, the video appears once, at the older copy's position. |
| Provider data | Kept from both. Where both have data from the same provider, the more recently fetched data wins. |
| Dismissed provider suggestions | The union of both. |
| A writeback still waiting to run | It still runs, against the surviving video. |
| The poster or thumbnail | The one the owner most recently chose, if any. Otherwise it is generated again as needed. |

**P0-7. The scan reports what it did.** A scan's summary in System activity
([system-activity.md](system-activity.md)) includes the number of moves recognised (P0-3 and P0-5)
and the number declined as ambiguous (P0-4). It reports no count when both are zero.

### Nice-to-Have (P1)

**P1-1. Move history.** For each recognised move, the activity record keeps the old path and the new
path. This lets the owner audit a surprising match. It needs a visible surface, so it would take a
design pass.

**P1-2. Ambiguity detail.** For an ambiguous match, the activity record lists the candidate paths,
so that the owner can resolve it by hand, for example by removing a stray copy.

### Future Considerations (P2)

- **Moving or renaming from inside Holodex.** Whatever recognises a move should make an in-app move
  trivial later, as the same identity-preserving step without the matching.
- **A trashed video whose file moves.** Today it would arrive as new, and the trash's eventual
  purge targets a path that no longer exists. That interaction belongs to the delete spec.

## Edge Cases

- **Moved during the minimum-age wait.** A new file younger than the scan's minimum age is skipped
  that scan, while its old path is already gone. The video is hidden for one scan, then recognised
  by P0-2 on the next. This is acceptable.
- **A whole drive unplugged.** A scan that finds no files at all hides nothing (existing behaviour).
  When the drive comes back at a new mount path, P0-2 recognises every file.
- **Moved and changed at once** (re-encoded, trimmed, or written to by another tool between scans).
  This is not a match; it arrives as new. See Non-Goals.
- **Moved back.** A file returning to its original path is the existing same-path return. Nothing
  new is needed.
- **Writeback and the content record.** The content record is refreshed whenever the file changes in
  place (P0-1). A move made before that refresh is a moved-and-changed file.

## Success Metrics

- **Leading:** moving a folder of N videos and rescanning produces **0** new arrivals in
  "Date added" and N recognised moves in the scan summary.
- **Leading:** after the first post-release scan on production, the number of visible videos with a
  hidden older copy of the same content drops from ~232 to **only the declined ambiguous pairs**.
  That count is reported, and the target is fewer than 5.
- **Lagging:** no owner report of a wrongly merged video. A false match is the failure mode P0-1
  exists to prevent.

## Open Questions

- **[engineering · blocking, architecture]** What evidence satisfies P0-1? The options are a partial
  content hash, a full hash, or a hash of the media streams that ignores metadata. Each has a
  different cost on a large library and a different tolerance to writeback. This is decided under
  the `needs-adr` gate.
- **[engineering · blocking, architecture]** How are the existing ~232 rows given a content record
  so that P0-5 can heal them? Their files are at the new path, so the record must come from reading
  the surviving file plus whatever the hidden row last recorded. If a hidden row has nothing strong
  enough to satisfy P0-1, the pair is not healed. How many that leaves is to be measured.
- **[data · non-blocking]** How many of the 232 hidden copies carry curation, links or provider data
  worth recovering? This sets the priority (currently Low, per the Jira comment of 2026-10-12).

## Timeline Considerations

- Built in parallel with HOLODEX-567, which is expected to merge first. Merge `main` in before
  marking ready. If both add a migration, this one takes the next free number.
- HOLODEX-567's F12.8 says files moved before HOLODEX-568 "rank by when they were moved". P0-5 heals
  those pairs, so F12.8 should be revisited once this ships.
