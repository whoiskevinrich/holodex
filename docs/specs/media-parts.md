# Spec: Media parts — denote the files of a multi-file media (`part`)

**Status**: Draft
**Phase**: Phase 3 (Enrichment / curation foundation) — one more file-layer field on the F60
edition template; no new subsystem
**Owner**: Project owner
**Date**: 2026-09-16
**Story**: [HOLODEX-389](https://whoiskevinrich.atlassian.net/browse/HOLODEX-389)

**Depends on** (all shipped):
- F60 edition ([entity-identity-card.md](entity-identity-card.md) RD6–RD8, RD11, RD12;
  [ADR-096](../architecture/ADR-096-entity-identity-card.md) D4) — the template this field copies
  line for line: a file fact with a container-tag baseline, a strict `{…}` filename marker, no
  provider source, an `optional` completeness facet, a `formatMap` row, a pill beside the title.
- F48 metadata extraction ([metadata-extraction.md](metadata-extraction.md)) — the `filename:`
  namespace, the marker lifter in `internal/extract/pattern.go` (`liftEdition`), and the
  auto-apply / review-queue routing every `filename:` candidate follows.
- Per-field source decisions ([field-source-of-truth.md](field-source-of-truth.md),
  [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md)) and the two-layer
  model ([ADR-090](../architecture/ADR-090-two-layer-entity-metadata-management.md)) — `part` is
  a precedence-layer field rendered through the generic field row + `SourceBadge`.
- Writeback ([ADR-041](../architecture/ADR-041-metadata-writeback.md),
  [ADR-093](../architecture/ADR-093-writeback-readback-and-tristate-in-sync.md)) — one more row
  in `formatMap`, round-tripped through read-back.
- Canonical field mapping ([ADR-013](../architecture/ADR-013-metadata-field-mapping.md)) —
  `part` is declared in `metadata-mappings.yaml`, not compiled in.

**ADR**: none — a canonical field is mapping config, exactly as edition was (ADR-096 D4 covered
the *pattern*; this spec instantiates it once more).
**Design**: [media-parts-handoff.md](../design/media-parts-handoff.md) +
[mockup](../design/media-parts-mockup.svg) — ratified 2026-09-16: card slot = bottom-left corner,
duration-style (RD9); wording = "Part N" everywhere via `partBadgeLabel` (OQ3); no "+ Set part"
link on the film page.

---

## Problem Statement

One canonical media — a concert, a compilation, a long recording — is sometimes stored as two or
three files. Enriching each file with the same provider candidate is *correct*: they are all the
same work. But once the provider's title, poster and overview win precedence, nothing the card
still shows distinguishes the files, and the owner sees an identical triplet on the browse grid,
in search results, in the enrichment queue and on the media page. The only fact that tells them
apart — *which slice this file is* — is not a field, so it dies the moment enrichment lands.
Today's workarounds are leaving the files un-enriched or hand-editing titles, both of which fight
the resolver model.

## Goals

1. A file's **part ordinal** is a first-class, resolved, file-layer field that survives
   enrichment, renames (via writeback) and rescans.
2. The owner can tell the parts of one media apart **everywhere the identical triplet appears**:
   browse/search cards, the media page header, the film page's full-film list, and the owner
   queue rows that name a video.
3. Declaring the part costs the owner one filename token (`{part-N}`) or one container tag — no
   UI step is required, and no step is *lost* if the owner prefers the UI.
4. Enrichment behaviour is **unchanged**. Each part is enriched on its own; the field is never
   provider-sourced.

## Non-Goals

- **Grouping the parts under any entity, key or card.** The media here is a concert or
  compilation, not a Film; there is nothing to group under and inventing a group identity
  (stem-minus-marker, sibling links) is the scope-creep vector. Films already have `film_videos`.
- **"Enrich once, propagate to siblings."** Needs the group identity above. Three applies is
  acceptable friction.
- **`part_total` / `{part-NofM}`.** Owner ruling 2026-09-16: the total is rarely used; its only
  payoff is a missing-part check, which is a completeness feature. Revisit if that ever wants it.
- **Plex stacking suffixes** (`- pt1`, `- cd1`, `- part1`, `- disc1`). Rejected for the same
  reason RD7 rejected loose edition forms: real titles contain "Part II" / "Part 1". May be added
  later as a second, opt-in `filename:` source.
- **Playlist / one-card sequential playback.** Needs grouping.
- **An adoption-time nudge** ("this candidate is already on another video — tag both as parts?").
  Possible later; rides on the field existing.
- **Any change to the `episode` mapping.** It stays commented out; it simply loses `PartNumber`
  as a listed source (RD3).

## Resolved Decisions

- **RD1 — `part` is an ordinal-only canonical field**: the positive integer position of this file
  within its media (`1`, `2`, `3`). A file fact, never a film or provider property. No total.
- **RD2 — Sources, in the edition shape**: file tag `PartNumber` (Matroska `PART_NUMBER`, which
  the extractor already surfaces into the file layer unmapped) and `DiskNumber` (MP4/MOV, RD5);
  candidate `filename:part` from the marker (RD4); curated custom value (RD8). **No provider
  source, ever** — a `tmdb:part` (or any `<provider>:part`) must be rejected by the mapping
  loader, not merely omitted from the example.
- **RD3 — `PartNumber` is claimed by `part`.** The dormant `episode` example mapping listed it as
  a source; it is removed from that list so the same container tag never feeds two fields.
- **RD4 — Filename grammar is strict**: `{part-N}` anywhere in the stem, `N` = one or more ASCII
  digits, leading zeros allowed and normalised (`{part-02}` → `2`). The marker is lifted out of
  the stem before pattern matching exactly as `{edition-…}` is, and is a recognised convention on
  its own (a file no pattern fits still yields its part). Both markers may appear on one file in
  either order. A `{part-…}` whose body is not digits (`{part-2of3}`, `{part-}`, `{part-two}`,
  `{part-0}`) is **not** a marker: it is left in the stem untouched so the file visibly fails to
  parse rather than silently dropping the total — the owner ruled "reject `NofM`, don't drop it".
- **RD5 — Writeback keys**: Matroska/WebM `PART_NUMBER` SimpleTag in the GENERAL block via
  mkvpropedit, read back by exiftool as `PartNumber` (the tag the extractor already sees).
  MP4/MOV: the iTunes **`disk`** atom, exiftool `DiskNumber` — owner ruling 2026-09-16 over an
  XMP tag: it is native and every tagger/player renders it. Holodex writes the bare ordinal;
  read-back of a Holodex-written file is the bare ordinal, so ADR-093 `in_sync` holds. A
  *foreign* file may carry `2 of 3`; the resolved value is the **leading integer** (see OQ1 for
  where that normalisation lives).
- **RD6 — Precedence and routing are inherited, not invented.** File tag vs filename marker vs
  custom value resolve through ADR-051 exactly as edition does; `filename:part` follows the F48
  auto-apply flag and confidence like every other `filename:` candidate. Nothing part-specific.
- **RD7 — `part` is an `optional` completeness facet**, like edition: most files have no part and
  that is correct, not a gap. Listed so the deep-linked empty row knows the field exists; no
  weight, never counts as missing, never enters the remediation queue.
- **RD8 — The owner can set `part` by hand in v1 through the generic mechanism only**: the
  `SourceBadge` custom chip on the media page's field row and, when no value exists, the same
  "+ Set part" empty curatable row edition uses (F60 RD11/RD12 found-in-build). **No
  validation** on the custom chip — owner ruling 2026-09-16: the strict grammar is a filename
  rule, and adding the first per-field validator to the generic chip for one field is not worth
  it. The pill renders whatever was typed. **Found in build 2026-09-16:** the empty curatable row
  is the F60 deep-link landing — it renders only on `#field-part` — and since `optional` facets
  never enter the completeness queue (RD7) and the film page gets no "+ Set part" link (RD9), a
  file with no part had no route to it. **Ruled (option A):** the media header renders an
  owner-only dashed "+ Set part" link to `#field-part` in the slot the pill occupies once set —
  the film page's "+ Set edition" idiom, on one page. Visitors see nothing.
- **RD6a — found in build 2026-09-16, F48 scoring:** `classifySpecificity` rated any non-entity
  value under three runes as *partial*, so a one- or two-digit ordinal scored 0.30 + 0.25 = 0.55
  and never cleared TierHigh — every `{part-N}` file queued a review row even though the value
  was applied. A bare integer is now *full* specificity (the length floor is for text fragments);
  no part-specific branch. The one other field with digit values, `scene_number` (TierLow, 0.40),
  now scores a lone marker 0.80 instead of 0.55 (applied either way) and a tag conflict 0.50
  instead of 0.25 — which auto-applies, as every other full-specificity TierLow conflict already
  does; it can only arise if an operator declares a container-tag source for `scene_number`, and
  the example declares none.
- **RD9 — Display: a pill, beside the title, on every surface that shows the triplet.** Media
  page: the same read-only pill slot edition uses beside the h1 (the Metadata row remains the
  curation mount). Film page full-film list: beside the edition pill. **Browse/search
  `VideoCard`: `part` earns a card slot that edition never had** — the grid is where the triplet
  appears. Ratified in the handoff: the **bottom-left poster corner** (the one free corner — top-left is
  the resolution bucket, top-right the scene badge, bottom-right the duration), in the duration
  badge's neutral treatment; label "Part N" on every surface via a shared `partBadgeLabel`. Owner queue rows that name a video by title (`/owner/enrichment`,
  the F48 review queue) show the same marker beside the title.
