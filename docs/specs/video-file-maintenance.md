# Spec: Video File Maintenance (F77)

**Status**: Draft
**Phase**: Media management (owner-facing)
**Depends on**: the owner gate and Admin mode; the async file-write model of
[fire-and-forget writeback](fire-and-forget-writeback.md); re-extraction after a write
([metadata refresh](metadata-refresh.md)); [media parts](media-parts.md).
**Related**: [delete media](delete-media.md) (the grace-window pattern reused for held originals);
[video poster sizes](video-poster-multi-size.md) (thumbnail regeneration).
**Architecture**: [writeback.md](../architecture/writeback.md) (to be extended: a second kind of
file write, one that replaces the picture rather than the tags) ·
[observability-and-jobs.md](../architecture/observability-and-jobs.md).
**Design handoff**: _pending (`/design-handoff`)_.

---

## Objective

Let the **owner** repair a video file's picture from the **Media Details** page without leaving
Holodex or losing anything the file carries. The first repair is **Flip horizontally**, for files
that were recorded or published mirrored. More repairs, starting with aspect-ratio corrections, will
join it later as entries in the same **Maintenance** list, so the surface is built as a list from day
one even though v1 holds only one entry.

A flip changes the picture and **nothing else**. The output keeps the same path, filename, container
and streams, and every metadata tag the original carried. The library item is the same item
afterwards: its people, tags, edits, source decisions, history and identity are untouched.

A flip has to re-encode the video, so it cannot be reversed by replaying tag values the way a tag
write can. As a safety net, Holodex **holds the original file** for a configurable window (default
**7 days**). During the window the owner can **restore the original** or **keep the change**, which
discards the original early. When the window expires, the held original is deleted automatically.

> **Why re-encode the pixels, not set a display flag.** Some containers can carry a "display
> mirrored" flag that flips playback without touching a pixel, which is lossless and instant. Browser
> support for flip flags is unreliable, so the in-app player could keep showing the mirrored
> picture, and the flag is lost the first time anything remuxes the file. A re-encoded flip plays
> correctly everywhere. Rejected 2026-10-10.

> **Why hold the original instead of an atomic replace alone.** A tag write is undone by writing
> the old values back. A re-encode has no such inverse: once the swap lands, the original pixels
> exist nowhere else. A time-boxed hold gives the owner a real undo without a library that slowly
> fills with `.orig` files. Chosen over "keep until confirmed" (unbounded disk use) and "no backup"
> (no undo) on 2026-10-10.

---

## Scope

### In scope
- A **Maintenance** section on the Media Details page, visible only to the owner in Admin mode, that
  lists the available operations. v1 has one: **Flip horizontally**.
- **Per file.** A maintenance operation targets one file. On a multi-part video the owner picks which
  part's file to act on, and the other parts are not touched.
- **A confirm step** that names the file, says the video will be re-encoded, gives the free-space
  requirement, and gives the date the held original will be deleted.
- **Background execution.** The operation runs asynchronously, like a tag write. Its status (queued,
  running, done, failed) belongs to the video and shows on the page, and each run is recorded in
  activity / job history.
- **Lossless-everything-but-the-picture output** (see "Output contract").
- **A verification gate.** The new file replaces the original only after Holodex has checked it
  against the output contract. A file that fails verification is discarded and the original stays in
  place, untouched.
- **A held original** with **Restore original**, **Keep change** (discard the original now) and
  **automatic expiry**.
- **Refresh after the swap.** File facts (duration, resolution, size) are re-read, and the thumbnail
  is regenerated from the flipped picture. An owner-uploaded poster is never replaced.
- **Refusing up front** when the operation cannot succeed: the media library is mounted read-only,
  there is not enough free space, the file's video codec has no available encoder, or the file has no
  video stream.

### Out of scope (each a tracked follow-up or a deliberate no)
- **Aspect-ratio operations** (correcting a wrong display aspect ratio, crop, pad). These are the
  next entries in the list and are P2 below. The list and the output contract are designed so they
  slot in without a new surface.
- **Vertical flip and rotation.** Cheap to add as further entries, but not asked for yet.
- **Bulk maintenance** across many videos from Browse or a selection. Repairs are judged one video
  at a time, and a wrong batch re-encode is expensive to undo.
- **Changing codec, container or quality** (transcoding to "fix" a file). Maintenance repairs the
  picture. It does not modernise the file.
- **Trimming or cutting.** That changes duration and chapters, and so breaks the output contract.
- **Undo after the window.** Once a held original expires or the owner keeps the change, the flip is
  permanent. Flipping again is the only way back, and it costs a second generation of re-encoding.

---

## Output contract

