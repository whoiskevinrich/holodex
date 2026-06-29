# Spec: People Images (F25)

**Status**: Draft
**Phase**: 3 (Enrichment foundation)
**Depends on**: the thumbnail pipeline ([ADR-009](../architecture/ADR-009-thumbnail-strategy.md)), media-file/asset serving ([ADR-015](../architecture/ADR-015-media-file-serving.md)), the data layout ([ADR-014](../architecture/ADR-014-configuration-and-data-layout.md)), the access-control gating seam ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md)), metadata source plugins / enrichment ([ADR-033](../architecture/ADR-033-metadata-source-plugins.md), F22), and frontend theming ([ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md)).
**Realizes / supersedes**: [Phase 3 F14.3](phase-3-enrichment.md) (person profile image) and the deferred F22 photo-download follow-up — expanded from a single profile image to four image roles, themed/gendered placeholders, and an owner upload path.
**Architecture**: [ADR-038](../architecture/ADR-038-person-images.md) — person image on-disk store, typed real-or-placeholder serving (version-stamped cache), shared ingest normalization, and placeholder resolution. Access reuses the existing owner gate (ADR-030) — no access-model change.
**Design handoff (to be produced)**: `docs/design/people-images-handoff.md` (placeholder artwork set across all three skins; upload/gallery UI; loading/empty/error states).

---

## Objective

Give every **person** a set of images so people read as people across the app — a **1:1 headshot** in
people lists, a **16:9 banner** on the person page, a **2:3 vertical poster** wherever a person appears
on a video, and a **free-form gallery** of extra images on the person page. When a real image is
missing, fall back to a **theme-appropriate, gender-appropriate default placeholder** (gender-neutral
when gender is unknown) so no surface is ever empty or visually broken. Images come from two sources:
**enrichment asset download** (auto-populated from metadata providers) and **owner uploads**
(owner-curated, behind the existing access gate). Each of the three core roles is a single image slot;
the gallery holds up to **20 extra images per person** on top of those.

> **Why this is needed.** People today are name-only rows (`people(id, name)`) auto-created from the
> person tags embedded in media (see [Person Aliases](person-aliases.md)). Every people surface is pure
> text — a list of names, a name header, and name chips on a video. The library looks like a database,
> not a media app. F22 enrichment already *parses* provider asset URLs but
> [deferred actually downloading them](../architecture/ADR-033-metadata-source-plugins.md); this spec
> turns those parsed URLs into displayed images and adds an owner upload path for everything a
> provider doesn't supply.

---

## Scope

### In scope

- **Four image roles per person:**
  - **Headshot — 1:1** (square). Shown in people lists/cards and as the person-page avatar.
  - **Banner — 16:9** (wide). Shown as the hero on the person detail page.
  - **Poster — 2:3** (vertical). Shown wherever a person is represented on a video (video detail page).
  - **Extras — free-form gallery.** Arbitrary additional images, person-page only, no fixed ratio.
- **Themed + gendered default placeholders.** Any unfilled headshot/banner/poster role resolves to a
  built-in placeholder selected by **(active skin × role × gender)**. Gender resolves from **enrichment**
  (a provider-supplied gender field); when unknown, a **gender-neutral** placeholder is used.
- **Admin override of the global placeholder set.** The owner can replace the built-in default artwork
  (the whole `skin × role × gender` matrix) with their own; the override applies to *every* person that
  lacks a real image. (This is a global fallback swap, **not** a per-person override.)
- **Two image sources:**
  - **Enrichment asset download** — activate the deferred F22 asset path: fetch image URLs returned by
    metadata providers (subject to the existing SSRF allowlist, redirect refusal, and response caps in
    `internal/enrich`) and store them as person images.
  - **Owner upload** — the **owner**, behind the existing access gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md)),
    can upload an image and assign it a role: fill/replace one of the three core slots, or add to the
    gallery. (Uploads are owner-curated; non-owner viewers do not upload — see §9.)
- **Core roles are single-slot.** Each person has at most one `headshot`, one `banner`, and one
  `poster`; uploading (or downloading via enrichment) for a filled core role **replaces** the current
  image for that role.
- **The gallery is capped at 20 `extra` images per person**, on top of the three core slots (so a person
  can hold up to 23 images total). Adding a 21st extra is rejected with a clear message.
- **Server-side image normalization** on ingest (decode-and-re-encode, strip metadata, bound
  dimensions/bytes) for both upload and enrichment-downloaded images.
- **Typed serving routes** that always return a usable image (real image, else the resolved placeholder),
  mirroring the thumbnail route contract (cacheable, no client-supplied filesystem paths).
- **Themed display across all three skins** (Cinémathèque, Broadcast, Brutalist) using semantic design
  tokens only; placeholders and image frames honor the active skin.

