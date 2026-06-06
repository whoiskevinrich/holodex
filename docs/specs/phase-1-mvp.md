# Phase 1 Spec: MVP — Indexer + Browse + Search

**Status**: Draft  
**Phase**: 1 of 3  
**Depends on**: ADR-001 (Backend), ADR-002 (Frontend), ADR-003 (Database), ADR-004 (Metadata extraction), ADR-010 (MKV tag precedence), ADR-011 (Symlink handling), ADR-012 (Resolution classification), ADR-013 (Metadata field mapping — data capture only), ADR-014 (Config & data layout), ADR-015 (Media file serving), ADR-016 (Migrations), ADR-017 (Search), ADR-018 (Scan change detection), ADR-019 (Observability)

---

## Objective

A working Docker container that scans a local/mounted video library, indexes metadata from MP4/MKV files, and serves a fast dark-mode web UI for browsing and filtering.

---

## Scope

### In Scope
- File scanner and background indexer
- Metadata extraction (MP4 + MKV embedded tags)
- Browse view (grid of video cards)
- Search/filter bar (title, people, tags, duration, resolution, date)
- People index page and person detail page
- Tags index page and tag detail page
- Video detail page
- Dark mode (default), light mode toggle
- Docker packaging with `docker-compose.yml`
- Environment variable configuration

### Out of Scope (Phase 1)
- MCP server (Phase 2)
- Thumbnail generation via ffmpeg (Phase 2)
- Sort controls (Phase 2)
- Codec/container detail display (Phase 2)
- People enrichment / aliases (Phase 3)
- Tag graphs / aliases (Phase 3)
- Metadata writeback (Phase 3)

---

## Functional Requirements

### F1: File Scanner

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F1.1 | On startup, recursively scan `MEDIA_PATH` for `.mp4` and `.mkv` files | All files under the path are discoverable within 30s of startup for a 10k-file library |
| F1.2 | Watch for filesystem changes and index new/modified files automatically | New file added to `MEDIA_PATH` appears in the database within 60 seconds |
| F1.3 | On re-scan, detect removed files and mark them inactive | File deleted from disk is no longer returned in any query after next scan cycle |
| F1.4 | Scan interval configurable via `SCAN_INTERVAL_SECONDS` env var (default: 300) | Setting `SCAN_INTERVAL_SECONDS=60` causes re-scan every 60 seconds |
| F1.5 | Filesystem watching via OS-level events (inotify/FSEvents/ReadDirectoryChangesW) as primary mechanism, periodic scan as fallback | Tested on Linux container environment |
| F1.6 | Follow symlinks (default), resolving to canonical path and indexing each real file once; detect loops; allow targets outside `MEDIA_PATH` (see ADR-011) | A file reachable via two symlinks appears once; a symlink loop does not hang the scan |
| F1.7 | `FOLLOW_SYMLINKS` (default `true`) and `SCAN_MAX_DEPTH` (default 64) configurable via env var | Setting `FOLLOW_SYMLINKS=false` causes symlinked entries to be skipped |
| F1.8 | Incremental scan: re-extract only when (size, mtime) differs from the stored record; skip unchanged files (see ADR-018) | Second scan of an unchanged 10k library performs no extraction subprocess calls |
| F1.9 | Skip files modified within `SCAN_MIN_AGE_SECONDS` (default 5) to avoid indexing mid-copy files | A file actively being copied is not indexed until it settles |

### F2: Metadata Extraction

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F2.1 | Extract **title** from MP4 (`©nam` atom) and MKV (`TITLE` tag) | Title field populated on detail page for files with embedded titles |
| F2.2 | Extract **people/cast** from MP4 (`©ART`, `aART`, iTunes `cast` or custom atoms) and MKV (`ACTOR` tag, `ARTIST` tag) | People field populated; each name becomes a distinct Person record |
| F2.3 | Extract **tags/genres** from MP4 (`©gen`, `gnre`) and MKV (`GENRE`, `KEYWORDS` tags) | Tags field populated; each term becomes a distinct Tag record |
| F2.4 | Extract **duration** in seconds | Duration displayed as HH:MM:SS on cards and detail pages |
| F2.5 | Extract **resolution** (width × height) from video stream metadata; classify into SD/HD/FHD/4K+ bucket by width with 10% tolerance (see ADR-012) | Resolution displayed as badge (SD / HD / FHD / 4K) on cards; a 3792-wide file classifies as 4K+ |
| F2.6 | Extract **creation/recording date** from MP4 (`©day`) and MKV (`DATE_RECORDED`, `DATE_TAGGED`) | Date displayed on detail page; available as filter dimension |
| F2.7 | Fall back to file modification date when no embedded date exists | Date field is always populated |
| F2.8 | Store raw file path, file size, and last-modified timestamp | Shown on detail page |
| F2.9 | Capture **all human-meaningful container/format-level tag key-values** per video into `video_metadata` (excluding cover-art blobs, core-six source keys, and stream-level tags), so later mapping config (Phase 2) needs no re-scan | A file with a `Publisher` tag has a `video_metadata` row `(source_key=Publisher, value=...)` after indexing |

