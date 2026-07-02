# Spec: Holodex Metadata Provider Contract

**Status**: Draft (hand-off spec for external provider implementers)
**Feature block**: F22 (Metadata Source Plugins) — the provider-agnostic contract
**Audience**: Any team building a metadata-provider sidecar for Holodex, in **any language**, in a **separate repository**
**Implements**: The Holodex provider HTTP contract defined by [F22 / ADR-033](#references) — protocol version **1**

> **Self-contained by design.** This document specifies everything a provider container
> must do without any access to the Holodex source tree. The endpoints, request/response
> schemas, headers, timeouts, and size caps below are the **exact** contract the Holodex
> client enforces; conform to them and a container drops in with no Holodex code change.
> This spec is **source-neutral** — it describes the protocol, not any particular upstream
> (IMDb, TMDB, MusicBrainz, Wikidata, a local CSV, …). For a fully worked example that
> maps a real upstream onto this contract, see the TMDB provider spec
> ([`docs/specs/tmdb-provider.md`](tmdb-provider.md)).

---

## 1. Overview & purpose

### What a provider is

A **Holodex metadata provider** is a standalone HTTP/JSON service (typically a sidecar
container) that Holodex calls to enrich local entity records — initially **People** — with
data the media files do not carry: bios, birthdates, nationality, websites, aliases, and a
portrait photo. A provider translates **your** upstream data source into Holodex's small,
provider-agnostic contract. Holodex never learns anything source-specific; it only speaks
the four-endpoint protocol below.

You can build a provider over anything: a public web API, a licensed dataset, an internal
service, or a static file. The only requirement is that it answers the contract.

### How Holodex consumes a provider (architecture context)

Holodex runs as a single Go binary plus optional sidecar containers on an internal network.
A provider is **just another source feeding a unified field-resolution layer**:

- **Shadow store + provenance.** Provider-fetched values land in a *shadow enrichment
  store* kept separate from file-extracted metadata. The two are merged at display time by a
  per-field **precedence** list, and every value is badged with its provenance ("from
  `<provider>`" vs "from file"). A provider is never authoritative; it is one ranked source.
- **Owner-gated.** Enrichment is locked behind the Holodex admin gate. Only an authenticated
  owner can trigger a `/resolve` or `/enrich`. Providers themselves are reached only over the
  trusted internal network and are not exposed to end users.
- **On-demand only.** Every fetch is an explicit owner action (a click). There is **no
  crawler, scheduler, or background sync** in v1. Your provider is called when, and only
  when, an owner asks to enrich a specific entity.
- **SSRF-allowlisted.** Holodex only ever dials the exact `base_url` an operator configured
  for the provider. It **never** dials a URL derived from a provider response, a file, or
  user input, and it **refuses to follow a redirect to a different host**. (See
  [§6](#6-security-requirements).)
- **Provider owns its upstream.** The provider holds its own upstream credentials, rate
  limiting, backoff, caching, and parsing. Holodex holds **none** of your upstream secrets.

The container must be **stateless** with respect to Holodex — Holodex persists everything
it needs; restarting your container loses nothing Holodex relies on.

---

## 2. The HTTP contract

A provider MUST expose exactly four endpoints. All request and response bodies are
`application/json`. The contract is **protocol version 1**.

### 2.0 Transport behaviour the Holodex client enforces

These constraints come from the calling client; a provider must stay within them.

| Constraint | Value | Notes |
|---|---|---|
| Request methods / paths | `GET /healthz`, `GET /describe`, `POST /resolve`, `POST /enrich` | Exactly these four; other methods/paths are unused. Reject unknown paths/methods |
| Request headers sent by Holodex | `Accept: application/json`; `Content-Type: application/json` on POSTs | **No auth header is sent to the provider** — the provider is reached only over the trusted internal network. Do not depend on Holodex authenticating to you |
| Per-call timeout | **8 seconds** | Holodex aborts any single call (incl. `/healthz`, `/describe`, `/resolve`, `/enrich`) at 8 s. Answer well within this; do your own upstream calls on a tighter budget |
| Response body cap | **1 MiB** | Holodex reads at most 1 MiB of any response body and decodes that as JSON. Keep responses small (cap candidates, trim long text — see [§5](#5-non-functional-requirements)) |
| Success status | `2xx` | Any non-2xx is treated as a failed call (see error handling below) |
| Redirects | Cross-host 30x is **not followed** (treated as the final response); ≤5 same-host hops are followed | Respond **directly** with the JSON body and a 2xx; never redirect Holodex to another host |
| Request body (inbound) | Tiny (well under 64 KiB) | Holodex request bodies are small; bound the bodies you accept anyway |

**Error handling contract.** A non-2xx status, a transport error (timeout, connection
refused), or a malformed/oversized JSON body causes Holodex to fail **that single
enrichment call only** — it never crashes Holodex and never affects other providers.
Holodex surfaces a generic `502 Bad Gateway` ("provider lookup failed" / "enrichment
failed") to its owner UI and logs a warning. A provider should therefore return a clean
non-2xx (e.g. `502`/`503`) on upstream trouble and a well-formed 2xx JSON body on success;
it must never hang past 8 s.

### 2.1 `GET /healthz` — liveness / readiness

Holodex polls this for the `/status` page (provider health) and as a reachability check.

**Request:** `GET /healthz` (no body).

**Response `200`:**

```json
{ "status": "ok", "provider": "<name>", "version": "1.0.0" }
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `status` | string | yes | `"ok"` when healthy. Holodex treats any 2xx as healthy; the string is for display |
| `provider` | string | yes | The provider's id, lowercase — must match the configured `name` and the `provider` in `/describe` |
| `version` | string | yes | The container's own version string (display only) |

`/healthz` MUST NOT require the upstream data source to be reachable to return `200` (it is
a liveness check of the container). A degraded-upstream signal, if desired, is **TBD /
confirm with Holodex maintainers** — v1 Holodex only checks for a 2xx. No secrets in this body.

### 2.2 `GET /describe` — capability manifest

Holodex calls this before `/resolve` and `/enrich` to verify the protocol version and the
supported entity types. A mismatched **major protocol version** makes Holodex refuse the
provider loudly.

**Request:** `GET /describe` (no body).

**Response `200`:**

```json
{
  "provider": "<name>",
  "version": "1.0.0",
  "protocol_version": 1,
  "entity_types": ["person"],
  "id_namespaces": ["<name>"],
  "fields": ["bio", "birthdate", "nationality", "website", "aliases"],
  "asset_kinds": ["photo"]
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `provider` | string | yes | Provider id, lowercase |
| `version` | string | yes | Container version (display) |
| `protocol_version` | integer | yes | **MUST be `1`.** Holodex refuses any other major version |
| `entity_types` | string[] | yes | The entity types you support. v1: `["person"]` (see [§3](#3-entity-types-and-matching)) |
| `id_namespaces` | string[] | yes | The external-ID namespaces you understand (see [§4.1](#41-external-ids-and-namespaces)). Usually `["<name>"]`; include more if you can resolve foreign IDs (e.g. an `imdb` id) |
| `fields` | string[] | yes | The canonical **text** field keys you can supply (see [§4.2](#42-canonical-fields)). **Do not list `photo` here** — a portrait is an *asset*, not a field; advertise it in `asset_kinds` |
| `asset_kinds` | string[] | optional | The binary asset kinds you can supply (see [§4.3](#43-assets)). v1 person kinds: `"photo"`, `"banner"`, `"poster"`. Omit if you supply no assets. **Backward compat:** a provider may instead still list `photo` in `fields` during the deprecation window — Holodex treats that as `asset_kinds: ["photo"]` — but new providers SHOULD use `asset_kinds` |
| `credits` | boolean | optional | `true` when a `video`/`media` enrich response can include the structured **`people`** array (per-cast/crew person references + headshots — see [§4.5](#45-video-credits--per-person-castcrew-with-headshots)). Omit/`false` for flat `actors`/`director` text only. Additive — does not change the `person` entity contract |

### 2.3 `POST /resolve` — identity match (disambiguation)

Given a name query and/or embedded external IDs, return **ranked candidate matches** for the
owner to confirm. Holodex always shows the owner a picker and never auto-applies a candidate
in v1, so `confidence` is advisory.

**Request body** (exact shape Holodex sends):

```json
{ "entity_type": "person", "hint": { "query": "Ada Lovelace", "external_ids": ["wikidata:Q7259"] } }
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `entity_type` | string | yes | `"person"` in v1 |
| `hint` | object | yes | The identity input |
| `hint.query` | string | optional | Free-text name to search (the dominant path for People). Omitted/empty when only IDs are given |
| `hint.external_ids` | string[] | optional | Namespace-qualified IDs (e.g. `"wikidata:Q7259"`) for a deterministic path. **Note:** Holodex's v1 People flow sends only `query` (from the owner's search box). Implement the `external_ids` path for forward compatibility, but `query` is what v1 exercises |

**Response `200`:**

```json
{
  "candidates": [
    {
      "external_id": "<name>:1234",
      "namespace": "<name>",
      "label": "Ada Lovelace",
      "confidence": 0.97,
      "disambiguation": "Mathematician · 1815–1852"
    }
  ]
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `candidates` | array | yes | Ranked best-first. May be empty (`[]`) for a no-match — Holodex renders a "no results" state, **not** an error. **Holodex caps the list at 25**; return fewer (e.g. top 5–10) |
| `candidates[].external_id` | string | yes | Provider-stable, **namespace-qualified** id (e.g. `"<name>:1234"`). This is the exact value Holodex echoes back to `/enrich` |
| `candidates[].namespace` | string | yes | The id namespace (the prefix before the first `:`) |
| `candidates[].label` | string | yes | Human-readable name shown in the picker. Holodex sanitizes (strips control chars, caps 4096 chars) |
| `candidates[].confidence` | number | optional | 0–1 advisory score for ranking. Holodex does not threshold on it in v1 (it always asks the owner to confirm) |
| `candidates[].disambiguation` | string | optional | Short distinguishing line to separate same-named entities. Sanitized/capped by Holodex |

### 2.4 `POST /enrich` — fetch fields

Given a chosen `external_id`, return the canonical field values plus optional asset URLs.

**Request body** (exact shape Holodex sends):

```json
{ "entity_type": "person", "external_id": "<name>:1234" }
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `entity_type` | string | yes | `"person"` in v1 |
| `external_id` | string | yes | A namespace-qualified id previously returned by `/resolve`. **Note:** the request carries `external_id` but **not** a separate `namespace` — parse the namespace off the prefix (everything before the first `:`). A bare/unknown id should yield a non-2xx |

**Response `200`:**

```json
{
  "fields": {
    "bio": ["English mathematician, regarded as the first computer programmer."],
    "birthdate": ["1815-12-10"],
    "nationality": ["British"],
    "aliases": ["Augusta Ada King", "Ada Byron"]
  },
  "assets": [
    { "kind": "photo", "url": "https://cdn.example.org/portraits/ada.jpg" }
  ]
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `fields` | object | yes | Map of **canonical field key → array of string values**. Always an array, even single-valued (e.g. `"birthdate": ["1815-12-10"]`). Keys SHOULD be drawn from the `fields` advertised in `/describe`. Holodex sanitizes every value (control chars stripped, newlines → space, capped 4096 chars), caps **50 values per field** and **40 fields total** |
| `assets` | array | optional | Binary assets (e.g. a portrait). **Omit when empty** — never send `[]` (mirrors the `fields` omit rule). Preference-ordered, most-preferred first within a kind. Full rules in [§4.3](#43-assets) |
| `assets[].kind` | string | yes (if asset present) | Asset kind, from the v1 enum: `"photo"` (portrait), `"banner"` (16:9), `"poster"` (2:3). Unknown kinds are **ignored** by Holodex (forward-compat) — see [§4.3](#43-assets) |
| `assets[].url` | string | yes (if asset present) | Absolute, directly-fetchable URL to the image (see [§4.3](#43-assets) for the scheme/host/credential rules) |
| `people` | array | optional | **`video`/`media` only** — structured cast/crew with optional per-person headshots, enabling Holodex to create/link real Person records (additive; see [§4.5](#45-video-credits--per-person-castcrew-with-headshots)). Omit for `person` enrichment or when returning flat `actors`/`director` text |

**Asset note (load-bearing).** Holodex **downloads** `assets` during the owner's enrich
action (the asset-storage follow-up has shipped — [ADR-038](../architecture/ADR-038-person-images.md)/[ADR-039](../architecture/ADR-039-provider-asset-urls.md)). It fetches each asset URL through the **same SSRF
perimeter** as the provider API calls, runs the bytes through an ingest normalizer
(decode → bound → **re-encode → strip all metadata**), and stores its own copy. Practical
consequences for you: (1) an asset's `url` host must be **allowlisted** — your own
`base_url` host, or a host the *operator* listed in `asset_hosts` (see [§4.3](#43-assets)/[§6](#6-security-requirements)); a cross-host
URL on an un-allowlisted host is refused; (2) Holodex serves a **normalized copy**, so EXIF,
animation, and transparency are **not** preserved — don't depend on them; (3) never put an
image into a `fields` entry expecting it to render (`fields` values are text). Full asset
rules, formats, sizes, and aspect guidance are in [§4.3](#43-assets).

### 2.5 Status codes summary

| Endpoint | Success | Failure the provider should return |
|---|---|---|
| `GET /healthz` | `200` | `503` if the container itself is not ready |
| `GET /describe` | `200` | — (static; should always succeed) |
| `POST /resolve` | `200` (incl. empty `candidates`) | `502`/`503` on upstream failure; `400` on a malformed request body |
| `POST /enrich` | `200` | `404`/`400` for an unknown/bad `external_id`; `502`/`503` on upstream failure |

Holodex maps **any** non-2xx to a generic owner-facing error and a warning log — exact codes
beyond "2xx vs not" are for the provider's own clarity/observability.

---

## 3. Entity types and matching

v1 supports **one** `entity_type`: the literal string **`"person"`**. Holodex sends this
verbatim in `/resolve` and `/enrich`. A provider should accept `"person"` and MAY return an
error for unknown entity types (Holodex will not send others in v1).

The contract is **designed to extend** to more entity types (e.g. `series`, `media`) without
a protocol change: advertise them in `entity_types`, and Holodex sends the matching
`entity_type` string. Do not hardcode assumptions that there is only ever one entity type if
you intend to grow — but you are free to support `person` only.

Matching is **embedded-ID-first, name-search fallback**:

- If the owner's entity carries a known external id (the `hint.external_ids` path), resolve
  it deterministically and return a single high-confidence candidate.
- Otherwise, name-search on `hint.query` and return ranked candidates for the owner to pick.

---

## 4. Field & identity mapping (source-neutral)

This section is where **your** upstream meets the contract. Holodex stays agnostic; you
decide how your data maps onto stable namespaces and canonical keys.

### 4.1 External IDs and namespaces

- An `external_id` is **`<namespace>:<id>`** — a stable namespace prefix, a colon, then an
  id that is stable in your source (e.g. `acme:998211`, `wikidata:Q7259`).
- Pick a **lowercase namespace** that is stable across releases. Usually it equals your
  provider `name`. If you can also resolve foreign ids (e.g. you accept an `imdb:` id and map
  it internally), advertise those namespaces too in `/describe.id_namespaces`.
- The `external_id` is the durable link Holodex stores to re-fetch the same record later, so
  it must be **stable and reversible**: the same input must resolve to the same id, and
  `/enrich` must accept any id your `/resolve` emitted.

### 4.2 Canonical fields

`fields` is a map of **canonical key → array of string values**. Canonical keys are how
Holodex labels, orders (precedence), and merges values across sources, so use the shared
vocabulary rather than your source's native names. The recommended v1 **person** keys:

| Canonical key | Meaning | Cardinality | Format guidance |
|---|---|---|---|
| `bio` | Short biography / description | single | Plain text. Trim to a sane length on a clean boundary (Holodex caps 4096 chars/value) |
| `birthdate` | Date of birth | single | `YYYY-MM-DD` preferred (partial dates acceptable if that is all you have) |
| `nationality` | Nationality / country | single or few | Plain text (e.g. `"British"`). Omit if you cannot derive it confidently |
| `website` | Official/home page | single or few | Absolute URL string. Holodex stores it as text |
| `aliases` | Alternate / native-script names | multi | One name per array element. Include native-script forms (e.g. CJK) directly — feeds Holodex's Person aliases store |
| `photo` | Portrait | — | **Not a `fields` entry** — emit as an `assets[]` entry with `kind: "photo"` and advertise it in `asset_kinds` (see [§4.3](#43-assets)) |

Rules:

- **Always arrays.** Even a single value is a one-element array (`"bio": ["…"]`).
- **Omit, don't empty.** If your source has no value for a field, omit the key entirely
  rather than sending `[]` or `[""]`.
- **No embedded newlines.** Holodex strips control characters and uses newline as a
  multi-value separator; send multi-value data as separate array elements, not as one
  newline-joined string.
- **New keys are allowed but coordinate them.** You MAY emit a key outside the recommended
  set, but it only renders/orders well once Holodex has a label + precedence entry for it.
  Propose new canonical keys to the Holodex maintainers (e.g. a `deathdate`) rather than
  inventing display-only keys that won't be configured. Mark any such key **TBD / confirm
  with Holodex maintainers** in your provider's own docs.
- **Reserved `_`-prefixed keys are internal sidecars.** A field key beginning with an
  underscore is provider→core **plumbing**: Holodex persists it in the shadow store like any
  other field but **never displays it** and **never resolves it** (it is not a canonical
  field). Use one only for a defined contract — do not invent your own. v1 defines
  **`_studio_external_ids`** (ADR-054): on a **video** enrich, one self-describing value
  `"<namespace>:<id> <name>"` per production company (e.g. `"tmdb:174 Warner Bros. Pictures"`),
  so Holodex can de-dup studio entities by provider company id. The id token has no space, so
  the name is the unambiguous remainder; emit only companies with a non-empty name and a real
  id. It rides the normal sanitize/caps and is aligned with the `studio` field.

### 4.3 Assets

An asset is a binary image Holodex downloads, normalizes, and stores against the entity.
Emit one `assets[]` entry per image:

```json
{ "kind": "photo", "url": "https://image.tmdb.org/t/p/original/<path>.jpg" }
```

The object has exactly two defined keys in v1; **Holodex ignores any other key** (so future
hints like `expires_at`/`width` can be added later without a protocol bump):

| Key | Type | Required | Notes |
|---|---|---|---|
| `kind` | string | yes | One of the v1 enum below. An **unknown kind is ignored** (the asset is skipped), so a provider may emit future kinds safely |
| `url` | string | yes | Absolute image URL meeting the scheme/host/credential rules below |

#### Asset kinds (v1 enum)

| `kind` | Meaning | Target aspect | Notes |
|---|---|---|---|
| `photo` | Person portrait / headshot | ~1:1 (square) | The common case. Synonyms `portrait`/`headshot` are also accepted |
| `banner` | Wide hero image | ~16:9 | Synonym `backdrop` accepted |
| `poster` | Tall poster | ~2:3 | |
| `gallery` | Additional photos | any | Multiple assets allowed per enrich — see ordering note below |

Holodex maps each kind to one image **role**; an unknown kind is **dropped** (never stored
under a guessed role). Holodex does **not** crop to the target aspect (cropping is a separate
owner action), so supply an image already close to the role's aspect to avoid letterboxing.

#### URL rules (what makes a URL fetchable)

- **Absolute**, and **directly fetchable** — respond `2xx` with the image bytes; **no
  cross-host redirect** (a 30x to another host is refused, [§6](#6-security-requirements)).
- **Scheme:** use **`https` for any cross-host (public-internet) URL** (e.g. a CDN like
  `image.tmdb.org`). Plain `http` is accepted **only** for your own `base_url` host on the
  trusted internal network.
- **Host must be allowlisted.** Holodex only fetches an asset whose host is either **your
  provider's `base_url` host** *or* a host the **operator** has listed in `asset_hosts` for
  your source (see [§10](#10-deliverable--operator-wiring)). Trust comes from operator config,
  never from your response — so if your images live on a CDN host different from your sidecar
  (the usual case), **tell operators which host(s) to allowlist** in your provider's docs.
- **No credentials in the URL.** Holodex fetches **without** auth and without your upstream
  token; the URL must be retrievable anonymously. Never embed your API key/signature-secret.
  Short-lived/**signed** CDN URLs are fine **if valid at enrich time** — Holodex fetches
  promptly (fetch-soon) and stores its own copy, so it does not persist or re-use your URL.

#### Image fitness (Holodex normalizes untrusted bytes)

Holodex treats your image as untrusted and runs every asset through a fixed pipeline —
**sniff type → bound dimensions/area → decode → re-encode → strip all metadata** — then
serves its own copy. Stay inside these so nothing is rejected or silently altered:

- **Format:** supply **JPEG, PNG, or GIF** (the formats Holodex decodes today). **WebP and AVIF
  are not currently decoded — do not send them.** **Raster only:** **no SVG, HTML, or PDF**
  (rejected as unsafe vector/markup formats). Holodex re-encodes whatever you send to JPEG.
- **Size:** each asset body is capped at **16 MiB**; oversized bodies are rejected. A portrait
  is realistically well under 2 MiB.
- **Dimensions:** Holodex bounds dimensions/pixel-area before decode (a decompression-bomb
  guard). Keep the **longest edge ≤ 4096 px** (≈1500 px is plenty for display); absurd
  dimensions are rejected.
- **Not preserved:** because Holodex re-encodes and strips, **EXIF/metadata, animation
  (animated GIF/WebP collapse to a still frame), and transparency (flattened)** do **not**
  survive. Don't rely on them.
- **`Content-Type`:** send a correct `image/*` content-type, but note Holodex **sniffs** the
  bytes and ignores the declared type/extension for safety.

#### Multiple assets, ordering, and emptiness

- You MAY return more than one asset. Order them **most-preferred first** within a `kind`.
- **Core roles** (`photo`, `banner`, `poster`): Holodex fetches the first asset of each role
  it can successfully store, then skips the rest of that role. Emit at most one per core kind.
- **`gallery` role**: Holodex fetches all `gallery` assets in order until an operator-configured
  cap is reached. You may include multiple `gallery` entries; they are stored as additional
  photos for the person.
- **Omit `assets` entirely when you have none** — never send `[]`.
- If a response approaches the 1 MiB body cap, **shed `assets` before dropping any `fields`**
  (fields are canonical text; an asset URL is recoverable on a later enrich).

### 4.4 Upstream credentials & rate limits (provider-owned)

- **Credentials** for your upstream (API keys, tokens) are read from the **container's own
  environment** (see [§7](#7-configuration)). They are **never** passed to, stored by, logged
  by, or returned to Holodex.
- **Rate limits / backoff** are entirely the provider's responsibility. Because Holodex's
  per-call budget is **8 s**, any retry must fit inside that window — prefer at most one quick
  retry, then return a non-2xx so Holodex fails the single call cleanly rather than hanging.
- Any caching of upstream responses is internal to the provider and must not be required for
  correctness (the container stays stateless w.r.t. Holodex).

### 4.5 Video credits — per-person cast/crew with headshots

> **Status: additive extension** for `video`/`media` enrichment (Holodex F30 "populate",
> [ADR-048](../architecture/ADR-048-metadata-curation-and-write-queue.md)). **Backward
> compatible:** a provider may keep returning cast/crew as flat text in `fields` (`actors`,
> `director`); Holodex still consumes those. The structured `people` array below is the
> richer, **opt-in** shape that lets Holodex create/link real **Person** records and download
> their **headshots**. Advertise support with `"credits": true` in `/describe`
> ([§2.2](#22-get-describe--capability-manifest)).

A `video`/`media` `/enrich` response MAY include a top-level **`people`** array alongside
`fields` and `assets`. Each entry is one cast or crew member:

```json
{
  "fields": { "title": ["Dune"], "genres": ["Science Fiction", "Adventure"] },
  "people": [
    { "name": "Timothée Chalamet", "role": "actor", "external_id": "tmdb:1190668",
      "order": 0, "headshot": { "kind": "photo", "url": "https://image.tmdb.org/t/p/original/x.jpg" } },
    { "name": "Denis Villeneuve", "role": "director", "external_id": "tmdb:137427" }
  ]
}
```

| Key | Type | Required | Notes |
|---|---|---|---|
| `people[].name` | string | yes | Display name. Holodex sanitizes (strips control chars, caps 4096) — it is the match key when `external_id` is absent |
| `people[].role` | string | yes | Credit role. v1 enum: `"actor"`, `"director"`, `"writer"`, `"producer"`, `"composer"`, `"crew"`. An **unknown role is stored generically** (never dropped) — forward-compatible |
| `people[].external_id` | string | optional | Namespace-qualified id (`<namespace>:<id>`, [§4.1](#41-external-ids-and-namespaces)). When present it is the **stable, deterministic** link Holodex stores; the same person across films de-duplicates to one record. Omit only if your source has no stable id |
| `people[].order` | integer | optional | Billing order within a role (0 = top-billed) for display ordering. Holodex caps the list |
| `people[].headshot` | object | optional | A single **asset object** ([§4.3](#43-assets)) — `{ "kind": "photo", "url": "…" }` — for that person's portrait. Subject to **all** the [§4.3](#43-assets)/[§6](#6-security-requirements) asset rules: allowlisted host (your `base_url` host or an operator `asset_hosts` entry), `https` cross-host, no credentials in the URL, raster JPEG/PNG/GIF, ≤16 MiB, ≤4096 px. Omit when you have none |

**How Holodex consumes it.** On the owner's video enrich, Holodex (a) resolve-or-creates a
Person per entry — keyed by `external_id` when given, else by normalized `name` — and links it
to the video with its `role`; (b) downloads each `headshot` through the **same SSRF perimeter**
as every other asset ([§4.3](#43-assets)) and stores it as that person's headshot. The flat
`fields.actors`/`fields.director` text remains the fallback for providers that don't emit
`people`, so emitting both is harmless (Holodex prefers `people` when present).

**Caps & degradation.** Holodex caps `people` (≈50 entries) and applies the per-value caps in
[§5](#5-non-functional-requirements) to `name`. If a response nears the 1 MiB body cap, shed
`people[].headshot` URLs before `fields` (a headshot is recoverable on a later enrich; canonical
text is not) — same precedence as `assets` in [§4.3](#43-assets).

---

## 5. Non-functional requirements

These mirror the caps the Holodex client enforces — stay within them so nothing is silently
truncated.

| Requirement | Value / behaviour |
|---|---|
| **Per-call latency** | Answer each request well under **8 s** (Holodex's hard timeout). Budget upstream calls accordingly |
| **Response body size** | Keep every response under **1 MiB** (Holodex reads at most 1 MiB). Trim long text; cap candidate lists |
| **Candidate count** | Return a small ranked list (≤ ~10). Holodex hard-caps at **25** |
| **Values per field** | ≤ **50** per field (Holodex cap); realistically 1–few |
| **Field count** | ≤ **40** fields (Holodex cap); v1 person set is ~6 |
| **Value length** | Each value ≤ **4096 chars** (Holodex truncates beyond). Prefer trimming text yourself on a clean boundary |
| **Statelessness** | The container holds no Holodex-side state. It MAY cache upstream responses internally, but Holodex persists the enrichment; restarting the container must not lose anything Holodex relies on |
| **No secrets in output** | Upstream credentials MUST NOT appear in any response body, `/healthz`, `/describe`, error message, or log line |
| **Graceful upstream outage** | On upstream outage/timeout/`5xx`/rate-limit-exhausted, return a non-2xx (e.g. `502`/`503`) **within the timeout**. Holodex maps it to a single failed call and shows the owner a generic error; it never blocks page loads or other providers |
| **No newlines in values** | Holodex uses newline as its multi-value separator and strips control chars; send multi-value data as separate array elements |
| **Asset body size** | Each downloaded asset ≤ **16 MiB** (Holodex rejects beyond). A portrait is realistically < 2 MiB |
| **Asset dimensions** | Longest edge ≤ **4096 px** (Holodex bounds dimensions/area before decode — a decompression-bomb guard); ≈1500 px is plenty for display |
| **Asset format** | **JPEG/PNG/GIF** decoded today (**not** WebP/AVIF); **raster only — no SVG/HTML/PDF**. Holodex re-encodes to JPEG + strips metadata, so EXIF/animation/transparency are not preserved (see [§4.3](#43-assets)) |

---

## 6. Security requirements

| ID | Requirement |
|---|---|
| S1 | **Upstream credentials only via env/secret.** Any upstream key/token is read from an environment variable (Docker/Compose secret). It is never hardcoded in the image, baked into a layer, committed, logged, or returned in any response |
| S2 | **No SSRF amplification.** The container calls **only** the fixed upstream host(s) it is built for. It must **not** take a URL from the request (or from upstream data) and fetch it; `external_id`/`query` are used only to build calls against your known hosts. The provider is not a general fetch proxy |
| S3 | **Respond directly — never redirect Holodex.** Holodex refuses cross-host redirects and treats a 30x to another host as the final (failing) response. Return your JSON with a 2xx directly; do not 30x Holodex elsewhere |
| S4 | **Treat upstream data as untrusted.** Validate types, coerce to the contract shapes, drop unexpected fields, and keep values within the caps in [§5](#5-non-functional-requirements). (Holodex also sanitizes on its side — strips control characters, caps lengths/counts — but do not rely solely on that; defense in depth) |
| S5 | **Bounded responses.** Keep every response under 1 MiB and cap list sizes, so Holodex's 1 MiB read never truncates a body mid-JSON |
| S6 | **Minimal surface.** Expose only the four contract endpoints. Reject unknown paths/methods. Do not add a debug/proxy/exec endpoint to the same service |
| S7 | **Asset URLs point only at allowlisted hosts.** Holodex downloads `assets[].url` through the same SSRF perimeter as the API: it fetches **only** a host that is your `base_url` host **or** one the operator listed in `asset_hosts` for your source — trust comes from operator config, never from your response. The URL must be **directly fetchable** (no cross-host redirect) and use **`https` for any cross-host host** (`http` only for your own internal `base_url` host). Holodex caps the body (16 MiB), bounds dimensions, sniffs the type, and re-encodes/strips metadata. See [§4.3](#43-assets) |
| S8 | **No credentials in asset URLs.** An `assets[].url` must be fetchable **anonymously** — never embed your upstream API key, token, or signing secret. (Signed CDN URLs that are valid at enrich time are fine; Holodex fetches promptly and stores its own copy.) |
| S9 | **Assets are raster images only.** Emit JPEG/PNG (or WebP/GIF); **never** SVG/HTML/PDF or other markup/vector formats — Holodex rejects them as an unsafe parse/script surface |

> **Why these matter.** Holodex deliberately treats every provider as a semi-trusted source
> on an internal network: it allowlists your `base_url`, refuses redirects off that host,
> caps and sanitizes everything you return, and never hands you its own secrets. A provider
> that follows S1–S7 stays inside that perimeter; one that (say) proxies arbitrary URLs or
> echoes its upstream key would punch a hole in it. When in doubt, do less.

---

## 7. Configuration

A provider is configured entirely by environment variables (no Holodex-side config file).
Exact names are the provider's choice; recommended conventions:

| Env var | Required | Default | Purpose |
|---|---|---|---|
| `<UPSTREAM>_API_TOKEN` / `<UPSTREAM>_API_KEY` | as needed | — | Upstream credential, if your source needs one. Read from env/secret only (S1) |
| `PORT` | no | `9100` | Port the HTTP server listens on (Holodex examples use `9100`) |
| `HOST` / bind address | no | `0.0.0.0` (in-container) | Bind address. **TBD / confirm naming** — pick a conventional one (`HOST` or `BIND_ADDR`) |
| `LOG_LEVEL` | no | `info` | Log verbosity. Logs MUST never include credentials |

- **Exposed port:** the HTTP server listens on `PORT` (default `9100`).
- **Healthcheck:** define a container `HEALTHCHECK` hitting `GET /healthz` and expecting
  `200` (e.g. every 30 s). This drives both Compose health and Holodex's `/status` view.
- The container **must not** require any Holodex env var or volume; it shares nothing with
  Holodex except the contract over the network.

---

## 8. Reference contract examples

Concrete request/response pairs for a sample person, using a placeholder provider named
`acme`. (These are illustrative shapes, not real data.)

### `GET /healthz`

```http
GET /healthz
```
```json
{ "status": "ok", "provider": "acme", "version": "1.0.0" }
```

### `GET /describe`

```http
GET /describe
```
```json
{
  "provider": "acme",
  "version": "1.0.0",
  "protocol_version": 1,
  "entity_types": ["person"],
  "id_namespaces": ["acme"],
  "fields": ["bio", "birthdate", "nationality", "website", "aliases"],
  "asset_kinds": ["photo"]
}
```

### `POST /resolve` (name search)

```http
POST /resolve
Content-Type: application/json

{ "entity_type": "person", "hint": { "query": "Ada Lovelace" } }
```
```json
{
  "candidates": [
    {
      "external_id": "acme:998211",
      "namespace": "acme",
      "label": "Ada Lovelace",
      "confidence": 0.97,
      "disambiguation": "Mathematician · 1815–1852"
    }
  ]
}
```

A no-match query returns (still `200`):

```json
{ "candidates": [] }
```

### `POST /enrich`

```http
POST /enrich
Content-Type: application/json

{ "entity_type": "person", "external_id": "acme:998211" }
```
```json
{
  "fields": {
    "bio": ["English mathematician and writer, known for her work on Babbage's Analytical Engine; regarded as the first computer programmer."],
    "birthdate": ["1815-12-10"],
    "nationality": ["British"],
    "aliases": ["Augusta Ada King", "Ada Byron", "Countess of Lovelace"]
  },
  "assets": [
    { "kind": "photo", "url": "https://cdn.example.org/portraits/ada-lovelace.jpg" }
  ]
}
```

(`website` is omitted because the source had none — omit rather than send empty.)

---

## 9. Testing / conformance

### Reference implementation to mirror

Holodex ships a **dependency-free Node reference stub** (`testdata/enrich-stub/`) that
implements this exact contract with canned data. It is the closest worked example of the four
endpoints and their shapes; a provider should produce response shapes compatible with it. Its
behaviour:

- `GET /healthz` → `{ "status": "ok", "provider": "<name>", "version": "…" }`
- `GET /describe` → manifest with `protocol_version: 1`, `entity_types: ["person"]`, an
  `id_namespaces` list, and a `fields` list.
- `POST /resolve` → reads `body.hint.query`; returns one candidate for a query that
  substring-matches a known name, else `{ "candidates": [] }`.
- `POST /enrich` → returns `{ "fields": { … } }` with at least one CJK alias to exercise
  font/encoding handling.

### Conformance smoke test

A conforming provider should satisfy each of these:

1. **Health** — `GET /healthz` returns `200` with `provider` equal to your configured name.
2. **Describe** — `GET /describe` returns `200`, `protocol_version: 1`, `entity_types`
   contains `"person"`, and a non-empty `fields` list. If you supply portraits, `asset_kinds`
   contains `"photo"` (and `photo` is **not** in `fields`) — or, transitionally, `photo`
   appears in `fields`.
3. **Resolve hit** — `POST /resolve` with a known name returns `200` with ≥1 candidate whose
   `external_id` is `<namespace>:<id>` and whose `namespace` matches the prefix.
4. **Resolve miss** — a nonsense query returns `200` with `{ "candidates": [] }` (no error).
5. **Enrich** — `POST /enrich` with the `external_id` from step 3 returns `200`, a `fields`
   object whose values are **arrays of strings**, and (when you have a portrait) an `assets`
   entry with `kind: "photo"` and an absolute, directly-fetchable URL. The `photo` is **not**
   present as a `fields` key.
6. **Enrich bad id** — `POST /enrich` with a bogus `external_id` returns a non-2xx (not a
   crash, not a 2xx with garbage).
7. **Caps** — no response exceeds 1 MiB; no single value exceeds 4096 chars; the candidate
   list is small.
8. **No secrets** — grep responses and logs: no upstream credential ever appears.
9. **Timeout** — every endpoint answers within 8 s even under a slow upstream (bound your own
   upstream calls and return a non-2xx rather than hanging).
10. **No-photo omits assets** — `POST /enrich` for a person with no portrait **omits** the
    `assets` key entirely (does not send `[]`).
11. **Asset URL fetchable** — the asset `url` is absolute, retrievable **anonymously** (no
    embedded credential), `https` if cross-host, and resolves with **no cross-host redirect**;
    its host is your `base_url` host or one you document for operators to put in `asset_hosts`.
12. **Asset is a bounded raster** — the asset is a JPEG/PNG (not SVG/HTML/PDF), under 16 MiB,
    with a longest edge ≤ 4096 px.
13. **Degradation order** — under an oversized response, `assets` is shed before any `fields`
    value.

A reasonable acceptance gate is to point a Holodex instance at the container
(`metadata-sources.yaml` entry, `enabled: true`), run an end-to-end enrich of one Person, and
confirm the fields render with "from `<name>`" provenance.

---

## 10. Deliverable & operator wiring

### Deliverable

1. A **container image** exposing the four contract endpoints on `PORT` (default `9100`),
   configured via environment variables, with a `HEALTHCHECK` on `/healthz`.
2. Minimal docs: the image name/tag, required env, and the compose snippet below.

### Operator wiring (how a Holodex operator drops it in)

**(a) Run the sidecar in the same Compose project as Holodex**, on the shared network:

```yaml
services:
  holodex-acme:
    image: <registry>/holodex-acme:1.0.0
    environment:
      ACME_API_TOKEN: ${ACME_API_TOKEN}   # operator's own credential, from .env / secret
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
  - name: acme
    base_url: http://holodex-acme:9100   # the Compose service name on the internal network
    entity_types: [person]
    asset_hosts: [cdn.acme.example]      # OPTIONAL: extra image hosts to allow (see below)
    enabled: true
```

**`asset_hosts` (asset download allowlist).** Holodex fetches an `assets[].url` only from
your `base_url` host *or* a host listed here — this is the **operator's** explicit trust
decision, never taken from your provider response. **If your portraits live on a CDN host
different from your sidecar** (e.g. a TMDB-backed provider serving from `image.tmdb.org`),
the operator **must** add that host here or the photo fetch is refused. So your provider docs
should state exactly which image host(s) to list. Omit `asset_hosts` if you serve images from
your own `base_url` host (then no extra config is needed). Hosts are matched **exactly** (no
wildcards).

**(c) Activate** without restarting Holodex: `POST /api/v1/admin/reload-config` (with the
admin token if one is set). The owner-only "Enrich from `<name>`" action then appears on the
relevant entity pages.

Any upstream credential lives **only** in the sidecar's env — it never enters
`metadata-sources.yaml`, Holodex config, Holodex logs, or the Holodex read-model.

---

## References

These are the Holodex internal documents this contract derives from. An external team does
**not** need them — everything required is reproduced above — but they are the source of
truth if a clarification is needed:

- **F22 spec** — Metadata Source Plugins ([`docs/specs/metadata-plugins.md`](metadata-plugins.md)):
  provider protocol, registry/allowlist, shadow store, People v1 slice.
- **ADR-033** — Metadata source plugins ([`docs/architecture/ADR-033-metadata-source-plugins.md`](../architecture/ADR-033-metadata-source-plugins.md)):
  the sidecar decision, SSRF perimeter, untrusted-response handling, on-demand-only posture.
- **ADR-038** — Person images ([`docs/architecture/ADR-038-person-images.md`](../architecture/ADR-038-person-images.md)):
  the on-disk image store and the ingest normalizer (decode → bound → re-encode → strip) that
  every downloaded asset passes through.
- **ADR-039** — Provider asset URLs ([`docs/architecture/ADR-039-provider-asset-urls.md`](../architecture/ADR-039-provider-asset-urls.md)):
  the asset object schema, `asset_kinds` advertisement, and the operator-configured
  `asset_hosts` download allowlist this section ([§4.3](#43-assets)) specifies.
- **Worked example** — TMDB provider spec ([`docs/specs/tmdb-provider.md`](tmdb-provider.md)):
  this same contract mapped onto a real upstream (TMDB), with a concrete field-mapping table.
- **Reference stub** — `testdata/enrich-stub/` (Node, dependency-free): the worked contract
  example mirrored in [§9](#9-testing--conformance).

### Open items flagged for Holodex maintainers

- **`confidence` semantics** for `/resolve` — Holodex does not threshold on it in v1, so any
  monotonic 0–1 value is acceptable; confirm if a specific scheme is ever required.
- **New canonical field keys** ([§4.2](#42-canonical-fields)) — any key outside the
  recommended person set needs a Holodex label + precedence entry to render/order well;
  propose the key rather than inventing display-only keys.
- **`/healthz` upstream signal** ([§2.1](#21-get-healthz--liveness--readiness)) — whether it
  should reflect upstream reachability or stay a pure container-liveness check.
- **Bind-address env var name** ([§7](#7-configuration)) — pick a convention (`HOST` vs
  `BIND_ADDR`).
- **Additional entity types** ([§3](#3-entity-types-and-matching)) — `series`/`media` are
  designed-in but not yet exercised; coordinate the canonical field vocabulary before
  shipping a non-person provider.