- **RD10 — Enrichment is untouched.** No dedupe, no propagation, no "already applied" check.

## User Stories

- As the owner, I want the three files of one concert to each show which part they are on the
  browse grid so that I can pick the one I mean without opening each.
- As the owner, I want to declare the part by naming the file `… {part-2}.mkv` so that a rescan
  is all it takes and I never touch the UI for it.
- As the owner, I want Holodex to write the part into the file so that renaming the file later
  (or another tool reading it) does not lose it.
- As the owner, I want to enrich each part with the same provider candidate and still tell them
  apart afterwards so that enrichment stops being the step that makes them indistinguishable.
- As the owner, I want a file whose part is wrong or missing to be fixable on its media page so
  that I am not forced to rename and rescan.
- As the owner working the enrichment queue, I want the queue rows for three parts to show which
  is which so that I do not apply a candidate to the wrong row or wonder whether the list is
  duplicated.
- As a visitor, I want a part pill to be visible when it exists so that I can pick the right
  file, and to see no affordance to change it.
- As the owner, I want a file that carries both `{edition-Extended}` and `{part-2}` to resolve
  both so that the two facts never compete.

## Requirements

### Must-have (P0)

- [ ] `metadata-mappings.yaml.example` declares `part` with sources `PartNumber`, `DiskNumber`,
  `filename:part`; label "Part"; the `episode` example no longer lists `PartNumber` (RD2/RD3).
  The mapping loader rejects a `<provider>:part` source with a startup error naming the field.
