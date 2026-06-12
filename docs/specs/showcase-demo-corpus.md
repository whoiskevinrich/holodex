# Spec — Showcase Demo Corpus

**Status**: Implemented (showcase tooling)
**Date**: 2026-06-11
**Related**: [ADR-004](../architecture/ADR-004-metadata-extraction.md) (extraction),
[ADR-009](../architecture/ADR-009-thumbnail-strategy.md) (cover art / Tier 1),
[ADR-012](../architecture/ADR-012-resolution-classification.md) (resolution buckets),
[ADR-017](../architecture/ADR-017-search-architecture.md) (FTS folding).
Generator: [`testdata/demo/`](../../testdata/demo/).

## Problem

A brand-new visitor has no way to *see* what Holodex offers. The app's front door is
the media grid, which is empty until you mount your own library — and the README has no
screenshots. To showcase the product (and its three skins) we need the app to render a
full, attractive, believable library on demand, without shipping real video files or any
third-party IP.

## Goal

A one-command generator that produces a deterministic demo media library. Pointing
`MEDIA_PATH` at its output yields a populated grid that exercises every user-visible
surface — for screenshots (README), a future live demo, and manual QA of all three skins.

## Non-goals

- Not a test fixture corpus — that is [`testdata/gen.sh`](../../testdata/gen.sh) (adversarial
  encoder/tag fragmentation + golden files). This corpus optimizes for *looking good*, not
  for covering extraction edge cases.
- No real footage. Each "video" is a tiny static clip whose frames are its key-art.
- Not committed media — generated output and `node_modules` are gitignored; only the
  generator (`items.mjs`, `poster.mjs`, `generate.mjs`) is tracked.

## Approach

For each curated item the generator:

1. **Renders key-art** (`poster.mjs`) — a deterministic 16:9 SVG "still" (flat cinematic
   background + abstract motif + title/genre/year), rasterized to JPEG by `sharp`. Posters
   are skin-independent: they become the embedded cover art, identical across all skins.
2. **Muxes an MP4** (`ffmpeg`):
   - video stream: the poster encoded at the item's **target resolution** (so the
     width-based resolution badge is correct, ADR-012) for the item's **runtime** at 1 fps
     — a near-zero-bitrate static clip (small + fast to encode, plays on the detail page).
   - `attached_pic`: the same poster as the container's cover art, which the scanner
     extracts at index time (ADR-009 **Tier 1**) and shows on the card immediately.
   - tags: `title`, `artist` (people, comma-joined), `genre` (tags, comma-joined),
     `date` — read back via exiftool as `Title` / `Artist` / `Genre` / `ContentCreateDate`.
     People/tags split on `, ; /` per the extractor (`splitMulti`).

The result is a fully **metadata-driven** library: no real video files, but every card,
badge, filter, and the People/Tags pages populate from the embedded tags — the same path a
real library uses.

## Content requirements

The curated set (`items.mjs`, ~18 titles) must, in aggregate:

- **R1** — cover every resolution bucket: at least one each of SD / HD / FHD / 4K.
- **R2** — show varied, realistic runtimes (not all identical) so the duration badge reads true.
- **R3** — repeat several people and tags across titles so People/Tags pages and the facet
  filters look connected (e.g. a person appearing in 2+ titles).
- **R4** — include a diacritic title (e.g. "Amélie en Hiver") to demonstrate FTS folding (ADR-017).
- **R5** — span a range of years (2008–2024) so the year filter is meaningful.
- **R6** — use only **fictional** titles, people, and studios (no third-party IP).

## Acceptance criteria

- **A1** — `npm install && npm run generate` (in `testdata/demo/`) writes N `.mp4` files to
  `./library` with only `ffmpeg` + Node as external needs.
- **A2** — Pointing Holodex at the output (`MEDIA_PATH=…/library`) and scanning produces one
  card per title, each showing its **cover art** (Tier 1, no generation wait), correct
  **resolution badge**, **duration**, **title**, and **tags**.
- **A3** — The People and Tags index pages list the corpus's people/tags with correct counts;
  person/tag filters and detail pages return the expected subsets.
- **A4** — The year filter and title search (including the diacritic title) behave.
- **A5** — Generation is deterministic: re-running yields the same library.
- **A6** — Verified across **all three skins** (Cinémathèque, Broadcast, Brutalist) per
  `.claude/CLAUDE.md`.

## Usage

```bash
cd testdata/demo
npm install                       # once (pulls sharp)
npm run generate                  # full corpus -> ./library
node generate.mjs --only nightshade        # single title
node generate.mjs --max-seconds 8          # cap runtimes (fast smoke build)
node generate.mjs --out /path/to/media     # custom output directory
```

## Showcase surfaces built on this corpus

- **README** — product-first rewrite with a three-skin gallery + detail shot
  (`docs/assets/screenshots/`, captured from this corpus).
- **Landing page** — [`site/`](../../site/), a self-contained static page whose hero swaps the
  real grid + detail screenshots between skins, tinted with each skin's accent. It reuses the
  existing theming design system (ADR-021) rather than introducing new UX, so no new design
  handoff is required; deploying it is a separate infra step (see [`site/README.md`](../../site/README.md)).

## Follow-ups (tracked separately)

- Deploy the landing page (GitHub Pages / Cloudflare) — needs a security review for the CI/infra.
- Optional: reuse this corpus as the seed for a hosted live demo / in-app demo mode.