### Out of scope (tracked follow-ups, not gaps)

- **Per-person placeholder override** — admins override the *global* placeholder set, not an individual
  person's fallback. (A specific person gets a real image by upload/enrichment instead.)
- **An owner-editable gender field.** Gender is read from enrichment only; there is no manual gender
  attribute in v1. People with no enriched gender always get the neutral placeholder.
- **Server-side / automatic cropping — face detection, smart-crop.** Direct uploads and
  enrichment-downloaded images are stored normalized and displayed with CSS `object-fit: cover` at each
  target ratio; the owner picks the role and is responsible for a reasonable source crop. The **one**
  in-app crop is the manual **zoom/crop on promote** (F25.15), where an arbitrary-ratio gallery extra
  must be fitted to a fixed core ratio — that is a client-side, owner-driven crop, not automatic.
- **Non-owner / contributor uploads.** Uploads are **owner-only** in this spec (reverted from an earlier
  open-contribution model). Letting viewers contribute images would be a separate access-model change
  (its own ADR + security review) and is out of scope here.
- **Tag images** (F15.3) — same storage/serving model, separate feature.
- **MCP exposure** of person image URLs (`get_person` / `list_people`) — mirrors the deferred F22.5f /
  F14.5 MCP-parity follow-ups.
- **Animated images / video avatars, EXIF-derived attribution, broader in-app editing** (rotate, filters,
  crop-on-direct-upload). The only in-app editing in v1 is the zoom/crop on **promote** (F25.15).

---

## Personas

- **Owner / admin** — runs the instance, curates the library. The only mutator (ADR-030 owner gate):
  uploads, assigns roles, reorders/deletes images, and swaps the global placeholder set.
- **Viewer** — a non-owner visitor to the (publicly viewable) instance. **Read-only**: sees images and
  placeholders but does not upload or modify anything.
- **Metadata provider (system actor)** — an F22 sidecar provider that returns asset URLs the system
  downloads.

---

## User stories

Ordered by priority.

1. **As a viewer browsing the people list, I want each person to show a headshot** so I can recognize
   people at a glance instead of scanning a wall of names.
2. **As a viewer on a person's page, I want a banner and a clean profile layout** so the page feels like
   a real profile, not a bare list of videos.
3. **As a viewer on a video page, I want each credited person shown as a poster card** so I can see who's
   in it and click through.
4. **As a viewer, when a person has no real image, I want a tasteful placeholder that matches the current
   skin and the person's gender** so the layout never looks broken or empty.
5. **As the owner, I want to upload an image for a person and choose which role it fills** so I can
   improve the library when a provider didn't supply art.
6. **As the owner, I want to add extra images to a person's gallery** so notable people can have more
   than the three core images (up to 20 extras).
7. **As the owner, I want enrichment to auto-download provider images** so well-known people get art
   without my uploading anything.
8. **As the owner, I want to replace the built-in placeholder artwork with my own set** so unfilled
   slots match my instance's look across all three skins.
9. **As the owner, I want to delete any image** so I can remove anything wrong or low-quality.
10. **As the owner, when I hit the 20-extra gallery limit, I want a clear message** so I understand why
    the upload was rejected.

---

## Functional requirements

### Must-have (P0)