- [ ] Extractor surfaces `PartNumber` (MKV/WebM) and `DiskNumber` (MP4/MOV) into the file layer
  — expected to be a no-change for MKV (unclassified keys already land there, `-und` stripped;
  pin with a `PartNumber-und` case) and verified for MP4 on a generated sample.
- [ ] Filename lifter recognises `{part-N}` per RD4 and emits `filename:part` with the normalised
  ordinal. Pinned cases: `{part-2}`, `{part-02}` → `2`, marker-only stem, marker + edition marker
  in both orders, marker inside a `^…$` pattern still matching, and the four rejected bodies of
  RD4 left in the stem.
- [ ] `filename:part` follows F48 routing unchanged (auto-apply flag + confidence); no
  part-specific branch exists in `process.go` / `confidence.go`.
- [ ] Media page renders the Part row via the generic field row; `SourceBadge` offers the custom
  chip per F60 RD12; empty + owner → "+ Set part" empty curatable row (RD8); `part` is listed as
  an `optional` completeness facet (RD7).
- [ ] `formatMap` gains `part` per RD5 for Matroska/WebM (`PART_NUMBER`) and MP4/MOV (`disk`);
  `WritebackFormDialog` lists it; read-back reports `in_sync` after write + re-extract on
  generated MKV **and** MP4 samples; a foreign `PART_NUMBER` tag written by another tool survives
  the merge (the existing fixture in `writeback_test.go` already asserts this — keep it).
