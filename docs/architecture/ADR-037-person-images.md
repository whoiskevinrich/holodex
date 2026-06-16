# ADR-037: Person images — on-disk store, typed real-or-placeholder serving, shared ingest normalization

**Status**: Proposed
**Date**: 2026-06-16
**Deciders**: Project owner
**Relates to**: spec [People Images (F24)](../specs/people-images.md); extends [ADR-009](ADR-009-thumbnail-strategy.md) (on-disk image store), [ADR-014](ADR-014-configuration-and-data-layout.md) (data layout), [ADR-015](ADR-015-media-file-serving.md) (serving discipline), [ADR-016](ADR-016-database-migrations.md) (migrations), [ADR-030](ADR-030-access-control-gating-seam.md) (owner gate — reused, **not** changed), [ADR-033](ADR-033-metadata-source-plugins.md) (enrichment providers & asset URLs), [ADR-013](ADR-013-metadata-field-mapping.md) (canonical field vocabulary — adds `gender`), [ADR-021](ADR-021-frontend-theming-and-skins.md)/[ADR-025](ADR-025-tailwind-v4-css-first.md) (skins/tokens).

---

## Context

People are name-only rows today (`people(id, name)`, [ADR-036](ADR-036-person-alias-search-indexing.md)). Spec F24 gives each person up to four image **roles** — `headshot` (1:1), `banner` (16:9), `poster` (2:3), and a multi-image `extra` gallery — sourced from **owner uploads** and **enrichment asset download** (the deferred F22 asset path), and falls back to **themed + gendered placeholders** when a role is empty. The forces:

- **Modest hardware** (NAS/Pi), single process, SQLite ([ADR-003](ADR-003-database.md)/008). The thumbnail decision ([ADR-009](ADR-009-thumbnail-strategy.md)) already established the project's posture for binary images: **on disk, generated/stored once, served with cache headers and the OS, never as DB BLOBs**. Person images should not invent a second storage philosophy.
- **Untrusted bytes.** Even though uploads are owner-only, the system now ingests arbitrary image files (upload) and remote bytes (enrichment download). Both must be neutralized before they touch disk or a browser.
- **No new access model.** F24 reverted open contribution; mutations are owner-only and must reuse the existing `requireOwner` choke point ([ADR-030](ADR-030-access-control-gating-seam.md)) rather than add identity machinery.
- **Skins.** Placeholders and image frames must react to the active skin (Cinémathèque/Broadcast/Brutalist) and to gender — a `skin × role × gender` matrix — without shipping per-skin component markup ([ADR-021](ADR-021-frontend-theming-and-skins.md)).
- **Correctness under caching.** A replaced core image must not be masked by the long `max-age` that on-disk images want for efficiency.

## Decision

### 1. Storage — one `person_images` table + on-disk files (mirror ADR-009)

A single table holds **metadata**; the bytes live on disk, generated/stored once, exactly as thumbnails do.

```
person_images(
  id          INTEGER PK,
  person_id   INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
  role        TEXT    NOT NULL,            -- headshot | banner | poster | extra
  source      TEXT    NOT NULL,            -- upload | enrichment | promoted
  provider    TEXT    NOT NULL DEFAULT '', -- enrichment provenance (F22)
  external_id TEXT    NOT NULL DEFAULT '',
  width       INTEGER NOT NULL,
  height      INTEGER NOT NULL,
  byte_size   INTEGER NOT NULL,
  sort_order  INTEGER NOT NULL DEFAULT 0,  -- gallery ordering
  created_at  TEXT    NOT NULL
)
-- one image per CORE role per person; gallery (extra) is unconstrained except the count cap
CREATE UNIQUE INDEX ux_person_images_core
  ON person_images(person_id, role) WHERE role <> 'extra';
```

- **Disk layout**: `DATA_PATH/person-images/{person_id}/{image_id}.{ext}` — under the existing data dir ([ADR-014](ADR-014-configuration-and-data-layout.md)), one subdir per person. Filenames are **server-assigned** from the row id; **no client-supplied path ever reaches the filesystem** (the ADR-015/serveThumbnail rule).
- **The row `id` is the cache-version stamp.** Replace = delete the old core row + insert a new one ⇒ a new `id` ⇒ a new `?v=` (see §3). No separate version column.
- **Caps as DB invariants**: the partial unique index makes a core role single-slot (replace-on-reupload is an upsert); the **20-extra gallery cap** is a counted insert guarded in a transaction on `role = 'extra'`.

