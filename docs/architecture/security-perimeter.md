# Security perimeter

This doc owns who may reach what: the owner gate on mutations and owner-only surfaces, the signed
session cookie that carries owner identity in the browser, the network perimeter around metadata
providers (the `base_url` and `asset_hosts` allowlists), and the read gates that keep non-public
content from visitors. The provider contract itself lives in [metadata-providers.md](metadata-providers.md);
how config is loaded and layered lives in [config-and-settings.md](config-and-settings.md).

## One owner gate: `requireOwner` over an optional `ADMIN_TOKEN`

Owner identity is a single optional secret, `ADMIN_TOKEN` (env or config). `internal/api/auth.go`'s
`Auth` is the one choke point; every check goes through `Auth.authorized(r)`.

- **Token set:** owner-only routes require a credential — the `X-Admin-Token` header (compared with
  `subtle.ConstantTimeCompare`, never `==`) or a valid session cookie (next section). Missing or
  wrong is `401`.
- **Token unset:** the gate is a pass-through, so a zero-config local install behaves as owner. When
  the server also binds beyond loopback, `controlsUnauthenticated()` raises the fail-loud
  `controls_unauthenticated` signal on the activity payload.
- **Routes:** owner-only routes are mounted inside the `r.Use(h.requireOwner)` group in
  `Handlers.Mount`; adding one means mounting it there. A public handler with an owner-only branch
  (an owner-only sort or filter input) calls `requireOwnerInline`, which is the same check, renewal and
  dead-cookie clearing without the middleware wrapper. No handler hand-rolls its own token check.
- **Ungated by design:** `/healthz`, `/readyz` and `/metrics` (read-only, secret-free, needed by
  orchestrators and scrapers), `GET /capabilities`, and the `POST`/`DELETE /session` exchange.
- **Capability signal:** `GET /api/v1/capabilities` reports `owner` and `auth_required`; the SPA renders
  owner controls only from that flag and never decides owner-ness itself.
- **Other transports share it:** the MCP server calls the exported `Auth.Authorized` so it applies the
  same gate as REST on the same bind address.
- **Swap point:** real multi-user auth, if it ever lands, replaces the identity source behind `Auth`;
  call sites and the capability flag do not move.

**Rejected:** an auth framework or user/session store — a single-owner tool needs one secret, not
accounts, and the project keeps a lean `go.mod`.
**Rejected:** default-closed when no token is set — it would break the zero-config local install; the
non-loopback warning covers the exposed case instead.

