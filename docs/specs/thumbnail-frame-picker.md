# Spec: Thumbnail frame picker (F84)

**Status**: Draft
**Story**: HOLODEX-35
**Owner**: Project owner
**Date**: 2026-10-26

**Builds on**: the still thumbnail ([`phase-2-mcp-polish.md`](phase-2-mcp-polish.md), F11) and the
uploaded poster tier ([`video-poster-multi-size.md`](video-poster-multi-size.md), F52/F53). A picked
frame is stored as an uploaded poster.

**Design handoff**: [`thumbnail-frame-picker-handoff.md`](../design/thumbnail-frame-picker-handoff.md),
with a committed SVG mockup. **Test plan**:
[testing-strategy §36](../testing-strategy.md#36-thumbnail-frame-picker-f84-holodex-35).

---

## Problem statement

A video's automatic thumbnail is the frame at a fixed fraction of its length, or the cover art
embedded in the file. That moment is often a fade, a title card or a blur. "Regenerate thumbnail"
only re-runs the same choice. To get a better image, the owner has to export a frame in another
tool and upload it as a poster.

## Goals

1. **The owner picks the exact frame.** Clicking Regenerate thumbnail opens a picker in which the
   owner scrubs through the video, compares the frame with the current thumbnail, and keeps it with
   one click.
2. **What you see is what you get.** The saved image is the frame on screen, pixel for pixel, at the
   video's own resolution. It is never a neighbouring frame.
3. **The picker is silent.** No sound plays while it is open, whatever the owner does in it.

## Non-goals

- Cropping or editing the chosen frame.
- Picking frames for files this browser can't play. The picker needs the browser to decode the
  video. The automatic thumbnail and poster upload still cover those files.
- Remembering the chosen time, so that a later automatic regenerate lands on it again.

## Requirements

### P0

- **P0-1 Opening.** For the owner only, Regenerate thumbnail opens the picker instead of
  regenerating straight away. Opening the picker pauses the page's player.
- **P0-2 Comparison.** The picker shows the video at the scrub position, a progress bar, the
  position as a time plus a frame number, the video's length, and the current thumbnail beside it.
  It starts at the page player's position, or at the start when the player hasn't moved.
- **P0-3 Drag to scrub.** Pressing on the progress bar jumps to that point. Dragging along it
  follows the pointer, including when the pointer leaves the bar.
- **P0-4 One frame per key.** The left and right arrow keys step back or forward exactly one frame.
- **P0-5 One frame per wheel notch.** Each notch of a mouse wheel over the video or the bar steps
  one frame (down or toward the owner is forward). A trackpad steps one frame per short swipe
  distance, not one per event. The page doesn't scroll while the wheel steps.
- **P0-6 Frame length.** One frame is the video's real frame interval, measured when the picker
  opens. Where the browser can't measure it, a frame is 1/30 s.
- **P0-7 Silent.** The picker never plays the video and never makes a sound, whatever the owner
  presses. Scrubbing shows still frames only.
- **P0-8 Use this frame.** Saves the frame on screen as the video's thumbnail and poster, then
  closes the picker. The grid card, the page's poster and the current-thumbnail preview show the new
  image. The frame is kept at the video's native resolution, capped at the same maximum as an
  uploaded poster.
- **P0-9 Stored as an uploaded poster.** A picked frame is an owner-supplied image. Like an uploaded
  poster, a rescan or the automatic sweep never replaces it, and Remove poster returns the video to
  its automatic thumbnail.
- **P0-10 Cancel.** Cancel, Escape and a click on the backdrop close the picker and change nothing.
  While a save is in flight, none of them close it.
- **P0-11 Reset to automatic.** A quiet action in the picker runs the previous Regenerate behaviour
  (embedded cover art, else a frame at the automatic position) and closes the picker.
- **P0-12 Failure.** If the save fails, the picker stays open with the reason, and the frame is
  still selected so the owner can retry. If the video can't be decoded, the picker says so and
  offers only Reset to automatic and Cancel.

### P1

- **P1-1 Keyboard parity.** The progress bar is a focusable slider that announces the position. Home
  and End jump to the first and last frame.

## Rationale: capture in the browser, not by timestamp

Earlier projects asked ffmpeg for "the frame at time t" and kept landing a frame or more off.
Container start offsets, edit lists and rounding at frame boundaries mean the browser and ffmpeg
disagree about which frame a timestamp names, and by a different amount per file. Capturing the
decoded frame the owner is looking at removes the question entirely (Goal 2), and the cost is a
browser-side JPEG encode of a frame at its native resolution.
