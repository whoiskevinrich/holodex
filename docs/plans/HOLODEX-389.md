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
- [ ] backend — mapping example + loader rejection of `<provider>:part`, lifter generalised to both markers, `formatMap` rows, summary payload
- [ ] frontend — media header pill + "+ Set part" row, film list pill, `VideoCard` marker, queue-row marker, writeback dialog
- [ ] testing `testing-strategy` — strategy row landed in [docs/testing-strategy.md](../../docs/testing-strategy.md) (2026-09-16, target coverage); flips when the named tests exist: lifter cases (RD4 incl. rejected bodies), loader rejection (*new*), MKV+MP4 round trip incl. `PART_NUMBER` replace, list-path `part` incl. container-tag-only, triplet-enrich invariance, three-skin QA
- [~] security `security-review` — n/a: no auth/access/infra change; one more `formatMap` row in an existing perimeter

## Up next — ordered (position = priority)

1. [ ] [backend] OQ1: decide where `2 of 3` → `2` normalises (recommend extractor); OQ2: whether the
   title-sort tiebreak (P1) is cheap — if not, leave it open and say so
2. [ ] [backend] **payload gap** (found by testing-strategy): `applyBrowseTitles` passes `extra=nil`
   and walks `Browse` fields only, so a container-tag-only `part` (`PART_NUMBER`, MP4 `disk`) is invisible on
   cards/queue rows. Put `part` on `model.Video`, fill it with one batch pass that loads just
   `part`'s `file:` keys (no migration; a materialised column would reopen the ADR gate and is
   OQ2's lever, not this one). `FilmVideo.Video` then carries it to scene cards too — stamp full-film
   **and** scene rows, unlike edition.
3. [ ] [backend] implement P0 per spec, in the edition-PR order (mapping → lifter → formatMap → payload);
   the loader's `<provider>:part` rejection is new behaviour (`parseSources` validates nothing today)
   and wants a registry-level file-only fact on the `FieldDef`
4. [ ] [frontend] surfaces per RD9; QA all three skins at the 8-column tier
5. [ ] [frontend] `partBadgeLabel` helper + `video/CLAUDE.md` table row; queue-row payload needs `part` (handoff §4 backend note)
6. [ ] [fixture] stress seeder `part` dimension (source × value rungs + same-title triplet; the
   container-tag rung is gap-shaped until #2 lands) + handoff §3 rects as geometry-harness
   assertions; demo generator items `{part-N}.mp4` + `-metadata disk=2` for the real-scan path
7. [ ] [jira] mark PR ready → In Review fires

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-16 · brainstorm + spec + design handoff
- skills: product-brainstorming, write-spec, design-handoff
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