- [ ] Video summary payload carries resolved `part`.
- [ ] Media page shows the pill beside the h1 when a value exists, in the edition slot (RD9).
- [ ] Film page full-film rows render the part pill beside the edition pill (RD9).
- [ ] `VideoCard` renders the part marker per the design handoff (RD9); it is visible at every
  density tier, including the 8-column maximum, and does not change card height.
- [ ] Owner queue rows that name a video (`/owner/enrichment` `EnrichQueueRow`, the F48 review
  queue row) show the marker beside the title when the video has a resolved part.
- [ ] Given three files that share a resolved title and each carry a different part, when the
  owner applies the same provider candidate to all three, then all three keep their part pill and
  the provider fields resolve identically — enrichment writes nothing to `part`.
- [ ] Given a file with tag `PART_NUMBER=1` and filename `{part-2}`, the resolved value follows
  the ADR-051 precedence for that library (file baseline unless a decision says otherwise) and
  the `SourceBadge` shows both chips — same shape as the F60 edition conflict case.
- [ ] Visitors see the pill wherever it exists and no edit affordance (owner/visitor gating rule).
- [ ] Three-skin QA on every surface in RD9.

### Should-have (P1)

- [ ] **Title-sort tiebreak on `part`**: when two videos tie on title under title-sort, they order
  by ascending part, so the parts land adjacent and in order. Today's tiebreak is `v.id` (files
  scanned in filename order usually land right already — `{part-1}` sorts before `{part-2}`), so
  this is a correction for out-of-order additions, not the core fix. Blocked on OQ2.