### 2. Shared ingest normalization — decode → bound → re-encode → strip

One internal pipeline neutralizes **every** ingested image, whether from a multipart upload, an enrichment download, or a promote-crop:

1. **Decode** the bytes with the Go image decoder for the *sniffed* type (never trust the declared content-type or extension). A failed decode ⇒ reject; nothing is written.
2. **Bound** dimensions and decoded pixel area (reject absurd dimensions before allocating — decompression-bomb guard) and input byte size.
3. **Re-encode** to a single safe output format (JPEG for opaque, with a size/quality bound), which **drops all metadata** (EXIF/GPS/ICC/embedded payloads) as a side effect — defense against polyglots and metadata leakage.
4. **Persist** the re-encoded bytes to the server-assigned path; record `width/height/byte_size`.

The enrichment fetch reuses the **existing F22 network guards verbatim** (`internal/enrich`: SSRF allowlist, refuse cross-host redirects, response-size cap, timeout) and then hands the bytes to the *same* normalization pipeline — remote bytes are exactly as untrusted as uploaded ones.

### 3. Serving — typed real-or-placeholder routes, version-stamped immutable cache

- `GET /api/v1/people/{id}/image/{role}` → the real image if the role is filled, else the **resolved placeholder**. Never 404s for a valid `(person, role)`; unknown role → 400, unknown person → 404. Mirrors `serveThumbnail`'s contract.
- Gallery: a list endpoint returns ordered `extra` image ids; each is fetchable by id.
- **Caching**: a real image is served `Cache-Control: public, max-age=31536000, immutable`, and the **read model hands out the URL already stamped `?v={image_id}`**. Long-lived caching + instant busting on replace, with no CDN/ETag round-trip. Placeholders cache long and bust via a **placeholder-set version** token (bumped when the override set changes).

### 4. Placeholder resolution — deterministic, server-side, programmatic SVG, override dir

- Selection is a pure function `(active_skin, role, gender_bucket) → asset`, computed server-side at serve time. **A placeholder is never stored against a person** and never occupies a core slot or counts toward the cap.
- **Gender bucket** is derived from the enriched `gender` field (see §6): `male → male`, `female → female`, everything else (`nonbinary`, unknown, absent) → `neutral`. Three art buckets only.
- **Art is programmatic SVG** driven by the same design tokens as the skins, so the matrix (3 skins × 3 roles × 3 buckets) is generated, themeable, and committable — no binary art blobs in the repo (final hand-drawn art is a design follow-up, not a blocker).
- **Owner override**: an optional configurable dir keyed `{skin}-{role}-{bucket}` overrides individual cells; missing cells fall back to the built-in SVG. Replacing the override set bumps the placeholder-set version.
- Active skin is a client runtime choice, so the resolved-placeholder route takes the skin as a query parameter (e.g. `?skin=brutalist`); gender is resolved server-side.

### 5. Access — reuse `requireOwner`, no new identity

All mutations (upload, role assign, replace, delete, reorder, promote, placeholder-set override) mount behind the **existing `requireOwner` group** ([ADR-030](ADR-030-access-control-gating-seam.md)). Viewing images/placeholders is public (public metadata), like thumbnails. **No access-model change** — this ADR explicitly does not touch ADR-030's seam, only consumes it.

### 6. Gender as a canonical enrichment field (extends ADR-013/ADR-033)

Add `gender` to the canonical person field vocabulary ([metadata-provider-contract §4.2](../specs/metadata-provider-contract.md)) with values `male | female | nonbinary | unknown`. Providers map their native codes (a TMDB sidecar: `1→female`, `2→male`, `3→nonbinary`, `0→omit`); Holodex stores the value faithfully in `entity_enrichment` with provenance and reads it only to pick a placeholder bucket. Holodex-side normalization is defensive (lowercase/trim; unrecognized → neutral bucket). No `gender` column on `people` in v1.

