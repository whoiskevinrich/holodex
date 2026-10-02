---
key: HOLODEX-505
status: in-review
profile: backend             # behavior with no UI surface — an MKV tag read-back bug fix
depends-on: []
release_note: Writing a title and tags to an MKV file at the same time no longer makes some of the video's tags disappear from Holodex. Before, Holodex couldn't find the tags it had just written, and unlinked every tag that came from the file.
---

# HOLODEX-505 · MKV writeback intermittently drops tags that the UI showed before the write

This is done when every exiftool reader of a Matroska file sees tags that mkvpropedit put out of
exiftool's default reach. On an ffmpeg-muxed, untitled MKV, a write of Title plus a larger tag set
puts them there. That covers the scanner's re-read, the pre-write snapshot / Genre-delete check
(`ReadCurrentValues`) and the tag-key filter (`ReadTagKeys`).

**Diagnosis (media #3545, 2026-10-01):** the 22:36:15 write put all 8 tags on the file with the
title. The re-read without `-ee` saw no Genre, so `replaceAssociations` deleted the two `file`-sourced
links (tag ids 20, 33). The six `manual` links survived. The dialog then re-wrote the six at 22:36:59.
HOLODEX-506 (`-ee` in the extractor, merged ~23:00) already fixed the re-read. This branch extends
`-ee` to the two writeback readers. Spun out: HOLODEX-510 (the 22:36:59 write skipped its snapshot,
because a reused job id collides with an old snapshot's batch id).

**Design package:** bug fix, no spec or ADR · [testing-strategy row](../testing-strategy.md) (HOLODEX-505)

## Gates — definition of done

- [~] spec `write-spec` → `docs/specs/**`. Skipped deliberately: a bug fix restoring documented tag writeback behavior (ADR-110/111). No requirement changes.
- [x] backend → `{cmd,internal,providers}/**`. `ReadCurrentValues` and `ReadTagKeys` prepend `metadata.MatroskaSeekArgs`.
- [x] testing `testing-strategy`. `TestWriteMKV_TitleAndTagsReadBackEverywhere` was red on both writeback readers without the fix in the production image and is green with it. The full writeback integration suite passes in-image. A row is in `docs/testing-strategy.md`.

## Up next — ordered (position = priority)

1. [ ] [—] After merge reaches `:edge`, re-attach the two lost tags (ids 20, 33) to media #3545 and write tags once. Then look for other MKVs whose title and tags were written together before HOLODEX-506 — prod

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-01 · root-caused the dropped MKV tags; extended -ee to the writeback readers
- skills: code-review, handoff
- handoff: Fix and regression test are committed, and every gate is settled. Once it merges and reaches `:edge`, re-attach tags 20 and 33 to media #3545 and sweep other affected MKVs. HOLODEX-510 (snapshot batch-id collision) is filed separately.

## Dropped — newest first (the reason is the point)
