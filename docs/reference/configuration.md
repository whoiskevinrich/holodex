# Configuration Reference

Holodex is configured in three layers, applied in increasing precedence order:

```
built-in defaults → holodex.yaml → environment variables → CLI flags
```

A value set by a higher-precedence layer always wins. `holodex.yaml` is the standard operator config file; environment variables are preferred for secrets and container deployments; CLI flags are for one-off overrides. Copy `holodex.yaml.example` to `holodex.yaml` as your starting point.

**Local `.env` support (development).** A `.env` file in the working directory is loaded at startup before the env layer. It feeds key-value pairs into the environment only for keys not already set, so real env vars still win. The file is gitignored and never shipped in Docker images — safe for local dev config (e.g. `MEDIA_PATH=E:/Media`) without exporting vars.

---

## Paths

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `media_path` | `MEDIA_PATH` | *(none)* | **Required.** Root of the read-only media library. Holodex scans this tree for video files. Mount read-only in Docker. |
| `data_path` | `DATA_PATH` | `./data` | Read-write data directory: SQLite database, thumbnails, person images, and runtime config. |
| `database_path` | `DATABASE_PATH` | `${data_path}/holodex.db` | Explicit database path. Defaults to `holodex.db` inside `data_path`. Override only if you need the database on a separate volume. |

`thumbnail_path` and `person_image_path` are derived from `data_path` automatically and cannot be set directly.

---