The rules every maintenance output must satisfy before it may replace the original. The
verification gate enforces them; it does not trust the encoder's exit status alone.

1. **Same place.** Same path, same filename, same container format.
2. **Same streams, same order.** Every stream in the original is present in the output in the same
   order, with the same type, language, title and default/forced flags.
3. **Only the video changes.** The video stream is re-encoded. Every audio, subtitle, data and
   attachment stream (embedded cover art included) is copied bit-for-bit.
4. **Same tags.** Every container-level and stream-level metadata tag in the original is present in
   the output with an identical value, and the output carries no tag the original lacked. This covers
   tags Holodex itself wrote. The encoder must not add its own name or settings as tags. _(See open
   question Q1 about stream-statistics tags.)_
5. **Same chapters.** Chapter count, titles and timestamps are identical.
6. **Same shape of picture.** The re-encoded video keeps the original's codec, resolution, frame
   rate, pixel format / bit depth, colour metadata (primaries, transfer, matrix, range) and HDR
   metadata where present. Quality targets visually lossless: no visible degradation at normal
   viewing.
7. **Same length.** Duration matches the original to within one frame, and audio stays in sync.

If any rule cannot be met for a file (for example, its codec has no encoder available), the
operation is refused before it starts. It never produces a file that "mostly" matches.

---

## User stories

**Owner**
1. As the owner, I want to flip a mirrored video from its details page so that it plays the right
   way round without my leaving Holodex for a separate tool.
2. As the owner, I want every tag in the file to survive the flip so that the file's metadata, and
   anything else that reads it, is unaffected.
3. As the owner, I want the original kept for a while so that I can undo a flip I got wrong, for
   example on the wrong part or a video that was never mirrored.
4. As the owner, I want to discard the held original as soon as I'm satisfied so that I get the disk
   space back without waiting out the window.
5. As the owner, I want to be told before starting that a flip can't work (read-only library, not
   enough space, unsupported codec) so that I'm not left waiting on a job that was always going to
   fail.
6. As the owner, I want to keep browsing while a flip runs, and see its result on the video when I
   come back, so that a long re-encode doesn't hold the page hostage.
7. As the owner, I want the thumbnail to show the flipped picture afterwards so that the library
   doesn't keep showing the mirrored frame.

**Visitor**
8. As a visitor, I never see maintenance controls, held-original notices or maintenance status. The
   flipped video simply plays.

---

## Requirements

### P0 — must have

**R1. Owner-only Maintenance list on Media Details.**
- [ ] The Maintenance section renders only for the owner with Admin mode on. Visitors and a signed-in
      owner with Admin mode off see nothing of it.
- [ ] The section lists operations, and v1 lists exactly one: Flip horizontally.
- [ ] The server rejects maintenance requests from non-owners. Hiding the button is not the control.

**R2. Per-file targeting.**
- [ ] On a single-file video, Flip targets that file.
- [ ] On a multi-part video, the owner chooses one part's file, and only that file changes.
- [ ] The confirm step names the targeted file by filename (and part number when there are parts).

**R3. Preflight refusals.** Before anything is queued, Holodex checks the operation can succeed,
refuses with a plain-language reason when it can't, and creates no job:
- [ ] Media library not writable (the common read-only mount). The message says the library must be
      mounted read-write.
- [ ] Not enough free space on the file's volume for the output plus the held original. The message
      gives the space needed and the space available.
- [ ] No encoder available for the file's video codec, or no video stream at all.
- [ ] The file is missing on disk, or the video is soft-deleted.

**R4. Confirm step.**
- [ ] States that the video stream will be re-encoded and that everything else is preserved.
- [ ] States the free space required.
- [ ] States the date the held original will be deleted automatically.

**R5. Asynchronous run with visible status.**
- [ ] Confirming queues the operation and returns at once. The owner can leave the page.
- [ ] The video shows the operation's state (queued, running, done, failed), and so does the video's
      row in activity / job history.
- [ ] A failed operation shows its reason, with **Retry** and **Dismiss**.
- [ ] One file write at a time per file. While a maintenance operation is queued or running on a
      file, a tag write for that same file waits rather than interleaving, and the reverse also holds.
      A second maintenance operation on a file that already has one queued or running is refused.

**R6. Output contract and verification gate.**
- [ ] The output satisfies every rule in "Output contract" above.
- [ ] Holodex verifies the output against the original before replacing it: streams, tags, chapters,
      picture shape and duration.
- [ ] On a verification failure the output is discarded, the original is left exactly as it was
      (bytes and modification time), and the failure reason names the rule that failed.
- [ ] A crash or restart mid-operation leaves the original in place. No partial output is left
      behind once Holodex is running again.

