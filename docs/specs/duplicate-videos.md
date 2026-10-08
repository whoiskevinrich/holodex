# Spec: Duplicate videos — find two files of one scene, compare them, keep the right one (F76)

**Status**: Draft
**Phase**: Phase 2 (identity) — a video lane on the F43 Duplicates page, next to the person,
studio, tag and film lanes; first slice of the scene model ([HOLODEX-520](https://whoiskevinrich.atlassian.net/browse/HOLODEX-520))
**Owner**: Project owner
**Date**: 2026-10-08
**Jira**: [HOLODEX-521](https://whoiskevinrich.atlassian.net/browse/HOLODEX-521)
**Design**: [handoff](../design/duplicate-videos-handoff.md) · [mockup](../design/duplicate-videos-mockup.svg)
**Feature block**: **F76** — two files in the library that are matched to the **same provider
item** are flagged as a possible duplicate. The owner compares them side by side and either
**keeps one** (the other goes to Trash, and the owner's own work on it moves to the kept copy),
**keeps both**, or **labels them** as different editions or parts of the same item.

## Problem Statement

The library is about 3,800 files, mostly scenes. Some scenes have arrived more than once — a 4K
copy and a 1080p copy, a copy from another source with a different intro or a watermark, or a
scene split into parts. Nothing tells the owner this has happened: each file is its own card, and
the owner finds a copy only by stumbling on it.

Measured on the live library 2026-10-03 (counts only): **21 pairs of live files share a provider
match, and no item has three or more files.** 13 of the 21 differ in resolution, 19 have durations
within 5% of each other (copies), and 2 differ by more (parts, clips or editions). Only 984 of
3,808 files are matched to a provider yet, so more will surface as matching continues. **No two
live files are byte-identical**, so every copy is a different encode and the owner's judgment is
the deciding evidence.

The cost of not solving it is wasted space, a cluttered library, and copies that drift apart as
the owner curates one and not the other.

## Goals

1. **Every file pair that shares a provider match is visible in one place**, without the owner
   having to go looking.
2. **Decide a pair without leaving the page.** Both files' poster and technical facts are on
   screen together.
3. **Keeping one copy never loses the owner's work.** Playlists, film links and manual edits on the
   trashed copy end up on the kept copy.
4. **Legitimate pairs leave the queue for good.** Parts and editions are a normal reason for two
   files to share a match, and resolving them as such is as cheap as trashing a copy.

## Non-Goals

| Not doing | Why |
|---|---|
| Detecting copies among files with no provider match | No reliable cheap signal exists. Byte size found no copies, and duration plus resolution is mostly coincidence because durations are whole seconds and most files are 1080p. Coverage grows as matching continues. |
| Frame strips (frames at the same timestamps from both files) | **P1-1.** Needs a new frame-extraction capability. v1 relies on the poster, the technical facts and opening each file. |
| A recommended verdict or a "best copy" marker | The panel reports facts and never adjudicates (the F70 rule). Higher resolution is not always the copy the owner wants, for example when the 4K copy carries a watermark. |
| Permanent delete from the queue | Keeping one copy moves the other to **Trash**, which is reversible. Permanent delete stays on the media page and in Trash. |
| Grouping a scene's files, a preferred copy, clips and trailers as related media, or scene-level metadata | The scene model, [HOLODEX-520](https://whoiskevinrich.atlassian.net/browse/HOLODEX-520). This spec doesn't contradict it. |
| Keeping a renamed or moved file's row | Move detection, [HOLODEX-522](https://whoiskevinrich.atlassian.net/browse/HOLODEX-522). |
| Changing the entity duplicate detector | The shared-provider-id detector for people, studios and films ([F71](duplicates-shared-external-id.md)) still excludes videos. Two files sharing a match is not two *entities* claiming one id. It is a different question with different answers, and it gets its own lane. |

## Resolved Decisions

Locked in the 2026-10-03 – 10-08 system-design session.

| # | Decision | Rationale |
|---|---|---|
| RD1 | **The signal is a shared provider match**: two live files whose current match is the same item from the same provider. | It already finds all 21 known pairs with no hashing. No copies are byte-identical, so a file hash finds nothing. |
| RD2 | **The verbs are file verbs, not entity verbs.** The answers are *keep this one*, *keep both*, *label as editions* and *label as parts*. There is no merge. | A file is not an entity. Two copies are two files, and the answer is which file to keep, not how to fold one identity into another. |
| RD3 | **Keeping one moves the other to Trash**, never to permanent delete. | Trash is reversible and lossless. The owner can restore within the grace period. |
| RD4 | **The owner's work on the trashed copy moves to the kept copy, additively, and never overwrites.** The kept copy gains what it lacks and keeps its own choices wherever both have one. | Owner decision, 2026-10-08. A copy is usually bare, but when it isn't, the work must not vanish at purge. Additive-only means carrying over can't make the kept copy worse. |
| RD5 | **Part and edition never carry over.** | They describe the trashed *file*, not the scene. Carrying them would mislabel the kept copy. |
| RD6 | **Labeling as editions or parts resolves the pair as keep-both.** | Two editions or two parts legitimately share a match. Once labeled, the pair is answered and must not come back. |
| RD7 | **Keep both is durable for that pair of files.** It survives re-matching, rescans and restarts. | The same contract as keep-separate everywhere else on the Duplicates page. |
| RD8 | **v1 evidence is poster plus technical facts plus a way to open each file. Frame strips are P1.** | Owner decision, 2026-10-08. It avoids a new frame-extraction capability in v1. |

## User Stories

1. As the owner, I want to be told when two of my files are the same scene, so that I don't keep
   redundant copies without knowing.
2. As the owner, I want to see both files' resolution, duration, size, codec and bitrate side by
   side, so that I can pick the copy to keep at a glance.
3. As the owner, I want to keep one copy and have my playlists, film links and edits follow it, so
   that cleaning up a copy never costs me curation work.
4. As the owner, I want to see what will carry over before I confirm, so that nothing moves
   without my knowing.
5. As the owner, I want to mark two files as different editions or as parts of one scene, so that
   a legitimate pair is labeled correctly and leaves the queue.
6. As the owner, I want to keep both files without labeling them, so that I can dismiss a pair I
   don't want to act on.
7. As the owner, I want to open either file to watch it, so that I can check for intros or
   watermarks when the facts aren't enough.

## Requirements

### Must-Have (P0)

**P0-1 · Detection.** A pair exists while both files are in the library (not in Trash and still
present on disk), both are matched to the same item from the same provider, and the owner has not
resolved that pair.

- [ ] Given two live files matched to the same provider item, when the owner opens Duplicates, then the pair is listed
- [ ] Given two files matched to the same item ID but from different providers, then no pair is listed
- [ ] Given a file with no provider match, then it never appears in a video pair
- [ ] Given a listed pair, when either file is moved to Trash, goes missing from disk or is re-matched to a different item, then the pair leaves the queue
- [ ] Given a pair left the queue because one file was trashed, when that file is restored and the pair is still unresolved, then the pair returns
- [ ] Given three or more files share one match, then every two-file combination is listed as its own pair
- [ ] Given a file is newly matched to an item another live file already carries, then the pair appears without a restart

**P0-2 · A Videos lane on the Duplicates page.** Video pairs appear on the existing owner
Duplicates page as their own group, with a count, and can be filtered to on their own like every
other type.

- [ ] Given at least one video pair, when the owner opens Duplicates, then a Videos group shows the number of pairs
- [ ] Given the owner filters to videos, then only video pairs are shown
- [ ] Given no video pairs, then no Videos group is shown
- [ ] A visitor never sees video pairs, or any sign of them

**P0-3 · Side-by-side compare.** Each pair shows both files in the same layout, so the facts line
up across the two sides: poster, file name and folder, resolution, duration, file size, video
codec, bitrate, container, and the edition and part when set. It also shows a summary of the
owner's work on each file: playlists, film link, and manual edits.

- [ ] Given a pair, when the compare view is shown, then both sides list the same facts in the same order
- [ ] Given a fact is unknown for one side, then that side shows nothing for it. It never shows a dash or "unknown" (the F68 rule)
- [ ] Given two files with different resolutions, then the panel shows both values and never marks one "better" or "recommended"
- [ ] Given a side, when the owner chooses to open it, then that file's media page opens without losing the owner's place in the queue
- [ ] At the mobile breakpoint the two sides stack, first above second

**P0-4 · Keep one.** Each side offers **Keep this one**. It is two-step: the confirm names the copy
going to Trash and lists exactly what will carry over to the kept copy. If nothing will carry
over, it says so.

- [ ] Given the owner confirms Keep this one, then the other file moves to Trash and the pair leaves the queue
- [ ] Given the confirm is open, then it lists what will carry over, for example "2 playlists · film link · 3 field edits", or says that nothing will
- [ ] Given the owner cancels, then nothing changes
- [ ] No single click trashes a file
- [ ] Given the trashed copy is later restored, then it returns without the playlist places and film link that moved, and with all of its own edits intact

**P0-5 · What carries over.** Carrying over is additive. The kept copy keeps everything of its own,
and gains:

- **Playlists.** The kept copy takes the trashed copy's place in every playlist the kept copy isn't already in, at the same position. In playlists that hold both, the kept copy stays where it is.
- **Film link.** If the kept copy has no link to that film, it takes the trashed copy's link, including the full-film flag and scene number. If it already has one, its own link stands.
- **Field edits.** For each field the kept copy has no edit of its own on, it adopts the trashed copy's edit. This covers the owner's chosen source or value, added or removed values, and not-applicable marks. **Part and edition never carry** (RD5).
- **Manual tags.** Tags the owner added by hand are added to the kept copy.

Acceptance criteria:

- [ ] Given both copies have an edit on the same field, then the kept copy's edit is unchanged
- [ ] Given only the trashed copy has an edit on a field, then the kept copy ends up with that edit
- [ ] Given the trashed copy is labeled Part 2, then the kept copy's part is unchanged
- [ ] Given both copies are in the same playlist, then that playlist holds the kept copy once, at its own position
- [ ] Given the kept copy is not in a playlist that holds the trashed copy, then the kept copy appears at the trashed copy's position
- [ ] Given the carry-over cannot complete, then nothing changes, neither file is trashed, and the owner sees an error

**P0-6 · Keep both.** Resolves the pair with no other change.

- [ ] Given the owner chooses Keep both, then the pair leaves the queue and never returns for those two files (RD7)
- [ ] Given a kept-both pair, when either file is re-matched, rescanned or the app restarts, then the pair still does not return

**P0-7 · Label as editions.** The owner gives each file an edition label, or one file a label and
the other none (for example "Director's Cut" against the unlabeled original). Saving sets the
editions and resolves the pair as keep-both (RD6).

- [ ] Given the owner saves edition labels, then each file's edition shows the label and the pair leaves the queue
- [ ] Given both labels are left empty, then the owner can't save
- [ ] Given edition is not a settable field on this library, then the action is not offered

**P0-8 · Label as parts.** The owner gives each file a part number. Saving sets them and resolves
the pair as keep-both.

- [ ] Given the owner saves Part 1 and Part 2, then each file shows its part and the pair leaves the queue
- [ ] Given the same part number on both files, then the owner can't save
- [ ] Given a file already has a part, then the field starts with it

**P0-9 · Keyboard and focus.** Every action is reachable by keyboard. When a pair is resolved,
focus moves to the next pair, or to the group heading when it was the last one.

**P0-10 · Cinémathèque QA.** Tokens only.

### Nice-to-Have (P1)

- **P1-1 · Frame strips.** Frames taken at the same few timestamps from both files, shown in aligned rows. This is the evidence for spotting a different intro, a watermark or a re-encode. It needs a frame-extraction capability that does not exist yet.
- **P1-2 · Duplicates banner on the Media list.** "N possible duplicate videos", linking to the Videos group, as the people, studios and tags lists already have.
- **P1-3 · Difference emphasis.** A neutral marker on the facts that differ between the two sides. It never says which side is better.

### Future Considerations (P2)

- **P2-1 · One group per item for three or more files**, instead of one row per pair. None exist today.
- **P2-2 · Detecting copies among unmatched files**, once frame signatures exist (see the scene model).
- **P2-3 · Mark as related media** (clip, trailer, promo), and a preferred copy. Both belong to the scene model ([HOLODEX-520](https://whoiskevinrich.atlassian.net/browse/HOLODEX-520)).
- **P2-4 · Carry over a custom poster.** For now a custom poster stays with its file.

## Success Metrics

- **Leading:** the 21 known pairs are worked to zero within two weeks of ship.
- **Leading:** no owner work is lost. A counts-only probe after the queue is worked finds no trashed copy holding a playlist place, film link or field edit that its kept copy lacks.
- **Lagging:** as provider matching coverage grows past 984 files, new pairs keep appearing and keep being resolved. A growing open count means the queue isn't being worked.

## Open Questions

None blocking. The two decisions that could have blocked are RD4 (carry-over) and RD8 (v1
evidence), both decided by the owner 2026-10-08. Layout, the lane's position among the existing
groups, and whether a pair starts expanded are for the design handoff.

## Timeline / routing

No hard deadline. Routing per `CLAUDE.md`:

1. Spec (this document).
2. `/design-handoff`: done 2026-10-08, [duplicate-videos-handoff.md](../design/duplicate-videos-handoff.md).
3. `/implement`: design sign-off and the draft PR.
4. Build.
5. `/testing-strategy`.
6. `/security-review`: an owner-only surface, with a new destructive action (Trash) and a multi-record carry-over.
7. `/code-review high --fix`.

**Architecture:** no technology fork. The pairs are a new producer on the existing duplicate review
queue, and nothing new is extracted from files in v1. The build PR adds that producer to the
producer list in [`entity-identity.md`](../architecture/entity-identity.md) so the topic doc stays
accurate.

**Dependency:** [HOLODEX-457](https://whoiskevinrich.atlassian.net/browse/HOLODEX-457) plans to
re-home the per-file provider match record that this detector reads. Whichever lands second keeps
video pair detection working.
