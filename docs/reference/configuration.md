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

`thumbnail_path`, `person_image_path`, `studio_image_path`, and `film_image_path` are derived from `data_path` automatically and cannot be set directly.

---

## Server

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `host` | `HOST` | *(all interfaces)* | TCP bind address. Empty string binds all interfaces — the right default inside Docker. Set `127.0.0.1` for loopback-only (local dev; avoids the Windows Firewall prompt). |
| `port` | `PORT` | `7800` | HTTP port. |
| `log_level` | `LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, or `error`. |
| `admin_token` | `ADMIN_TOKEN` | *(none)* | Token for the owner-only admin surface. **Empty = open (single-user default).** Set this whenever the server is reachable beyond loopback — Holodex warns at startup if it binds non-loopback with no token. See [Authentication](#authentication). |
| `session_secret` | `SESSION_SECRET` | *(derived from `admin_token`)* | Optional signing key for the owner session cookie (ADR-046). Empty derives the key from `admin_token`, so rotating the token invalidates all sessions; set it to rotate sessions independently. See [Authentication](#authentication). |

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
- **The SPA** exchanges the token once via `POST /api/v1/session`, which sets an **HttpOnly, `SameSite=Strict`** session cookie (ADR-046). The owner enters the token once in the UI and **stays signed in across reloads**; `DELETE /api/v1/session` signs out. The token itself is never stored in the browser (no `localStorage`/`sessionStorage`), and the cookie is unreadable by JavaScript, so an XSS payload cannot exfiltrate it. "Trust this device" issues a longer-lived cookie; active sessions slide their expiry, bounded by an absolute cap. `SameSite=Strict` is the CSRF mitigation for the cookie path. The cookie is marked `Secure` except over plain-HTTP loopback (local dev).

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
| `poster_width` | `POSTER_WIDTH` | `1200` | Width of the larger detail-page poster tier, in pixels (F53). Generated as a sibling of the thumbnail in the same extraction pass — a video's `media/{id}` detail page uses this instead of `thumbnail_width` so its poster isn't an upscaled list thumbnail. |

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

## Studio images

Icon/logo/poster images for studios — sourced from enrichment providers by default, owner-uploadable per role. Each role is single-slot (no gallery). (F51, ADR-079)

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `studio_image_max_bytes` | `STUDIO_IMAGE_MAX_BYTES` | `10485760` (10 MiB) | Maximum allowed request-body size for a studio image upload. Larger uploads are rejected with `413`. |
| `studio_image_max_dimension` | `STUDIO_IMAGE_MAX_DIMENSION` | `1000` | Maximum stored image dimension (longest side, in pixels). Images exceeding this are downscaled before storage. |

---

## Film images

Poster/thumb images for films — owner-uploadable per role (only `poster` has a consuming UI today). Requires `films_enabled: true`. (F56/HOLODEX-280, ADR-086)

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `film_image_max_bytes` | `FILM_IMAGE_MAX_BYTES` | `10485760` (10 MiB) | Maximum allowed request-body size for a film image upload. Larger uploads are rejected with `413`. |
| `film_image_max_dimension` | `FILM_IMAGE_MAX_DIMENSION` | `1500` | Maximum stored image dimension (longest side, in pixels). Images exceeding this are downscaled before storage. |

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

### In-app field promotion overrides `metadata-mappings.yaml` (F44, ADR-062)

`metadata-mappings.yaml` is **operator config** — the app never writes it. But an owner can, from an
entity page, **promote** a provider's auto-registered non-canonical field (F39/ADR-056) into a first-class
curatable field — setting its label, render mode, group, and order, and opting it into the source-decision
+ curation controls — with **no YAML editing**. These promotions live in the DB (`field_promotions`,
migration `0023`), are owner-gated, and are global per `(entity_type, field_key)`.

A live in-app promotion is a new **tier-0** that **outranks** an operator `metadata-mappings.yaml` entry for
the same non-canonical key (the owner *is* the operator on a single-user server). Full precedence for a
field's label/render/group/order and its curatable status:

```
0. In-app promotion  (field_promotions, this feature)   ← wins
1. Operator metadata-mappings.yaml
2. Code registry (canonical keys)
3. Provider render hint (non-canonical keys)
4. Title-case fallback
```

A promotion may only target a **non-canonical** key (one the registry does not know) — you cannot promote a
canonical field like `bio`. Removing a promotion reverts the field to its display-only auto-registered state;
the underlying shadow value and any per-entity decisions/curation are untouched. This affects **video, person,
and studio** — person and studio have no `metadata-mappings.yaml` surface at all, so promotion is their only
in-app remap path.

**Promote or claim?** Promote a key when it is *its own thing* deserving a row and curation. When it is
instead the *same thing* as a field you already have — a second provider's name for your overview — **claim**
it onto that field so it stops rendering twice (F49,
[ADR-074](../architecture/ADR-074-claimed-provider-keys.md)). The two are mutually exclusive per key; the full
decision table and worked examples live in
[canonical-fields.md § Claiming a provider key](canonical-fields.md#claiming-a-provider-key).

### Genre → Tag materialization, deny-list & hierarchy (F50, ADR-075)

The `genres` canonical field does double duty: resolved values also auto-materialize into real `Tag` rows the
moment a video is enriched, feed genre writeback, and are subject to two owner-managed governance
controls — a global deny-list (`/owner/tags`) and a tag hierarchy (`/tags`' parent-setter). None of this is a
`holodex.yaml` or env-var setting — it's config-free, driven entirely by the `genres` mapping you already
maintain in `metadata-mappings.yaml`. Full mechanism in
[canonical-fields.md § Genre tag materialization & governance](canonical-fields.md#genre-tag-materialization--governance-f50-adr-075).

### `actors` / `director` / `studio` drive the People and Studios pages (F40, ADR-072)

Since F40, the People and Studios pages are **not** populated by raw file-tag extraction — `video_people` and
`video_studios` are *derived* from the video's **resolved** `actors`, `director`, and `studio` canonical
fields, re-run on every scan. If your `metadata-mappings.yaml` doesn't map these three fields, nobody and no
studio can ever link, no matter what your file tags or enrichment provider say. This trips up two situations
in particular:

- **An instance whose config predates F40.** If you copied `metadata-mappings.yaml` from an older example
  before these fields existed, add them — see `metadata-mappings.yaml.example`'s "Cast & crew" section for
  the full, commented block (file tags + `tmdb:actors`/`tmdb:director`, unioned as `multi: true` fields).
- **A `metadata-mappings.yaml` that's missing or unreadable at startup.** A missing file loads as an *empty*
  mapping (no error, per the table above) — same effect as omitting `actors`/`director`/`studio` specifically.

As of HOLODEX-256 (`actors`/`director`) and its studio follow-up, an unmapped field now leaves any
**existing** `video_people`/`video_studios` links alone (a missing mapping means "no opinion," not "resolved
to nobody") — but new links still can't form until the field is mapped, so the People/Studios pages stay
empty for a config that's never had these fields at all. Before both fixes, an unmapped field wiped every
existing link on the next relink trigger; `studio` had it worse, since `ReconcileVideoStudios` prunes
immediately with no orphan grace period (unlike person, ADR-072 §4) — an unmapped `studio` field used to
delete every studio link *and every studio entity* outright, not just leave them displayed-but-stale. Map
all three fields before your first boot regardless — the guard only stops the wipe, it doesn't create links
a missing mapping was never going to produce.

Minimum snippet to restore person- and studio-linking (copy the fuller commented version from
`metadata-mappings.yaml.example` when you also want the additional file-tag aliases and the F48 filename-token
source):

```yaml
fields:
  - canonical: actors
    sources: ["Artist", "Actor", "Actors", "tmdb:actors"]
    multi: true
  - canonical: director
    sources: ["Director", "tmdb:director"]
    multi: true
  - canonical: studio
    sources: ["Publisher", "Label", "Studio", "tmdb:studio"]