### F3: Browse View (`/`)

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F3.1 | Grid of video cards, virtual-scrolled or paginated (≥ 50 per page) | 1,000-video library renders without layout thrash; 60fps scroll |
| F3.2 | Each card shows: cover art (or placeholder), title, duration, resolution badge, first 3 tags | Visually verified |
| F3.3 | Clicking a card navigates to the video detail page | Route changes to `/media/:id` |
| F3.4 | Default sort: date added descending | Newest files appear first on initial load |

### F4: Search and Filter

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F4.1 | Full-text search on title (FTS5, unicode61 + diacritic folding), updating results as user types (debounced 200ms) | Typing "sun" returns all titles containing "sun"; "amelie" matches "Amélie"; no full page reload |
| F4.2 | Filter by People (multi-select autocomplete from indexed people) | Selecting "Alice" shows only videos where Alice is in the cast |
| F4.3 | Filter by Tags (multi-select autocomplete from indexed tags) | Selecting "documentary" filters correctly |
| F4.4 | Filter by Duration (range slider, min/max in minutes) | Setting 60–120 min shows only videos in that range |
| F4.5 | Filter by Resolution (single-select: All / SD / HD / FHD / 4K+), bucketed by width per ADR-012 | Selecting "4K+" shows only videos with width ≥ 3456 (3840 − 10% tolerance) |
| F4.6 | Filter by Date (year range picker) | Setting 2020–2022 shows only videos with date in that range |
| F4.7 | All active filters reflected in URL query params | Sharing the URL with filters applied loads the same filtered state |
| F4.8 | Clear-all-filters button visible when any filter is active | One click resets to unfiltered state |
| F4.9 | Filter API response time ≤ 300ms at p95 for 50k-record library | Measured via API latency with seeded test dataset |
| F4.10 | Global search box (header / `Ctrl-K` palette) returns mixed results — videos, people, tags — via `GET /api/v1/search?q=` (see ADR-017) | Typing a term shows matching videos, people, and tags in grouped sections; selecting any navigates to it |

### F5: People Navigation

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F5.1 | `/people` page lists all indexed people, sortable by name (A-Z) and video count | All people with at least one video appear |
| F5.2 | `/people/:id` page shows person name and a browse grid of all videos featuring that person | Filtered grid uses same card component as browse view |
| F5.3 | People links on video detail page navigate to the person's page | Clicking a person's name on a detail page goes to their page |

### F6: Tags Navigation

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F6.1 | `/tags` page lists all indexed tags, sortable by name (A-Z) and video count | All tags with at least one video appear |
| F6.2 | `/tags/:id` page shows tag name and a browse grid of all videos with that tag | Filtered grid uses same card component as browse view |
| F6.3 | Tag chips on video detail and card pages navigate to the tag's page | Clicking a tag chip goes to its tag page |

### F7: Video Detail Page (`/media/:id`)

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F7.1 | Shows all extracted metadata: title, people, tags, duration, resolution, date, file path, file size | All fields visible |
| F7.2 | Inline `<video>` player streams from `GET /api/v1/media/:id/stream` with HTTP Range support (seeking works); falls back to a download/open link if the browser can't decode the codec (see ADR-015) | Scrubbing the player seeks correctly (server returns 206); unsupported codec shows fallback link without crashing |
| F7.3 | People and tag values are links to their respective navigation pages | Clickable, navigates correctly |
| F7.4 | A collapsible "raw extracted metadata" panel lists every captured `video_metadata` key-value for the file (see ADR-013) | A file's `Publisher` tag is visible in the raw panel; verifies F2.9 capture |

