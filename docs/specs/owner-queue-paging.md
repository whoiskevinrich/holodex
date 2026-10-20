# Spec: Owner queues load a page at a time (F81)

**Status**: Draft
**Story**: HOLODEX-587, a subtask of HOLODEX-354. Its sibling [`entity-list-paging.md`](entity-list-paging.md)
(F80) pages the entity lists and leaves the owner queues to this spec.
**Owner**: Project owner
**Date**: 2026-10-18

**Design handoff**: none. The only new control is the Load more button the browse grid already uses,
and every queue keeps its layout, grouping and order.

---

## Problem Statement

The stress fixture (2000 of every entity) showed the owner queues are unbounded end to end. Opening
Enrichment, Completeness, Duplicates, Extraction or Trash reads and renders every row in the
library at once: 8,000 enrichment rows, or 2,000 rows under a single missing facet. The page stalls,
and every row action re-renders the whole list.

## Goals

- Every owner queue loads at most one page (50 items) per read, server and client alike.
- Load more fetches the next page. Nothing is skipped and nothing is shown twice, even after the
  owner resolves, restores or deletes rows in between.
- Headings and "left" counts keep counting the whole queue, not the loaded rows.

## Non-Goals

- **Virtualisation.** HOLODEX-354 chose server paging plus Load more for the entity lists, and the
  queues follow suit. *(Why: one paging idiom across the app.)*
- **Making the queue's own computation cheaper.** The completeness queue still scores the whole
  library on each read; only what crosses the wire is bounded. *(Why: separate work, see Open
  Questions.)*
- **Changing any queue's order, grouping or copy.**

## Requirements

### P0: Must-have

- **P0-1. One page per read.** A queue read returns at most the requested page, 50 by default and
  never more than 1,000. It reports the total across every page and the page size and offset it
  actually used, which is the clamped value, not the requested one. The browse grid follows the same
  rule. Owner gating is unchanged: a visitor still can't read any queue.
- **P0-2. Load more.** Each queue shows `Load more (N left)` under its loaded rows while more remain,
  styled like the browse grid's. A failed Load more keeps the loaded rows and says so in place.
- **P0-3. Actions never skip a row.** When the owner resolves, restores, deletes, merges or dismisses
  a row, the next Load more starts where the queue now stands. No row is skipped. If the queue shifted
  under the owner, a repeated row is dropped rather than shown twice.
- **P0-4. Counts are whole-queue counts.** Group headings (`People · 2064`, `Missing Photo · 2027`,
  `Videos · 3`) and the Extraction commit bar's "rows left" count the whole queue. They fall as the
  owner works through it.
- **P0-5. Each queue pages by its own unit.**
  - **Trash, Enrichment, video duplicates:** a row at a time, in their existing order.
  - **Name duplicates:** a pair at a time, a pair never split, across the Tags → Studios → People →
    Films groups as one stream. The Load more sits after the last group, ahead of Videos. A
    deep-linked type filter (`?type=person`) pages that type's pairs only. The list-page "N possible
    duplicates" banners still count the whole type.
  - **Extraction:** a video at a time. A video's pending fields always arrive together, and "N videos
    left" counts videos.
  - **Completeness:** each facet group pages on its own. Every group arrives with its first page and
    its own Load more. Groups keep the order they first loaded in. The order is size-based, so
    every resolve would otherwise reshuffle it, which would make a single cross-group offset skip rows.
- **P0-6. Enrichment rows still resolve in place.** A row handled in place stays on screen as before.
  It no longer counts as waiting, so it is left out of the heading count and of where the next page
  starts.
- **P0-7. Refresh keeps what's loaded.** Extraction's Refresh and its post-Extract-all polling
  re-read every loaded video, not just the first page. Duplicates re-reads its loaded pairs after a
  merge or keep-one, because those can end other pairs too.

### P1: Nice-to-have

- None.

## Success Metrics

- With the 2000-entity stress fixture, no owner queue read returns more than 50 items by default.
- On Trash, restoring rows from the first page and then pressing Load more shows every remaining
  item exactly once. Verified 2026-10-18: 60 items, two restored, 58 shown, no gaps or repeats.

## Open Questions

- **Completeness read cost.** Each completeness read, including a group's Load more, still scores
  the whole library (about 1.8 s on the fixture). Caching the scored queue would make Load more
  cheap. Track it separately if the owner feels it.

## Resolved Decisions

- **RD1. Per-group paging for Completeness, not one stream.** A single offset across groups breaks
  whenever a resolve changes which group is larger. That happens on nearly every resolve when two
  facets are missing on the same entities.
- **RD2. Extraction keeps most-pending-first.** A video that loses a field can sort behind a video
  not yet loaded. The next page therefore reaches back by the number of such videos and drops the
  repeats. Re-ordering the queue by title would be simpler, but it would change the order the
  design handoff chose.