| ID | Requirement | Acceptance criteria |
|----|-------------|---------------------|
| F25.1 | A person has images, each with a **role** (`headshot` \| `banner` \| `poster` \| `extra`) and a normalized stored asset. Core roles are single-slot; `extra` is a multi-image gallery. | Given a person with one `headshot` image, `GET` its headshot route returns that image; the role is queryable on the person read model. |
| F25.2 | **Headshot (1:1)** renders on the people list cards and as the person-page avatar. | Given a person with a headshot, the `/people` card and the person header show it cropped to a square; given none, both show the resolved placeholder. |
| F25.3 | **Banner (16:9)** renders as the person-page hero. | Given a banner, the person page shows a 16:9 hero; given none, it shows the resolved 16:9 placeholder. |
| F25.4 | **Poster (2:3)** renders for each person on the video detail page. | Given a video with people, each person appears as a 2:3 poster card linking to the person; missing posters show the placeholder. |
| F25.5 | **Placeholder resolution** picks `(active skin × role × gender)`; gender comes from the enriched gender field, defaulting to **neutral** when absent. | Switching skins changes the placeholder; a person with enriched `gender=female` shows the female placeholder; a person with no enriched gender shows the neutral one. |
| F25.6 | **Typed serving route per role** returns the real image if present, else the resolved placeholder; never 404s for a valid person+role; never accepts a client-supplied filesystem path. Real-image URLs are **version-stamped** (`?v=<image_id>`) so a replace busts caches immediately. | `GET /api/v1/people/{id}/image/{role}` returns 200 with an image for any existing person and valid role; an unknown role → 400; an unknown person → 404. A real-image response carries a `?v=` stamp and a long `Cache-Control: public, max-age=…, immutable`; after a replace the read-model emits a new `?v=`. |
| F25.7 | **The owner can upload** an image for a person and assign its role; the image is normalized server-side before storage. Uploading again for a **filled core role replaces** the current image (old asset cleaned up). Upload is behind the owner gate (ADR-030); non-owners cannot upload. | An owner POST with a valid image stores a normalized asset (re-encoded, metadata stripped, dimensions/bytes bounded) and it appears on the relevant surface; a second upload for a filled core role swaps it with no orphaned files; a non-owner POST is rejected by the gate. |
| F25.8 | **Gallery cap on `extra` images per person** — default **20**, configurable via `PERSON_GALLERY_MAX` (core slots are separate and never counted; see F25.23–25). | An over-cap `extra` for a person is rejected with a clear, themed error; the cap is enforced server-side regardless of client. **Filling/replacing a core role (headshot/banner/poster) is never blocked by the gallery cap** — proven by repo + API tests inserting a core role at a full gallery (the F25.8 bug fix). |
| F25.9 | **Upload validation**: only real raster images of an allowed type and within size/dimension bounds are accepted; the bytes are decoded to confirm they are an image (not a polyglot/renamed file). | A non-image, oversized, or malformed file is rejected with a clear error and nothing is written to disk. |
| F25.10 | **Enrichment asset download**: provider-supplied image URLs are fetched (through the existing enrich SSRF allowlist + redirect refusal + response caps), normalized, and stored as person images with provenance. | After enriching a person whose provider returns an asset, the corresponding core role shows the downloaded image (replacing any current one for that role); the fetch obeys the F22 network guards. |
| F25.11 | **Owner can delete any image** (a core-role image or a gallery extra). | An owner delete removes the asset and its DB row; a deleted core-role image leaves that role empty (placeholder resolves); a deleted extra leaves the gallery; a non-owner cannot delete. |
| F25.12 | **All three skins** render every people image surface and every loading/empty/error state correctly, using semantic tokens only (no hardcoded palette/radii/fonts). | QA in Cinémathèque, Broadcast, and Brutalist: lists, person page, video page, gallery, and placeholders all read correctly; `rg 'zinc-\|sky-\|emerald-\|amber-\|rounded-(lg\|md\|sm\|xl)'` over new components is empty. |

### Nice-to-have (P1)

| ID | Requirement | Acceptance criteria |
|----|-------------|---------------------|
| F25.13 | **Free-form gallery** on the person page: owner adds/removes/reorders extra images. | Extras render in an ordered gallery on the person page only; not shown on lists or video pages. |
| F25.14 | **Admin override of the global placeholder set** — owner supplies replacement artwork for the `skin × role × gender` matrix; built-ins are the fallback when no override exists. | After the owner installs an override for `(brutalist, banner, neutral)`, every person lacking a banner shows the override under the Brutalist skin; other cells keep the built-in. |
| F25.15 | **Promote a gallery extra into a core slot.** The owner picks a gallery image, **zooms/crops** it to the target role's aspect ratio in a client-side editor, and saves it as a **new core image — a copy**; the gallery original is left untouched. The cropped copy is normalized server-side like any upload. | Promoting an `extra` as `poster` opens a 2:3 crop tool; saving creates a `poster` (replacing any current one) from the cropped copy; the original `extra` still appears in the gallery; no orphaned files remain. |
| F25.16 | **Lazy / progressive load** of images on grids; the placeholder shows immediately while a real image loads (no layout shift). | Scrolling the people list shows no broken-image flashes and no cumulative layout shift as images resolve. |
| F25.17 | **Activity history** records image events (uploaded / downloaded-via-enrichment / deleted), consistent with F21/ADR-028. | Uploading and deleting an image each appear in the activity surface. |

### Future considerations (P2)

| ID | Requirement | Notes |
|----|-------------|-------|
| F25.18 | Extend the crop editor to **direct upload** (and add rotate) — not just promote. | v1 ships zoom/crop on promote only (F25.15); applying it to every upload avoids "uploader must pre-crop". |
| F25.18b | A distinct **nonbinary placeholder** art bucket (→ 36 assets across skins/roles). | v1 collapses nonbinary→neutral art while storing the true value; a 4th art bucket is purely additional artwork. |
| F25.19 | Non-owner / contributor uploads (open contribution) with moderation queue, reporting, and identity-keyed rate limits. | A separate access-model change (own ADR + security review); reverted out of this spec. |
| F25.20 | MCP `get_person` / `list_people` expose image URLs. | Mirrors deferred F22.5f / F14.5. |
| F25.21 | Tag images reuse this storage/serving/placeholder model (F15.3). | Same machinery, different entity. |
| F25.22 | Owner-editable gender (and other person attributes) feeding placeholder selection when enrichment has none. | Removes the "neutral-only when unenriched" limitation. |

