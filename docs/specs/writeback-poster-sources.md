# Spec: Choose the video poster in the writeback dialog (F85)

**Status**: Draft
**Epic**: HOLODEX-599 (absorbs HOLODEX-597)
**Owner**: Project owner
**Date**: 2026-10-27

**Builds on**: the writeback dialog's Poster row
([`field-source-of-truth.md`](field-source-of-truth.md), HOLODEX-403), the uploaded poster
([`video-owner-mode-editing.md`](video-owner-mode-editing.md), F52), and the frame picker
([`thumbnail-frame-picker.md`](thumbnail-frame-picker.md), F84).

**Follow-up**: HOLODEX-600 lets a provider offer several posters. Until then each provider offers
the one poster it has today.

**Architecture**: an image Holodex holds as a poster decision
([field-resolution.md](../architecture/field-resolution.md)), the poster source slots and the served
poster ([media-ingest.md](../architecture/media-ingest.md)), embedding it at writeback
([writeback.md](../architecture/writeback.md)), and its perimeter
([security-perimeter.md](../architecture/security-perimeter.md)). **Design handoff**:
[writeback-poster-chooser-handoff.md](../design/writeback-poster-chooser-handoff.md).

---

## Problem statement

The owner can't manage a video's poster from one place. The page shows whichever image last landed
in its thumbnail: the file's cover art, the automatic frame, an upload, or a picked frame. The
writeback dialog offers only the file's cover art and the provider posters. A frame picked with
the frame picker, or a poster the owner uploaded, never reaches the file, which is why HOLODEX-597
was filed. A writeback can also quietly replace an uploaded poster on the page with the file's
cover art.

## Goals

1. **Every poster the owner can see is one they can put in the file.** The writeback dialog's
   Poster row offers the file's cover art, the owner's picked frame, the owner's upload, the
   automatic frame, and each provider's poster.
2. **The page shows the owner's choice at once.** A picked frame, an upload, or a poster written
   from the dialog becomes the page's poster straight away, for owner and visitors alike.
3. **A choice reaches the file on the next writeback.** Picking a frame or uploading stages that
   image, and the next Write embeds it. Nothing is written behind the owner's back.
4. **The file is left alone when nothing is chosen.** The automatic frame is only what the page
   shows. It reaches the file only when the owner picks it.

## Non-goals

- **Choosing the poster on the media page.** The dialog is the only chooser. The page keeps its
  existing controls, Pick a frame and Upload / Remove, which make images rather than choose them.
- **Several posters per provider.** That's HOLODEX-600.
- **A history of picked frames or uploads.** Holodex keeps the newest frame and the newest upload,
  one of each.
- **Writing immediately.** Picking a frame or uploading doesn't start a writeback.
- **Pasting a poster URL.** It still couldn't pass the provider image allowlist.

## User stories

- As the owner, I want to pick a frame and know it will end up in the file, so that other players
  show the same poster Holodex does.
- As the owner, I want to switch back to my upload, or to the frame I picked earlier, without
  redoing either one.
- As the owner, I want to see every candidate poster side by side before I write one.
- As the owner, I want a file with no cover art left untouched unless I choose something for it.
- As a visitor, I want to see the poster the owner chose, not whatever was generated last.

## Requirements

### P0

- **P0-1 Poster row tiles.** The Poster row in the writeback dialog shows these groups, in this
  order, each with a short caption:
  - **In the file**: `·file`, the cover art embedded in the file. When there is none, the tile
    says so.
  - **Holodex**: `·frame`, the owner's newest picked frame, and `·upload`, the owner's newest
    upload. Each appears only when it exists.
  - **Automatic**: `·automatic`, the frame Holodex generates. Holodex generates it only for a file
    with no cover art, so the group appears only when that frame exists. For any other frame the
    owner uses Pick a frame.
  - **One group per provider**: that provider's poster. A provider with no poster shows its
    caption with "no poster" and no tile.
- **P0-2 Same row contract.** The tiles keep today's behaviour for the row: picking a tile stages
  it, Write confirms it, the gutter shows will-write or in-sync, and selection never relies on
  colour alone.
- **P0-3 Picking a frame stages it.** "Use this frame" in the frame picker makes that frame the
  page's poster at once and makes `·frame` the decided poster. The next time the dialog opens, the
  Poster row shows `·frame` selected and will-write.
  - Given a video whose file has cover art, when the owner picks a frame, then the page shows the
    frame, and the dialog's Poster row has `·frame` checked and marked will-write.
- **P0-4 Uploading stages it.** Uploading a poster on the page does the same as P0-3, for
  `·upload`.
- **P0-5 Write embeds the choice.** Writing with `·frame`, `·upload` or `·automatic` chosen embeds
  that image as the file's cover art. Holodex already holds the image, so nothing is downloaded.
  Writing with a provider tile chosen behaves as it does today.
  - Given `·frame` is chosen, when the owner writes an MKV or an MP4, then the file's cover art is
    the picked frame and the row then reads in-sync.
- **P0-6 The page follows the choice.** After a successful Write, the page's poster is the chosen
  image, whichever tile it came from. Only a new choice changes it: a writeback that leaves the
  poster alone never swaps it for the file's cover art or anything else.
  - Given an uploaded poster is chosen, when any writeback for the video completes, then the page
    still shows the upload. (Today the file's cover art can replace it. Reproduce this with a
    failing test first.)
- **P0-7 Automatic is a fallback only.** With no choice made, the page shows the file's cover art,
  or the automatic frame when the file has none, as it does today. Writeback never embeds the
  automatic frame unless the owner chose `·automatic`.
- **P0-8 Newest of each kind.** A new picked frame replaces the previous `·frame` image. A new
  upload replaces the previous `·upload` image. Each keeps its own tile, so picking a frame never
  discards the upload, and the other way round.
- **P0-9 Removing the chosen upload.** Removing the upload while `·upload` is the choice clears the
  choice: the page falls back as in P0-7, and the file keeps what it has until the owner picks
  again. Removing an upload that isn't the choice just removes its tile.
- **P0-10 Button label.** The page's "Regenerate thumbnail" button is relabelled **Pick a frame**.
  "Reset to automatic" stays inside the picker.
- **P0-11 Retire the old note.** The dialog's "Your uploaded poster stays on the page" note goes
  away. With P0-6 the page always shows the choice, and an upload is a tile like any other.

### P1

- **P1-1 Where a tile came from.** Hovering or focusing `·frame` shows the picked time, for
  example "frame at 12:40".

## Open questions

- **Engineering.** If the owner's chosen image is missing from storage, for example after a data
  restore, does the tile disappear, and does the choice fall back as in P0-9? This doesn't block
  the work; decide it during the build.
## Timeline considerations

- HOLODEX-600 changes the provider groups from one tile to several. Build P0-1's groups so that
  each one holds a list, even though today's lists have one item.
- The architecture doc edits (an owner-made image as the decided poster) must land before the
  build, because P0-5 writes an image Holodex made, not one a provider sent, into the owner's file.
