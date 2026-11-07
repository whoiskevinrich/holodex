# Spec: Video Trimming (F88)

**Status**: Draft
**Phase**: Media management (owner-facing)
**Depends on**: [video file maintenance](video-file-maintenance.md) (F77: the Maintenance list,
background runs, preflight refusals, verification gate and held original);
[thumbnail frame picker](thumbnail-frame-picker.md) (F84: scrubbing and in-browser frame capture);
[duplicate videos](duplicate-videos.md) (labelling two files as editions).
**Related**: [entity identity card](entity-identity-card.md) (the edition field);
[delete media](delete-media.md) (undoing an edition); [moving thumbnails](moving-thumbnails.md).
**Architecture**: [file-maintenance.md](../architecture/file-maintenance.md) (the trim sections).
**Design handoff**: [video-trimming-handoff.md](../design/video-trimming-handoff.md).

---

## Objective

Let the **owner** cut unwanted material off the **start and/or end** of a video file, such as an intro
bumper, an ad or end cards, from the **Media Details** page. The owner chooses the first and last
frames to keep by scrubbing the video, starts the trim, and walks away. The cut runs in the
background, is checked against the original file and, where the frames differ enough to tell, against
what the owner saw on screen, and then either:

- **replaces the original**: the same library item, with the original held for the F77 window, or
- **saves as an edition**: a new library item in its own file, carrying everything the original had
  except its edition, and labelled as an edition of the original.

Trimming is the second entry in the F77 **Maintenance** list. It reuses that feature's confirm step,
background run, refusals, verification gate and held original. It brings its own output contract,
because a trim changes the duration and the chapters, which F77's contract forbids.

> **Why change the file, not skip at playback.** Storing in and out points and having the player skip
> past them would be instant, lossless and reversible. But the extra material would still be in the
> file for every other player, for backups and for the file on disk. The owner wants it gone from the
> file. Rejected 2026-10-31.

> **Why any frame, not keyframes only.** A cut made without re-encoding can only land on a keyframe,
> which may be seconds from the frame the owner chose. Snapping the handles to keyframes would be
> lossless and fast, but the owner wants the exact frame. The run is a background job, so the
> re-encode's time costs nothing at the screen. Chosen 2026-10-31. A lossless keyframe mode is P2.

> **Why an edition is copied once, then diverges.** Keeping the two items in sync would match "same
> film, different cut", but it needs a shared identity Holodex doesn't have. A one-time copy makes the
> edition an ordinary item from then on. Chosen 2026-10-31. Revisit if the owner finds themself fixing
> the same metadata on both.

---

## Scope

### In scope
- A **Trim** entry in the Maintenance list on Media Details, for the owner in Admin mode only.
- **A trim picker** built on the F84 frame picker. The owner scrubs and sets an **in frame** (the first
  frame to keep) and an **out frame** (the last frame to keep), stepping one frame at a time. Either
  end may be left at the file's own start or end, but not both.
- **Two save modes**: **Replace original** and **Save as edition**.
- **Background execution** with visible status, exactly as F77.
- **Frame verification**: the cut file's first and last frames are checked against the original
  file's in and out frames, and against the frames the owner chose on screen where those can be
  told apart from their neighbours (R6).
- **Edition creation**: a new file beside the original and a new library item copied from it.
- **Per file.** A trim targets the file of the page it's run from. Other parts of a multi-part video
  are untouched.

### Out of scope (each a tracked follow-up or a deliberate no)
- **Cutting from the middle**, or keeping several ranges. A trim keeps one continuous range.
- **Save as a new, unrelated item** (clip extraction). That's a different job from tidying a video.
- **Editions that stay in sync** with their original after the trim (see the Objective).
- **Bulk trimming**, for example the same intro across a series. Each trim is judged per video, as
  in F77.
- **Trimming files this browser can't play.** The picker needs the browser to decode the video, as
  F84 does.

---

## Output contract

The rules a trim output must satisfy before it lands. They are F77's contract, with the rules a trim
must break replaced. The verification gate enforces them.

1. **Place.** **Replace**: same path, filename and container as the original. **Edition**: same folder
   and container as the original, with a new filename that carries the edition (see R9).
2. **Same streams, same order.** As F77 rule 2.
3. **One kept range.** Every stream (video, audio and subtitles) is cut to the same range: from the in
   frame to the out frame, inclusive. Audio stays in sync with the picture.