---

## Addendum — configurable cap, owner override & enrichment suppression ([ADR-043](../architecture/ADR-043-gallery-cap-and-enrichment-suppression.md), 2026-06-25)

A follow-up slice that hardens the gallery cap and gives the owner control over it,
plus a "don't bring back what I deleted" guarantee for enrichment. Motivated by a
bug report ("setting the headshot/poster at a full gallery errored with *gallery
full*") — which on investigation was **already** correct in the backend (core roles
skip the cap), so the fix is regression coverage + a non-alarming UI for the full
state (F25.8 acceptance, and the banner re-tone below).

| ID | Requirement | Acceptance criteria |
|----|-------------|---------------------|
| F25.23 | **Configurable gallery cap.** The per-person `extra` cap is set by `PERSON_GALLERY_MAX` (default 20, env/yaml `person_gallery_max`); a non-positive value falls back to 20. The effective cap is advertised to the SPA via `/capabilities` (`person_gallery_max`) so the UI warns at the right number. | Setting `PERSON_GALLERY_MAX=3` makes the 4th `extra` the one rejected; `/capabilities` reports `3`; the gallery's "full" note reads "3 of 3". |
| F25.24 | **Owner over-cap override.** The owner-gated upload accepts `allow_over_cap`; when set, the insert bypasses the cap (no 409). **Enrichment never sets it** — provider-driven growth stays bounded. The default UI still warns at the cap and offers an explicit **"Add anyway"** action that sets the flag. | At a full gallery, a normal `extra` upload → 409; the same upload with `allow_over_cap=true` → 201; an enrichment run never exceeds the cap. |
| F25.25 | **Enrichment URL suppression on delete.** Deleting a gallery `extra` that arrived from a provider records its source asset URL in a per-person suppression list; a later re-enrich **skips** that URL so the deleted image is not silently re-added. Core-role deletions do **not** suppress (a re-enrich may legitimately refill an empty headshot/banner/poster). Owner uploads have no source URL and never suppress. | Enrich a person (stores a gallery image from URL *U*); delete it; re-enrich → *U* is not re-fetched/stored. Deleting an enrichment **headshot** does not suppress. |

**Data/architecture (ADR-043).** Migration `0012` adds `person_images.source_url`
(empty for uploads/promotes) and a `person_image_suppressions(person_id, source_url,
created_at)` table (composite PK, `ON DELETE CASCADE`). `InsertPersonImage` takes a
`PersonImageInsert` struct (adds `SourceURL` + `OverCap`); `DeletePersonImage`
records suppression in the same write transaction; `downloadAssets` consults the
per-person suppressed set (fail-open) before fetching each asset.

**Frontend.** The full-gallery state is **informational, not an error** — neutral
tokens (`text-muted`, `border-rule`, `bg-surface-2`), never `text-warn`; a full
gallery is a status, not a failure of the action just taken. The "Add anyway" button
issues the over-cap upload. Suppression is entirely server-side (no UI).

---

## Addendum — owner-set core images take precedence over enrichment ([ADR-049](../architecture/ADR-049-manual-image-precedence.md), F33, 2026-06-28)

The sibling of F25.25's delete-suppression: where F25.25 keeps a *deleted* provider
image deleted, this keeps an *owner-set* core image from being overwritten. A person's
core slots (headshot/banner/poster) already record `source ∈ {upload, promoted,
enrichment}`; enrichment must yield to the first two and only ever (re)fill or refresh
its own. No migration, no new config — the rule reads off the existing `source` column.

| ID | Requirement | Acceptance criteria |
|----|-------------|---------------------|
| F25.31 | **Manual core images are never overwritten by enrichment.** A core slot whose current image has `source` `upload` or `promoted` is **locked**: an enrich/re-enrich skips the provider asset for that role (before fetching it) and does not seed a poster over a locked poster. An empty or `enrichment`-sourced slot is **not** locked — it still fills/refreshes. To let a provider image replace a manual one, the owner deletes theirs first. The lock lookup **fails open** (a query error locks nothing rather than blocking enrichment). Gallery `extra` is append-only and unaffected. | Upload a headshot (source=upload), then enrich a person whose provider returns a headshot + banner + gallery image, with the headshot owner-set → the headshot is kept, banner/gallery flow. Promote a gallery image to poster, then re-enrich → poster kept. An `enrichment` headshot is replaced on re-enrich (refresh). With only a portrait returned and the poster owner-set, the headshot seed does **not** overwrite the poster. |

