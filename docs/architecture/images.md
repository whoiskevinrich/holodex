# Images

This doc owns self-hosted entity images: the stores under `DATA_PATH`, the normalization spine,
serving and cache-busting, the provenance lock and gallery rules enrichment writes under, and the
studio-role, provider-icon and halo tables. Which hosts bytes may come from is
[security-perimeter.md](security-perimeter.md); the `/enrich` `assets[]` and `/describe.brand_icon`
contracts (and the icon's writer) are [metadata-providers.md](metadata-providers.md); video
thumbnails are [media-ingest.md](media-ingest.md).

## On-disk stores with a metadata row per image

Image bytes live on disk under `DATA_PATH`; the DB holds one metadata row per image (`role`,
`source`, `provider`, `external_id`, `width`, `height`, `byte_size`, `created_at`), never a BLOB.

| Store (config field) | Layout | Table |
|---|---|---|
| `person-images/` (`PersonImagePath`) | `{person_id}/{image_id}{ext}` | `person_images` |
| `studio-images/` (`StudioImagePath`) | `{studio_id}/{image_id}{ext}` | `studio_images` |
| `film-images/` (`FilmImagePath`) | `{film_id}/{image_id}{ext}` | `film_images` |
| `provider-icons/` (`ProviderIconPath`) | `{icon_id}{ext}` (flat) | `provider_icons` |

- **Server-assigned paths only.** A filename is the row id; no request value, provider name or URL
  ever becomes a path component. `internal/entityimage` owns the per-entity layout (`Store`, `Find`,
  `Remove`) behind `personimage` / `studioimage` / `filmimage`; `providericon` mirrors it flat.
- **The row id is the cache version.** A replace is delete + insert, so the id changes and the
  served `?v={id}` changes. There is no version column.
- **Separate tables per entity, by design.** They diverge (person has a gallery and `sort_order`;
  film is keyed by `(film, role, source)`), so one polymorphic table would special-case each anyway.

**Rejected:** SQLite BLOBs — bloat the DB and lose sendfile. **Rejected:** an external object store —
breaks the single-process, runs-on-a-NAS posture.

Decided in [`0ebf20ef`](https://github.com/whoiskevinrich/holodex/commit/0ebf20ef).

## One normalization spine: decode, bound, re-encode (PNG when alpha, else JPEG)

Every ingested image, from an owner upload or a provider download, goes through
`personimage.Normalize` before it touches disk. Remote bytes are treated exactly like uploads.

1. **Decode** with the stdlib decoder for the sniffed type, ignoring declared type and extension. A
   failed decode rejects the image; SVG has no decoder, so it is refused.
2. **Bound** bytes and dimensions before the full decode (`maxPixels` bomb guard), then downscale.
3. **Re-encode** with `image/png` when the image reports `Opaque() == false`, else `image/jpeg` (an
   unknown type counts as opaque). Either encoder drops all metadata and polyglot payloads.
4. **Extension from the bytes:** `entityimage.Ext` returns `.png` for a PNG signature, else `.jpg`,
   over `Normalize`'s own output only. `Find` stats `.jpg` then `.png`; `Remove` clears both.

`personimage.NormalizeJPEG` drops the PNG branch; only the video poster upload uses it.

**Rejected:** always PNG — photographs grow several times. **Rejected:** a `format` column — a
migration per table to store what the first eight bytes say. **Rejected:** server-side SVG
rasterizing — puts an SVG parser on the untrusted-bytes perimeter.

Decided in [`0ebf20ef`](https://github.com/whoiskevinrich/holodex/commit/0ebf20ef).

## Typed serve routes with an immutable, version-stamped cache

Each store has a public typed route; mutations sit behind `requireOwner`.

- **Routes:** `GET /api/v1/people/{id}/image/{role}` and `/people/{id}/images/{imageId}`,
  `GET /api/v1/studios/{id}/images/{role}`, `GET /api/v1/films/{id}/images/{role}`,
  `GET /api/v1/providers/{name}/icon` (`{name}` selects a row, never a file).
- **Headers:** a stored file is served `Cache-Control: public, max-age=31536000, immutable` with
  `X-Content-Type-Options: nosniff` and a `Content-Type` from `entityimage.ContentType` (an explicit
  extension map, not `mime.TypeByExtension`, which reads the Windows registry). Read models emit the
  URL already stamped `?v={id}`.
- **Empty slot:** a person role serves a generated placeholder SVG (gender bucket from the enriched
  `gender` field, `max-age=300`). Studio, film and provider-icon routes `404`; the SPA owns that state.

**Rejected:** ETag or short `max-age` revalidation — a round-trip per view where a stamp is free.

Decided in [`0ebf20ef`](https://github.com/whoiskevinrich/holodex/commit/0ebf20ef).

## Entity-generic asset orchestration over per-entity stores

`enrich.Service.downloadAssets` and `enrich.ImageSink` take a leading `entityType`;
`assetRoleFor(entityType, kind)` maps an asset kind to that entity's role and drops an unmapped one.
`internal/imagesink.Sink`, wired once in `main`, dispatches to the person, studio or film repo.
Entities without a gallery return empty `SuppressedAssetURLs` / `ExistingAssetURLs` by construction.
A new image-bearing entity adds a role set and a repo adapter, not an orchestration.

**Rejected:** cloning the person pipeline per entity — duplicates gallery logic others don't use, and
drifts from person's fixes.

Decided in [`1ef23b87`](https://github.com/whoiskevinrich/holodex/commit/1ef23b87).

## Provenance lock: enrichment never overwrites an owner image

The lock is implicit in the `source` column; there is no `locked` column or toggle. A core slot whose
row has an owner source (`upload` or `promoted` for person, `upload` for studio and film) is locked.

`LockedCoreRoles` / `LockedStudioImageRoles` / `LockedFilmImageRoles` return the locked roles;
`downloadAssets` loads the set once per run and skips a locked role **before fetching** (and skips the
person poster seed when poster is locked). The lookup fails open. The rule lives in the enrich path,
not the repo insert, because owner upload and promote must still replace a slot unconditionally.
Deleting the owner image empties the slot and so unlocks it.

**Rejected:** an explicit per-slot lock column — a migration, endpoint and toggle for a need
provenance already answers.

Decided in [`1ef23b87`](https://github.com/whoiskevinrich/holodex/commit/1ef23b87).

## Studio image roles: `icon`, `logo`, `poster` as asset slots

`studio_images` holds three core roles, each single-slot via `UNIQUE (studio_id, role)`; `source` is
`upload` or `enrichment`. There is no gallery, `sort_order`, `promoted` source, suppression store or
content dedup. A studio logo is a provider **asset** (`{kind: "logo"}`) flowing through
`downloadAssets`, not a resolved field; the registry has no studio `logo` field and there is no field
decision over studio images. Owner writes are `POST` / `DELETE /studios/{id}/images/{role}`.

**Rejected:** a cache derived from a resolved `logo` field — a field decision pins a value and
cannot hold owner-supplied bytes.

Decided in [`1ef23b87`](https://github.com/whoiskevinrich/holodex/commit/1ef23b87).

## Person gallery: a bounded append-only role with URL suppression

The person `extra` role is the only multi-image role. `repo.InsertPersonImage` takes a
`PersonImageInsert`; core roles replace their slot and never count against the cap.

- **Cap as a storage bound:** an `extra` insert counts the gallery in its write transaction and
  returns `ErrGalleryFull` at the cap: `PersonGalleryMax` (`PERSON_GALLERY_MAX`, default 20), read
  via `Repo.GalleryCapValue()` and advertised on `/capabilities` as `person_gallery_max`.
- **Over-cap flag:** `PersonImageInsert.OverCap` skips the count. Only the owner-gated upload handler
  sets it (`allow_over_cap`); the enrichment sink never does.
- **Suppression:** deleting an `extra` with a `source_url` records it in `person_image_suppressions`
  in the same transaction; `downloadAssets` skips suppressed URLs before fetching (fails open).
  Core-role deletes never suppress, so an empty core slot can refill.

**Rejected:** suppressing by `provider` + `external_id` — blocks a whole record's imagery, not the one
image deleted.

Decided in [`5e6a72a7`](https://github.com/whoiskevinrich/holodex/commit/5e6a72a7).

## Content dedup by hash of the normalized bytes

`person_images.content_hash` is the hex sha256 of the normalized output (`personimage.Hash`), set on
every ingest path, with a non-unique index on `(person_id, content_hash)`.

- An **enrichment-sourced `extra`** whose hash exists for that person under any role returns
  `ErrDuplicateImage` in the insert transaction; the sink skips it silently. Owner uploads and core
  roles never run the check. Scope is per person, never global.
- **URL fast path:** `downloadAssets` skips a gallery asset whose `source_url` is already stored
  (`ExistingAssetURLs`, fails open) without fetching; the hash catches the same image at a new URL.
- **Backfill:** `personimage.Backfill` runs at boot over unhashed rows and removes duplicate `extra`
  rows (keeping the earliest, or the core image). It never deletes a core image and is idempotent.

**Rejected:** a DB `UNIQUE (person_id, content_hash)` — core refreshes and the poster seed repeat a
hash legitimately, and the outcome wanted is a silent skip. **Rejected:** `source_url` alone — misses
the same image from another provider or size variant.

Decided in [`5f6c3eac`](https://github.com/whoiskevinrich/holodex/commit/5f6c3eac).

## Provider icon store: one row per provider, no FK

`provider_icons` has `UNIQUE (provider)`; a refresh is delete + insert, and `source_url` is the
idempotency key (an unchanged URL is a no-op). Providers are registry entries, not rows, so there is
no foreign key: the refresh pass prunes orphans ([metadata-providers.md](metadata-providers.md)), and
the route resolves by current provider name, so an orphan is never served.

Decided in [`435da9cf`](https://github.com/whoiskevinrich/holodex/commit/435da9cf).

## Studio image halo as a presence table

`studio_image_halo (studio_id, role, mode)`: `WITHOUT ROWID`, composite primary key, CHECKed `role`
and `mode` (`dark`/`light`) enums, `ON DELETE CASCADE` from `studios`. A row means on and absence
means off, so the default needs no backfill. Keyed on studio + role, not the `studio_images` row, the
choice survives a replace. Owner-gated `PUT /studios/{id}/images/{role}/halo` writes it; studio
payloads carry `image_halo`. The server stores both modes; the client picks the one that applies.

**Rejected:** a column on `studio_images` — resets on every replace. **Rejected:** server-side
light/dark classification — duplicates palette logic for a value only CSS consumes.

Decided in [`38f0775f`](https://github.com/whoiskevinrich/holodex/commit/38f0775f).