- [ ] Search-result cards render the same marker as browse cards (should fall out of `VideoCard`
  reuse; listed so it is QA'd, not assumed).

### Future considerations (P2)

- `part_total` + a missing-part completeness facet (explicitly declined for v1; keep the field
  ordinal-only so a later `part_total` is a *second* field, not a format change to this one).
- Plex stacking-suffix grammar as an opt-in second `filename:` source.
- Group identity → propagate enrichment → playlist playback, in that order, each its own story.
- Adoption-time nudge when a candidate is applied to a second video.

## Behavior detail

### Part (RD1–RD8)

| Layer | Key / grammar | Notes |
|---|---|---|
| File baseline | container tag `PartNumber` (MKV/WebM) · `DiskNumber` (MP4/MOV) | MKV GENERAL `PART_NUMBER`; MP4 iTunes `disk`. Foreign `2 of 3` → `2` (OQ1) |
| Candidate | `filename:part` from `{part-N}` | F48 routing, auto-apply per flag + confidence; invalid bodies are not markers (RD4) |
| Decision | curated custom value | ADR-051; DB only; no validation (RD8) |
| Writeback | `formatMap[Matroska/WebM]["part"] = "PART_NUMBER"` · `formatMap[MP4]["part"] = "disk"` | one WriteBatch per file, atomic — non-negotiable |

The film page and `VideoCard` never resolve `part` themselves; they render the value the video
summary carries.

### Marker lifter (RD4)

`liftEdition` generalises to lift both markers from the stem in any order, each at most once,
collapsing the surrounding whitespace to one space exactly as today. A `{part-…}` body that is
not `[0-9]+` or normalises to `0` does not match the marker regex and is therefore untouched — it
stays in the stem and fails or passes pattern matching like any other literal text.

## Data model

None. `part` lives where every canonical field lives: file-layer rows (`video_metadata`),
`filename:` candidates and the review queue, `field_decisions`. No migration.

## API

No new endpoints. `part` appears in `resolved[]` and in the video summary payload as any
canonical field does; the MCP `get_video` surface inherits it through the same path.

## UI

- Media page: Part field row (generic) + header pill in the edition slot + "+ Set part" empty row.
- Film page: part pill beside the edition pill on full-film rows.
- `VideoCard`: part marker per handoff (poster-corner pill is the lean).
- Owner queue rows: marker beside the video title.
- Writeback dialog: `part` listed.

The handoff owns pill wording, placement on the card, and the three-skin tokens.

## Success Metrics

This is a single-owner library; metrics are acceptance, not analytics.

- **Leading**: after a rescan of a library whose multi-part files carry `{part-N}`, every such
  file resolves a `part` and shows the pill on all RD9 surfaces — zero identical-triplet
  sightings on the browse grid for marked files.
- **Leading**: writeback round-trip `in_sync` on MKV and MP4 samples in CI.
- **Lagging**: no reopened ask for grouping within the following month — if one comes, it is
  a new story (P2 list), not a defect here.

## Open Questions

- ~~**OQ1 (engineering, non-blocking)** — Where does the `2 of 3` → `2` normalisation for foreign
  MP4 `DiskNumber` values live~~ **Resolved 2026-09-16: the extractor.** `mapExiftool` keeps an
  `ordinalKeys` set (`PartNumber`, `DiskNumber`) whose stored value is the leading integer with
  leading zeros dropped; no leading integer → untouched; no other key is affected. The ADR-067
  pre-write snapshot still records the raw `2 of 3` so a revert restores what was there; `in_sync`
  comes from the post-write re-extract and sees the bare ordinal.
- **OQ2 (engineering, blocks P1 only)** — The browse sort is a column sort
  (`v.title COLLATE NOCASE, v.id`, `repo.go:349`); `part` is a resolved value, not a column. The
  tiebreak needs either a correlated subquery on the file-layer `PartNumber`/`DiskNumber` row
  (ignores filename candidates and decisions) or a materialised resolved-part column maintained
  at decision/scan time. If neither is cheap, the P1 stays open and `v.id` order stands.
  **Ruled 2026-09-16 at the payload step: neither is cheap enough — the P1 stays open.** The
  payload resolves `part` after the page is fetched (one batch pass per list), so it cannot feed
  `ORDER BY`; a correlated subquery would be a second precedence implementation in SQL that drifts
  from ADR-051 the first time a decision or filename candidate disagrees with the tag, and a
  materialised column is a data-model change that reopens the ADR gate this story deliberately
  has none of. `v.id` order stands; files scanned in name order still land right.
- ~~**OQ3 (design)** — Pill wording~~ **Resolved 2026-09-16**: "Part N" everywhere, one shared
  `partBadgeLabel` helper (handoff §5); the long form fits beside the duration badge even at the
  16-column tier, so no per-surface abbreviation.

## Timeline / routing

- Routing per `.claude/CLAUDE.md`: functionality → this spec · UX → `/design-handoff`
  (`media-parts-handoff.md` + committed SVG) · tests → `/testing-strategy` · no ADR · no
  security review (no auth/access/infra change; writeback plumbing is an existing perimeter with
  one more table row).
- Ships as one PR on `HOLODEX-389-media-parts`, Draft until the handoff and testing-strategy
  gates land.
- No hard deadline. Dependencies are all shipped.
