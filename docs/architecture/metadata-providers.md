# Metadata providers

This doc owns how Holodex talks to metadata providers and where their data lands: the sidecar HTTP
contract and how it evolves, the provider registry in `metadata-sources.yaml`, the
`entity_enrichment` shadow store, auto-apply and dismissals, provider links and the traffic contract.
The `base_url` / `asset_hosts` perimeter is [security-perimeter.md](security-perimeter.md); merging
enrichment with the file layer at read time is [field-resolution.md](field-resolution.md); image
storage and serving are [images.md](images.md).

## Sidecar providers behind a versioned HTTP/JSON contract

Each source is a separately deployed container that core calls over HTTP: `GET /healthz`,
`GET /describe` (capability manifest), `POST /resolve` (identity match → ranked candidates) and
`POST /enrich` (chosen external id → canonical `fields` plus `assets`). Providers are **declared, not
compiled in**: `metadata-sources.yaml` lists each source (`name`, `base_url`, `entity_types`,
`enabled` and the per-source knobs below); a new source is a container plus a config entry.

- **Core owns** enablement, pacing, the shadow store, orchestration and the perimeter. **The
  provider owns** its upstream API key, upstream parsing and caching; core holds no upstream keys.
- Core calls through `ProviderClient`; HTTP is the default and an in-process fake
  (`internal/enrich/fake.go`) runs the whole flow offline in CI.
- `verifiedClient` reads `/describe` on every owner-initiated call and refuses a provider whose
  `protocol_version` differs from `ProtocolVersion`.

**Rejected:** compiled-in Go providers — every SDK and key in one binary, provider releases tied to
Holodex releases. **Rejected:** subprocess plugins or WASM — machinery that doesn't fit a
compose-a-few-containers deployment.

