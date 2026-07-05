# ADR-059: Provider brand icon — advertised in `/describe`, self-hosted, monogram fallback

**Status:** Proposed
**Date:** 2026-07-05
**Deciders:** Project owner

**Relates to:** [ADR-057](ADR-057-self-hosted-studio-logo.md) (self-hosted studio logo — the derived-cache / normalize-spine / immutable-serve / monogram-empty-state pattern this reuses almost verbatim; the provider icon is the **per-provider** analogue of the per-studio logo) · [ADR-039](ADR-039-provider-asset-urls.md) (provider asset URLs — the `asset_hosts` allowlist + redirect/scheme/size guard the icon download passes through; `enrich.Service.FetchAsset` is the seam) · [ADR-038](ADR-038-person-images.md) (person images — `personimage.Normalize` untrusted-bytes spine + the "server-assigned id in the filesystem path" traversal invariant) · [ADR-033](ADR-033-metadata-source-plugins.md) (metadata plugins — this is the first **additive** change to the `/describe` manifest that adds a *provider-level* asset) · [ADR-056](ADR-056-provider-field-render-hints.md) (the previous additive `/describe` extension — `field_hints` — the precedent for growing the manifest with an optional key and no protocol bump). **Spec:** [Metadata Provider Contract](../specs/metadata-provider-contract.md) (§2.2 `/describe`, new §4.8). **Ticket:** HOLODEX-134 (keystone; blocks HOLODEX-135/136/137).

---

## Context

The People page badges every resolved field with its provenance — `ProvenanceBadge.svelte`
renders the literal string **"from {provider}"** on every provider-sourced row (born, height,
bio, …). Repeated down a column it out-shouts the values it annotates (the HOLODEX-135
feedback). The same repetition shows up in the per-provider Enrich/Clear controls
(HOLODEX-136) and the website field (HOLODEX-137). The shared fix the owner chose is to let a
provider's **brand identity be carried by a compact icon** instead of its name spelled out
over and over.

Today a provider has **no way to express a brand icon**. The registry `Source`
(`internal/enrich/enrich.go`) is operator config — `{name, base_url, entity_types,
asset_hosts, enabled}` — and the `/describe` `Manifest` carries only capabilities
(`entity_types`, `id_namespaces`, `fields`, `asset_kinds`, `field_hints`). The API type the
SPA consumes, `SourceInfo` (`/enrich/sources`), is `{name, entity_types}`. Nowhere is there a
provider-branding asset.

**Why this is *not* ADR-057 over again.** The studio logo is a **per-entity canonical field**
(`logo`): every studio carries its own, resolution already produces the URL, so ADR-057 needed
**zero contract change** — it just self-hosts whatever the `logo` field resolves to. A provider
brand icon is categorically different: it is a property of the **provider**, not of any entity.
TMDB's mark is identical across every person, studio, and film it enriches, and it is tied to
no `external_id`. There is no per-entity field to derive it from, so it **must be advertised
somewhere new**. The only provider-level manifest we have is `/describe` — hence a contract
change, unlike ADR-057.

Everything *downstream* of "where does the URL come from" is identical to ADR-057, and we reuse
it wholesale: fetch through the ADR-039 SSRF/asset-host perimeter, `personimage.Normalize`
(metadata strip + bomb guard + downscale), one row per subject, bytes on disk, an immutable
typed serve route, and a client-side monogram as the empty state.

## Decision

### 1. A provider advertises its brand icon in `/describe` — additive, no protocol bump

`/describe` gains one optional key, `brand_icon`, an **asset object** carrying a single URL:

```json
{
  "provider": "tmdb",
  "protocol_version": 1,
  "entity_types": ["person", "studio"],
  "fields": ["bio", "birthdate", "nationality", "website", "aliases"],
  "brand_icon": { "url": "https://www.themoviedb.org/assets/tmdb.svg" }
}
```

- **Additive, forward-compatible** — an unknown top-level key is ignored by older Holodex, and
  an older provider that omits it is unchanged (monogram fallback). This is the exact latitude
  ADR-056 used to add `field_hints`; **`protocol_version` stays `1`**.
- **A provider-level asset, deliberately *not* an `asset_kind`.** `asset_kinds` (photo/banner/
  poster) are per-entity images returned by `/enrich` against a chosen `external_id`; the brand
  icon has no entity and no `external_id`, so overloading that channel (Option D) would be a
  category error. It lives in the capability manifest because it *is* a capability of the
  provider.