4. **Same tags.** As F77 rule 4, with the same exceptions. Stream-statistics and duration tags are
   recomputed for the new length on every stream, not only the video.
   An edition's file carries the new edition value, and no other tag differs.
5. **Chapters follow the cut.** A chapter wholly inside the kept range keeps its title and is moved
   earlier by the length cut from the start. A chapter wholly outside the range is dropped. A chapter
   that straddles a cut point is clamped to the range. A file whose chapters were all dropped has none.
6. **Same shape of picture.** As F77 rule 6.
7. **Right length.** The duration equals the kept range to within one frame.
8. **Right frames.** The output's first frame is the in frame and its last frame is the out frame
   (see R6).

If a rule can't be met for a file, the trim is refused before it starts.

---

## User stories

**Owner**
1. As the owner, I want to cut a bumper off the start of a video from its details page, so that I
   don't need a separate editor and a rescan.
2. As the owner, I want to pick the exact first and last frames to keep, so that I don't lose a second
   of the scene or keep a second of the bumper.
3. As the owner, I want to start a trim and leave, so that I can trim several videos in a sitting and
   check them later.
4. As the owner, I want a trim checked against the frames I picked, so that I can trust it without
   rewatching the edges.
5. As the owner, I want to replace the original and still be able to undo for a while, so that a
   wrong trim isn't permanent.
6. As the owner, I want to keep the original and save the trimmed cut as an edition, so that I have
   both versions with all their metadata in place.
7. As the owner, I want a new edition not to show up as a duplicate of its original, so that the
   duplicates queue doesn't fill with my own trims.

**Visitor**
8. As a visitor, I never see trim controls or trim status. A trimmed video simply plays, and an
   edition is an ordinary item.

---

## Requirements

### P0 — must have

**R1. Owner-only Trim entry.**
- [ ] Trim appears in the Maintenance list only for the owner in Admin mode.
- [ ] The server rejects trim requests from non-owners.

**R2. Trim picker.**
- [ ] The picker scrubs the video and steps one frame at a time, as F84 does, and shows no sound.
- [ ] The owner sets an in frame and an out frame. The kept range is shown on the scrubber, with the
      time cut from the start, the time cut from the end and the kept length.
- [ ] The in frame comes before the out frame. A range that keeps the whole file can't be confirmed.
- [ ] When the owner confirms, the picker captures the in frame and the out frame as they appear on
      screen. R6's screen check uses those captures.

**R3. Preflight refusals.** F77's refusals apply (read-only library, free space, encoder, missing
file, deleted video), with the free space sized for the kept range. Also:
- [ ] A file with a second video track is refused, because that track could only be cut on a keyframe.
- [ ] A Matroska file with FLAC audio is trimmed like any other, with its audio intact and in sync.
- [ ] **Save as edition** is refused when the edition name is empty, equals the original's edition, or
      the target filename already exists.

**R4. Confirm step.**
- [ ] States the save mode, the kept range, and that the video will be re-encoded.
- [ ] **Replace**: states the date the held original will be deleted, as F77 does.
- [ ] **Edition**: asks for the edition name, prefilled with "Trimmed", and shows the new filename.

**R5. Background run and status.** As F77 R5: confirming returns at once, the video shows queued,
running, done or failed, failures offer Retry and Dismiss, and one file write runs at a time per
file. For an edition, the status is shown on the original's page until the edition exists.

**R6. Frame verification.** Two checks, because they catch different mistakes.
- [ ] **The cut check (always).** After cutting, Holodex decodes the in frame and the out frame from
      the original file, the same way it decoded the cut. The output's first frame must be the in
      frame and its last frame the out frame, not a neighbour. This check is exact, so it holds on a
      still image, a black frame or a slow fade too.
- [ ] **The screen check (only where it can tell).** Holodex compares each frame the owner saw on
      screen (R2) with the original's in or out frame and its neighbours. This catches a browser and
      Holodex disagreeing about which frame a moment in the video names. It compares the shape of
      the picture, not its brightness, because a browser shows HDR video at a different brightness.
- [ ] The screen check is skipped for a frame that can't be told apart from its neighbours: a
      still image, black, or a slow fade. A one-frame difference there can't be seen either. The
      trim's status says the check was skipped and why.
- [ ] If either check finds the cut a frame or more off, Holodex shifts the cut by the measured
      offset and runs once more.
- [ ] If the second run still doesn't match, the trim fails with a reason that names the check and
      the offset. Nothing lands: the original is untouched and no edition is created.
- [ ] The rest of the output contract is verified as F77 R6 does. A crash mid-trim leaves no partial
      output.

