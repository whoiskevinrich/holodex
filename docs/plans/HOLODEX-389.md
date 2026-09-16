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
- [ ] design `design-handoff` — `media-parts-handoff.md` + committed SVG; owns the `VideoCard` slot (RD9) and pill wording (OQ3)
- [ ] backend — mapping example + loader rejection of `<provider>:part`, lifter generalised to both markers, `formatMap` rows, summary payload
- [ ] frontend — media header pill + "+ Set part" row, film list pill, `VideoCard` marker, queue-row marker, writeback dialog
- [ ] testing `testing-strategy` — lifter cases (RD4 incl. rejected bodies), MKV+MP4 round trip, triplet-enrich invariance, three-skin QA
- [~] security `security-review` — n/a: no auth/access/infra change; one more `formatMap` row in an existing perimeter

## Up next — ordered (position = priority)

1. [ ] [design] `/design-handoff` — card slot (poster-corner pill lean) + wording, grounded in `VideoCard`; commit SVG
2. [ ] [backend] OQ1: decide where `2 of 3` → `2` normalises (recommend extractor); OQ2: whether the
   title-sort tiebreak (P1) is cheap — if not, leave it open and say so
3. [ ] [backend] implement P0 per spec, in the edition-PR order (mapping → lifter → formatMap → payload)
4. [ ] [frontend] surfaces per RD9; QA all three skins at the 8-column tier
5. [ ] [jira] clear `needs-design` when the handoff lands; mark PR ready → In Review fires

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-16 · brainstorm + spec
- skills: product-brainstorming, write-spec
- handoff: Draft PR open on `HOLODEX-389-media-parts` with the spec only; owner rulings locked
  (ordinal-only, strict `{part-N}`, `disk` atom for MP4, manual set via generic chip with no
  validation, no grouping). Next is the design handoff — no code yet.
