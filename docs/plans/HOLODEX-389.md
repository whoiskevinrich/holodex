---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-389
status: in-progress
release_note: Files that are parts of one media can now carry a part number — `{part-2}` in the filename or a container tag — and show it on cards, the media page, film lists and queue rows, so three files enriched with the same provider entry stay tellable apart.
---

# HOLODEX-389 · Media parts — denote the files of a multi-file media (`part`)

A concert or compilation stored as three files gets the same (correct) provider enrichment on
each, and once the provider title/poster win, nothing on the card tells the parts apart. Fix the
survivor, not the enrichment: a new ordinal-only canonical field `part`, on the F60 edition
template — `PartNumber`/`DiskNumber` container tag + strict `{part-N}` filename marker, no
provider source ever, writeback row per container, pill beside the title on every surface that
shows the triplet (incl. `VideoCard`, which edition never earned). No grouping, no total, no
enrichment change — all ruled out on purpose ([spec](../specs/media-parts.md) Non-Goals).

## Gates — definition of done

- [x] spec `write-spec` — [docs/specs/media-parts.md](../specs/media-parts.md) (RD1–RD10; OQ1/OQ2 engineering, OQ3 design)
- [~] architecture `architecture` — n/a: a canonical field is mapping config, as edition was (ADR-096 D4)
- [x] design `design-handoff` — [docs/design/media-parts-handoff.md](../design/media-parts-handoff.md) + [SVG](../design/media-parts-mockup.svg); card slot = bottom-left duration-style, "Part N" everywhere, no film-page "+ Set part"
- [x] backend — mapping example + loader rejection of `<provider>:part`, lifter generalised to both markers, `formatMap` rows, summary payload (`part` on `model.Video`, batch pass on every list surface + both queues)
- [ ] frontend — media header pill + "+ Set part" row, film list pill, `VideoCard` marker, queue-row marker, writeback dialog
- [ ] testing `testing-strategy` — strategy row landed in [docs/testing-strategy.md](../../docs/testing-strategy.md) (2026-09-16, target coverage); flips when the named tests exist: lifter cases (RD4 incl. rejected bodies), loader rejection (*new*), MKV+MP4 round trip incl. `PART_NUMBER` replace, list-path `part` incl. container-tag-only, triplet-enrich invariance, three-skin QA
- [~] security `security-review` — n/a: no auth/access/infra change; one more `formatMap` row in an existing perimeter

## Up next — ordered (position = priority)