**R7. Replace: library continuity.**
- [ ] The video is the same library item afterwards, as F77 R7: same id, people, Tags, edits,
      decisions, enrichment and history.
- [ ] Duration and size are refreshed without a manual rescan.
- [ ] The automatic thumbnail and the moving thumbnails are regenerated from the trimmed file.
- [ ] A thumbnail or poster the owner uploaded or picked is kept, even if its frame was cut.
- [ ] The original is held as F77 R8 describes, with Restore original and Keep change.

**R8. Edition: what is copied.**
- [ ] The edition is a new library item for the new file.
- [ ] It copies the original's current people, Tags, per-field edits, source decisions, enrichment,
      poster, film links and playlist places. Its edition is the name from R4. Nothing else differs,
      with three exceptions. A numbered scene in a film keeps the link without the number, because the
      original still holds it. It starts with no rename history and no locked name, because its file
      has never been renamed. It starts unwatched, with no plays, because nobody has watched this cut
      yet (chosen 2026-11-07).
- [ ] Its history starts with one entry, "trimmed from" the original, linking to it. The original's
      earlier history stays with the original.
- [ ] After creation the two items are independent. An edit to one never changes the other.
- [ ] The original item and file are untouched. Nothing is held, because there is nothing to undo.
      The owner removes an unwanted edition with the existing delete.

**R9. Edition: file and duplicates.**
- [ ] The new file sits in the original's folder. Its name carries the edition in the form Holodex
      already reads from filenames, so a rescan from scratch still recognises it as that edition.
- [ ] The original and the edition are labelled as editions of each other, as a duplicates answer
      would do. They never enter the duplicates queue as a pair.
- [ ] A part number on the original is copied to the edition, so a trimmed part stays the same part.

### P1 — nice to have
- **Progress and cancel**, as F77 P1.
- **Preview the edges.** Play two seconds either side of each cut point in the picker before
  confirming.
- **Jump to scene changes.** Step the picker to the next or previous detected cut, since a bumper
  usually ends on one.

### P2 — future (design for, don't build)
- **Smart cut.** Re-encode only the frames around each cut point and copy the rest, for lossless
  quality away from the edges. Same output contract.
- **Lossless keyframe mode.** Handles snap to keyframes and nothing is re-encoded.
- **Chaining with other maintenance**, as F77 P2: trim and flip in one re-encode.

---

## Success measures

- **Frame accuracy: 100% of landed trims.** Every trim that lands starts on the owner's in frame and
  ends on their out frame. Checked by tests over MP4 and MKV fixtures, including an MP4 with an edit
  list and a variable-frame-rate file.
- **Zero library drift.** A replace changes no person, tag, edit or decision. An edition matches its
  original in everything except edition and history. Checked by before-and-after tests.
- **Zero duplicate noise.** No edition created by a trim appears in the duplicates queue with its
  original.
- **Owner time.** A trim is set up entirely from the details page. Today it takes an external editor,
  a re-tag and a rescan.

---

## Open questions

- ~~Q1~~ **Resolved 2026-11-02 by a spike:** no. On an MKV and an MP4, each longer than ten minutes,
  cut near both ends, comparing a frame shown in the browser with Holodex's own decode can't tell
  neighbouring frames apart on still images, black or slow fades. On an HDR file it can't compare
  brightness at all. Comparing Holodex's cut with its own decode of the original is exact. So R6
  checks the cut against the original always, and against the screen only where the frames differ.
  Where the frames did differ, the browser and Holodex agreed on the frame at every point tested.
  Results are on the epic's ticket.
- ~~Q2~~ **Resolved 2026-10-31:** an edition's history starts fresh with a "trimmed from" link.
- ~~Q3~~ **Resolved 2026-10-31:** a mismatch is corrected once by the measured offset, then fails.

---

## Timeline / phasing

- **Phase 0 (done 2026-11-02):** the Q1 spike, on an MKV and an MP4 each longer than ten minutes. It
  reshaped R6 into the cut check and the screen check.
- **Phase 1 (this spec, P0):** both save modes, frame verification, edition copy.
- **Phase 2:** P1 items. Previewing the edges comes first, because it is the owner's last look
  before walking away.

**Gates this feature will need:** architecture (full re-encode versus smart cut, frame verification,
and how an edition's library state is cloned), design handoff (the trim picker and the edition
confirm step), testing strategy (the fixture matrix for frame accuracy), and security review (a new
write path that creates files in the media library).