Decided in [`90b3b4a0`](https://github.com/whoiskevinrich/holodex/commit/90b3b4a0).

## Additive contract evolution under protocol v1

The contract grows without a `protocol_version` bump (a bump would make core refuse every existing
provider), by three rules:

- **Manifest:** new `/describe` keys are optional and unknown keys are ignored. `fields[]` stays a
  flat `string[]`; per-field data rides a separate keyed map, never a polymorphic list.
- **`/resolve` request:** unknown request keys carry no guarantee, so a new key goes **only** to a
  provider that opts in via `/describe.resolve_hints`. `gateHint` in `Service.Resolve` applies the
  gate against the manifest fetched in the same call.
- **Responses:** core ignores unknown response keys, so a provider may add one before core reads it.

Decided in [`1a1d63a2`](https://github.com/whoiskevinrich/holodex/commit/1a1d63a2).

## `entity_enrichment` as an additive shadow store

Provider values persist in `entity_enrichment`, one row per `(entity_type, entity_id, provider,
field_key)` with `value` (multi-values newline-joined), `external_id` (the confirmed match, so a
re-enrich skips identity) and `fetched_at`. It sits apart from the file-extracted tables: a rescan
never touches it and it is never flattened into the file layer.

- **Additive:** an enrich upserts per key and never deletes a key absent from the response;
  `Service.Clear` (`DeleteEnrichmentByProvider`) removes one provider's rows.
- **`_`-prefixed keys** (`model.InternalFieldPrefix`: `_studio_external_ids`, `_source_url`, …) are a
  provider→core sidecar channel: ordinary rows, but hidden from field listings, refused by claims and
  promotions, skipped by the resolver. Every reader of enrichment rows keeps that filter.
- **Id-carrying values** use the `<namespace>:<id>` shape ([entity-identity.md](entity-identity.md)),
  including the `external_provider_id` field value, so it is self-describing outside its row. That is
  a provider obligation, not a `CHECK` constraint.

Decided in [`1bc88c80`](https://github.com/whoiskevinrich/holodex/commit/1bc88c80).

## Field hints in `/describe`, persisted for the read path

A provider may attach presentation intent to advertised keys via the optional
`/describe.field_hints` map (`label`, `render`, `group`, `order`). Hints are untrusted: sanitized and
allowlist-validated on ingest. Each owner action that reads `/describe` replaces that provider's rows
in `provider_field_hints` (delete-then-insert under `writeMu`); the read path reads only the table
and never calls a provider. How hints rank and when a key auto-registers is
[field-resolution.md](field-resolution.md).

**Rejected:** an in-memory manifest cache — a cold start shows fallback labels until an owner acts.

Decided in [`e3d63825`](https://github.com/whoiskevinrich/holodex/commit/e3d63825).

## Asset objects on `/enrich`, fetched by core at enrich time

`/enrich` returns images in `assets[]` as `{kind, url}`; unknown kinds and keys are ignored.
`assetRoleFor` maps a kind to a per-entity image role and drops an unmapped kind. Within a role the
array is preference-ordered and core keeps the **first asset it fetches and stores**. A provider omits
`assets` when empty and sheds assets before fields at the body cap. Kinds are advertised in the
advisory `/describe.asset_kinds`, not in `fields`. Core downloads the bytes synchronously during the
owner's enrich and stores the image, never a long-lived URL, so signed short-lived URLs work.

**Rejected:** providers re-hosting all bytes — taxes every provider author; kept as the escape hatch
if core must become fully network-isolated.

Decided in [`f6c58805`](https://github.com/whoiskevinrich/holodex/commit/f6c58805).

## Provider brand icon advertised in `/describe`

A provider's brand mark is the optional `/describe.brand_icon` (`{url}`): a provider-level asset,
not an `asset_kind`, since it has no entity or external id. It passes the same asset perimeter; a
refused URL means no icon. `RelinkProviderIcon` (via `Service.DescribeProvider`) is the sole writer;
`RefreshProviderIcons` runs it for every enabled provider and prunes removed ones at boot and after a
config reload, best-effort and off the request path, so enrich never touches icons. The public
`GET /api/v1/providers` directory and `/enrich/sources` carry `icon_url` from one helper
(`providerInfos`).

**Rejected:** an operator-configured icon — the icon belongs to the provider. **Rejected:** a
`GET /icon` provider endpoint — `/describe` already advertises capabilities.

Decided in [`435da9cf`](https://github.com/whoiskevinrich/holodex/commit/435da9cf).

## Core-rendered search query for `hint.query`

`hint.query` stays one string, rendered by `internal/enrich/query.go` from a pattern of
`{token}`/`{token?}` placeholders over resolved video fields (`studio`, `title`, `performers`,
`year`), space-joined. The pattern resolves: the source's `search_pattern` →
`/describe.preferred_search_pattern` (cached in memory) → top-level `default_search_pattern` → the
sanitized title. A required token with no value fails its tier; an unknown token drops that pattern
with a warning, never the provider.

- `sanitizeTitle` strips bracket/paren/brace/comma punctuation and resolution tokens from `{title}`
  and the floor.
- **Residue rule:** `{title}` renders empty when the other tokens in that pattern already carry all
  its words. Rendered-empty is not missing, so it does not fail the tier.
- Video only; person and studio stay name-seeded. The video response carries the rendered value per
  provider as `enrich_queries`.

Decided in [`1a1d63a2`](https://github.com/whoiskevinrich/holodex/commit/1a1d63a2).

## Opt-in structured resolve hints and `searched[]`

A provider listing them in `/describe.resolve_hints` also receives:

- `hint.fields`: resolved values (after decisions and curation) for `title`, `studio`, `actors`,
  `director` and `release_date`, intersected with its advertised `fields`. Nothing else leaves the box.
- `hint.filename`: the media basename (`filepath.Base`), verbatim. Holodex never renames media, so
  writeback cannot poison it. The operator can withhold it per source with `send_filename: false`.
- `hint.query_source`: `pattern` or `user`, derived server-side by re-rendering and comparing.

The response may carry `searched[]` (the upstream queries issued, sanitized and capped like candidate
labels). The interactive and batch paths build hints per provider through the same code. A provider
must issue a `user` query first and may fall back to `hint.fields` after a miss.

Decided in [`1a1d63a2`](https://github.com/whoiskevinrich/holodex/commit/1a1d63a2).

## Auto-apply routing and the `enrichment_dismissals` store

`sanitizeCandidates` sets `Candidate.AutoApply` from `StrongMatchThreshold`, and `SingleStrongMatch`
returns a candidate only when exactly one is strong. That candidate goes through the same apply path
as a manual pick, so its rows are indistinguishable and need no separate undo. `confidence` keeps its
wire shape and stays provider-native.

An owner's "not matched" verdict is a row in `enrichment_dismissals` (`entity_type`, `entity_id`,
`provider`, `dismissed_at`). A dismissed pair is never dialed until the row is deleted
(`POST`/`DELETE …/{id}/enrich/{provider}/dismiss`, owner-gated). With no TTL, no polling is needed.

**Rejected:** a `dismissed` flag on `entity_enrichment` — a row there means "data is stored", and
every reader would need a discriminator.

Decided in [`f022df1a`](https://github.com/whoiskevinrich/holodex/commit/f022df1a).

## Provider links: declared templates, `_source_url` fallback

Core builds outbound provider links server-side; the frontend never builds a URL from an id.

- `/describe.link_templates` maps namespace → entity kind → an http(s) URL with exactly one `{id}`
  (`ValidateLinkTemplate`). They persist in `provider_link_templates`, keyed by `(namespace,
  entity_type)` rather than provider, because a namespace is a shared identity space.
- If a provider's pages aren't template-shaped, it returns the page URL as the `_source_url` sidecar
  field on `/enrich`, checked with `validHTTPURL`. Invalid values overwrite the stored URL with empty;
  an absent key leaves it.
- `Service.ProviderLink` resolves a link as: the template for `(ns, kind)`, else the stored
  `_source_url` of provider `ns` itself, else no href. Keying on `provider = namespace` stops one
  provider's page from appearing behind another namespace's link.
- Person, studio and film links are a read-only projection of their external-id rows
  ([entity-identity.md](entity-identity.md)), one per row, and never pass through the resolver.
  Video uses its resolved `external_provider_id`.

**Rejected:** a frontend per-namespace URL map — breaks for any provider the SPA doesn't know.

Decided in [`1bc88c80`](https://github.com/whoiskevinrich/holodex/commit/1bc88c80).

## Traffic contract: per-provider pacing in core, `429` as a typed pause

Core paces `/resolve` and `/enrich` with one token bucket per provider (`internal/enrich/pacer.go`,
`golang.org/x/time/rate`), held on `enrich.Service` and never re-created, so tokens and a pause survive
a config reload. `/describe` and `/healthz` are not paced. The limit resolves on every acquire:
`rate_limit` on the source → `/describe.rate_limit` (clamped, cached in memory) → `DefaultRateLimit`.

- A sidecar's `429` is its one back-pressure signal. Core reads `Retry-After` as delta-seconds,
  capped, and keeps the bucket closed until then. Acquiring during a pause returns
  `*ErrProviderPaused` at once; the pacer never sleeps through a pause.
- The caller sets the policy: interactive handlers answer `503` + `Retry-After`, and a batch caller
  may wait and retry. A sidecar passes an upstream `429` through and never holds the connection.
- The clock (`now`, `sleep`) is injected and monotonic.

**Rejected:** pacing only in sidecars — one non-compliant provider turns a batch into a hammer.
**Rejected:** retrying a `429` inside the client — it would block an interactive request for minutes.

Decided in [`a9815b1d`](https://github.com/whoiskevinrich/holodex/commit/a9815b1d).
