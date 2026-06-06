# ADR-010: MKV (Matroska) Tag-Target Precedence

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

Matroska (MKV) tags are scoped by a `TargetTypeValue` level that describes *what* a tag applies to. A single MKV file can carry overlapping tags at different levels — e.g. a `TITLE` at the collection level (a box-set name), at the season level, at the movie/episode level, and even per A/V track. The extractor (ADR-004) must decide which level is authoritative for each of Holodex's metadata fields, or it will mislabel videos (e.g. stamping every episode with the series title, or pulling an audio track's name as the video title).

Relevant target levels:

| Level | Semantic |
|-------|----------|
| 70 | COLLECTION (box set, franchise) |
| 60 | SEASON / EDITION / VOLUME |
| **50** | **MOVIE / EPISODE — the whole work contained in this file** |
| 40 | PART / SESSION |
| 30 | TRACK / CHAPTER — an individual A/V stream or chapter |
| 20–10 | SUBTRACK / SCENE / SHOT |

Per the Matroska spec, a tag with **no** `TargetTypeValue` defaults to level **50**.

## Decision

Treat **level 50 (MOVIE/EPISODE) and untargeted tags as authoritative** for all per-video metadata fields, with field-specific fallback rules:

| Field | Source precedence |
|-------|-------------------|
| **Title** | level 50 / untargeted → 60 (season) → 70 (collection) → filename |
| **Date** | level 50 / untargeted → 60 → 70 → file mtime |
| **People** (ACTOR, ARTIST, DIRECTOR) | level 50 / untargeted **only** (aggregated, de-duplicated) |
| **Tags / genres** (GENRE, KEYWORDS) | level 50 / untargeted **only** (aggregated, de-duplicated) |

**Level 30 (track/chapter) tags are ignored** for all of the above — track-level tags describe a specific audio/subtitle stream, not the video as a whole.

## Rationale

- **Level 50 is the file's own identity.** It describes the movie or episode contained in this file, which is exactly what a Holodex `Video` record represents.
- **Multi-value fields must not inherit from higher levels.** Pulling people/genres from level 60/70 would smear a series' entire cast and genre set across every individual episode. Title *may* fall back to higher levels (a sensible "better than nothing" name), but cast/genre may not.
- **Track-level tags are a different entity.** A `TITLE` on an audio track ("Director's Commentary") must never become the video's title.

## Tool Mapping

This rule maps cleanly onto the ADR-004 toolchain:

- **`ffprobe format.tags`** ≈ file-level / level-50 tags → **use these**.
- **`ffprobe streams[].tags`** ≈ track-level (level 30) tags → **ignore** for title/people/genre/date.
- **exiftool** remains the primary, richer source; where it surfaces target levels, the same precedence applies. Where exiftool flattens ambiguously, `ffprobe`'s format-vs-stream split is the disambiguator.

## Consequences

- The extractor's MKV path explicitly reads only file/format-level tags for the four metadata fields; stream-level tags are skipped.
- If a future need arises to capture series/season context (Phase 3 enrichment), higher-level tags (60/70) can be read into dedicated `series` / `season` fields without disturbing this per-video rule.
- Chapter data (also level 30-adjacent) is out of scope for all current phases.
