# ADR-017: Search Architecture — Global Mixed-Entity Search + FTS5

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

The product goal is to "filter and search media by People, Title, Tags…" and to "navigate by Media, People, or Tags." Two distinct surfaces are needed: a fast global search for navigation, and in-grid filtering for narrowing. The search must also handle non-English titles and accented names correctly.

## Decision

Provide **two search surfaces**, backed by SQLite **FTS5**:

### 1. Global search (command-palette style) — primary search box
```
GET /api/v1/search?q=<term>
```
Returns **grouped, mixed-entity results** for navigation:
```json
{
  "videos": [{ "id": "...", "title": "...", "thumbnail_url": "..." }],
  "people": [{ "id": "...", "name": "...", "video_count": 0 }],
  "tags":   [{ "id": "...", "name": "...", "video_count": 0 }]
}
```
- Videos matched by title (FTS5).
- People and tags matched by name (FTS5 over name columns, prefix + substring).
- Intended for a header search box / `Ctrl/Cmd-K` palette: type, see videos + people + tags, jump to any.

### 2. In-grid filtering — browse narrowing
```
GET /api/v1/media?q=<title>&people=...&tags=...&<facets>...
```
- `q` is title FTS; combined with the structured filters (people, tags, duration, resolution, date, mapped facets per ADR-013).
- Returns only videos. This is the existing filter bar (F4).

### FTS configuration
- FTS5 virtual tables use the **`unicode61` tokenizer with `remove_diacritics=2`**, so "Amélie" matches "amelie" and case/accents are folded.
- `videos_fts(title)` mirrors `videos.title`; `people_fts(name)` and `tags_fts(name)` mirror their name columns. All kept in sync via triggers (created in migrations, ADR-016).

## Rationale

- **Two surfaces, distinct jobs.** Global search is for *finding and navigating* across entity types; the filter bar is for *narrowing a result set*. Conflating them compromises both.
- **FTS5 over LIKE.** Proper tokenization, ranking (`bm25`), and prefix search outperform `LIKE '%term%'` and stay fast at 50k+ records.
- **Diacritic folding** is essential for a personal library with international titles and names.
- The mixed-entity result directly serves the "navigate by Media, People, or Tags" goal from a single box.

## Consequences

- Three FTS5 tables (`videos_fts`, `people_fts`, `tags_fts`) and their sync triggers are part of the schema.
- The global `search` endpoint caps results per group (e.g. top 10 each) with a "see all" affordance that links into the filtered grid / index pages.
- Mapped field *values* (ADR-013) are **not** in the global search index in v1 (they are filter facets); folding them into a per-video combined FTS document is a possible future enhancement, noted but out of scope.
- `bm25` ranking parameters can be tuned later without schema change.
- Cache (ADR-008) stores hot global-search queries with a short TTL.
