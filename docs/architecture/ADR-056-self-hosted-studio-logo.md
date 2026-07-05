# ADR-056: Self-hosted studio logo — derived, normalized cache of the resolved `logo` field

**Status:** Proposed
**Date:** 2026-07-04
**Deciders:** Project owner

**Relates to:** [ADR-038](ADR-038-person-images.md) (person images — the on-disk store + untrusted-bytes normalize spine this reuses; studio logo is the **single-slot, enrichment-only** analogue) · [ADR-039](ADR-039-provider-asset-urls.md) (provider asset URLs — the `asset_hosts` allowlist + redirect/scheme/size guards the logo download passes through) · [ADR-053](ADR-053-studio-entity-and-resolved-link-derivation.md) (studio entity — the logo cache is **derived from the resolved `logo` field** and re-synced on the same enrich/decision triggers, exactly as `video_studios` is derived from the resolved `studio` field) · [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (per-field decisions — the `logo` field, its record-first default, and its provider/blank-pin decision stay unchanged; this ADR only changes how the *chosen* URL is stored and served) · [ADR-033](ADR-033-metadata-source-plugins.md) (metadata plugins — no contract or provider change). **Spec:** [Studio as a first-class entity / F38](../specs/studio-entity.md) (realizes deferred **P2-3**). **Ticket:** HOLODEX-130.

---

## Context

F38 (ADR-053) made `studio` a first-class entity and shipped a `logo` field: a canonical
`image_url` enrichment field whose value is a URL. The TMDB sidecar emits it as a plain field —
`fields["logo"] = "https://image.tmdb.org/t/p/original" + logo_path` (`providers/tmdb/tmdb.go`) —
and the SPA renders it directly: the `/studios` list logo-well (`<img src={s.logo_url}>`,
HOLODEX-126) and the detail Details section both point an `<img>` at the **raw provider CDN URL**.

That works, but it is the one image surface in Holodex that **bypasses the F25/ADR-038 hardening
every person portrait gets.** A hotlinked provider URL means:

- **Viewer IP leaks to the provider.** Every visitor's browser fetches `image.tmdb.org` directly,
  so the provider sees the library's whole audience — a privacy regression relative to person
  images, which are served from Holodex's own origin.
- **No metadata strip, no bomb guard, no sniff-decode.** The bytes never pass `personimage.Normalize`
  (re-encode-to-JPEG that drops EXIF/ICC and rejects SVG/polyglots/decompression bombs). Holodex
  vouches for an image it never inspected.
- **Hotlink fragility.** A rotated CDN path or a provider that blocks hotlinking silently breaks the
  logo; there is no local copy.

The person subsystem already solves all three — download once through the SSRF-guarded `AssetClient`
(ADR-039), `Normalize`, store on disk, serve from a typed route with an immutable cache. F38's own
spec listed **P2-3 "Studio logo in the image store (F25 generalization)"** as the deferral that
closes this gap. HOLODEX-130 is that deferral.

**The scope tension.** Person images (ADR-038) are a *multi-role* subsystem: headshot/banner/poster
core slots plus an unbounded gallery, with owner uploads, promote/reorder, per-URL delete-suppression
(ADR-043), manual-slot locking (ADR-049), and content-hash dedup (ADR-050). A studio needs **exactly
one image — a logo** — and has no upload UI, no gallery, and no second slot. Cloning the whole
subsystem for one image would be a large, mostly-dead surface (and a large security-review surface).
The maintainer chose the **minimal** direction: self-host the one logo, reuse the person normalize
spine and the ADR-039 asset perimeter verbatim, and add none of the multi-role machinery.

## Decision

### 1. The studio logo is a derived, self-hosted cache of the resolved `logo` field — not a new curation surface

The `logo` **field** is unchanged: still a canonical `image_url` replace field, still record-first
with a standing provider/blank-pin decision (ADR-051), still emitted by the provider as a URL. What
changes is **downstream of resolution**: Holodex maintains an on-disk, normalized copy of whatever
URL the `logo` field currently **resolves** to, and serves *that* copy. The provider contract, the
`/describe` field list, the decision/curation endpoints, and the sidecar all stay exactly as F38
shipped them. **Zero provider or contract change.**

This mirrors ADR-053's central move — `video_studios` is a derived index over the resolved `studio`
field, re-synced by `RelinkVideoStudios` on every write that can change that value. The studio logo
cache is the same shape one layer over: a derived index over the resolved `logo` field, re-synced by
**`RelinkStudioLogo`** on every write that can change *that* value.

### 2. Data model — one logo per studio (migration 0019)

```sql
CREATE TABLE studio_logos (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    studio_id   INTEGER NOT NULL UNIQUE REFERENCES studios(id) ON DELETE CASCADE,
    source_url  TEXT    NOT NULL,            -- the resolved logo URL this cache was derived from
    provider    TEXT    NOT NULL DEFAULT '', -- winning provider (provenance; the allowlist used to fetch)
    width       INTEGER NOT NULL,
    height      INTEGER NOT NULL,
    byte_size   INTEGER NOT NULL,
    created_at  TEXT    NOT NULL             -- RFC3339 UTC, like every other timestamp
);
```

- **Bytes live on disk, not in the DB** (ADR-014, like thumbnails and person images) at
  `DATA_PATH/studio-logos/{studio_id}/{id}.jpg`. The `id` is server-assigned; a filesystem path is
  never built from a request value (ADR-038's rule).
- **`UNIQUE(studio_id)` enforces one logo per studio** — the single-slot invariant as a DB constraint,
  the studio analogue of person's partial-unique core-slot index. A refresh is **delete + insert**, so
  the row `id` changes → the `?v={id}` cache-buster changes → the browser re-fetches past the immutable
  cache (ADR-038's id-is-the-version trick, no separate version column).
- **`source_url` is the idempotency key.** `RelinkStudioLogo` skips the download when the resolved URL
  already equals the stored `source_url` — a re-enrich or an unrelated decision change costs one string
  compare, not a refetch.
- **No `content_hash`, no suppression, no `sort_order`, no `role`.** Those exist only to serve person
  dedup/gallery/lock semantics that a single enrichment-only logo does not have. `ON DELETE CASCADE`
  keeps a logo from outliving its studio; studio pruning (ADR-053 prune-on-empty) therefore also drops
  the logo row (and `RelinkStudioLogo` removes the file).

### 3. `RelinkStudioLogo(ctx, studioID)` is the sole writer — same trigger set as the field

`RelinkStudioLogo` (in `internal/api`, beside `RelinkVideoStudios` — it needs the resolver + mapping
registry) is the single entry point:

1. Load the studio. If it is gone → delete the row + remove the file. Done.
2. Resolve the studio's fields (the existing `studioResolved` path, lifted to take a `context.Context`
   so it is callable off the request path) and read the **resolved `logo`** value + its winning provider
   (the namespace of `winning_source`).
3. **Resolved logo empty** (no enrichment, or a `record`/blank pin): delete the row + remove the file.
   A blank-pin now genuinely hides the logo everywhere (see Consequences — this tightens HOLODEX-126).
4. **Resolved URL == stored `source_url`**: no-op (idempotent).
5. **Otherwise**: fetch the URL through the **winning provider's** `AssetClient`
   (`enrich.Service.FetchAsset(ctx, provider, url)` — a thin new method that resolves the provider by
   name against the registry allowlist and fetches under the full ADR-039 guard), run the bytes through
   `personimage.Normalize` (metadata strip + bomb guard + downscale), insert the new row (replacing any
   prior via the unique constraint), `studioimage.Store` the JPEG, and remove the superseded file.

It is **best-effort at every call site** — a fetch/normalize/store failure is logged and swallowed,
never failing the user action that triggered it (the enrichment/decision already committed; the cache
self-heals on the next trigger or the backfill). This is the exact posture of `relinkStudios`
(ADR-053 §4) and person asset download (ADR-038).

**Triggers** — every path that can move the resolved `logo` value, mirroring the studio-field relink set:

| Trigger | Handler | Why |
|---|---|---|
| Studio enrich apply | `enrichStudioApply` | a provider's logo becomes available |
| Studio enrich clear | `enrichStudioClear` | the winning logo may vanish → delete |
| Studio `logo` decision set/clear | `setStudioFieldDecision` / `clearStudioFieldDecision` | blank-pin hides it; provider-pin re-selects (gated on `canonical == "logo"`, like `relinkIfStudio`) |
| One-time backfill | `cmd/holodex` boot | populate caches for already-enriched studios without a re-enrich |

Curation is **not** a trigger: `logo` is a replace field, and value-level curation (ADR-048) applies to
merge fields only, so it can never change the resolved logo.

### 4. Serving — one public typed route, immutable cache

`GET /api/v1/studios/{id}/logo` streams the on-disk JPEG with
`Cache-Control: public, max-age=31536000, immutable` + `X-Content-Type-Options: nosniff`, keyed by the
`?v={id}` the model emits. **No placeholder route**: an absent logo returns **404**, and the SPA already
renders the F38 monogram in that case — the monogram *is* the empty state, so studios need none of
person's themed-placeholder-SVG machinery. Public read, matching every other studio read (ADR-053).

The model gains a served URL exactly like `ThumbnailURL`:

```go
// setStudioLogoURL fills LogoURL from the studio_logos cache; empty → the SPA monogram.
s.LogoURL = fmt.Sprintf("/api/v1/studios/%d/logo?v=%d", s.ID, logoRowID)
```

`Studio.LogoURL` therefore keeps its JSON shape (`logo_url`, a string) but now points at **our own
origin** instead of `image.tmdb.org`. The `/studios` list `<img src={s.logo_url}>` is unchanged; the
list logo attach (`attachStudioLogos`) swaps its source table from `entity_enrichment` to
`studio_logos`. The detail page renders `studio.logo_url` (served) for the image while keeping the F38
`logo` decision chip for owner curation.

### 5. Reuse, not a parallel spine

- **Normalize + bomb guard**: `personimage.Normalize` / `Hash` are the single security-critical image
  spine; `internal/studioimage` reuses them and owns only the disk layout (`ImagePath` / `Store` /
  `Remove` under `studio-logos/`). No second copy of the decode/re-encode guard. (If a *third* entity
  ever needs image storage, extract the spine to `internal/imagenorm` then — noted, not done now.)
- **SSRF perimeter**: `enrich.Service.FetchAsset` is the *only* new download seam and it delegates
  entirely to the existing per-source `AssetClient` (ADR-039) — same host allowlist, https-for-cross-host,
  redirect refusal, 16 MiB cap, 15 s timeout. The logo host (`image.tmdb.org`) is already the operator's
  configured `asset_hosts` entry for person portraits, so no new trust surface is introduced.

## Options considered

| # | Option | Verdict |
|---|---|---|
| **A** | **Full studio-image subsystem** — clone `internal/personimage`: multi-role table, gallery, uploads, promote/reorder, placeholder SVGs, `StudioImageFrame` components, owner mutations | **Rejected.** A studio has one image and no upload/gallery use case; ~90% of the surface (and its security-review load) would be dead. Over-engineering for one logo. |
| **B (chosen)** | **Derived self-hosted logo cache** — one-row-per-studio table, reuse the normalize spine + ADR-039 fetch, re-synced from the resolved `logo` field, monogram as the empty state, no provider change | **Chosen.** Captures the entire privacy/hardening win with a single table, one new package (disk layout only), one new fetch method, and a tiny SPA delta. |
| **C** | **Status quo (hotlink)** — keep `<img src>` at the raw provider CDN URL | **Rejected.** Leaves the IP-leak / no-metadata-strip / hotlink-fragility gap open; the one image surface that skips ADR-038. |
| **D** | Switch the provider to emit the logo as an **asset** (`{kind:"logo"}`) and route it through the person `downloadAssets` path | **Rejected for v1.** Forces a provider + contract change and drags in the person lock/suppress/gallery/seed logic that a single logo does not use. Option B fetches the field-URL through the *same* ADR-039 perimeter with none of that. A future multi-provider-logo need could revisit the asset channel. |

## Consequences

**Easier / better**
- The last hotlinked image surface is closed: studio logos are now downloaded once, metadata-stripped,
  bomb-guarded, and served from Holodex's origin — parity with person portraits, no viewer-IP leak.
- List and detail now agree: because the cache tracks the **resolved** logo, a blank-pin hides the logo
  in the `/studios` list too. This **supersedes** the HOLODEX-126 "raw, not resolved" caveat
  (`attachStudioLogos`), which existed only because the list read the shadow field directly; the
  derived cache removes the divergence rather than documenting it.
- No provider, sidecar, or `metadata-provider-contract` change — the whole feature is core-side.

**Harder / to revisit**
- **New derivation call sites.** The logo now re-syncs on enrich apply/clear and `logo` decision
  set/clear. These are best-effort and cheap (a string compare in the no-change case), but they are new
  places that touch the network. Mitigated by the `source_url` short-circuit and the best-effort posture.
- **No owner override of the logo image in v1.** Because the logo stays a *field* (not a person-style
  image slot), there is no "upload a studio logo" or "delete this specific logo" — the owner curates via
  the existing provider/blank-pin decision only. Cloning person's upload/lock/suppress is the deferred
  path (a future ADR extending this one) if that need appears.
- **Single logo, last-writer-wins across providers.** With one provider (TMDB) this is moot; the resolved
  `logo` field's decision already disambiguates when multiple providers ever supply one.

**Security review touch-points** (for the `/security-review` gate)
- New public serve route streams only server-assigned-id paths under `studio-logos/` — no request-value
  path component (ADR-038 traversal invariant).
- The one new outbound-fetch seam (`FetchAsset`) delegates to the unchanged ADR-039 `AssetClient`; it
  adds no host, scheme, redirect, or size latitude.
- All triggers are already owner-gated (enrich + decision endpoints); the serve route is a public read of
  a normalized JPEG, like thumbnails.

## Action items

- [ ] Migration `0019_studio_logos.{up,down}.sql` (table + `UNIQUE(studio_id)`; down drops the table).
- [ ] `internal/studioimage` — `ImagePath` / `Store` / `Remove` (disk layout under `studio-logos/`),
      reusing `personimage.Normalize` / `Hash`.
- [ ] `repo`: `GetStudioLogo` / `UpsertStudioLogo` / `DeleteStudioLogo`; `attachStudioLogos` reads
      `studio_logos`; `GetStudio` fills `LogoURL`.
- [ ] `enrich.Service.FetchAsset(ctx, provider, url)` — provider-scoped ADR-039 fetch.
- [ ] `RelinkStudioLogo(ctx, id)` + best-effort `relinkStudioLogo` wrapper; wire the four triggers;
      one-time boot backfill (gated like `backfillStudioLinks`).
- [ ] `GET /studios/{id}/logo` public serve route; `setStudioLogoURL` helper.
- [ ] Config: derive `StudioLogoPath = DataPath/studio-logos`; `Handlers.SetStudioImages(dir, maxDim)`;
      `MkdirAll` at boot.
- [ ] SPA: detail renders `studio.logo_url` (served) for the logo image; list unchanged.
- [ ] `metadata-sources.yaml.example`: add `studio` to the TMDB `entity_types` (studio enrich prerequisite).
- [ ] Tests: normalize/store round-trip, `RelinkStudioLogo` (fresh/no-op/blank-pin/gone), serve
      route (200 + immutable, 404), SSRF refusal via `FetchAsset`. Update `docs/testing-strategy.md`.
- [ ] Update the ADR index (README) and F38 spec (mark P2-3 realized).