### F8: Dark Mode

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F8.1 | Dark mode is the application default | No white flash on initial page load; background is dark |
| F8.2 | Light mode toggle in the header persists preference to localStorage | Preference survives page refresh |
| F8.3 | Color scheme passes WCAG AA contrast ratio (4.5:1 for normal text) | Verified with browser accessibility tool |

### F9: Docker Packaging

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F9.1 | `docker-compose.yml` in repo root starts the full service | `docker compose up` succeeds on a clean machine |
| F9.2 | Web UI accessible at `http://localhost:7800` by default | Browser loads the browse page |
| F9.3 | Two volumes: `MEDIA_PATH` (read-only) and `DATA_PATH` (read-write, holds DB/thumbnails/config — see ADR-014) | Restarting the container preserves indexed data and config |
| F9.4 | Multi-arch image (linux/amd64, linux/arm64) | `docker buildx` manifest includes both platforms |
| F9.5 | Config precedence: CLI flags > env vars > `holodex.yaml` > defaults (ADR-014); all keys documented in README + `holodex.yaml.example` | Setting `PORT` via env overrides the config file value |
| F9.6 | Schema migrations embedded and auto-applied on startup (ADR-016); startup aborts on migration failure | Fresh volume is migrated to current schema on first run |
| F9.7 | `GET /healthz` (liveness) and `GET /readyz` (readiness) endpoints; Docker `HEALTHCHECK` uses `/healthz` (ADR-019) | `/readyz` returns 503 until migrations + bootstrap complete, then 200 |
| F9.8 | Graceful shutdown on SIGTERM: drain requests, stop scanner/workers, checkpoint WAL (ADR-019) | `docker stop` exits cleanly without DB recovery on next start |

---

## Non-Functional Requirements

| Category | Requirement |
|----------|-------------|
| Performance | Search API p95 ≤ 300ms for 50k records on 4-core / 8 GB machine |
| Performance | Client-side route transitions ≤ 150ms perceived |
| Reliability | Scanner crash does not crash the web server |
| Reliability | Corrupt or unreadable media files are logged and skipped; they do not stop the scan |
| Security | Files are served only by video ID (canonical path looked up from the index); clients never supply paths, so traversal is structurally impossible (ADR-011/ADR-015) |
| Reliability | Schema migrations auto-apply on startup; failure aborts startup rather than serving a half-migrated DB (ADR-016) |
| Observability | Structured logging (`slog`, `LOG_LEVEL`); each scan logs a seen/added/updated/removed/skipped/errors summary (ADR-019) |
| Portability | Runs on Linux x86_64 and arm64; Docker image is the primary delivery mechanism |

---

## Data Model (Technology-Agnostic)

```
Video
  id            UUID / serial PK
  file_path     string (unique, canonical absolute path — ADR-011)
  file_size     int64 (bytes)
  title         string
  duration_sec  int
  width         int
  height        int
  recorded_at   date (nullable)
  indexed_at    timestamp
  file_mtime    timestamp
  active        bool

Person
  id            UUID / serial PK
  name          string (unique)

Tag
  id            UUID / serial PK
  name          string (unique)

VideoPersons  (junction)
  video_id → Video
  person_id → Person

VideoTags  (junction)
  video_id → Video
  tag_id → Tag

VideoMetadata  (extended/extra tags — see ADR-013)
  video_id → Video
  source_key   string   (tag key as extracted; matched case-insensitively)
  value        string
  -- index on (source_key, value) for Phase 2 facet queries
```

---

## Open Questions (Phase 1 Specific)

1. ~~**Symlinks**~~: **Resolved (ADR-011)** — follow symlinks by default, dedup by canonical path, allow targets outside `MEDIA_PATH`; configurable via `FOLLOW_SYMLINKS`.
2. ~~**MKV tag priority**~~: **Resolved (ADR-010)** — Matroska target level 50 (MOVIE/EPISODE) and untargeted tags are authoritative; track/chapter (level 30) tags ignored; people/genres never inherited from higher levels.
3. ~~**Cover art source**~~: **Resolved (ADR-009)** — embedded cover art is extracted at index time (Tier 1, near-free); generated frame thumbnails are a Phase 2 background job.
4. ~~**Resolution buckets**~~: **Resolved (ADR-012)** — width-based buckets with 10% tolerance: SD <1152, HD 1152–1727, FHD 1728–3455, 4K+ ≥3456.

_All Phase 1 open questions resolved._
