# Spec: Moving thumbnails — a hover montage on grid cards (F83)

**Status**: Draft
**Story**: HOLODEX-578. It supersedes HOLODEX-27 (media previews), whose "short hover preview clips
in the grid" this covers.
**Owner**: Project owner
**Date**: 2026-10-25

**Builds on**: the still thumbnail ([`phase-2-mcp-polish.md`](phase-2-mcp-polish.md), F11) and the
two image tiers ([`video-poster-multi-size.md`](video-poster-multi-size.md), F53). A montage is a
third derivative of the same video. It never replaces the still, the poster or an uploaded poster.
**Pipeline**: [`media-ingest.md`, "Tiered thumbnails"](../architecture/media-ingest.md) owns how
derived media is made, stored and served.

**Design handoff**: pending (`/design-handoff`). **Test plan**: pending (`/testing-strategy`).

---

## Problem Statement

A grid card shows one frame. For most videos that frame is an arbitrary moment chosen by a seek
percentage, so it says little about what the video is. To learn more, the owner or a visitor has
to open the video and scrub. Across a page of 40 cards, that is the slow way to browse.

Video sites solve this with a "moving thumbnail": while the pointer rests on a card, it plays a
few seconds sampled from across the video. Holodex already makes a still for every video in the
background, so it can make a montage the same way.

## Goals

1. **Resting on a card previews the video.** Within about half a second of the pointer settling
   on a card (or the card taking keyboard focus), the card plays a short silent montage drawn
   from across the whole video, then returns to its still when the pointer or focus leaves.
2. **Nothing moves unasked.** At rest every card shows its still. Touch devices and viewers who
   ask for reduced motion always see stills.
3. **Browsing never gets worse.** A card whose montage isn't ready behaves exactly as it does
   today. Montages never delay a still, a page load or a scan.
4. **The owner controls the cost.** Montage generation can be turned off, and its share of the
   machine is set separately from stills.

## Non-Goals

- **Scrubbing by pointer position** (the frame follows the pointer across the card). That was
  part of HOLODEX-27's wording, and it is dropped with it. *(Why: a different interaction with its
  own asset. Raise it as its own story if it is wanted.)*
- **Autoplay when a card scrolls into view.** *(Why: decided RD1. A full grid of moving cards is
  noisy and costs bandwidth on every page.)*
- **Sound.** Montages have no audio track. *(Why: a preview should never make noise, and silent
  files are smaller.)*
- **Montages in the Table view or anywhere a thumbnail is smaller than a grid card.** *(Why: too
  small to read as motion.)*
- **Montages for people, studios, films or tags.** *(Why: those images are not videos.)*
- **Choosing which moments the montage uses.** Sampling is automatic. *(Why: per-video curation
  is a larger feature with no ask behind it yet.)*
- **Removing a montage when its video is purged.** Stills are not removed on purge today either.
  The montage follows the still's rule, and changing that rule is separate work.

## Resolved Decisions

*(Locked with the owner on 2026-10-25 using question cards.)*

- **RD1: plays on hover or keyboard focus only.** There is no autoplay, and on touch the still
  stays.
- **RD2: the montage is a short silent video file**, not an animated image. It is several times
  smaller for the same clip, and every browser decodes it in hardware. The file choice belongs to
  the pipeline doc.
- **RD3: montages are made in the background, after the still.** They run at lower priority than
  stills and have their own on/off switch and worker count. A card without a montage keeps its
  still. Browsing a page moves its videos up the montage queue, as browsing already does for
  stills.
- **RD4: HOLODEX-27 is closed as superseded by this spec.**

## User Stories

- As the owner, I want to rest the pointer on a card and see a few seconds from across the video,
  so I can tell videos apart without opening each one.
- As a visitor, I want the same preview, so browsing someone's library is just as quick for me.
- As a keyboard user, I want the focused card to play its montage, so I get what a pointer user
  gets.
- As someone who has asked the system for reduced motion, I want cards to stay still.
- As the owner, I want to turn montages off, or limit how much of the machine they use, so a large
  library's backfill doesn't compete with everything else.
- As the owner, I want to see montage generation progress where I see still-thumbnail progress, so
  I know a backfill is running and when it will finish.

## Requirements

### Must-have (P0)

- **P0-1: What a montage is.** It is a silent loop of **6 one-second clips**, taken at evenly
  spaced points between **10% and 90%** of the video's duration. That skips most intros and
  credits. Clips play in time order, and the loop restarts at the first clip. Each frame is the
  card's width or smaller (never larger than the still), with the video's own aspect ratio.
- **P0-2: Short videos.** A video under **30 seconds** gets proportionally fewer clips, with at
  least 2. A video under **6 seconds**, or one with no known duration, gets no montage and keeps
  its still.