**R7. Library continuity.**
- [ ] After a successful flip the video is the same library item: same id and URL, and the same
      people, tags, per-field edits, source decisions, enrichment and history.
- [ ] File facts (duration, resolution, size) are refreshed from the new file without a manual
      rescan.
- [ ] The next library scan treats the file as changed in place, never as a new video.
- [ ] The thumbnail is regenerated from the flipped file. An owner-uploaded poster is left alone.

**R8. Held original.**
- [ ] On success, the original file is held for the retention window, which is configurable and
      defaults to 7 days.
- [ ] The held original never appears in the library as an item of its own, and scans never ingest it.
- [ ] The video page tells the owner an original is being held and when it expires, and offers
      **Restore original** and **Keep change**.
- [ ] **Keep change** deletes the held original immediately.
- [ ] **Restore original** puts the original back in place of the flipped file, refreshes file facts
      and the thumbnail, and deletes nothing else. Before restoring, the owner is warned that any tag
      writes to this file since the operation will be lost from the file. The library's own values
      are not lost.
- [ ] A periodic pass deletes held originals whose window has expired, and the pass is recorded in
      activity like other background passes. An original that can't be deleted (for example after a
      permission change) is retried and surfaced, never silently forgotten.
- [ ] **At most one held original per file.** If a second operation runs while an original is held,
      the held copy stays the one from *before the first operation*, so Restore always returns the
      file to how it was before maintenance began. The window does not restart.

### P1 — nice to have

- **Progress.** Show percentage complete for a running re-encode in the video's status.
- **Cancel.** Stop a queued or running operation. The original is untouched and no held copy is
  created.
- **Carry tag writes through a restore.** Restoring re-applies the tags written to the file since
  the operation, so the warning in R8 isn't needed.
- **Before/after frame.** Show the current and flipped frame side by side in the confirm step, as a
  last check before committing to a re-encode.
- **Held originals in the owner tooling hub.** List every held original, its video, its size and its
  expiry, so disk use is visible in one place.

### P2 — future (design for, don't build)

- **Aspect-ratio operations**: correct a wrong display aspect ratio (ideally without re-encoding,
  where the container allows), crop letterboxing, or pad. Each is a new entry in the Maintenance
  list. Each obeys the same output contract, with rule 6 relaxed only in the dimension the operation
  changes.
- **Vertical flip / rotate 90°/180°**: further entries under the same rules.
- **Chaining**: several operations applied in one re-encode, so two fixes cost one quality
  generation instead of two.

---

## Success measures

Holodex is a single-owner, self-hosted library, so success is correctness rather than adoption:

- **Contract fidelity: 100%.** Every completed operation passes the verification gate. Checked by
  the automated tests over representative MP4 and MKV fixtures, including multi-audio, subtitles,
  chapters, cover art and HDR.
- **Zero library drift.** No item loses or changes a person, tag, edit or decision because of a
  maintenance operation. Checked by tests comparing the item before and after.
- **Zero orphans.** No held original outlives its window by more than one expiry pass, and no partial
  output survives a restart.
- **Owner time.** The owner fixes a mirrored file entirely from the details page. Today that takes
  an external ffmpeg run, a re-tag and a rescan.

---

## Open questions

- **Q1 (owner, blocking R6).** Some containers carry *stream-statistics* tags (Matroska's `BPS`,
  `NUMBER_OF_BYTES`, `DURATION` on the video track) that describe the encoded bytes. Copied verbatim
  they would be false after a re-encode. Should they be **recomputed** (the only exception to "same
  tags", limited to the re-encoded video stream), or **copied verbatim** to honour "same tags"
  literally?
- **Q2 (engineering, non-blocking).** Is "visually lossless" a fixed quality setting per codec, or
  should a bitrate band relative to the original also be enforced, so a flip never doubles or halves
  file size? A decision for the architecture doc, not the spec.
- **Q3 (engineering, non-blocking).** Where held originals live (beside the file, or in a
  Holodex-managed area on the same volume) is an architecture decision. The spec requires only R8:
  held originals are never ingested, and a restore never crosses volumes.

---

## Timeline / phasing

- **Phase 1 (this spec, P0):** Flip horizontally, per file, held original, verification gate.
  Wanted "somewhat soon".
- **Phase 2:** P1 items as fast-follows, progress and cancel first, since a long re-encode is where
  their absence hurts most.
- **Phase 3:** aspect-ratio operations, each as its own small spec edit adding a Maintenance entry.

**Gates this feature will need:** architecture (`writeback.md`: a re-encode write class, held
originals, and serializing with tag writes), design handoff (the Maintenance section, the
held-original notice, the confirm step), testing strategy (fixture matrix for the output contract),
and security review (a new destructive write path into the media library).