## Options considered

### Storage of bytes

| Option | Complexity | Fit | Notes |
|---|---|---|---|
| **A. On-disk files + metadata table (chosen)** | Low | Matches ADR-009 exactly | OS sendfile + cache headers; SQLite stays small; trivially inspectable |
| B. SQLite BLOBs | Med | Conflicts with ADR-009 | Bloats the DB, no sendfile, awkward cache story; rejected for the same reasons thumbnails aren't BLOBs |
| C. External object store (S3/MinIO) | High | Premature | Violates the single-process, zero-dependency, runs-on-a-Pi posture (ADR-008/023); revisit only if multi-node ever happens |

### Cache invalidation on replace

| Option | Complexity | Correctness | Notes |
|---|---|---|---|
| **A. Versioned URL `?v={id}` + `immutable` (chosen)** | Low | Instant bust | One stamp from the read model; best cacheability; standard pattern |
| B. Short `max-age` / `must-revalidate` | Low | Eventually | Loses the long-cache win; a revalidation request per view |
| C. ETag / Last-Modified | Med | Conditional | Still a round-trip per view; more handler logic than a stamp |

### Placeholder art

| Option | Complexity | Theming | Notes |
|---|---|---|---|
| **A. Programmatic token-driven SVG (chosen)** | Med | Automatic per skin | Committable, no binaries, 27 cells generated; final art is a clean follow-up |
| B. Bundled binary PNGs per cell | High | Manual | 27–36 binary assets to produce + QA across skins; large diff; least flexible |
| C. Single neutral placeholder | Low | None | Fails the gendered/themed requirement; deferred-by-omission |

## Consequences

- **New package** `internal/personimage` (store + on-disk paths + the normalization pipeline + placeholder resolver), parallel to `internal/thumbnail`; `internal/api` gains the public serving routes and the owner-gated mutation group; `internal/enrich` gains an asset-fetch that feeds the shared normalizer.
- **Migration** `0008_person_images` ([ADR-016](ADR-016-database-migrations.md), embedded golang-migrate) creates the table + partial unique index.
- **go.mod**: image decode/encode uses the **standard library** (`image`, `image/jpeg`, `image/png`) plus a small resize helper; a decompression-bomb dimension guard is added before decode. No heavy dependency.
- **Config**: `DATA_PATH/person-images/` derived like `ThumbnailPath`; optional placeholder-override dir; upload bounds (max bytes, max dimension) as env vars with safe defaults.
- **Frontend**: a shared image-frame/avatar component + gallery + (P1) a client-side zoom/crop editor for promote; all token-only, QA'd across the three skins; thumbnail-style lazy/placeholder-first rendering avoids layout shift.
- **Provider contract** gains `gender` (a coordinating doc change; label + precedence per ADR-013).
- **What gets easier**: tag images (F15.3) later reuse this store/serve/placeholder machinery unchanged.
- **What we'll revisit**: a distinct `nonbinary` art bucket (4th variant), crop-on-direct-upload, and an external object store only if the deployment model ever stops being single-node.
- **Supersession**: immutable per the repo convention — superseded, not edited, if person-image storage is ever rehomed (e.g. object store) or the access model changes.

## Action items

1. [ ] Migration `0008_person_images` (+ down).
2. [ ] `internal/personimage`: store, on-disk path helper, normalization pipeline, placeholder resolver + programmatic SVG matrix.
3. [ ] `internal/enrich`: asset fetch (reuse SSRF guards) → shared normalizer; wire into the person enrich run.
4. [ ] `internal/api`: public serving routes (version-stamped) + owner-gated mutation group; extend person read model with per-role presence + `?v=` stamp + gallery ids.
5. [ ] Add `gender` to the provider contract (§4.2) + label/precedence; note the TMDB native-code mapping.
6. [ ] Frontend: image-frame/avatar/gallery components, upload + promote-crop UI, owner-gated; lazy render; QA all three skins.
7. [ ] Add `holodex_person_image_*` counters consistent with ADR-019/026 (activity events per ADR-028).
8. [ ] Update the ADR index (this row) and cross-reference the spec.