- **Subject to the full ADR-039 asset perimeter.** The `brand_icon.url` host must be the
  provider's own `base_url` host or an operator-listed `asset_hosts` entry; https for
  cross-host; no credentials; the 16 MiB / 4096 px / 15 s caps apply. A URL on a
  non-allowlisted host is **refused** and the provider simply has no icon (monogram). No new
  trust surface: it is the same download seam person portraits and the studio logo already use.
- **Raster on ingest.** Providers may serve any format their host allows, but Holodex stores a
  normalized **JPEG** (`personimage.Normalize` re-encodes; SVG/polyglots are rejected by the
  decoder, exactly as for every other ingested image). Providers wanting crisp small-size
  rendering should advertise a reasonably square raster.

### 2. Data model — one icon per provider (migration 0021), a derived cache

```sql
CREATE TABLE provider_icons (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    provider    TEXT    NOT NULL UNIQUE,       -- registry provider name (the identity)
    source_url  TEXT    NOT NULL,              -- the /describe brand_icon URL this cache derives from
    width       INTEGER NOT NULL,
    height      INTEGER NOT NULL,
    byte_size   INTEGER NOT NULL,
    created_at  TEXT    NOT NULL               -- RFC3339 UTC
);
```

Mirrors `studio_logos` (ADR-057 §2) one layer over, keyed by **provider name** instead of a
studio id:

- **`UNIQUE(provider)`** — one icon per provider. A refresh is **delete + insert**, so the row
  `id` changes → the `?v={id}` cache-buster changes → the browser re-fetches past the immutable
  cache (ADR-038's id-is-the-version trick, no version column).
- **`source_url` is the idempotency key.** The relink skips the download when the advertised
  `brand_icon.url` already equals the stored `source_url` — a re-verified provider costs one
  string compare, not a refetch.
- **No FK.** Providers are YAML registry entries, not a DB table, so there is no `REFERENCES`
  and no `ON DELETE CASCADE`. A provider removed from the registry leaves an orphan row; a
  best-effort **boot reconcile** prunes `provider_icons` rows (and files) whose `provider` is no
  longer enabled in `Store.Current()`.
- **Bytes on disk, not in the DB** (ADR-014), at `DATA_PATH/provider-icons/{id}.jpg`. The `id`
  is server-assigned; **no filesystem path is ever built from `provider`** or any request value
  (ADR-038 traversal invariant). `internal/providericon` owns only this disk layout
  (`ImagePath`/`Store`/`Remove`) and reuses `personimage.Normalize`/`Hash` — no second copy of
  the security-critical decode/re-encode spine (ADR-057 §5).

### 3. `relinkProviderIcon` is the sole writer — hooked on the describe choke point

The provider icon is **static per provider** and changes only when the provider's `/describe`
changes — so, unlike the studio logo (re-synced on enrich/decision/curation), its natural
trigger is **provider verification**, which already fetches `/describe`.

`enrich.Service.verifiedClient` (`internal/enrich/service.go`) is the choke point for every
provider action (resolve, enrich, status) — it already calls `c.Describe(ctx)` and
`persistFieldHints`. We add one best-effort call there with the `Manifest` already in hand:

1. `brand_icon` **absent/empty** → delete the row + remove the file (provider dropped its icon).
2. Advertised URL **== stored `source_url`** → no-op (idempotent, the common path).
3. **Otherwise** → `FetchAsset(ctx, provider, url)` (the unchanged ADR-039 per-provider asset
   client), `personimage.Normalize`, `ReplaceProviderIcon` (delete+insert → new id),
   `providericon.Store` the JPEG, remove the superseded file.

It is **best-effort at every call site** — a fetch/normalize/store failure is logged and
swallowed, never failing the resolve/enrich the owner triggered (same posture as
`RelinkStudioLogo` and person asset download). A one-time **boot backfill** (gated on a job-run
marker + an empty-table fast-path, like `backfillStudioLogos`) calls `/describe` once per
enabled provider so already-configured providers get an icon without waiting for the first
enrich.

**Placement note (a deliberate divergence from ADR-057).** ADR-057 put `RelinkStudioLogo` in
`internal/api` because it needs the resolver to read the resolved `logo` field. The provider
icon needs **no resolver** — the URL comes straight off the `Manifest` — so its relink lives in
`internal/enrich` beside `FetchAsset` and the describe call, where the data already is. The API
layer only *serves* it and *exposes* its URL (below). `SetProviderIcons(dir, maxDim)` is wired
to the service at boot, mirroring `SetStudioImages`.

### 4. Serving — one public typed route, immutable cache, 404 → monogram

`GET /api/v1/providers/{name}/icon` streams the on-disk JPEG with `Cache-Control: public,
max-age=31536000, immutable` + `X-Content-Type-Options: nosniff`, keyed by the `?v={id}` the
payload emits. The `{name}` is looked up against the registry/`provider_icons` — it is **not**
used to build a path (the file path comes from the row `id`). An absent icon returns **404**,
and the SPA renders a **monogram** (provider initials) in that case — the monogram *is* the
empty state, exactly as the studio list monogram is (no themed-placeholder-SVG machinery).

Public read is correct and leaks nothing new: provider **names are already visitor-visible**
today via the "from {provider}" provenance badges, so serving their icons — and listing them
(below) — exposes no identity that the provenance line does not already show.

### 5. Two exposure seams, split by audience — owner controls vs. visitor provenance

The icon URL must reach **two** render sites with different audiences:

- **Owner** — the Enrich/Clear controls (HOLODEX-136). `SourceInfo` (`/enrich/sources`, owner-
  gated) gains an optional `icon_url`, built the same way as `Studio.LogoURL`:
  `/api/v1/providers/{name}/icon?v={id}` when a row exists, omitted otherwise.
- **Visitor** — the provenance badges and website field (HOLODEX-135/137) render for everyone,
  but `/enrich/sources` is owner-only, so a visitor has no provider→icon map. We add a **public**
  `GET /api/v1/providers` directory returning `[{ "name", "entity_types", "icon_url"? }]` for all
  enabled providers. Both audiences can resolve a provider name (which the SPA already has from a
  field's `winning_source`) to an icon URL + version; `icon_url` is present iff an icon is cached,
  so the presence check drives real-vs-monogram, consistent with the `Studio.logo_url` idiom.

`icon_url` on `SourceInfo` is retained (rather than folding the owner path onto the public list)
so the existing owner enrich page keeps working with a one-field addition; both handlers build
the URL through one shared helper, so there is a single source of truth for the string.

**This §5 split is the one reversible design call** (see Consequences). The alternative —
skip the directory and render `<img src="/api/v1/providers/{name}/icon" onerror=…>` with a
client `onerror`→monogram fallback — needs no public list at all, at the cost of a 404 round-trip
+ flash per unknown provider and losing the up-front real-vs-monogram signal. The public
directory is chosen for a cleaner first paint and because the SPA benefits from a provider list
elsewhere (badge tooltips, the website label). It is a small, isolated endpoint to revisit if it
proves unnecessary.

### 6. Monogram fallback — client-side, themed, tokens-only (the `needs-design` surface)

When no icon is cached the SPA renders the provider's first initial in a token-styled plate,
reusing the studio list monogram treatment (`bg-logo-plate` / `text-logo-plate-ink`,
`font-display`). This generalizes to a small shared `ProviderIcon.svelte` (icon-or-monogram,
given a name + optional `icon_url`) that HOLODEX-135/136/137 all consume. Tokens only; QA all
three skins (Cinémathèque, Broadcast, Brutalist).

## Options considered

| # | Option | Verdict |
|---|--------|---------|
| **A** | **Operator configures the icon** per provider in `metadata-sources.yaml` (a path/URL field), no contract change | **Rejected.** The feedback is explicit that icons are "supplied from the sidecars"; pushing a hand-maintained icon path onto every operator is the wrong owner and doesn't travel with a third-party provider. |
| **B (chosen)** | **Additive `/describe.brand_icon`**, self-hosted via the ADR-038/039 spine, one row per provider, public directory + `SourceInfo.icon_url`, monogram fallback | **Chosen.** The provider supplies its own brand mark; the whole hardening story (SSRF perimeter, normalize, self-host, no viewer-IP leak) comes for free by reusing ADR-057's spine; the manifest grows by one optional key with no protocol bump. |
| **C** | A new **`GET /icon` provider endpoint** returning the bytes directly | **Rejected.** A fifth endpoint for a static capability that `/describe` already exists to advertise; heavier for every provider author, and still needs the same core-side fetch/normalize/store. |
| **D** | Emit the icon as a per-entity **`asset_kind: "brand_icon"`** through `/enrich` | **Rejected.** The icon has no entity and no `external_id`; the asset channel is per-entity/per-match. Wrong shape, and it would fetch once *per enriched entity* instead of once per provider. |
| **E** | **Bundle known provider icons** as static assets in the Holodex image | **Rejected.** Doesn't scale to third-party/unknown providers, goes stale, and defeats "supplied from the sidecar." |

## Consequences

**Easier / better**
- Provenance, enrich controls, and the website field can all lead with a compact brand glyph
  (HOLODEX-135/136/137 unblocked) instead of repeating the provider name.
- Provider icons get the same hardening as every other ingested image — downloaded once through
  the SSRF perimeter, metadata-stripped, bomb-guarded, served from Holodex's own origin (no
  viewer-IP leak to the provider's CDN).
- One optional manifest key, no protocol bump; providers with no icon are unaffected.

**Harder / to revisit**
- **First contract change that adds a provider-level asset.** The provider spec and every
  provider author's mental model grow by one concept. Mitigated by keeping it optional and
  reusing the existing asset-host trust rules verbatim.
- **New public endpoint (`GET /api/v1/providers`).** The §5 split adds a small public surface.
  It exposes only what provenance already reveals (provider names) plus `entity_types`; revisit
  (collapse to the `onerror` fallback) if it earns its keep poorly.
- **New network touch on the describe path.** `verifiedClient` now may fetch an icon. Guarded by
  the `source_url` short-circuit (one string compare after first fetch) and the best-effort
  posture, so a slow/broken icon host never degrades resolve/enrich.
- **Registry churn leaves orphan rows** until the boot reconcile prunes them — cosmetic (an
  unreferenced file), never served (the route resolves by current provider name).

**Security review touch-points** (for the `/security-review` gate)
- New public serve route streams only server-assigned-id paths under `provider-icons/` — no
  request-value path component; `{name}` selects a row, never a filename (ADR-038 traversal
  invariant).
- The one new outbound-fetch path reuses `enrich.Service.FetchAsset` (ADR-039) unchanged — adds
  no host, scheme, redirect, or size latitude; `brand_icon.url` is subject to the same
  `asset_hosts` allowlist as person portraits.
- The new public `GET /providers` directory exposes provider `name`/`entity_types`/`icon_url`
  only — names are already visitor-exposed via provenance; no secrets, no `base_url`, no config.
- All write triggers ride the already-owner-gated resolve/enrich path (and boot); the serve +
  directory routes are public reads of normalized data, like thumbnails and the studio logo.

## Action items

- [ ] Contract: `metadata-provider-contract.md` — add `brand_icon` to the `/describe` example +
      field table (§2.2) and a new **§4.8 Provider brand icon** (asset rules, one image,
      self-hosted, monogram fallback). No protocol bump.
- [ ] Migration `0021_provider_icons.{up,down}.sql` (table + `UNIQUE(provider)`; down drops it).
- [ ] `internal/providericon` — `ImagePath`/`Store`/`Remove` under `provider-icons/`, reusing
      `personimage.Normalize`/`Hash`.
- [ ] `repo`: `GetProviderIcon`/`ReplaceProviderIcon`/`DeleteProviderIcon`/`ProviderIconCount`
      + `ListProviderIcons` (for the directory + reconcile).
- [ ] `enrich`: `Manifest.BrandIcon` (asset object) parse; `relinkProviderIcon(ctx, provider,
      url)` hooked in `verifiedClient` (best-effort); `SetProviderIcons(dir, maxDim)`;
      `SourceInfo.IconURL`; boot backfill + orphan reconcile in `cmd/holodex`.
- [ ] `api`: `GET /providers/{name}/icon` serve route; public `GET /providers` directory;
      shared `providerIconURL(name, id)` helper.
- [ ] SPA: `ProviderIcon.svelte` (icon-or-monogram) + a providers store from `GET /providers`;
      `EnrichSource`/`SourceInfo` type gains `icon_url`. (Consumers HOLODEX-135/136/137 land
      separately.)
- [ ] `metadata-sources.yaml.example` + the TMDB sidecar (`providers/tmdb/`): advertise a
      `brand_icon` (its host is already an allowlisted `asset_hosts` entry).
- [ ] Tests: normalize/store round-trip, `relinkProviderIcon` (fresh/no-op/absent/orphan), serve
      route (200 immutable, 404), directory shape, SSRF refusal via `FetchAsset`, describe parse
      of `brand_icon`. Update `docs/testing-strategy.md`.
- [ ] Update the ADR index (README).
- [ ] `/security-review` on the implementation diff (new public routes + outbound fetch seam).