**Data/architecture (ADR-049).** New read `repo.LockedCoreRoles(personID)` returns the
core roles with `source IN ('upload','promoted')` (one indexed query). Exposed to the
enrich package via `ImageSink.LockedCoreRoles` (passthrough on `personimage.Sink`).
`enrich.downloadAssets` loads the locked set once per run alongside the suppressed set
and skips a locked core role before fetching its bytes; the poster auto-seed (F25.29) is
skipped when poster is locked. The owner upload/promote write paths are unchanged (they
must replace a core slot) — the precedence lives only in the enrichment orchestration.

**Frontend.** No UI required for correctness (the protection is automatic, server-side).
Deferred follow-up: an owner-view provenance badge ("Yours" / "from {provider}") on the
core-image controls so the lock is legible — reuses the existing `ProvenanceBadge`
vocabulary; and an explicit per-slot pin to also freeze a *provider* image (ADR-049
"Alternatives").

---

## Image model — roles, ratios, surfaces

| Role | Ratio | Primary surface(s) | Placeholder? |
|------|-------|--------------------|--------------|
| `headshot` | 1:1 | People list cards; person-page avatar | Yes — `skin × 1:1 × gender` |
| `banner` | 16:9 | Person-page hero | Yes — `skin × 16:9 × gender` |
| `poster` | 2:3 | Video detail page (person cards) | Yes — `skin × 2:3 × gender` |
| `extra` | any | Person-page gallery **only** | No (gallery simply omits absent extras) |

**Placeholder matrix.** 3 skins × 3 core roles × 3 gender buckets (`male`, `female`, `neutral`) = **27
built-in placeholder assets**. The neutral bucket is the default and the fallback for every unknown or
unmapped gender value. Placeholder selection is deterministic and computed at serve/display time —
storing a placeholder against a person is not allowed (placeholders are never "real" images and never
occupy a core slot or count against the gallery cap).

**Gender source & vocabulary.** Gender comes from enrichment only — a new canonical field
`gender` ([metadata-provider-contract §4.2](metadata-provider-contract.md), added by this feature; see
§"Artifacts"). The provider maps its **native** values onto a small canonical string vocabulary —
`male` | `female` | `nonbinary` | `unknown` (omit when unknown, per the contract's "omit, don't empty")
— so Holodex never sees raw provider codes (e.g. a TMDB sidecar translates TMDB's integer gender codes
`1→female`, `2→male`, `3→nonbinary`, `0→omit`). The **true value is stored faithfully** in
`entity_enrichment` with full provenance.

Placeholder **art** has only **three buckets** — `male`, `female`, `neutral`. Mapping value → art bucket:
`male→male`, `female→female`, and **both `nonbinary` and unknown/absent → `neutral`**. (We collapse art
to three variants without erasing the underlying data; a distinct nonbinary placeholder is a P2 art
follow-up, F25.18b.) No gender is persisted on the `people` row in v1.

---

## Data, storage & serving (direction — finalized in the ADR)

These mirror the thumbnail pipeline ([ADR-009](../architecture/ADR-009-thumbnail-strategy.md)) and
data-layout ([ADR-014](../architecture/ADR-014-configuration-and-data-layout.md)) conventions; exact
column/route names are settled in the new ADR.

- **DB**: a `person_images` table — `id`, `person_id` (FK), `role`, `source` (`upload` | `enrichment`
  | `promoted`), `provider`/`external_id` (nullable, for enrichment provenance), `sort_order` (for
  gallery ordering), `created_at`. A uniqueness constraint enforces **one image per core role per
  person** (replace-on-reupload); the **20-extra gallery cap** is enforced with a counted insert on
  `role = extra`. A core image's `id` doubles as its cache-version stamp (replace ⇒ new row ⇒ new `id`
  ⇒ new `?v=`). Deleting a person cascades.
- **Disk**: under the existing data dir, e.g. `DATA_PATH/person-images/{person_id}/{image_id}.{ext}` —
  the disk file is the cache, as with thumbnails. Filenames are server-assigned; **no client-supplied
  path ever touches the filesystem**.
- **Placeholder override store**: owner-supplied override artwork lives in a configurable dir keyed by
  `{skin}-{role}-{gender}` with the embedded built-ins as the fallback when a cell is absent.
- **Serving**: typed routes only —
  - `GET /api/v1/people/{id}/image/{role}` → resolved real-or-placeholder image (the primary contract).
  - gallery list + per-image fetch for extras.
  - **Versioned caching**: real images serve `Cache-Control: public, max-age=…, immutable` and the
    read-model hands out the URL with a `?v=<image_id>` stamp, so a replace is picked up instantly while
    every stamped URL stays long-cacheable. Placeholders cache long and bust via a placeholder-set
    version. 404-contract discipline as `serveThumbnail`.