- **P0-3: When it plays.** On a device with a fine pointer, a grid card starts its montage once
  the pointer has rested on it for **400 ms**. It starts at once when the card takes keyboard
  focus. Leaving the card, or the card losing focus, stops playback and shows the still again.
  The next play starts at the first clip.
- **P0-4: One at a time.** Only one card plays at a time. Starting one stops any other.
- **P0-5: Never moves unasked.** Cards are still at rest. On touch-only devices, and whenever the
  viewer's system asks for reduced motion, cards never play.
- **P0-6: No blank flash.** The still stays on screen until the montage can play its first frame.
  If the montage fails to load, the card keeps its still with no error, spinner or retry loop.
- **P0-7: Missing montage.** A card whose video has no montage yet behaves exactly as it does
  today. Hovering it does nothing new.
- **P0-8: Where it applies.** Every video card in a grid or a shelf (the library grid, Home's
  shelves, related videos, a film's videos). Cards still link, open, and show their badges and
  hover tint exactly as before. The montage plays in the card's image frame only. In the poster
  card layout, the montage fills the frame the same way the still does (the design handoff sets
  the crop).
- **P0-9: Who sees it.** A montage is visible to everyone who can see the video's still, owner
  and visitor alike. A soft-deleted video's montage is hidden exactly as its still is.
- **P0-10: Background generation.** A montage is made only for a video that already has a still.
  It runs after all waiting stills, at the lowest machine priority, and never blocks a scan. On
  startup, videos with no montage are queued (eager backfill), unless montages are turned off.
- **P0-11: Owner switches.** The owner can turn montage generation off, and can set how many
  montages are made at once, separately from stills. Turning it off stops new work. Existing
  montages are still played, so turning it back on doesn't start from zero. The settings follow
  the same config rules as the still-thumbnail settings.
- **P0-12: Staying current.** When a video's file changes (file maintenance, or a writeback that
  rewrites the file), its montage is remade, as its still is. An owner uploading a poster does not
  touch the montage, because the montage comes from the video, not the poster.
- **P0-13: Failure.** A video whose montage fails to generate keeps its still. Failed montages are
  retried on the next startup, as failed stills are, and never block other videos in the queue.
- **P0-14: Progress.** Montage queue progress (waiting, in progress, workers) appears in the
  activity view next to still-thumbnail progress.

### Should-have (P1)

- **P1-1: Regenerate includes the montage.** The existing per-video "regenerate thumbnail" action
  also remakes the montage.
- **P1-2: Configuration reference.** The new settings are documented with the thumbnail settings
  in the configuration reference.

### Future considerations (P2)

- **P2-1: Pointer scrubbing** (see Non-Goals). The montage's clips could double as scrub frames.
- **P2-2: Tuning the sampling.** Clip count, clip length and the sampled span could become owner
  settings if the defaults prove wrong for some libraries.
- **P2-3: A touch interaction.** For example, the card nearest the centre plays while scrolling
  stops. RD1 rules it out for now.

## Success Metrics

This is a single-owner library, so these are QA checks, not analytics.

- **Leading:** after backfill finishes, at least 95% of videos 6 seconds or longer have a montage.
  The rest are listed as failed in the activity view, not silently missing.
- **Leading:** resting the pointer on a card with a ready montage starts motion within about
  0.5 s on a local network. A page of 40 cards loads no montage data until a card is hovered.
- **Leading:** a typical montage is under **300 KB**. Total montage storage is reported on QA
  against the stress fixture, so the owner can judge the disk cost before backfill runs on the
  real library.
- **Lagging:** backfill on the owner's library finishes without stills or scans slowing in a way
  the owner notices.

## Open Questions

- **Q1 (design, non-blocking):** how a 16:9 montage sits in the 2:3 poster card layout (crop to
  fill, or letterbox). This is settled in the design handoff.
- **Q2 (engineering, non-blocking):** do the defaults (6 × 1 s, 10%–90%, 400 ms) feel right on real
  videos? Tune them during QA. Changing them is a spec edit.
- **Q3 (engineering, non-blocking):** how much disk the montages take across a large library.
  Measure this on the stress fixture before backfill runs on the owner's library (see Success
  Metrics).

## Timeline / routing

No deadline. Low priority (HOLODEX-578).

- **Architecture:** expected. Picking a short video file over an animated image or a sprite
  sheet, and adding a separate lower-priority queue, are technology forks. They are recorded by
  editing the "Tiered thumbnails" section of `media-ingest.md`, not in this spec.
- **Design:** required. The hover behaviour, the still-to-motion handover and the poster-layout
  crop are visible surface (`/design-handoff`).
- **Testing strategy:** required (`/testing-strategy`).
- **Security review:** required. This adds a new publicly served derived file and new background
  ffmpeg work that browsing can trigger.
