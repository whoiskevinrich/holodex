# Spec: TMDB Metadata Provider Sidecar

**Status**: Draft (hand-off spec for an external implementation team)
**Feature block**: F22 (Metadata Source Plugins) — first real provider container
**Audience**: A separate development team building the TMDB sidecar image in a **different repository**
**Implements**: The Holodex provider HTTP contract defined by [F22 / ADR-033](#references) — protocol version **1**. This is a **worked example** of the source-neutral [Metadata Provider Contract](metadata-provider-contract.md); that document is the generic spec, this one maps it onto TMDB.

> **Self-contained by design.** This document specifies everything the container
> must do without any access to the Holodex source tree. The endpoints, request/response
> schemas, headers, timeouts, and size caps below are the **exact** contract the Holodex
> client enforces; conform to them and the container drops in with no Holodex code change.
> Where the contract is genuinely unspecified, items are marked **TBD / confirm with
> Holodex maintainers** rather than guessed.

---

## 1. Overview & purpose

### What this is

A **Holodex metadata provider sidecar** is a standalone HTTP/JSON service that Holodex
calls to enrich local **People** records with data the media files do not carry — bios,
birthdates, nationality, websites, aliases, and a portrait photo. This spec covers the
**TMDB** (The Movie Database) provider: a container that translates Holodex's small,
provider-agnostic contract into calls against the public TMDB API and maps the responses
back into Holodex's canonical enrichment fields.

### How Holodex consumes it (architecture context)

Holodex runs as a single Go binary plus a few sidecar containers on an internal Docker
Compose network. A provider is **just another source feeding a unified field-resolution
layer**: provider-fetched values land in a *shadow enrichment store* kept separate from
file-extracted metadata, and the two are merged at display time by a per-field precedence
list, each value badged with its provenance ("from TMDB" vs "from file"). Enrichment is
**owner-gated** (locked behind an admin token), strictly **on-demand** (every fetch is an
explicit owner click — there is no crawler or scheduler), and **SSRF-allowlisted**: Holodex
only ever dials the exact `base_url` an operator configured for the provider, never a URL
derived from a provider or file response, and it refuses to follow a redirect to a
different host. The provider owns its own TMDB API key (Holodex holds **no** upstream keys),
its own rate-limiting and backoff, and its own upstream parsing. The container must be
**stateless** with respect to Holodex — Holodex persists everything it needs.

---

## 2. The HTTP contract

The container MUST expose exactly four endpoints. All request and response bodies are
`application/json`. The contract is **protocol version 1**.

### 2.0 Transport behaviour the Holodex client enforces

These constraints come from the calling client; the container must stay within them.

| Constraint | Value | Notes |
|---|---|---|
| Request methods | `GET /healthz`, `GET /describe`, `POST /resolve`, `POST /enrich` | Exactly these; other methods/paths are unused |
| Request headers sent by Holodex | `Accept: application/json`; `Content-Type: application/json` on POSTs | No auth header is sent to the provider — the provider is reached only over the trusted internal network |
| Per-call timeout | **8 seconds** | Holodex aborts any single call (incl. `/describe`, `/resolve`, `/enrich`, `/healthz`) at 8 s. The container must answer well within this, doing its own upstream TMDB calls with a tighter budget |
| Response body cap | **1 MiB** | Holodex reads at most 1 MiB of any response body and decodes that as JSON. Keep responses small (cap candidates, trim long bios — see [§5](#5-non-functional-requirements)) |
| Success status | `2xx` | Any non-2xx is treated as a failed call (see error handling below) |
| Redirects | Cross-host 30x is **not followed** (treated as the final response) | Respond **directly** with the JSON body and a 2xx; never redirect Holodex elsewhere |
| Request body cap (inbound) | Holodex request bodies are tiny (well under 64 KiB) | The container should also bound the bodies it accepts |

**Error handling contract.** A non-2xx status, a transport error (timeout, connection
refused), or a malformed/oversized JSON body causes Holodex to fail **that single
enrichment call only** — it never crashes Holodex and never affects other providers.
Holodex surfaces a generic `502 Bad Gateway` ("provider lookup failed" / "enrichment
failed") to its owner UI and logs a warning. The container should therefore return a
clean non-2xx (e.g. `502`/`503`) on upstream trouble and a well-formed 2xx JSON body on
success; it must never hang past 8 s.

### 2.1 `GET /healthz` — liveness / readiness

Holodex polls this for the `/status` page (provider health) and as a reachability check.

**Request:** `GET /healthz` (no body).

**Response `200`:**

```json
{ "status": "ok", "provider": "tmdb", "version": "1.0.0" }
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `status` | string | yes | `"ok"` when healthy. Holodex treats a 2xx as healthy; the string is for display |
| `provider` | string | yes | The provider's id, lowercase (must match the configured `name` — `"tmdb"`) |
| `version` | string | yes | The container's own version string (display only) |

`/healthz` MUST NOT require the upstream TMDB API to be reachable to return `200` (it is a
liveness check of the container). A degraded-upstream signal, if desired, is **TBD /
confirm with Holodex maintainers** — v1 Holodex only checks 2xx. No secrets in this body.

### 2.2 `GET /describe` — capability manifest

Holodex calls this before every `/resolve` and `/enrich` to verify the protocol version
and the supported entity type. A mismatched major protocol version makes Holodex refuse the
provider loudly.

**Request:** `GET /describe` (no body).

**Response `200`:**

```json
{
  "provider": "tmdb",
  "version": "1.0.0",
  "protocol_version": 1,
  "entity_types": ["person"],
  "id_namespaces": ["tmdb", "imdb"],
  "fields": ["bio", "birthdate", "nationality", "website", "aliases", "photo"]
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `provider` | string | yes | Provider id, lowercase — `"tmdb"` |
| `version` | string | yes | Container version (display) |
| `protocol_version` | integer | yes | **MUST be `1`.** Holodex refuses any other major version |
| `entity_types` | string[] | yes | v1: `["person"]` (see [§2.3](#23-entity-type-and-matching-fields)) |
| `id_namespaces` | string[] | yes | The external-ID namespaces understood — `["tmdb", "imdb"]` (TMDB exposes both) |
| `fields` | string[] | yes | The canonical fields the provider can supply — see [§4](#4-tmdb-specific-field-mapping). Recommended: `["bio", "birthdate", "nationality", "website", "aliases", "photo"]` |

### 2.3 Entity type and matching fields

v1 supports **one** `entity_type`: the literal string **`"person"`**. Holodex sends this
verbatim in `/resolve` and `/enrich`. The container should accept `"person"` and MAY return
an error for unknown entity types (Holodex will not send others in v1).

### 2.4 `POST /resolve` — identity match (disambiguation)

Given a name query and/or embedded external IDs, return **ranked candidate matches** for
the owner to confirm. Holodex always shows the owner a picker and never auto-applies a
candidate in v1, so confidence is advisory.

**Request body** (exact shape Holodex sends):

```json
{ "entity_type": "person", "hint": { "query": "Hayao Miyazaki", "external_ids": ["tmdb:608"] } }
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `entity_type` | string | yes | `"person"` in v1 |
| `hint` | object | yes | The identity input |
| `hint.query` | string | optional | Free-text name to search (the dominant path for People — see [§4.1](#41-resolve-name-search)). Omitted/empty when only IDs are given |
| `hint.external_ids` | string[] | optional | Namespace-qualified IDs (e.g. `"tmdb:608"`, `"imdb:nm0594503"`) for the deterministic path. **Note:** Holodex's v1 People flow sends only `query` (it sets `hint.query` from the owner's search box). The `external_ids` path is part of the contract for when People carry IDs / for future entities — implement it, but `query` is what v1 exercises |

**Response `200`:**

```json
{
  "candidates": [
    {
      "external_id": "tmdb:608",
      "namespace": "tmdb",
      "label": "Hayao Miyazaki",
      "confidence": 0.98,
      "disambiguation": "Director · 1941 · Studio Ghibli"
    }
  ]
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `candidates` | array | yes | Ranked best-first. May be empty (`[]`) for a no-match — Holodex renders a "no results" state. **Holodex caps the list at 25**; return fewer (e.g. top 5–10) |
| `candidates[].external_id` | string | yes | Provider-stable, **namespace-qualified** id, e.g. `"tmdb:608"`. This is the value Holodex echoes back to `/enrich` |
| `candidates[].namespace` | string | yes | The id namespace, `"tmdb"` |
| `candidates[].label` | string | yes | Human-readable name shown in the picker. Holodex sanitizes (strips control chars, caps 4096 chars) |
| `candidates[].confidence` | number | optional | 0–1 advisory score for ranking. Holodex does not threshold on it in v1 (always confirms) |
| `candidates[].disambiguation` | string | optional | Short distinguishing line (e.g. `"Director · 1941 · Studio Ghibli"`) to separate same-named people. Sanitized/capped by Holodex |

### 2.5 `POST /enrich` — fetch fields

Given a chosen `external_id`, return the canonical field values plus optional asset URLs.

**Request body** (exact shape Holodex sends):

```json
{ "entity_type": "person", "external_id": "tmdb:608" }
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `entity_type` | string | yes | `"person"` |
| `external_id` | string | yes | A namespace-qualified id previously returned by `/resolve` (e.g. `"tmdb:608"`). **Note:** the request carries `external_id` but **not** a separate `namespace` — the provider must parse the namespace off the prefix (everything before the first `:`). A bare/unknown id should yield a non-2xx |

**Response `200`:**

```json
{
  "fields": {
    "bio": ["Japanese filmmaker and co-founder of Studio Ghibli."],
    "birthdate": ["1941-01-05"],
    "nationality": ["Japanese"],
    "website": ["https://example.com/miyazaki"],
    "aliases": ["宮崎駿", "Miyazaki Hayao"]
  },
  "assets": [
    { "kind": "photo", "url": "https://image.tmdb.org/t/p/original/akhpeJSfFKMValElDDjsKi2jryl.jpg" }
  ]
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `fields` | object | yes | Map of **canonical field key → array of string values**. Always an array, even single-valued (e.g. `"birthdate": ["1941-01-05"]`). Keys SHOULD be drawn from the `fields` advertised in `/describe`. Holodex sanitizes every value (control chars stripped, newlines → space, capped 4096 chars), caps **50 values per field** and **40 fields total** |
| `assets` | array | optional | Binary assets (e.g. portrait). **See the asset note below** |
| `assets[].kind` | string | yes (if asset present) | Asset kind. For a person portrait use `"photo"` |
| `assets[].url` | string | yes (if asset present) | Absolute URL to the binary |

**Asset note (load-bearing).** Holodex's v1 client **parses** `assets` but **does not
download** them — person-photo storage is a deferred Holodex follow-up. Include the
`assets[]` array with a `kind: "photo"` entry so it is ready when Holodex enables download,
but do **not** depend on it being fetched in v1, and do **not** put the photo into a `fields`
entry expecting it to render as an image (`fields` values are text). Asset URLs MUST be
absolute, on a TMDB image host, and resolvable directly (no redirect) — see [§6](#6-security-requirements).

### 2.6 Status codes summary

| Endpoint | Success | Failure the container should return |
|---|---|---|
| `GET /healthz` | `200` | `503` if the container itself is not ready |
| `GET /describe` | `200` | — (static; should always succeed) |
| `POST /resolve` | `200` (incl. empty `candidates`) | `502`/`503` on upstream TMDB failure; `400` on a malformed request body |
| `POST /enrich` | `200` | `404`/`400` for an unknown/bad `external_id`; `502`/`503` on upstream TMDB failure |

Holodex maps **any** non-2xx to a generic owner-facing error and a warning log — exact
codes beyond "2xx vs not" are for the container's own clarity/observability.

---

## 3. Provenance & source naming

- The provider's **registry name** in Holodex config is `tmdb` (the `name` field in
  `metadata-sources.yaml` — see [§8](#8-deliverable--operator-wiring)). It MUST match the
  `provider` field returned by `/healthz` and `/describe`.
- Holodex labels every enriched value "from **tmdb**" using this name. Keep it lowercase
  and stable.
- The **id namespace** prefix (`tmdb:`) on `external_id` is what links a stored Holodex
  enrichment back to a TMDB record for re-fetch. Keep namespaces stable across releases.

---

## 4. TMDB-specific field mapping

The container translates TMDB's People API into the canonical contract fields. Use TMDB
API **v3** (REST). Auth is via the provider's own credential (see [§7](#7-configuration)).

### 4.1 `/resolve` (name search)

For `hint.query`, call TMDB **Search › Person**:

```
GET https://api.themoviedb.org/3/search/person?query=<urlencoded>&include_adult=false&language=en-US&page=1
```

Map each TMDB result to a candidate:

| Candidate field | From TMDB search result | Notes |
|---|---|---|
| `external_id` | `"tmdb:" + id` | `id` is the TMDB person id (integer) |
| `namespace` | `"tmdb"` | constant |
| `label` | `name` | |
| `confidence` | derived from `popularity` and/or search rank | TMDB has no match score; a reasonable mapping is rank-based (first result highest) optionally blended with `popularity`. **Exact formula TBD / confirm with Holodex maintainers** — Holodex does not threshold on it, so any sensible 0–1 monotonic value is acceptable |
| `disambiguation` | `known_for_department` + a representative `known_for[].title`/`known_for[].name` (+ year if available) | e.g. `"Directing · Spirited Away"`. Keep it short |

For `hint.external_ids` (deterministic path): if an id with namespace `tmdb` is present,
resolve it directly via **Person › Details** (below) and return a single high-confidence
candidate. If an `imdb` id is present (`imdb:nm…`), use TMDB **Find** (`GET /3/find/{imdb_id}?external_source=imdb_id`)
to map it to a TMDB person id. Return `[]` when nothing matches.

### 4.2 `/enrich` (person details)

Parse the TMDB id from `external_id` (strip the `tmdb:` prefix), then call TMDB
**Person › Details** (optionally with `external_ids` appended):

```
GET https://api.themoviedb.org/3/person/{id}?language=en-US&append_to_response=external_ids
```

Map TMDB person fields → canonical `fields` (each value an array of strings):

| Canonical field | TMDB source | Mapping notes |
|---|---|---|
| `bio` | `biography` | Single value. Trim to a sane length (Holodex caps at 4096 chars/value; prefer trimming on a sentence boundary). Omit the field entirely if empty |
| `birthdate` | `birthday` | `YYYY-MM-DD` string as TMDB returns it. Omit if null |
| `nationality` | `place_of_birth` | TMDB has no nationality field; derive a country/nationality from `place_of_birth` (e.g. last comma-segment → country). **Derivation is lossy — mark the precise rule TBD / confirm with Holodex maintainers.** If you cannot derive confidently, omit the field rather than guess |
| `website` | `homepage` | TMDB person details rarely include `homepage`; include only when present and non-empty |
| `aliases` | `also_known_as` | Array → array of strings directly. Include the native-script name (e.g. `宮崎駿`) as TMDB provides it. Feeds Holodex's Person aliases store |
| `photo` | `profile_path` | **Not a `fields` entry** — emit as an `assets[]` entry (see below) |
| (optional) `deathday` | `deathday` | **Not in the recommended v1 field set.** If you choose to expose it, propose a canonical key (e.g. `deathdate`) and confirm with Holodex maintainers so the label/precedence config can include it. Otherwise omit |
| (optional) `known_for_department` | `known_for_department` | Better used inside `disambiguation` than as a standalone field. Exposing it as a field is **TBD / confirm with Holodex maintainers** |

Omit any field whose TMDB value is null/empty rather than emitting an empty array.

### 4.3 Photo / `profile_path` → asset URL

TMDB returns `profile_path` as a path fragment (e.g. `/akhpeJSfFKMValElDDjsKi2jryl.jpg`).
Construct an absolute image URL using the TMDB image base. Recommended:

```
https://image.tmdb.org/t/p/original<profile_path>
```

(`original` is the full-size variant; a sized variant like `w500` is also acceptable.) The
robust approach is to fetch the TMDB **configuration** (`GET /3/configuration`) once at
startup and read `images.secure_base_url` + a `profile_sizes` entry, caching it — but the
stable `https://image.tmdb.org/t/p/` host is acceptable if you prefer not to call
configuration. Emit:

```json
{ "kind": "photo", "url": "https://image.tmdb.org/t/p/original/akhpeJSfFKMValElDDjsKi2jryl.jpg" }
```

Omit the `assets` array entirely when `profile_path` is null.

### 4.4 Auth & rate-limit handling (provider-owned)

- **Auth:** TMDB v3 accepts either the **API Read Access Token** (a bearer token, preferred:
  `Authorization: Bearer <token>`) or the legacy **API key** as a query param
  (`?api_key=<key>`). The container reads this from its own env (see [§7](#7-configuration)).
  It is **never** passed to or stored by Holodex, and **never** echoed in any response or log.
- **Rate-limit:** TMDB enforces rate limits and may return `429`. The container owns
  backoff/retry (respect `Retry-After`) and any short-lived caching of upstream responses.
  Because Holodex's per-call budget is **8 s**, retries must fit inside that window — prefer
  at most one quick retry, then return a non-2xx so Holodex fails the single call cleanly
  rather than hanging.

---

## 5. Non-functional requirements

These mirror the caps the Holodex client enforces — stay within them so nothing is silently
truncated.

| Requirement | Value / behaviour |
|---|---|
| **Per-call latency** | Answer each request well under **8 s** (Holodex's hard timeout). Budget your TMDB calls accordingly |
| **Response body size** | Keep every response under **1 MiB** (Holodex reads at most 1 MiB). Trim long bios; cap candidate lists |
| **Candidate count** | Return a small ranked list (≤ ~10). Holodex hard-caps at **25** |
| **Values per field** | ≤ **50** per field (Holodex cap); realistically 1–few |
| **Field count** | ≤ **40** fields (Holodex cap); v1 person set is ~6 |
| **Value length** | Each value ≤ **4096 chars** (Holodex truncates beyond). Prefer trimming bios yourself on a clean boundary |
| **Statelessness** | The container holds no Holodex-side state. It MAY cache TMDB responses internally, but Holodex persists the enrichment; restarting the container must not lose anything Holodex relies on |
| **No secrets in output** | The TMDB token/key MUST NOT appear in any response body, `/healthz`, `/describe`, error message, or log line |
| **Graceful upstream outage** | On TMDB outage/timeout/`5xx`/`429`-exhausted, return a non-2xx (e.g. `502`/`503`) **within the timeout**. Holodex maps it to a single failed call and shows the owner a generic error; it never blocks page loads or other providers |
| **No newlines in values** | Holodex uses newline as its multi-value separator and strips control chars; do not rely on embedded newlines surviving. Send multi-value data as separate array elements |

---

## 6. Security requirements

| ID | Requirement |
|---|---|
| S1 | **API credential only via env/secret.** The TMDB token/key is read from an environment variable (Docker/Compose secret). It is never hardcoded in the image, baked into a layer, committed, logged, or returned in any response |
| S2 | **No SSRF amplification.** The container calls **only** the fixed TMDB API + image hosts. It must not take a URL from the request and fetch it; `external_id`/`query` are used only to build TMDB API calls against known hosts |
| S3 | **Respond directly — never redirect Holodex.** Holodex refuses cross-host redirects and treats a 30x to another host as the final (failing) response. Return your JSON with a 2xx directly; do not 30x Holodex to TMDB or anywhere else |
| S4 | **Sanitize/validate TMDB responses.** Treat upstream TMDB data as untrusted: validate types, coerce to the contract shapes, drop unexpected fields, and keep values within the caps in [§5](#5-non-functional-requirements). (Holodex also sanitizes on its side — strips control characters, caps lengths/counts — but the container should not rely solely on that) |
| S5 | **Bounded responses.** Keep every response under 1 MiB and cap list sizes, so Holodex's 1 MiB read never truncates a body mid-JSON |
| S6 | **No request amplification / open proxy.** The container exposes only the four contract endpoints; it is not a general fetch proxy. Reject unknown paths/methods |
| S7 | **Asset URLs are TMDB image hosts only.** Any `assets[].url` must point at a TMDB image host and be directly fetchable (no redirect), since Holodex (when it later downloads) enforces size/content-type limits and the same no-cross-host-redirect rule |

---

## 7. Configuration

The container is configured entirely by environment variables (no config file required).

| Env var | Required | Default | Purpose |
|---|---|---|---|
| `TMDB_API_TOKEN` | yes (this **or** `TMDB_API_KEY`) | — | TMDB v3 **Read Access Token** (bearer). Preferred |
| `TMDB_API_KEY` | alternative | — | Legacy TMDB v3 API key (query param). Used only if `TMDB_API_TOKEN` is unset |
| `PORT` | no | `9100` | Port the HTTP server listens on (Holodex's example compose uses `9100`) |
| `HOST` / bind address | no | `0.0.0.0` (in-container) | Bind address. **TBD / confirm naming** — pick a conventional one (`HOST` or `BIND_ADDR`) |
| `LOG_LEVEL` | no | `info` | Log verbosity (`debug`/`info`/`warn`/`error`). Logs MUST never include the token/key |
| `TMDB_LANGUAGE` | no | `en-US` | Optional language for TMDB calls (`language=` param) |

- **Exposed port:** the HTTP server listens on `PORT` (default `9100`).
- **Healthcheck:** define a container `HEALTHCHECK` hitting `GET /healthz` and expecting
  `200` (e.g. every 30 s). This drives both Compose health and Holodex's `/status` view.
- The container **must not** require any Holodex env var or volume; it shares nothing with
  Holodex except the contract over the network.

---

## 8. Reference contract examples

Concrete request/response pairs for a sample person (Hayao Miyazaki, TMDB id 608).

### `GET /healthz`

```http
GET /healthz
```
```json
{ "status": "ok", "provider": "tmdb", "version": "1.0.0" }
```

### `GET /describe`

```http
GET /describe
```
```json
{
  "provider": "tmdb",
  "version": "1.0.0",
  "protocol_version": 1,
  "entity_types": ["person"],
  "id_namespaces": ["tmdb", "imdb"],
  "fields": ["bio", "birthdate", "nationality", "website", "aliases", "photo"]
}
```

### `POST /resolve` (name search)

```http
POST /resolve
Content-Type: application/json

{ "entity_type": "person", "hint": { "query": "Hayao Miyazaki" } }
```
```json
{
  "candidates": [
    {
      "external_id": "tmdb:608",
      "namespace": "tmdb",
      "label": "Hayao Miyazaki",
      "confidence": 0.98,
      "disambiguation": "Directing · Spirited Away · 1941"
    }
  ]
}
```

A no-match query returns:

```json
{ "candidates": [] }
```

### `POST /enrich`

```http
POST /enrich
Content-Type: application/json

{ "entity_type": "person", "external_id": "tmdb:608" }
```
```json
{
  "fields": {
    "bio": ["Hayao Miyazaki is a Japanese film director, producer, screenwriter, animator, author, and manga artist. A co-founder of Studio Ghibli…"],
    "birthdate": ["1941-01-05"],
    "nationality": ["Japan"],
    "aliases": ["宮崎駿", "Miyazaki Hayao", "Hayao Miyazaki"]
  },
  "assets": [
    { "kind": "photo", "url": "https://image.tmdb.org/t/p/original/akhpeJSfFKMValElDDjsKi2jryl.jpg" }
  ]
}
```

(`website` is omitted here because TMDB returned no `homepage`; omit rather than send empty.)

---

## 9. Testing / conformance

### Reference implementation to mirror

Holodex ships a **dependency-free Node reference stub** that implements this exact contract
with canned data. It is the closest worked example of the four endpoints and their shapes —
the TMDB container should produce byte-compatible response shapes. Its behaviour (reproduced
here so this spec stays self-contained):

- `GET /healthz` → `{ "status": "ok", "provider": "<name>", "version": "…" }`
- `GET /describe` → manifest with `protocol_version: 1`, `entity_types: ["person"]`,
  `id_namespaces: ["tmdb","imdb"]`, and a `fields` list.
- `POST /resolve` → reads `body.hint.query`; returns one candidate
  (`external_id: "tmdb:608"`, label `"Hayao Miyazaki"`, `confidence`, `disambiguation`)
  for a query that substring-matches a known name, else `{ "candidates": [] }`.
- `POST /enrich` → returns `{ "fields": { bio, birthdate, nationality, website, aliases } }`,
  with at least one CJK alias to exercise font/encoding (tofu) handling.

### Conformance smoke test

A passing TMDB container should satisfy each of these (run with a valid `TMDB_API_TOKEN`):

1. **Health** — `GET /healthz` returns `200` and `provider: "tmdb"`.
2. **Describe** — `GET /describe` returns `200`, `protocol_version: 1`, `entity_types`
   contains `"person"`, and a non-empty `fields` list.
3. **Resolve hit** — `POST /resolve` with `{ "entity_type":"person", "hint":{"query":"Hayao Miyazaki"} }`
   returns `200` with ≥1 candidate whose `external_id` starts with `tmdb:` and whose
   `namespace` is `tmdb`.
4. **Resolve miss** — a nonsense query returns `200` with `{ "candidates": [] }` (no error).
5. **Enrich** — `POST /enrich` with the `external_id` from step 3 returns `200`, a `fields`
   object whose values are **arrays of strings**, and (when TMDB has a `profile_path`) an
   `assets` entry with `kind: "photo"` and an absolute `https://image.tmdb.org/…` URL.
6. **Enrich bad id** — `POST /enrich` with a bogus `external_id` returns a non-2xx (not a
   crash, not a 2xx with garbage).
7. **Caps** — no response exceeds 1 MiB; no single value exceeds 4096 chars; candidate list
   is small.
8. **No secrets** — grep responses and logs: the TMDB token/key never appears.
9. **Timeout** — every endpoint answers within 8 s even under a slow upstream (the container
   bounds its own TMDB calls and returns a non-2xx rather than hanging).

A reasonable acceptance gate is to point a Holodex instance at the container
(`metadata-sources.yaml` entry, `enabled: true`), run an end-to-end enrich of one Person,
and confirm the bio/birthdate/aliases render with "from tmdb" provenance.

---

## 10. Deliverable & operator wiring

### Deliverable

1. A **container image** (published to a registry of the team's choosing) exposing the four
   contract endpoints on `PORT` (default `9100`), configured via the env vars in
   [§7](#7-configuration), with a `HEALTHCHECK` on `/healthz`.
2. Minimal docs: the image name/tag, required env (`TMDB_API_TOKEN`), and the compose
   snippet below.

### Operator wiring (how a Holodex operator drops it in)

**(a) Run the sidecar in the same Compose project as Holodex**, on the shared network:

```yaml
services:
  holodex-tmdb:
    image: <registry>/holodex-tmdb:1.0.0
    environment:
      TMDB_API_TOKEN: ${TMDB_API_TOKEN}   # operator's own token, from .env / secret
      PORT: "9100"
    # internal only — no host port needs to be published; Holodex reaches it by service name
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://127.0.0.1:9100/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3
```

**(b) Allowlist it in Holodex** by adding one entry to `metadata-sources.yaml` (the
`base_url` is the **SSRF allowlist** — Holodex only ever dials this host):

```yaml
sources:
  - name: tmdb
    base_url: http://holodex-tmdb:9100   # the Compose service name on the internal network
    entity_types: [person]
    enabled: true
```

**(c) Activate** without restarting Holodex: `POST /api/v1/admin/reload-config` (with the
admin token if one is set). The owner-only "Enrich from tmdb" action then appears on Person
pages.

The provider's TMDB token lives **only** in the sidecar's env — it never enters
`metadata-sources.yaml`, Holodex config, Holodex logs, or the Holodex read-model.

---

## References

These are the Holodex internal documents this spec conforms to. The external team does
**not** need them — everything required is reproduced above — but they are the source of
truth if a clarification is needed:

- **F22 spec** — Metadata Source Plugins (`docs/specs/metadata-plugins.md`): provider
  protocol, registry/allowlist, shadow store, People v1 slice.
- **ADR-033** — Metadata source plugins: sidecar providers over a unified resolution layer
  (`docs/architecture/ADR-033-metadata-source-plugins.md`): the sidecar decision, SSRF
  perimeter, untrusted-response handling, on-demand-only posture.
- **Reference stub** — `testdata/enrich-stub/` (Node, dependency-free): the worked
  contract example mirrored in [§9](#9-testing--conformance).

### Open items flagged for Holodex maintainers

- **`confidence` formula** for `/resolve` — TMDB has no native match score ([§4.1](#41-resolve-name-search)); Holodex doesn't threshold on it, so any monotonic 0–1 value is fine, but confirm if a specific scheme is wanted.
- **`nationality` derivation** from `place_of_birth` is lossy ([§4.2](#42-enrich-person-details)); confirm the exact country-extraction rule (or whether to omit when uncertain).
- **`deathday` / `known_for_department`** as standalone canonical fields ([§4.2](#42-enrich-person-details)) — not in the recommended v1 set; propose canonical keys if desired.
- **`/healthz` upstream signal** — whether to reflect TMDB reachability or stay a pure liveness check ([§2.1](#21-get-healthz--liveness--readiness)).
- **Bind-address env var name** ([§7](#7-configuration)) — pick a convention (`HOST` vs `BIND_ADDR`).
