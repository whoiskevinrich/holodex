# Spec: TMDB Metadata Provider Sidecar

**Status**: Accepted
**Feature block**: F22 (Metadata Source Plugins) — first real provider container
**Audience**: Implementers of the TMDB sidecar at **`providers/tmdb/`** in this repository ([ADR-040](../architecture/ADR-040-tmdb-provider-repo-placement.md))
**Implements**: The Holodex provider HTTP contract defined by [F22 / ADR-033](#references) — protocol version **1**. This is a **worked example** of the source-neutral [Metadata Provider Contract](metadata-provider-contract.md); that document is the generic spec, this one maps it onto TMDB.

> **Self-contained by design.** This document specifies everything the container
> must do. The endpoints, request/response schemas, headers, timeouts, and size caps
> below are the **exact** contract the Holodex client enforces; conform to them and the
> container drops in with no Holodex code change.

---

## 1. Overview & purpose

### What this is

A **Holodex metadata provider sidecar** is a standalone HTTP/JSON service that Holodex
calls to enrich local entities with data the media files do not carry. This spec covers the
**TMDB** (The Movie Database) provider, which supports three entity types:

- **People** (`entity_type: "person"`) — bios, birthdates, nationality, websites, aliases, and a portrait photo.
- **Films / Video** (`entity_type: "video"`) — title, overview, release date, runtime, genres, tagline, a homepage link (the film's TMDB page), original language/title, status, IMDb ID, poster URL, **studio(s)**, top-billed **actors**, and **director(s)**.
- **Studios** (`entity_type: "studio"`, F38 S3) — production-company `description`, origin `country`, `website` (the company homepage, TMDB page as fallback), and a `logo` image URL. Matched via `/3/search/company`, enriched via `/3/company/{id}`. The logo is a plain `image_url` **field** (the `poster_url` pattern), not a downloaded asset.

The container translates Holodex's small, provider-agnostic contract into calls against the public TMDB API and maps the responses back into Holodex's canonical enrichment fields.

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
  "entity_types": ["person", "video", "studio"],
  "id_namespaces": ["tmdb", "imdb"],
  "fields": [
    "bio", "birthdate", "nationality", "deathdate", "website", "aliases",
    "title", "overview", "release_date", "runtime", "genres", "tagline", "homepage",
    "original_language", "original_title", "status", "imdb_id", "poster_url",
    "actors", "director", "studio",
    "description", "country", "logo"
  ],
  "asset_kinds": ["headshot", "gallery", "banner"]
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `provider` | string | yes | Provider id, lowercase — `"tmdb"` |
| `version` | string | yes | Container version (display) |
| `protocol_version` | integer | yes | **MUST be `1`.** Holodex refuses any other major version |
| `entity_types` | string[] | yes | `["person", "video", "studio"]` — person for People enrichment, video for film enrichment, studio for production-company enrichment (see [§2.3](#23-entity-type-and-matching-fields)) |
| `id_namespaces` | string[] | yes | The external-ID namespaces understood — `["tmdb", "imdb"]` (TMDB exposes both) |
| `fields` | string[] | yes | The canonical fields the provider can supply — see [§4](#4-tmdb-specific-field-mapping). Do **not** include `photo` here; advertise it in `asset_kinds` instead |
| `asset_kinds` | string[] | yes (ADR-039) | Asset kinds this provider returns in `assets[]`. TMDB supplies person images: `["headshot", "gallery", "banner"]` (person only; video poster is a text `fields` entry) |

### 2.3 Entity type and matching fields

The TMDB provider supports three `entity_type` values:

- **`"person"`** — People enrichment: name search against `/3/search/person`, details from `/3/person/{id}`, photo asset.
- **`"video"`** — Film enrichment: title search against `/3/search/movie`, details from `/3/movie/{id}`, poster stored as a text URL field (`poster_url`).
- **`"studio"`** — Studio/company enrichment (F38 S3): name search against `/3/search/company`, details from `/3/company/{id}`, logo stored as a text URL field (`logo`). See [§4.5](#45-studio--company-enrichment-f38-s3).

The same `external_id` namespace (`tmdb:NNN`, `imdb:tt…`) is used for all entity types; the `entity_type` field in the request disambiguates which TMDB resource to look up. Holodex sends one or the other and the container dispatches accordingly. Unknown entity types should return `200` with `{"candidates":[]}` for resolve, or a non-2xx for enrich.

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
    { "kind": "headshot", "url": "https://image.tmdb.org/t/p/original/akhpeJSfFKMValElDDjsKi2jryl.jpg" },
    { "kind": "gallery",  "url": "https://image.tmdb.org/t/p/original/secondprofile.jpg" },
    { "kind": "banner",   "url": "https://image.tmdb.org/t/p/original/backdrop.jpg" }
  ]
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `fields` | object | yes | Map of **canonical field key → array of string values**. Always an array, even single-valued (e.g. `"birthdate": ["1941-01-05"]`). Keys SHOULD be drawn from the `fields` advertised in `/describe`. Holodex sanitizes every value (control chars stripped, newlines → space, capped 4096 chars), caps **50 values per field** and **40 fields total** |
| `assets` | array | optional | Binary assets. **See [§4.3](#43-person-photos--asset-urls) and the asset note below** |
| `assets[].kind` | string | yes (if asset present) | Asset kind: `"headshot"` (primary portrait), `"gallery"` (additional portraits), or `"banner"` (landscape backdrop). See [§4.3](#43-person-photos--asset-urls) |
| `assets[].url` | string | yes (if asset present) | Absolute URL to the binary |

**Asset note (load-bearing).** Holodex **downloads** assets synchronously at enrich time
(ADR-038/039). Each URL is fetched, normalized (decode → strip EXIF → re-encode), and
stored as a person image immediately when the owner clicks Enrich. Do **not** put photos
into `fields` entries — `fields` values are text; photos belong in `assets[]`.
Asset URLs MUST be absolute, on `image.tmdb.org` (an operator-allowlisted CDN host per
ADR-039), served over `https`, and directly fetchable without a cross-host redirect — see
[§6](#6-security-requirements) and [§10](#10-deliverable--operator-wiring).

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

The container translates TMDB's People and Movie APIs into the canonical contract fields. Use
TMDB API **v3** (REST). Auth is via the provider's own credential (see [§7](#7-configuration)).
Sections 4.1–4.3 cover **person** enrichment; §4.4 covers **movie/video** enrichment.

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
| `confidence` | derived from search rank and `popularity` | Rank-based: `1.0 − rank × 0.08` (first result = 1.0, second = 0.92, …), clamped to ≥0.1. Apply a +0.05 boost when `popularity > 10` if the result is not already ≥0.95. Holodex does not threshold on this value — any monotonic 0–1 score is acceptable |
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
| `bio` | `biography` | Single value. Trim to ≤4000 chars at a sentence boundary (Holodex hard-caps at 4096). Omit the field entirely if empty |
| `birthdate` | `birthday` | `YYYY-MM-DD` string as TMDB returns it. Omit if null |
| `nationality` | `place_of_birth` | Pass `place_of_birth` as-is (e.g. `"Tokyo, Japan"` or `"Bunkyo, Tokyo, Japan"`). Do not attempt country extraction — it is lossy and Holodex operators can read the full value. Omit if null |
| `deathdate` | `deathday` | `YYYY-MM-DD` string as TMDB returns it. Omit if null. Canonical key is `deathdate` |
| `website` | `homepage` | TMDB person details rarely include `homepage`; include only when present and non-empty |
| `aliases` | `also_known_as` | Array → array of strings directly. Include the native-script name (e.g. `宮崎駿`) as TMDB provides it. Feeds Holodex's Person aliases store |
| `photo` | `profile_path` | **Not a `fields` entry.** Emit as an `assets[]` entry with `kind: "photo"` (see [§4.3](#43-photo--profile_path--asset-url)) |
| `known_for_department` | `known_for_department` | Use inside `disambiguation` string, not as a standalone field |

Omit any field whose TMDB value is null/empty rather than emitting an empty array.

### 4.3 Person photos → asset URLs

TMDB exposes three sources of person imagery, fetched concurrently alongside the details
call. Details are required; image calls are best-effort (a failure yields an empty result,
not an error).

| TMDB endpoint | Purpose |
|---|---|
| `GET /3/person/{id}` | Details, including `profile_path` as a fallback |
| `GET /3/person/{id}/images` | All profile photos sorted by `vote_average` descending |
| `GET /3/person/{id}/tagged_images?language=en-US&page=1` | Images from films/shows the person appears in |

**Building the asset list** (cap the total at **20** assets):

1. Use `profiles[]` from `/images`. If the call fails or returns empty, fall back to `profile_path` from the details response as a single-entry list. If both are empty, emit no assets.
2. First profile → `"headshot"`. Remaining profiles → `"gallery"`. Both use `https://image.tmdb.org/t/p/original<file_path>`.
3. Scan `results[]` from `/tagged_images`. The first entry with `aspect_ratio >= 1.5` (landscape) → one `"banner"` asset. Stop after finding one banner.
4. Emit assets in order: headshot first, gallery entries next, banner last.

```json
[
  { "kind": "headshot", "url": "https://image.tmdb.org/t/p/original/akhpeJSfFKMValElDDjsKi2jryl.jpg" },
  { "kind": "gallery",  "url": "https://image.tmdb.org/t/p/original/secondprofile.jpg" },
  { "kind": "banner",   "url": "https://image.tmdb.org/t/p/original/backdrop.jpg" }
]
```

Omit `assets` entirely only when there are no profile photos and no fallback `profile_path`.

### 4.4 Film / Video enrichment

#### 4.4a `/resolve` (movie search)

For `hint.query`, call TMDB **Search › Movie**:

```
GET https://api.themoviedb.org/3/search/movie?query=<urlencoded>&language=en-US&page=1
```

Map each result to a candidate — same shape as person candidates but using movie fields:

| Candidate field | From TMDB search result | Notes |
|---|---|---|
| `external_id` | `"tmdb:" + id` | TMDB movie id (integer) |
| `namespace` | `"tmdb"` | constant |
| `label` | `title` | The movie's title |
| `confidence` | same rank formula as person | `1.0 − rank × 0.08` + popularity boost |
| `disambiguation` | release year (first 4 chars of `release_date`) | e.g. `"1999"` |

For `hint.external_ids` — if a `tmdb:NNN` id is present, call **Movie › Details** directly and return a single high-confidence candidate. If an `imdb:tt…` id is present, call TMDB **Find** (`GET /3/find/{imdb_id}?external_source=imdb_id`) and use `movie_results[]`.

#### 4.4b `/enrich` (movie details + credits)

Parse the TMDB id from `external_id` (strip the `tmdb:` prefix), then fetch **Movie › Details**
and **Movie › Credits** — **concurrently, and both are required** (a failure of either fails the
enrich; unlike person photos, credits is not best-effort):

```
GET https://api.themoviedb.org/3/movie/{id}?language=en-US
GET https://api.themoviedb.org/3/movie/{id}/credits?language=en-US
```

Map the responses → canonical `fields` (each value an array of strings):

| Canonical field | TMDB source | Notes |
|---|---|---|
| `title` | details `title` | Single value, trimmed. Omit if empty |
| `overview` | details `overview` | Single value. Trim to ≤4000 chars at a sentence boundary (Holodex caps 4096). Omit if empty |
| `release_date` | details `release_date` | `YYYY-MM-DD` string. Omit if empty |
| `runtime` | details `runtime` | Integer minutes, serialized as a string (e.g. `"139"`). Omit if 0 |
| `genres` | details `genres[].name` | Multi-value — one element per genre (drop empty names) |
| `tagline` | details `tagline` | Single value, trimmed. Omit if empty |
| `homepage` | *(derived)* the movie's **TMDB page** URL — `https://www.themoviedb.org/movie/{id}-{slug}` | **Not** TMDB's `homepage` field. TMDB `homepage` is the studio's own marketing site (often short-lived or region-gated); the film's TMDB page is the provider's durable record and the more useful destination. Always emitted (the id resolves even when the title slug is empty) |
| `original_language` | details `original_language` | BCP-47 code, e.g. `"en"`. Omit if empty |
| `original_title` | details `original_title` | Emitted **only** when non-empty and different from `title` (avoid redundancy) |
| `status` | details `status` | e.g. `"Released"`. Omit if empty |
| `imdb_id` | details `imdb_id` | e.g. `"tt0137523"`. Omit if empty |
| `poster_url` | details `poster_path` | **Text field** (not an asset). Absolute URL `https://image.tmdb.org/t/p/original` + `poster_path`. Holodex renders it as an `<img>` in the Film Details panel. Omit when `poster_path` is null |
| `studio` | details `production_companies[].name` | Multi-value — one per company (drop empty names) |
| `actors` | credits `cast[].name` | Top **10** by billing order (TMDB returns `cast` pre-sorted). Drop empty names |
| `director` | credits `crew[]` where `job == "Director"` | Multi-value (co-directors). Drop empty names |
| `_studio_external_ids` | details `production_companies[].{id, name}` | **Internal sidecar** — see the contract's [§4.6](metadata-provider-contract.md#46-studio-external-ids-_studio_external_ids). One self-describing value `"tmdb:<id> <name>"` per company with a non-empty name **and** `id > 0`, paired with `studio`. **Not advertised in `/describe`** and never displayed or resolved — it powers studio-entity de-dup by company id (HOLODEX-122 / [ADR-054](../architecture/ADR-054-studio-external-id-dedup.md)). Omit when no company has an id |

Omit any field whose TMDB value is null/empty rather than emitting an empty array.

**No `assets[]` for movies.** The poster is a text `fields` entry (`poster_url`), not an asset download — there is no film poster sink that maps to a stored image slot (unlike person photos which map to the headshot role). Holodex renders the URL directly as an image in the UI.

### 4.5 Studio / Company enrichment (F38 S3)

Studio enrichment mirrors the person/movie shape onto TMDB **production companies**. It adds
no new provider host, asset download, or SSRF surface — the logo is a plain field URL on the
same `image.tmdb.org` host as posters/photos, rendered client-side (never fetched by the core).

#### 4.5a `/resolve` (company search)

For `hint.query`, call TMDB **Search › Company**:

```
GET https://api.themoviedb.org/3/search/company?query=<urlencoded>&page=1
```

Company search takes **no `language` param** and returns **no popularity**, so confidence is
purely rank-based. Map each result (cap 10) to a candidate:

| Candidate field | From TMDB search result | Notes |
|---|---|---|
| `external_id` | `"tmdb:" + id` | TMDB company id (integer) |
| `namespace` | `"tmdb"` | constant |
| `label` | `name` | The company name |
| `confidence` | rank formula, popularity = 0 | `1.0 − rank × 0.08` (floored at 0.1) |
| `disambiguation` | `origin_country` | e.g. `"US"`, `"JP"` — the picker hint (may be empty) |

For `hint.external_ids` — if a `tmdb:NNN` id is present (a video's `_studio_external_ids`
sidecar hands it straight through, [ADR-054](../architecture/ADR-054-studio-external-id-dedup.md)),
call **Company › Details** directly and return one high-confidence candidate.

#### 4.5b `/enrich` (company details)

Parse the TMDB id from `external_id`, then fetch **Company › Details**:

```
GET https://api.themoviedb.org/3/company/{id}
```

Map the response → canonical `fields` (each value an array of strings):

| Canonical field | TMDB source | Notes |
|---|---|---|
| `description` | details `description` | Single value. Trim to ≤4000 chars at a sentence boundary. **Often empty upstream** — omit when so |
| `country` | details `origin_country` | e.g. `"US"`. Omit if empty |
| `website` | details `homepage`, else *(derived)* the company's TMDB page `https://www.themoviedb.org/company/{id}-{slug}` | Prefer the official homepage; fall back to the durable TMDB page so a link is always present (mirrors the person/movie website behaviour). Always emitted |
| `logo` | details `logo_path` | **Text `image_url` field** (not an asset). Absolute URL `https://image.tmdb.org/t/p/original` + `logo_path`. Holodex renders it as an `<img>`. Omit when `logo_path` is null |

Omit any field whose TMDB value is null/empty rather than emitting an empty array.

**No `assets[]` for studios.** The logo is a text `fields` entry (`logo`), not an asset
download — studios are not on the F25 person-image path (spec Non-Goal / P2-3). Holodex renders
the URL directly, exactly like the film `poster_url`.

### 4.6 Auth & rate-limit handling (provider-owned)

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
| `HOST` | no | `""` (all interfaces) | Bind address. Empty string binds all interfaces inside the container, which is correct for a compose sidecar |
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
  "fields": ["bio", "birthdate", "nationality", "website", "aliases", "deathdate"],
  "asset_kinds": ["headshot", "gallery", "banner"]
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
    { "kind": "headshot", "url": "https://image.tmdb.org/t/p/original/akhpeJSfFKMValElDDjsKi2jryl.jpg" },
    { "kind": "gallery",  "url": "https://image.tmdb.org/t/p/original/secondprofile.jpg" },
    { "kind": "banner",   "url": "https://image.tmdb.org/t/p/original/backdrop.jpg" }
  ]
}
```

(`website` is omitted because TMDB returned no `homepage`; omit rather than send empty.
`gallery` and `banner` are present when TMDB's `/images` and `/tagged_images` return
additional photos — omit any asset for which no source image exists.)

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
   object whose values are **arrays of strings**, and (when TMDB has profile images) one or
   more `assets` entries: the first with `kind: "headshot"`, any additional profiles with
   `kind: "gallery"`, and optionally one `kind: "banner"` from tagged landscape images. All
   asset URLs are absolute `https://image.tmdb.org/…` URLs.
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
    entity_types: [person, video]        # person = People pages; video = Media detail pages
    asset_hosts: [image.tmdb.org]        # TMDB CDN host for portrait downloads (ADR-039)
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

### Decisions made

All previously-open items were resolved when building `providers/tmdb/`:

- **`confidence` formula** — rank-based (`1.0 − rank × 0.08`) with popularity bonus; documented in [§4.1](#41-resolve-name-search).
- **`nationality`** — pass `place_of_birth` as-is; no country extraction; documented in [§4.2](#42-enrich-person-details).
- **`deathdate`** — included as a v1 field; documented in [§4.2](#42-enrich-person-details).
- **`/healthz`** — pure liveness (container health only, not upstream TMDB); documented in [§2.1](#21-get-healthz--liveness--readiness).
- **Bind address** — env var `HOST`, default `""` (all interfaces); documented in [§7](#7-configuration).
- **`asset_hosts`** — `image.tmdb.org` listed in `metadata-sources.yaml`; documented in [§8.d](#8-reference-contract-examples).
