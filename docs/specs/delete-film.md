# Spec: Delete a film (F78)

**Status**: Accepted
**Phase**: F78 (Jira [HOLODEX-562](https://whoiskevinrich.atlassian.net/browse/HOLODEX-562))
**Owner**: Project owner
**Date**: 2026-10-11
**Feature block**: **F78**. The owner can delete a film from its detail page with the same two-tier
safety net media already has: move it to Trash, where it can be restored until the grace period
runs out, or delete it permanently.

**Depends on** (all shipped): [Delete a media item (F24)](delete-media.md), which provides the Trash,
the grace period and the purge, and [Films as a first-class entity (F56)](films-entity.md).

**Design handoff**: [`docs/design/delete-film-handoff.md`](../design/delete-film-handoff.md).

---

## Problem Statement

A film can be created, but nothing can remove it. A duplicate, a film created by mistake, or a
film the owner no longer cares about stays in the Films list forever. Its scenes stay grouped under
it, and it keeps showing up on every person, studio and tag page it touches. Media has had a
deletion path with a safety net since F24. Films have none.

## Goals

1. **The owner can delete a film from the Film Detail page**, with controls that look and behave
   exactly like the media detail page's: same place, same wording, same confirm steps.
2. **Deleting a film never deletes media.** The film's videos stay in the library. Only their
   membership in this film goes away.
3. **A mistaken delete is recoverable.** Moving a film to Trash keeps everything about it. A
   restore brings it back exactly as it was, scenes and all.

## Non-Goals

- **Deleting the film's videos with it.** There's no option to trash or delete the linked videos
  as part of a film delete. *(Why: a film is a grouping the owner asserted. Removing the grouping
  shouldn't put files at risk, and the media page already deletes media one by one.)*
- **Bulk film delete** from the Films list. One film at a time, the same cut F24 made for media.
- **Deleting people, studios or tags.** That's still out of scope, as it is in F24.
- **A film-specific grace period.** Films use the media one.

## Resolved Decisions

All settled by the owner on 2026-10-11.

- **RD1, full parity with media.** Film deletion has the same two tiers as media: *Move to Trash*
  (restorable during the grace period, then purged automatically) and *Delete permanently*.
- **RD2, linked videos are only unlinked.** Neither tier touches a linked video. The video stays in
  the library and on disk. It just stops being a scene or the full-film file of this film. No file
  on disk is rewritten.
- **RD3, a permanent delete leaves nothing behind.** The film's cast roles, aliases, external
  links, enrichment, the owner's decisions about it, and its poster and banner images all go,
  including the image files on disk.
- **RD4, where the owner lands.** After either kind of delete, the owner goes back to where they
  came from if they reached the film from inside the app. Otherwise they land on the Films list.
  Media falls back to Home instead. Films fall back to the Films list because that's the film's
  own home.
- **RD5, a trashed film is hidden but intact.** While a film is in Trash it disappears everywhere
  a visitor or the owner could meet it: the Films list, search, the Films section of every media
  page, the Films row on person, studio and tag pages, and the film pickers. Opening its page
  directly shows not-found. Nothing about it is dropped while it waits, so a restore puts it back
  in every one of those places, with its scenes in their original order. Its full-film files,
  which the media list normally hides because the film stands in for them, show in the media list
  again while it is in Trash, because nothing stands in for them.
- **RD6, a trashed film still holds its name.** If the owner creates or renames a film to the same
  name and year as a film in Trash, the action is refused and the owner is offered a restore. The
  wording is roughly: "*Heat (1995)* is in Trash. Restore it instead?"
- **RD7, one Trash.** Films appear in the same Trash list as media, newest deletions first. Each row
  carries a small *Film* or *Media* label. Restore and Delete permanently work the same for both
  kinds.
- **RD8, one grace period.** A trashed film is purged on the same schedule as trashed media, under
  the same setting.
- **RD9, a trashed film stops naming its videos.** A film offers its name as a candidate for its
  videos' title and album. While the film is in Trash it withdraws that candidate, and restoring it
  brings the candidate back. Nothing already written to a file changes. A video whose owner
  explicitly chose this film as its title or album source shows no value for that field while
  the film is in Trash, the same as when a film is detached. The choice is kept so a restore
  brings it back. Only a purge clears it (RD10).
- **RD10, a standing choice of a purged film falls through.** If the owner chose this film as a
  video's title or album source, that choice behaves like any other chosen source that's no longer
  there once the film is permanently deleted: the video shows its next source. The stale choice is
  cleared at purge, so it can't come back.

## User Stories

- As the owner, I open a film I created by mistake, move it to Trash, and I'm taken back to the
  list I came from. The film is gone from it.
- As the owner, I trash a film, realise it was the wrong one, open Trash, and restore it. Its
  scenes, cast and poster are all back.
- As the owner, I permanently delete a duplicate film. Its videos are still in my library, and
  they no longer list that film.
- As the owner, I try to create "Heat (1995)" while the old one is in Trash, and I'm offered the
  chance to restore it instead.
- As a visitor, I never see delete controls on a film page.

## Requirements

### Must-have (P0)

- **P0-1, Manage section on the Film Detail page.** At the bottom of the film page's rail, the
  owner sees a *Manage* section that matches the media page's. It holds one split button: the main
  part says **Move to Trash**, and a chevron reveals **Delete permanently…**. Visitors, and the
  owner in visitor view, never see it.
- **P0-2, Move to Trash confirm.** Choosing Move to Trash asks for confirmation first. The dialog
  names the film with its year, says it will be hidden from the library and when it will be
  permanently deleted, says it can be restored from Trash until then, and says how many linked
  videos it has, all of which stay in the library. Cancel is the default.
- **P0-3, Delete permanently confirm.** Choosing Delete permanently asks for a distinct, stronger
  confirmation. It names the film and says its cast, images and aliases will be permanently
  removed and that this can't be undone. It also says how many linked videos it has, that they
  stay in the library unlinked from this film, and that no media file is touched. Unlike the media
  dialog, there's no file path, because a film has no file. Delete permanently works whether or not
  the film is already in Trash.
- **P0-4, After a delete.** On success the owner lands as described in RD4. On failure the dialog
  stays open and shows the error, exactly as on the media page.
- **P0-5, Hidden while trashed (RD5).** A trashed film is absent from every surface listed in RD5,
  and from search and listing counts. Its page and its images show not-found. Its videos show no
  link to it. Every owner action on a trashed film other than restore or permanent delete answers
  not-found, as its page does.
- **P0-6, Trash lists films (RD7).** The Trash page shows trashed films alongside trashed media,
  each with its kind label, when it was deleted, and when it will be purged. It offers Restore and
  Delete permanently. Restoring a film returns it to every surface in RD5 untouched.
- **P0-7, Automatic purge (RD8).** When a trashed film's grace period runs out, it's deleted
  permanently with the same result as P0-3, on the same schedule as media.
- **P0-8, Name held while trashed (RD6).** Creating, renaming or year-editing a film into the name
  and year of a trashed film is refused, and the owner is offered a restore of the trashed one.
- **P0-9, Owner only.** Every delete, restore and purge action is refused for anyone but the owner.

### Should-have (P1)

None.

### Future considerations (P2)

- **P2-1, Bulk film delete** from the Films list, if duplicate cleanup turns out to come in batches.

## Open Questions

None.