```

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

## Films

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `films_enabled` | `FILMS_ENABLED` | `false` | Enable the Films entity (F56, ADR-085): browsable films with owner-asserted video/scene attachments and resolver-source Album/Title writeback. |

Off by default, matching `mcp_enabled`'s precedent. Server-side and real, not cosmetic: `/films` routes are unregistered, films are excluded from search/MCP, and any per-video "attached to film X" decision is *suspended* (not deregistered) while off — the file on disk keeps whatever was last written, and re-enabling restores the same resolved value with no owner action. Turning this flag off never reverts Album/Title values already written to files. Migrations run regardless of this flag; it gates behavior, not schema. Film-library operators typically pair this with `card_layout: "poster"` above.

## Metadata source of truth

Controls which layer wins a **replace (scalar)** field when the owner has made no per-field decision (F36, [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md)).

| `holodex.yaml` key | Env var | Default | Description |
|--------------------|---------|---------|-------------|
| `default_source` | `DEFAULT_SOURCE` | `file` | Undecided-field winner: `file` (baseline wins; providers are candidates) or `mapping` (legacy first-non-empty precedence). See below. |
| `provider_trust_order` | `PROVIDER_TRUST_ORDER` | *(empty)* | Ranks providers for the undecided winner *among providers* when several match. See below. |

### `default_source`

```yaml
default_source: "file"     # baseline wins undecided replace fields (default)
default_source: "mapping"  # legacy: first non-empty source in each field's `sources` list wins
```

Holodex keeps three metadata layers per field — the **file** layer (your container tags), **provider** enrichment, and **manual** curation. For a replace field the owner can make a standing per-item decision (`keep file` / `adopt <provider>` / `custom`) via the detail-page source control; `default_source` only decides what happens **before** any such decision.

**`file` (default)** — the file/baseline value is the source of truth; a provider value is shown as a *candidate* you adopt deliberately, never an automatic winner. This fixes the case where a provider silently masks your own file tags (and where writeback would then overwrite them). Recommended for personal libraries and any non-film provider. Under this mode the `sources` order in `metadata-mappings.yaml` is only **candidate suggestion order**, not a display winner.

**`mapping`** — restores the legacy behavior: the first non-empty source in each field's `sources` list wins. Choose this for a film library that wants provider-first display without setting a decision on every item.

> **Scope:** `default_source` applies only to **replace** fields. **Merge/set** fields (`multi`/`merge`, e.g. genres, actors) always union every source and are unaffected. A per-field decision, when set, overrides `default_source` for that field.

### `provider_trust_order`

When more than one provider is configured and **several matched providers supply a value** for the same undecided replace field, this global list decides which provider wins.

```yaml
provider_trust_order:      # PROVIDER_TRUST_ORDER="tmdb,imdb"
  - tmdb
  - imdb
```

The winner is the **first-listed** provider that has a value. Providers not named in the list rank **behind** every listed one, keeping their `metadata-mappings.yaml` `sources` order among themselves. With the list empty (the default), the winner among providers is simply the first non-empty source in mapping order — today's behavior, unchanged.

This ranking sits **below** the two rules that already govern a replace field, so it never overrides them:

1. A **per-field decision** (`keep file` / `adopt <provider>` / `custom`) always wins — trust order is consulted only for *undecided* fields.
2. Under `default_source: file` (the default) the **file/baseline layer still beats every provider**; trust order only breaks the tie *among providers*, which surfaces when the file carries no value for the field (so the providers actually compete).

Trust order is applied under the **file-first** default only. Under `default_source: mapping` the literal `sources` order in `metadata-mappings.yaml` is authoritative — rank providers there by listing them in the order you want.

> **When this matters:** only with **two or more providers** enriching the same field, on a field where the file has no value of its own. A single-provider instance (the common case) is unaffected. The env form is a comma-separated list.