## Server

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `host` | `HOST` | *(all interfaces)* | TCP bind address. Empty string binds all interfaces — the right default inside Docker. Set `127.0.0.1` for loopback-only (local dev; avoids the Windows Firewall prompt). |
| `port` | `PORT` | `7800` | HTTP port. |
| `log_level` | `LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, or `error`. |
| `admin_token` | `ADMIN_TOKEN` | *(none)* | Token for the owner-only admin surface. **Empty = open (single-user default).** Set this whenever the server is reachable beyond loopback — Holodex warns at startup if it binds non-loopback with no token. See [Authentication](#authentication). |
| `session_secret` | `SESSION_SECRET` | *(derived from `admin_token`)* | Optional signing key for the owner session cookie (ADR-045). Empty derives the key from `admin_token`, so rotating the token invalidates all sessions; set it to rotate sessions independently. See [Authentication](#authentication). |

### CLI flags

The following CLI flags override config at the highest precedence level. Pass them to the binary directly (e.g. `holodex --port 9000`):

| Flag | Overrides |
|------|-----------|
| `--config <path>` | Path to `holodex.yaml` (optional; defaults to `./holodex.yaml`) |
| `--host <addr>` | `host` / `HOST` |
| `--port <n>` | `port` / `PORT` |
| `--media-path <dir>` | `media_path` / `MEDIA_PATH` |
| `--data-path <dir>` | `data_path` / `DATA_PATH` |
| `--log-level <level>` | `log_level` / `LOG_LEVEL` |
| `--migrate-only` | Apply migrations and exit (useful for migration-gated deployments) |
| `--healthcheck` | Probe `/healthz` and exit 0/1 (Docker `HEALTHCHECK` helper) |
| `--mcp-transport stdio` | Run as an MCP server over stdio instead of the web server |

### Authentication

`admin_token` is the single owner gate. With no token, all routes are open (zero-config single-user). With a token, admin routes (`/api/v1/admin/*`, enrichment, writeback) require owner authorization, established two ways:

- **API / script clients** send the token directly as the `X-Admin-Token: <token>` header. This header path is CSRF-immune — cross-site forms cannot set custom headers (ADR-030).
- **The SPA** exchanges the token once via `POST /api/v1/session`, which sets an **HttpOnly, `SameSite=Strict`** session cookie (ADR-045). The owner enters the token once in the UI and **stays signed in across reloads**; `DELETE /api/v1/session` signs out. The token itself is never stored in the browser (no `localStorage`/`sessionStorage`), and the cookie is unreadable by JavaScript, so an XSS payload cannot exfiltrate it. "Trust this device" issues a longer-lived cookie; active sessions slide their expiry, bounded by an absolute cap. `SameSite=Strict` is the CSRF mitigation for the cookie path. The cookie is marked `Secure` except over plain-HTTP loopback (local dev).

The session cookie is signed with `session_secret` (or a key derived from `admin_token` when unset).

---

## Scanner

Controls how often and how deeply Holodex walks `media_path` to detect new, changed, or removed files.

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `scan_interval_seconds` | `SCAN_INTERVAL_SECONDS` | `300` | How often the background scanner runs a full sweep (seconds). |
| `scan_workers` | `SCAN_WORKERS` | `4` | Parallel workers for metadata extraction (`exiftool`/`ffprobe`). Higher values extract faster at the cost of I/O. |
| `follow_symlinks` | `FOLLOW_SYMLINKS` | `true` | Whether to follow symbolic links inside `media_path`. |
| `scan_max_depth` | `SCAN_MAX_DEPTH` | `64` | Maximum directory nesting depth to walk. |
| `scan_min_age_seconds` | `SCAN_MIN_AGE_SECONDS` | `5` | Skip files modified more recently than this. Guards against scanning mid-copy files. |

---

## Soft-delete & purge

When an owner deletes a media item it is soft-deleted: hidden from all browse views immediately but kept in the database and on disk for the grace period. The purge sweep then hard-deletes expired items. Deleted items are visible (and restorable) in the Trash view until purged. (F24, ADR-037)

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `delete_grace_period_seconds` | `DELETE_GRACE_PERIOD_SECONDS` | `604800` (7 days) | How long a soft-deleted item waits in Trash before the purge sweep removes it. Set `0` to disable auto-purge entirely — items accumulate in Trash and must be manually purged. |
| `delete_remove_files` | `DELETE_REMOVE_FILES` | `true` | Whether purge unlinks the file from disk in addition to removing the DB row. Set `false` for read-only or externally-managed library mounts where Holodex must not modify the filesystem. |
| `purge_interval_seconds` | `PURGE_INTERVAL_SECONDS` | `3600` (1 hour) | How often the grace-period purge sweep runs. |

---

## Thumbnails

Cover art shown on browse cards. Embedded poster art (iTunes atoms / Matroska cover art) is extracted at scan time. Files without embedded art get a frame-grab thumbnail. (ADR-009)

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `thumbnail_enabled` | `THUMBNAIL_ENABLED` | `true` | Master switch. Set `false` to disable all thumbnail generation (browse cards show a placeholder). |
| `thumbnail_backfill` | `THUMBNAIL_BACKFILL` | `eager` | `eager` — generate thumbnails for all existing files on first run. `lazy` — generate only when a card is first viewed. |
| `thumbnail_workers` | `THUMBNAIL_WORKERS` | `2` | Parallel workers for thumbnail generation. Increase for faster backfill; reduce on low-memory systems. |
| `thumbnail_nice` | `THUMBNAIL_NICE` | `true` | Run thumbnail workers at lower process priority so generation does not compete with serving requests. |
| `thumbnail_seek_percent` | `THUMBNAIL_SEEK_PERCENT` | `10` | Which point in the video to grab the frame from, as a percentage of total duration (`0`–`100`). `10` skips the opening titles. |
| `thumbnail_width` | `THUMBNAIL_WIDTH` | `400` | Width of the generated thumbnail image in pixels. Height is derived from the source aspect ratio. Increase for sharper cards at a storage cost. |

> **What controls browse-card dimensions?** `thumbnail_width` sets the pixel width of the *stored image*. The *display aspect ratio* (16:9 vs 2:3) is set by `card_layout` (see [Presentation](#presentation)). These are independent: the image is always cropped/letterboxed at display time to match the layout.

---

## Person images

Portrait photos stored for people in the library. Uploaded manually or downloaded automatically by enrichment providers. (F25, ADR-038)

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `person_image_max_bytes` | `PERSON_IMAGE_MAX_BYTES` | `10485760` (10 MiB) | Maximum allowed request-body size for a person image upload. Larger uploads are rejected with `413`. |
| `person_image_max_dimension` | `PERSON_IMAGE_MAX_DIMENSION` | `2000` | Maximum stored image dimension (longest side, in pixels). Images exceeding this are downscaled before storage. |
| `person_gallery_max` | `PERSON_GALLERY_MAX` | `20` | Maximum number of free-form **gallery** (`extra`) images per person. The three core slots (headshot/banner/poster) are separate and never counted against this. A non-positive value falls back to `20`. The owner can deliberately exceed it per-upload via the gallery's "Add anyway" control; enrichment never exceeds it. (F25, ADR-043) |

---

## Cache

Holodex caches query results and rendered metadata to reduce database pressure. (ADR-008)

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `cache_backend` | `CACHE_BACKEND` | `memory` | `memory` — in-process Ristretto cache (zero config). `redis` — share cache across restarts or replicas. `none` — disable caching entirely (development / debugging). |
| `cache_max_memory_mb` | `CACHE_MAX_MEMORY_MB` | `128` | Maximum memory for the in-process cache (MiB). Has no effect when `cache_backend` is `redis` or `none`. |
| `redis_url` | `REDIS_URL` | *(none)* | Redis connection URL. Required when `cache_backend: redis`. Example: `redis://localhost:6379/0`. |

---

## MCP server

Holodex can expose its library catalog to AI assistants via the Model Context Protocol. (ADR-005)

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `mcp_enabled` | `MCP_ENABLED` | `false` | Enable the MCP server. |
| `mcp_transport` | `MCP_TRANSPORT` | `http` | `http` — serve MCP over HTTP (same process, separate port). `stdio` — pipe-based transport for `docker exec -i holodex holodex --mcp-transport stdio`. `both` — enable both. |
| `mcp_port` | `MCP_PORT` | `7801` | Port for the HTTP MCP transport. Ignored when `mcp_transport: stdio`. |

---

## Metadata field mapping

Maps raw container tags (iTunes atoms, Matroska tags) and enrichment fields to canonical display fields. Configured in a separate file, keeping the mapping declaration close to your library's tagging conventions. (ADR-013)

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `metadata_mappings_path` | `METADATA_MAPPINGS_PATH` | `./metadata-mappings.yaml` | Path to `metadata-mappings.yaml`. A missing file is silently treated as an empty mapping — no error. |

See `metadata-mappings.yaml.example` for the full syntax. The canonical field vocabulary is in [canonical-fields.md](canonical-fields.md).

---

## Metadata source plugins

Sidecar HTTP providers that enrich People and Film records with data from external databases (TMDB, etc.). Each provider is a separate container speaking a four-endpoint JSON protocol. (F22, ADR-033)

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `metadata_sources_path` | `METADATA_SOURCES_PATH` | `./metadata-sources.yaml` | Path to `metadata-sources.yaml`. A missing file means no providers — enrichment actions won't appear in the UI. |

See `metadata-sources.yaml.example` and `docs/specs/tmdb-provider.md` for wiring instructions.

---

## Presentation

Controls how the browse UI displays media cards.

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `card_layout` | `CARD_LAYOUT` | `wide` | Browse-card aspect ratio. `wide` shows 16:9 thumbnails; `poster` shows 2:3 cards. See below. |

### `card_layout`

```yaml
card_layout: "wide"    # 16:9 — suited to personal libraries, AMV collections, recorded video
card_layout: "poster"  # 2:3  — suited to film libraries with poster-format cover art
```

This is an operator setting: all visitors see the same layout. It is applied as a `data-layout` attribute on the `.video-grid` element, so the switch is CSS-only with no per-card logic.

**`wide` (16:9)** — the default. Thumbnails are wider than tall, matching the natural shape of most video. Frame-grab thumbnails always look correct in this mode.

**`poster` (2:3)** — portrait orientation, like a DVD or film poster. Works best when your files have embedded poster art (from a rip or a tool like MakeMKV) so the cover image fills the tall card naturally. Frame-grab thumbnails will be letterboxed in this mode. The Cinémathèque skin's letterbox bars are suppressed automatically in poster mode.

> **Related settings:** `thumbnail_width` controls the pixel width of the stored image; `card_layout` controls the display shape. Both are independent — changing `card_layout` does not regenerate thumbnails.