- **Read model**: the person payload gives the frontend, per role, whether a real image exists and the
  current `?v=` stamp (and the ordered gallery image ids) — without leaking filesystem detail.

---

## Access control & security

Uploads are **owner-only**, so there is **no access-model change** — they reuse the existing owner gate
([ADR-030](../architecture/ADR-030-access-control-gating-seam.md)), the same choke point as enrichment
and aliases. Viewing images is public (public metadata), as elsewhere. A `/security-review` is still
warranted before merge because the change ingests binary files and serves them, but the surface is much
smaller than open contribution.

- **Mutations behind the owner gate.** Upload, role assignment, reorder, delete, and placeholder-set
  override all go through `requireOwner`. Non-owners get read-only image/placeholder serving.
- **Harden image ingest (still required, even owner-sourced):**
  - **Decode-and-re-encode every ingested image** (upload *and* enrichment download); strip all metadata
    (EXIF/ICC/etc.); bound max dimensions and output bytes; output a single safe format. Never trust the
    declared content-type or extension.
  - **Reject non-images and polyglots** by attempting a real decode before writing anything.
  - **Enforce the 20-extra gallery cap server-side** as a product rule (and a disk-growth bound); core
    slots are bounded to one image each by construction.
  - **Enrichment downloads reuse the F22 guards**: SSRF allowlist, refuse cross-host redirects, response
    size caps, timeouts — these matter most since provider URLs are the only externally-influenced input.
  - **Serve from typed routes only** with server-assigned filenames; never echo a client path; force the
    correct image content-type with no HTML sniffing so stored bytes can't be served as executable content.
- **Privacy.** Stripping metadata on ingest prevents leaking EXIF/GPS from owner-supplied or
  provider-supplied images. No uploader PII is stored (owner identity is already known to the system).

### Pre-implementation threat model (design security review, 2026-06-16)

A STRIDE-style pass over the new surface. Each control is a hard requirement the post-implementation
`/security-review` verifies in code.