1. [ ] [backend] ~~OQ1~~ resolved (extractor, `ordinalKeys`); OQ2: whether the title-sort tiebreak
   (P1) is cheap — decide at the payload step (#2), which is the same lever
2. [x] [backend] **payload gap** — DONE: `model.Video.Part` + `partsFor`/`applyParts`
   (`internal/api/parts.go`), key-scoped batch loads, wired into every list surface, both queues and
   the detail `video` object; scene **and** full-film rows stamped. OQ2 ruled: P1 stays open (spec).
3. [ ] [backend] implement P0 per spec, in the edition-PR order — **mapping + lifter DONE**
   (registry `part` + `FieldDef.FileOnly`, loader rejects `<provider>:part`, example mapping, marker
   table lifts both markers, `part` TierHigh); **extractor DONE** (`ordinalKeys` normalisation +
   `PartNumber-und` pin); **formatMap DONE** (`PART_NUMBER` / `QuickTime:DiskNumber`, `readKey` folds
   `_`, replace-not-append pinned, round trip green on real MKV+MP4); **payload DONE** (#2) — backend
   gate complete
4. [ ] [frontend] surfaces per RD9 — **`VideoCard` DONE** (3 skins × 8/16 cols measured live);
   remaining: media header pill + "+ Set part" row, film full-film pill, `EnrichQueueRow` +
   extraction-queue row pills, `WritebackFormDialog` check
5. [x] [frontend] `partBadgeLabel` helper + Vitest + `video/CLAUDE.md` rows; `types.ts` `part?` on
   `Video` / `EnrichQueueRow` / `ExtractionQueueRow`
6. [ ] [fixture] stress seeder `part` dimension (source × value rungs + same-title triplet; the
   container-tag rung is gap-shaped until #2 lands) + handoff §3 rects as geometry-harness
   assertions; demo generator items `{part-N}.mp4` + `-metadata disk=2` for the real-scan path
7. [ ] [jira] mark PR ready → In Review fires

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-16 · brainstorm + spec + design handoff
- skills: product-brainstorming, write-spec, design-handoff, code-review
- handoff: Draft PR #340 on `HOLODEX-389-media-parts` carries spec + handoff + SVG; every
  design ruling is locked (ordinal-only, strict `{part-N}`, `disk` atom for MP4, manual set via
  generic chip with no validation, no grouping, bottom-left duration-style card badge, "Part N"
  everywhere, no film-page "+ Set part"). Gates left: backend, frontend, testing. No code yet —
  start at the mapping example + lifter, in the edition-PR order.

### 2026-09-16 · testing strategy
- skills: testing-strategy
- handoff: strategy row added beside the edition row (§4 table, line ~222) — every test is the
  edition test with the field swapped except three *new* ones (loader rejection, `PART_NUMBER`
  replace-not-append, list-path container-tag-only `part`). The list-path finding is a backend design input
  (Up next #2), not a test-only note. Handoff §2.2–2.4 are agent QA, not smoke — `web/` has no
  component harness. No code yet.

### 2026-09-16 · backend: mapping + lifter
- skills: code-review (high --fix, clean)
- handoff: first backend commit — registry `part` (optional, `FileOnly`), loader rejection with
  `TestLoadRejectsProviderSourceOnFileOnlyField` + `TestExampleMappingNoSharedFileSource`, example
  mapping (`PartNumber`, `DiskNumber`, `filename:part`; episode loses `PartNumber`), `liftEdition` →
  `liftMarkers` table with `TestMatchFirst_PartMarker` (13 cases; note a bad `{part-…}` beside a good
  `{edition-…}` still emits edition alone — RD7's "marker is a match on its own" wins). Next:
  extractor normalisation (OQ1) → formatMap → payload batch load.

### 2026-09-16 · backend: extractor
- skills: code-review (high --fix, clean)
- handoff: OQ1 resolved in the extractor — `ordinalKeys` (`PartNumber`, `DiskNumber`) store the
  leading integer, zeros dropped, non-numeric untouched, other keys verbatim
  (`TestMapExiftoolOrdinalKeys`); `PartNumber-und` pinned in the lang-suffix test. The ADR-067
  snapshot keeps the raw value on purpose (revert fidelity). Next: formatMap + round trip.

### 2026-09-16 · backend: formatMap
- skills: code-review (high --fix, clean)
- handoff: `part` → `PART_NUMBER` (Matroska/WebM) and `QuickTime:DiskNumber` (MP4); `readKey`
  now folds underscores so `TestExampleMappingCoversWriteTargets` sees `PartNumber` as the
  read-back of `PART_NUMBER` (exiftool CamelCases SimpleTag names); `TestMergeTagsXML_ReplacesPartNumber`
  pins one `PART_NUMBER` after a part write (a foreign target-30 copy is dropped — two would make
  `in_sync` order-dependent); `TestPartRoundTrip_BothContainers` (`-tags integration`) passes
  locally on real ffmpeg+exiftool (MKV via the ffmpeg fallback — no mkvpropedit here; CI has it).
  Next: payload — `part` on `model.Video` + batch load of its `file:` keys on the list path (#2).

### 2026-09-16 · backend: payload (gate closed)
- skills: code-review (high --fix — one finding, fixed: detail `video` object also carries `part`)
- handoff: `model.Video.Part` stamped by `partsFor`/`applyParts` (`internal/api/parts.go`) — one
  batch pass through the real resolver; `ExtraMetadataForVideosByKey` + `EnrichmentForVideosField`
  are key-scoped variants of loaders that already existed (the strategy row's "no batch loader"
  was wrong — F55 had one; the list path just never called it). Wired: browse, completeness sort,
  search, related, person/tag/studio, film scenes **and** full films, film candidates, enrich
  queue (video rows), extraction queue, media detail. Six API tests over a triplet fixture
  (tag / filename / decision / none). OQ2 ruled in the spec: P1 tiebreak stays open. Not touched:
  `videoEdition` is still N+1 per full-film file (pre-existing). Next: frontend (RD9 surfaces,
  `partBadgeLabel`, `types.ts` `part?` on Video/EnrichQueueRow/ExtractionQueueRow).

### 2026-09-16 · frontend: types + helper + VideoCard
- skills: code-review (high --fix, clean)
- handoff: `types.ts` `part?`, `partBadge.ts` + test, `VideoCard` bottom-left badge per handoff §1a.
  **Found in QA:** the Brutalist reel counter (`::before`, `left .45rem bottom .35rem`) sits in the
  badge's corner — fixed in `app.css` with `:has(> .part-badge)` stepping the counter to
  `bottom: 2.1rem` (first `:has()` in the file; old Firefox degrades to counter-under-badge).
  Measured live on all three skins at 8 and 16 columns: badge/duration never intersect (≥76px
  gap at 180px cards), 7px counter clearance, style identical to duration. Throwaway testbed:
  gitignored `backend-parts` launch entry → `%TEMP%/parts-media` (3× `{part-N}.mp4`, one
  `disk=2` MP4, one plain) + `%TEMP%/parts-data`, example mapping, auto-apply on — the
  container-tag-only file resolves `part=2` end to end. Next: media header + film row + queues.