Decided in [`e7495f8e`](https://github.com/whoiskevinrich/holodex/commit/e7495f8e).

## HttpOnly signed session cookie, no server-side store

The browser never holds `ADMIN_TOKEN` in a JS-readable place. It exchanges it once at
`POST /api/v1/session` (token in the `X-Admin-Token` header, never a URL or body) for the
`holodex_session` cookie; `DELETE /api/v1/session` expires it and is idempotent.

- **Value:** `base64(payload).base64(HMAC-SHA256(payload, secret))`, payload `iat|exp|class`. Never the
  raw token. Validation recomputes the HMAC (`hmac.Equal`) and checks expiry. No session table exists.
- **Secret:** derived as `HMAC-SHA256(ADMIN_TOKEN, "holodex/session/v1")`, so no new required config and
  rotating the token invalidates every session. Optional `SESSION_SECRET` overrides it to rotate
  sessions independently.
- **Attributes:** `HttpOnly`, `SameSite=Strict`, `Path=/`, bounded `Max-Age`, and `Secure` unless the
  request is plain HTTP to a loopback host (`secureCookie`), so local dev works and every exposed
  deployment gets `Secure`.
- **CSRF:** `SameSite=Strict` is the mitigation for the cookie path; the SPA and API share one origin
  ([deployment.md](deployment.md)). The header path stays CSRF-immune for scripts.
- **Lifetimes:** two server-defined classes (`sessionShortTTL` 7 days default, `sessionLongTTL` 30 days
  on `?remember=1`); the client picks a class, never a `Max-Age`. Past half-life, `maybeRenewSession`
  re-issues with the same class, capped at `sessionMaxAge` (90 days from first issue). A renewal never
  resurrects an expired cookie or upgrades its class.
- **Dead cookies:** an invalid or expired cookie counts as no credential, and the `401` response expires
  it so the browser stops resending it.
- With the gate open, `POST /session` is a `204` no-op and no cookie is ever set.

**Rejected:** `localStorage` / `sessionStorage` for the token — JS-readable, so one XSS exfiltrates it.
**Rejected:** a server-side session store keyed by an opaque cookie — it buys instant revocation at the
cost of new persistent state for one user; the 90-day cap and token rotation bound the risk instead.
**Rejected:** `SameSite=Lax` — no cross-site flow needs the cookie, so Strict's stronger guarantee is free.

Decided in [`8efa124e`](https://github.com/whoiskevinrich/holodex/commit/8efa124e).

## Provider SSRF perimeter: trust comes only from operator config

Every outbound provider request is dialled from `metadata-sources.yaml`, never from a URL found in a
provider response or a file. That file's `base_url` per source **is** the allowlist for API calls;
core never builds a request URL from provider or file data.

- **API client** (`internal/enrich/client.go`): requests go only to the source's `base_url`; cross-host
  redirects are not followed (`CheckRedirect` returns the 30x as final), at most 5 same-host hops;
  bodies capped at `maxResponseBytes` (1 MiB); request timeout bounded.
- **Untrusted responses:** values are length-capped (`maxFieldLen`) and run through `SanitizeValue`; a
  malformed response fails that one fetch, not the server. Core holds no upstream API keys and never
  serializes a `base_url` to the client.
- **Enrichment is on demand only.** Every enrich route sits behind the owner gate and there is no
  enrichment scheduler, so provider egress only ever happens because the owner asked.

Decided in [`90b3b4a0`](https://github.com/whoiskevinrich/holodex/commit/90b3b4a0).

## Asset-host allowlist: `{base_url host} ∪ asset_hosts`

Provider-supplied asset URLs (portraits, logos, posters) are fetched by core synchronously during an
owner's enrich, through a per-source `AssetClient` (`internal/enrich/assets.go`).

- **Allowed hosts:** the source's own `base_url` host (implicit) plus any host the operator lists in that
  source's `asset_hosts` in `metadata-sources.yaml`. Exact host match, no wildcards. A provider can name
  only a host the operator already approved; it cannot add one. Absent `asset_hosts` means
  same-host-only.
- **Scheme:** `http` or `https` for the `base_url` host (internal sidecars), `https` required for any
  other host.
- **Guards:** cross-host redirects refused, at most 5 same-host hops, `maxAssetBytes` (16 MiB) cap,
  15 s timeout. The bytes go through the image normalizer before touching disk ([images.md](images.md)).
- **The same gate covers URLs that reach the browser.** `assetHostAllowed` also decides whether a
  provider `image_url` value may render as an `<img>` (`Service.ImageURLAllowed`) and whether a
  candidate's `image_url` survives `sanitizeImageURL`; anything else is cleared. Studio logos are
  fetched through `Service.FetchAsset` under the winning provider's allowlist.
- Asset URLs are fetch-soon: the stored artifact is the image, never a long-lived URL.

**Rejected:** trusting whatever host a provider names — a compromised provider could point core at
cloud metadata or intranet services.
**Rejected:** strict same-host only — every CDN-backed provider (TMDB serves from `image.tmdb.org`)
would have to proxy image bytes through its sidecar.
**Rejected:** providers re-hosting all bytes so core has no internet egress — kept as the escape hatch if
a fully network-isolated core becomes a requirement; today it would tax every provider author.
**Rejected:** deferring downloads to a background job or lazy-on-view — the first fights the
no-scheduler posture, the second moves egress to a public viewer-triggered GET.

Decided in [`f6c58805`](https://github.com/whoiskevinrich/holodex/commit/f6c58805).

## Read gates: owner-ness is a request property, applied at read time

Reads of mixed public/private content are not router-gated; the handler computes
`isOwner := h.auth.authorized(r)` once and passes it down so the repo or response filter applies it.

- **Field redaction:** file-metadata fields are stripped for visitors by `redactFileMetadataForVisitors`
  before a video list or detail is serialized. Never wrap whole content in an owner check.
- **Private rows (playlists):** `playlists.visibility ∈ {private, public}`, default `private`, validated
  at the API, not by a `CHECK`. `ListPlaylists` / `GetPlaylist` take a `publicOnly` flag set from
  `!isOwner`. A visitor's request for a private id returns a `404` identical to an unknown id; a `403`
  would confirm the id exists. Visibility is checked first, ahead of any other check that could answer
  differently for a private id. Playlist mutations mount in the `requireOwner` group.
- **Smart playlists** evaluate their stored query under the reader's own posture, so an owner-only
  input on a public playlist is evaluated as a visitor `/media` call would be.
- **Trash** is decided by the shared `v.active = 1 AND v.deleted_at IS NULL` read seam, which playlist
  reads join through rather than restating.
- The ungated `/capabilities` exposes only a count (`public_playlists`), never ids or names.

Decided in [`8ea55a0e`](https://github.com/whoiskevinrich/holodex/commit/8ea55a0e).