| # | Threat | Vector | Required control |
|---|---|---|---|
| T1 | **Malicious upload → RCE/XSS** | Polyglot / renamed non-image / SVG-with-script as upload | Sniff-decode with the stdlib decoder; **reject on decode failure**; re-encode to JPEG (drops any script/markup); **never accept SVG** as an uploadable image; serve with an explicit raster `Content-Type` + `X-Content-Type-Options: nosniff` |
| T2 | **Path traversal / overwrite** | Crafted role/filename/person id in the upload or serving path | Filenames are **server-assigned from the row id**; `role` is validated against the fixed enum; person id is an int from the route; no request value ever concatenated into a filesystem path |
| T3 | **SSRF via enrichment asset URL** | Provider returns `http://169.254.169.254/…` or a redirect to an internal host | Reuse F22 guards verbatim: **allowlist**, **refuse cross-host redirects**, response-size cap, timeout; fetch only over the provider's configured base where applicable; the asset host must pass the same allowlist |
| T4 | **Decompression bomb / OOM** | Tiny file, enormous pixel dimensions; or huge upload body | `MaxBytesReader` on the request; **check declared dimensions/pixel-area before full decode**; bound decoded dimensions and re-encoded byte size |
| T5 | **Disk-fill DoS** | Many uploads / oversized images | 20-extra gallery cap (per person); per-image byte bound; (owner-gated, so attacker must already hold the token) |
| T6 | **Unauthorized mutation** | Non-owner hits upload/delete/promote/override | All mutations mounted behind the existing `requireOwner` group (ADR-030); constant-time token compare inherited; viewing stays public |
| T7 | **Stale/wrong image served** | Cache pins a replaced image | Version-stamped (`?v=image_id`) immutable URLs; replace yields a new id ⇒ new URL |
| T8 | **Metadata/PII leak** | EXIF GPS in an uploaded or downloaded photo | Re-encode strips all metadata as a side effect (T1's pipeline) |

Residual/accepted: with `ADMIN_TOKEN` unset the gate is open by design (ADR-030's documented single-user
posture) — upload is then as open as every other mutation, unchanged by this feature. No new secrets or
PII introduced.

---

## Frontend / theming requirements

- **Tokens only.** All new components use semantic utilities (`bg-surface`, `text-ink`, `text-muted`,
  `border-rule`, `rounded-theme`, `font-display`/`font-ui`, `text-warn`/`border-warn` for errors) — no
  literal palette, hex, named fonts, or fixed radii. Skin-specific image-frame flourishes belong in
  `app.css` gated by `[data-theme]` on shared hook classes (e.g. `.video-frame`-style), not per-component.
- **Reuse `EntityVideos`, `ProvenanceBadge`** and existing person-page structure; add an image
  frame/avatar component and a gallery component.
- **QA all three skins** for every state: loading, empty (placeholder), error (upload rejected/over cap),
  and the populated grid — regressions routinely show in only one skin.
- **Provenance**: enrichment-sourced images carry the F22 provenance badge; owner-uploaded images are
  distinguishable from provider images where it matters (e.g. in the owner's delete view).

---

## Success metrics

**Leading (days–weeks):**
- **Coverage** — % of people (weighted by video count) with a real headshot. Target: a real headshot for
  the **top 50 most-credited people within 1 week** of enabling enrichment download.
- **Placeholder correctness** — 0 broken-image / empty-frame states across all three skins in QA (hard
  gate, not a trend).
- **Upload success rate** — % of upload attempts that succeed vs. fail validation; a high *malformed*
  reject rate flags a confusing UI or an abuse probe.

**Lagging (weeks–months):**
- **People-page engagement** — click-through from people lists and from video person-cards, before vs.
  after (does showing faces increase navigation into people?).
- **Library completeness** — images added over time (owner upload vs. enrichment download) and how much
  of the people set ends up with real art vs. placeholders.
- **Storage growth** — disk used by person images vs. the per-person envelope (3 core + 20 extras);
  validates the cap and any storage bound.

Measurement: instrument via the existing activity/metrics surfaces (F21, ADR-026/028).

---

## Open questions

None outstanding. The three that shaped this spec are resolved below.

### Resolved decisions

- **[data] Enrichment `gender` field & vocabulary** → new canonical field `gender` with values
  `male`/`female`/`nonbinary`/`unknown` (provider maps its native codes; value stored faithfully).
  Placeholder art has three buckets — `nonbinary` and unknown both resolve to `neutral`. *(See
  "Gender source & vocabulary"; requires adding `gender` to the provider contract — see "Artifacts".)*
- **[design] Promote a gallery extra into a core slot** → **yes**, ships with the gallery slice. The
  promoted image is a **copy** (gallery original untouched) and the owner **zooms/crops** it to the
  target ratio in a client-side editor. *(F25.15.)*
- **[ops] Cache invalidation on replace** → **versioned URLs** (`?v=<image_id>`) with long `immutable`
  caching; a replace yields a new id → new URL → instant bust. *(F25.6; serving section.)*

---

## Timeline & phasing

The feature is too large for one slice; ship in order, each independently valuable. All writes are
owner-gated throughout — there is no access-model change to sequence around.

1. **Slice 1 — Storage, serving & placeholders (P0 core).** `person_images` table + disk layout + typed
   routes + the 27 built-in themed/gendered placeholders + display on all three surfaces. Owner upload +
   delete with ingest hardening. Delivers the visual win immediately. *(Realizes F14.3 fully.)*
2. **Slice 2 — Enrichment asset download.** Activate the deferred F22 asset path → auto-populate the top
   people. Pure system actor; reuses existing network guards.
3. **Slice 3 — Gallery + 20-extra cap + promote-with-crop** (P1). The free-form extras gallery
   (add/remove/reorder), the 20-extra cap enforcement, and the **promote-a-gallery-extra-into-a-core-slot**
   flow with client-side zoom/crop (F25.15). (Core-slot replace-on-reupload itself ships with slice 1.)
4. **Slice 4 — Admin global placeholder override** (P1 polish).

**Dependencies / gates:**
- A **`/security-review`** sign-off before slice 1 merges (binary file ingest + serving, even though
  owner-sourced).
- A **design handoff** (placeholder artwork across all three skins + upload/gallery UI) blocks the
  visual slices.

---

## Artifacts to produce (project working agreements)

- [ ] This spec (`docs/specs/people-images.md`) — **done** (draft).
- [x] **ADR**: [ADR-038](../architecture/ADR-038-person-images.md) — storage, serving, ingest
      normalization, placeholder resolution. Access reuses ADR-030; no access-model change.
- [x] **Design handoff**: [people-images-handoff.md](../design/people-images-handoff.md) + system pattern
      [people-images-design-system.md](people-images-design-system.md) (`.portrait-frame`, components,
      placeholder system, all states ×3 skins). QA checklist tracked below.
- [ ] **Provider-contract update**: add `gender` as a canonical person field in
      [metadata-provider-contract.md §4.2](metadata-provider-contract.md) (value vocabulary
      `male`/`female`/`nonbinary`/`unknown`) with a label + precedence entry per ADR-013/ADR-033, and
      note it in the [TMDB provider spec](tmdb-provider.md) (native code → canonical mapping).
- [x] **Testing strategy** — F25 block added to [testing-strategy.md](../testing-strategy.md) §9
      (ingest normalization, placeholder resolution, repo/API/enrichment/frontend).
- [ ] **Security review** before slice 1 merges — binary file ingest + serving (`/security-review`).
- [ ] Cross-reference from [Phase 3 enrichment spec](phase-3-enrichment.md) F14.3 and the
      [metadata-plugins spec](metadata-plugins.md) (asset download) to this spec; update the ADR index.

---

## F25.26–30 — Person-page polish (follow-ups)

Post-F25 refinements to the person hero and people list (F25.26–28, F25.30 are UI-only, tokens-only, on
the existing `.portrait-frame` seam — ADR-038; F25.29 adds a list-payload field + an enrichment-ingest
policy, no access change). Design handoff +
QA: [`person-page-polish-handoff.md`](../design/person-page-polish-handoff.md) ·
[`person-page-polish-qa-checklist.md`](../design/person-page-polish-qa-checklist.md).

- **F25.26 — Taller parallax banner.** The hero banner band goes from **5:1 (≤270px)** to **5:2
  (≤540px)**; its image drifts opposite to page scroll (depth) via a `--banner-shift` CSS variable updated
  by a passive, rAF-throttled scroll listener in `PersonBanner` (the image's overflow-hidden frame makes a
  pure-CSS `view()` timeline inert). Respects `prefers-reduced-motion`; collapses to a static cover crop
  where motion is off or frames aren't produced. `.crop-frame--banner` is kept at **5:2** in lockstep so
  the crop preview matches the rendered hero. (Supersedes the "16:9 banner" wording in the Objective —
  the realized banner ratio is the wide 5:N band, now 5:2.)
- **F25.27 — Poster on the person page.** The 2:3 **poster** now renders inline in the person hero (it
  was previously only on the video credits surface). Visitors see it only when present; the owner always
  sees the slot with a `Replace` overlay (replacing the old standalone "Replace poster" button).
- **F25.28 — People-list scroll restoration.** Returning from a person detail page to `/people` restores
  the prior scroll position (module-scoped, sort-keyed, one-shot cache — the ADR-032 browse pattern
  applied to the people list). New module `web/src/lib/peopleScroll.svelte.ts`.
- **F25.29 — Post-enrichment image freshness.** Two QA fixes so newly-enriched people read correctly
  without a manual refresh:
  - **List headshot cache-bust.** The people-list avatar URL carried no `?v=`, so the browser's
    `immutable` cache (and the 5-min placeholder cache) kept showing the stale image after enrichment.
    `GET /api/v1/people` now returns `headshot_version` per person (the headshot image id; `0`/absent =
    none), and the list avatar passes it as the `?v=` cache-buster — so the URL changes when the headshot
    does and the browser fetches fresh on Back. Mirrors the detail page, which already versioned its
    avatar. (`model.Person.HeadshotVersion`, a people-specific `ListPeople` subquery, `Person.headshot_version`.)
  - **Auto-seed the poster from the headshot.** Provider profiles (e.g. TMDB) are 2:3 portraits — a
    natural poster — but enrichment only filled the headshot, leaving posters empty and video-credit
    surfaces drab. Enrichment now seeds the poster from the **already-fetched** headshot bytes (no extra
    download) when the run filled a headshot but **no** poster and the poster slot is empty. Never
    overwrites an existing owner/provider poster; like other core roles it refills on re-enrich (core
    deletes don't suppress — ADR-043 F25.25). (`enrich.downloadAssets`, `ImageSink.StoreAssetIfAbsent`.)
- **F25.30 — Banner only when set.** Not every person has a banner-sized image, and the placeholder
  banner band — a 5:2 hero up to 540px tall — dominates the page with generic art when none exists. The
  hero banner now renders **only when a real banner image is present** (`images.roles.banner?.present`),
  for **everyone including the owner** — mirroring the F25.27 poster rule that "visitors see it only when
  present." This supersedes F25.3's "given none → resolved 16:9 placeholder" *for the person-page hero*:
  the banner placeholder is no longer shown there (placeholder resolution for the banner role still exists
  in the serving layer and is unchanged).
  - **No-banner layout (clean top, no overhang).** With no band above it, the headshot + name row no
    longer overhangs the banner (the `-mt-10`/`-mt-12` negative top margin is conditional on a banner
    being present). When there's no banner, the row sits flush at the top of the hero with normal spacing,
    so the headshot never pulls up into empty space.
  - **Owner add-banner affordance.** Because the banner band was the owner's only entry point for setting
    one, hiding it for the owner removes that path. When no banner is present, the owner sees an explicit
    **"Add banner"** control in the hero that reuses the existing core-slot upload (`pickCore('banner')`)
    — the same path the old "Edit" overlay invoked. Once a banner exists it shows normally with the
    `Replace` overlay as before; owners can still clear it via the gallery/delete path (F25.11), which
    returns the page to the no-banner state.
