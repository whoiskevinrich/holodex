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
  "asset_kinds": ["photo"],
  "brand_icon": { "url": "https://<name>.example/brand-icon.png" },
  "preferred_search_pattern": "{studio?} {title?} {performers?} {year?}"
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `provider` | string | yes | Provider id, lowercase |
| `version` | string | yes | Container version (display) |
| `protocol_version` | integer | yes | **MUST be `1`.** Holodex refuses any other major version |
| `entity_types` | string[] | yes | The entity types you support: any of `"person"`, `"video"`, `"studio"` (see [§3](#3-entity-types-and-matching)). A minimal provider may advertise `["person"]` only |
| `id_namespaces` | string[] | yes | The external-ID namespaces you understand (see [§4.1](#41-external-ids-and-namespaces)). Usually `["<name>"]`; include more if you can resolve foreign IDs (e.g. an `imdb` id) |
| `fields` | string[] | yes | The canonical **text** field keys you can supply (see [§4.2](#42-canonical-fields)). **Do not list `photo` here** — a portrait is an *asset*, not a field; advertise it in `asset_kinds` |
| `asset_kinds` | string[] | optional | The binary asset kinds you can supply (see [§4.3](#43-assets)). v1 person kinds: `"photo"`, `"banner"`, `"poster"`. Omit if you supply no assets. **Backward compat:** a provider may instead still list `photo` in `fields` during the deprecation window — Holodex treats that as `asset_kinds: ["photo"]` — but new providers SHOULD use `asset_kinds` |
| `credits` | boolean | optional | `true` when a `video`/`media` enrich response can include the structured **`people`** array (per-cast/crew person references + headshots — see [§4.5](#45-video-credits--per-person-castcrew-with-headshots)). Omit/`false` for flat `actors`/`director` text only. Additive — does not change the `person` entity contract |
| `field_hints` | object | optional | Per-field presentation hints (label / render mode / order) for **non-canonical** advertised keys, so they render first-class with **no** per-operator config — see [§4.7](#47-field-render-hints-describefield_hints). Keyed by field key; omit entirely if you have none. Additive (unknown key, ignored by older Holodex) |
| `brand_icon` | object | optional | Your provider's **brand icon** — an [asset object](#43-assets) `{ "url": "…" }` Holodex downloads, normalizes, self-hosts, and shows in place of the repeated "from `<name>`" provenance text. One provider-level image, **not** a per-entity asset. Subject to the full [§4.3](#43-assets)/[§6](#6-security-requirements) asset rules (allowlisted host, https cross-host, no credentials, ≤16 MiB, ≤4096 px). Omit if you have none — Holodex falls back to a monogram. See [§4.8](#48-provider-brand-icon-describebrand_icon). Additive (unknown key, ignored by older Holodex) |
| `preferred_search_pattern` | string | optional | **`video` only.** A search-query shape you'd like Holodex to build `/resolve`'s `hint.query` from instead of the raw/sanitized title — see [§4.9](#49-preferred-search-query-pattern-describepreferred_search_pattern). Consulted only when the *operator* hasn't configured their own override for you (operator config always wins). Malformed/unparseable → ignored (logged on the Holodex side), never an error to you. Omit if you have no opinion — the sanitized-title fallback already applies unconditionally either way. Additive (unknown key, ignored by older Holodex) |

### 2.3 `POST /resolve` — identity match (disambiguation)

Given a name query and/or embedded external IDs, return **ranked candidate matches** for the
owner to confirm.

> **Auto-apply (F47 / [ADR-066](../architecture/ADR-066-enrichment-auto-apply-and-dismissal.md)
> D1) — amends the earlier v1 posture.** This section previously stated "Holodex always shows the
> owner a picker and never auto-applies a candidate in v1, so `confidence` is advisory." That is no
> longer true: when a `/resolve` call returns **exactly one** candidate at or above Holodex's
> internal strong-match threshold (**`0.85`**), the Holodex client applies it immediately with no
> picker shown. Any other outcome — zero candidates, two-or-more at/above the threshold, or only
> lower-confidence ones — still stops at the owner's picker exactly as before, so ambiguity is
> never resolved automatically. Practical consequence for you: `confidence` is no longer purely
> advisory display — a well-calibrated score now determines whether a match applies with one click
> or waits for owner review, so favor a conservative score when a match is genuinely uncertain. The
> threshold itself is **not** part of the wire contract: it is not sent to or read from providers,
> not versioned, and this amendment is **documentation-only** — `candidates[].confidence`'s field
> shape, 0–1 range, and provider-native/non-normalized semantics are unchanged, so this does not
> bump `protocol_version`. An already-conformant provider needs no code change.

**Request body** (exact shape Holodex sends):

```json
{ "entity_type": "person", "hint": { "query": "Ada Lovelace", "external_ids": ["wikidata:Q7259"] } }
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `entity_type` | string | yes | `"person"`, `"video"`, or `"studio"` — whichever you advertised in `entity_types` (see [§3](#3-entity-types-and-matching)) |
| `hint` | object | yes | The identity input |
| `hint.query` | string | optional | Free-text name to search (the dominant path for People). Omitted/empty when only IDs are given. **For `video`:** the content may be a shaped query built from `studio`/`title`/`performers`/`year` rather than a bare title — see [§4.9](#49-preferred-search-query-pattern-describepreferred_search_pattern). The field shape itself never changes — it's always this one string either way |
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
      "disambiguation": "Mathematician · 1815–1852",
      "profile_url": "https://acme.example/people/998211-ada-lovelace"
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
| `candidates[].confidence` | number | optional | 0–1 score, provider-native and non-normalized — see the auto-apply note above: a lone candidate at/above `0.85` applies without owner confirmation, so a well-calibrated score now has a real behavioral effect, not just display |
| `candidates[].disambiguation` | string | optional | Short distinguishing line to separate same-named entities. Sanitized/capped by Holodex |
| `candidates[].profile_url` | string | optional | Absolute link to your own page for this candidate (e.g. a person/company profile page), so the owner can verify a match against your richer page instead of the picker's three-field summary (F47/RD6). Rendered as a "view source ↗" link, opened in a new tab, when present. **Must be `http`/`https`** — Holodex scheme-validates server-side and silently drops any other scheme or a malformed URL before it reaches the client (no error, the candidate itself is still usable). Omit if you have none — don't send an empty string |

### 2.4 `POST /enrich` — fetch fields

Given a chosen `external_id`, return the canonical field values plus optional asset URLs.

**Request body** (exact shape Holodex sends):

```json
{ "entity_type": "person", "external_id": "<name>:1234" }
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `entity_type` | string | yes | `"person"`, `"video"`, or `"studio"` — whichever you advertised in `entity_types` (see [§3](#3-entity-types-and-matching)) |
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

This spec's v1 text originally specified a single `entity_type`, `"person"`. **That has since
been extended additively** (no protocol-version bump — see §4.5/§4.6): Holodex today also
sends `entity_type: "video"` (a file/media entity) and `entity_type: "studio"` for providers
that advertise them. All three are live, exercised entity types — not just designed-in future
ones. Advertise whichever you support in `/describe.entity_types` (e.g.
`["person", "video", "studio"]`); Holodex sends only the `entity_type` strings you advertised.
`"series"`/`"media"` beyond the file-per-video model remain designed-in but unexercised.

**Films are not yet a provider entity type — but the design is decided.** Holodex
F56/[ADR-085](../architecture/ADR-085-films-entity.md) added a fourth entity, **Film** — a
durable, owner-asserted grouping of videos (e.g. scenes of the same production) with its own
detail page. In v1 a film's membership is created entirely in the Holodex UI; there is no
`entity_type: "film"` a provider can send or receive yet, and no `/resolve`/`/enrich` traffic for
films exists today. A film's name does, however, compete as a **synthetic, non-provider** decision
source for a linked video's `collection` (the "Film" field,
[§4.2a](#42a-canonical-fields--videomedia)) and `title` fields, through the ordinary
field-decision/precedence UI — see the reserved-namespace note in
[§4.1](#41-external-ids-and-namespaces). Provider-driven film enrichment (matching a Film to an
upstream source) is **live** as of [ADR-086](../architecture/ADR-086-film-provider-enrichment.md):
a film gets its **own `entity_type: "film"`** (never a reuse of `video`) with its own canonical
field vocabulary ([§4.2c](#42c-canonical-fields--film)), and its poster is an **asset** (the
existing `poster` kind, [§4.3](#43-assets)), not a `fields` entry. The shipped TMDB provider
advertises `film` and serves `/resolve` + `/enrich` for it.

> **A film's canonical vocabulary is narrower than what a movie-shaped provider will naturally
> send.** Holodex consumes only `description` and `release_date` from a film `/enrich` response
> today ([§4.2c](#42c-canonical-fields--film)), plus the `poster` asset. Keys such as `title`,
> `studio`, `actors` and `director` are accepted and stored, but a film's title is owner-asserted
> identity (not provider-writable) and its cast/studio are derived from the videos attached to it
> — so those keys are **not** applied to the film. Sending them is harmless; expecting them to
> land is not. Widening this is tracked as F59
> ([film-provider-enrichment-ux.md](film-provider-enrichment-ux.md),
> [ADR-089](../architecture/ADR-089-film-enrichment-field-vocabulary.md)).

**Video and studio each have their own canonical field vocabulary** — see [§4.2a](#42a-canonical-fields--videomedia)
(video) and [§4.2b](#42b-canonical-fields--studio) (studio). Critically, **a video's poster and
a studio's logo are `fields` entries (`poster_url`/`logo`), never `assets[]` entries** — v1 has
**no non-person image sink**. See the entity-type-scoping callout in [§4.3](#43-assets).

A provider MAY choose to support `person` only, `video` only, any combination, or all three. Do
not hardcode assumptions that there is only ever one entity type if you intend to support more
than one.

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
- **The id is mandatory and is the identity** ([ADR-055](../architecture/ADR-055-enrichment-unique-key-invariant.md)).
  Every record you resolve or enrich — a `/resolve` candidate, an `/enrich` target, and **every**
  `people[]` credit ([§4.5](#45-video-credits-people)) — MUST carry a well-formed `<namespace>:<id>`.
  Holodex uses it as the **sole identity/de-dup key** for the entity; there is **no name fallback**. A
  record with an empty or malformed id is **refused**, not matched by name. If your source has no stable
  id, synthesize a deterministic one (e.g. a stable hash of a canonical key) and advertise its namespace —
  a source with no stable identity cannot be safely de-duplicated.
- Pick a **lowercase namespace** that is stable across releases. Usually it equals your
  provider `name`. You MUST emit ids only in namespaces you advertise in `/describe.id_namespaces`
  (Holodex validates this). If you can also resolve foreign ids (e.g. you accept an `imdb:` id and map
  it internally), advertise those namespaces too — a **namespace is a shared identity space**: two
  providers that both emit `imdb:tt1160419` refer to the **same** entity and Holodex converges them to one.
- **`film:` is a reserved namespace prefix — do not use it.** Holodex's internal Films entity
  (F56/[ADR-085](../architecture/ADR-085-films-entity.md)) injects synthetic per-video decision
  sources named `film:<film-id>` so an owner-asserted film can compete for a video's
  `collection`/`title` fields exactly like a real provider ([§3](#3-entity-types-and-matching)). A
  provider whose own `name`/`id_namespaces` began with `film:` would collide with that reserved
  prefix and risk being misread internally as a film source. Real provider namespaces never need a
  `:`-suffixed numeric id, so this should never come up in practice — but avoid it regardless.
- The `external_id` is the durable link Holodex stores to re-fetch the same record later, so
  it must be **stable and reversible**: the same input must resolve to the same id, and
  `/enrich` must accept any id your `/resolve` emitted.

> **Enforcement is rolling out** (HOLODEX-124 perimeter validation, HOLODEX-125 person identity). Update
> your provider to emit ids everywhere now; the name-fallback path is being removed.

### 4.2 Canonical fields

`fields` is a map of **canonical key → array of string values**. Canonical keys are how
Holodex labels, orders (precedence), and merges values across sources, so use the shared
vocabulary rather than your source's native names. The recommended v1 **person** keys:

| Canonical key | Meaning | Cardinality | Format guidance |
|---|---|---|---|
| `bio` | Short biography / description | single | Plain text. Trim to a sane length on a clean boundary (Holodex caps 4096 chars/value) |
| `birthdate` | Date of birth | single | `YYYY-MM-DD` preferred (partial dates acceptable if that is all you have) |
| `nationality` | Nationality / country | single or few | Plain text (e.g. `"British"`, or a place of birth `"London, England, United Kingdom"`). Omit if you cannot derive it confidently. **Flag hint (HOLODEX-139):** Holodex derives a small country flag beside the person's name from this value, so **put the country last** in a place-of-birth string (it reads the segment after the final comma) — a plain nationality word (`"French"`) or bare country (`"Japan"`) also works. Unrecognized values simply render no flag |
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
- **Reserved `_`-prefixed keys are internal sidecars — do not invent your own.** A `fields`
  key beginning with an underscore (`_`) is a **defined provider→core plumbing channel**, not a
  displayable field: Holodex persists it in the shadow store like any other field but **never
  renders it** in the UI and **never resolves it** into a canonical value. You **MUST NOT** coin
  your own `_`-prefixed keys — emit only the ones this contract defines, exactly as specified,
  and a provider that omits them stays fully conformant. v1 defines one: **`_studio_external_ids`**
  for studio de-dup — see [§4.6](#46-studio-external-ids-_studio_external_ids).

### 4.2a Canonical fields — video/media

> **Status: additive extension**, exercised today (not merely designed-in — see [§3](#3-entity-types-and-matching)).

`fields` for a `video`/`media` `entity_type` follows the same shape as [§4.2](#42-canonical-fields)
(canonical key → array of string values) but with its own vocabulary:

| Canonical key | Meaning | Cardinality | Format guidance |
|---|---|---|---|
| `title` | Primary title | single | Plain text |
| `original_title` | Title in original language | single | Plain text |
| `overview` | Synopsis | single | Plain text, `render: long_text` |
| `tagline` | Marketing tagline | single | Plain text |
| `release_date` | Release date | single | `YYYY-MM-DD` preferred |
| `runtime` | Runtime in minutes | single | Integer as string |
| `genres` | Genre list | multi | One genre per array element |
| `status` | Release status | single | Plain text (e.g. `"Released"`) |
| `original_language` | Original language | single | ISO 639-1 code preferred (e.g. `"en"`) |
| `homepage` | Official/home page | single | Absolute URL, `render: url` |
| `external_provider_id` | External metadata-provider identifier | single | Namespace-qualified `"<provider>:<id>"`, e.g. `"imdb:tt1160419"` — [ADR-082](../architecture/ADR-082-external-provider-id-namespace-qualified-value.md) |
| `poster_url` | Poster / cover art | single | Absolute image URL, `render: image_url`. **This is a `fields` entry — never an `assets[]` entry.** Holodex downloads it on writeback and embeds it as the file's cover art (see [§4.3](#43-assets) for why `assets[]` doesn't apply here) |
| `actors` | Cast (flat text) | multi | One name per element, billing order first. Prefer the structured [`people[]`](#45-video-credits--per-person-castcrew-with-headshots) shape instead if you want headshots/Person linking |
| `director` | Director(s) (flat text) | multi | One name per element |
| `studio` | Production compan(ies) | multi | One name per element. Pair with [`_studio_external_ids`](#46-studio-external-ids-_studio_external_ids) if you have stable studio ids |

Rules mirror [§4.2](#42-canonical-fields): always arrays, omit rather than send empty, no
embedded newlines, and `_`-prefixed keys are reserved sidecar channels you must not invent.

### 4.2b Canonical fields — studio

> **Status: additive extension** ([ADR-054](../architecture/ADR-054-studio-external-id-dedup.md)), exercised today.

| Canonical key | Meaning | Cardinality | Format guidance |
|---|---|---|---|
| `description` | Studio description | single | Plain text |
| `country` | Country of origin | single | Plain text or ISO country code |
| `website` | Official/home page | single | Absolute URL (shared canonical key with the person `website` field — same meaning, different entity) |

> **`logo` is not a `fields` key.** As of the F51/ADR-079 image-slot generalization, a studio's
> logo is an **asset** — emit it as `{ "kind": "logo", "url": "…" }` in `assets[]` (see
> [§4.3](#43-assets)), exactly like a person's `photo`. A provider still sending
> `fields["logo"]` has that value silently dropped (Holodex's studio registry no longer has a
> `logo` field to resolve it into).

### 4.2c Canonical fields — film

> **Status: decided ([ADR-086](../architecture/ADR-086-film-provider-enrichment.md)), not yet
> exercised** — no provider sends `entity_type: "film"` today (see [§3](#3-entity-types-and-matching));
> this vocabulary is the target shape for when film enrichment ships.

| Canonical key | Meaning | Cardinality | Format guidance |
|---|---|---|---|
| `description` | Film synopsis | single | Plain text — same canonical key as the studio `description` field, different entity |
| `release_date` | Release date | single | `YYYY-MM-DD` preferred — same canonical key as the video `release_date` field, different entity |

A film's poster is **not** a `fields` key — it is an `assets[]` entry (`kind: "poster"`), exactly
like Person and Studio. See [§4.3](#43-assets).

### 4.3 Assets

> **`assets[]` is consumed for `entity_type: "person"`, `"studio"` (F51, ADR-079) and `"film"`
> ([ADR-086](../architecture/ADR-086-film-provider-enrichment.md)) — all three are live.** See the
> Film kind table below. There is still **no** `video`/`media` image sink: a video's own poster/cover art
> stays a **`fields` entry** — `fields["poster_url"]` (video, `render: image_url`), exactly like
> `bio`/`website`/any other canonical text field, just holding an image URL as the value. See
> [§4.2a](#42a-canonical-fields--videomedia). **If your `/enrich` response for a `video` entity
> includes a non-empty `assets[]`, Holodex silently drops it** (logged as a server-side warning
> the provider never sees) — the response still returns `200` and your other fields still land,
> but the image itself never reaches Holodex. This is a common mistake for a provider that
> mirrors the working person-photo pattern for a video's poster; don't — use a
> `fields["poster_url"]` value instead. (Per-cast/crew headshots on a video's `people[]` entries
> are the one exception, and are **not** affected by this — see
> [§4.5](#45-video-credits--per-person-castcrew-with-headshots).)

An asset is a binary image Holodex downloads, normalizes, and stores against the entity.
Emit one `assets[]` entry per image, for a `person` **or** `studio` entity:

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

**Person** (`entity_type: "person"`):

| `kind` | Meaning | Target aspect | Notes |
|---|---|---|---|
| `photo` | Person portrait / headshot | ~1:1 (square) | The common case. Synonyms `portrait`/`headshot` are also accepted |
| `banner` | Wide hero image | ~16:9 | Synonym `backdrop` accepted |
| `poster` | Tall poster | ~2:3 | |
| `gallery` | Additional photos | any | Multiple assets allowed per enrich — see ordering note below |

**Studio** (`entity_type: "studio"`, F51/ADR-079):

| `kind` | Meaning | Target aspect | Notes |
|---|---|---|---|
| `logo` | Studio logo | any | The studios-list/detail-page image. Omitted/empty `kind` also maps to `logo` |
| `icon` | Small list icon | ~1:1 | No provider emits this yet — reserved |
| `poster` | Tall poster | ~2:3 | No provider emits this yet — reserved |

**Film** (`entity_type: "film"`, [ADR-086](../architecture/ADR-086-film-provider-enrichment.md) —
live, see [§3](#3-entity-types-and-matching)):

| `kind` | Meaning | Target aspect | Notes |
|---|---|---|---|
| `poster` | Portrait movie poster | ~2:3 | The films-list/detail-page image. Same `poster` kind as Person/Studio. Currently the only film kind Holodex stores |

> A landscape film image — the existing `banner` kind (~16:9, `backdrop` accepted as a synonym),
> reusing Person's, replacing the consumer-less `thumb` role — is **decided but not yet built**
> ([ADR-089](../architecture/ADR-089-film-enrichment-field-vocabulary.md) D4). A `banner` sent for
> a film today is dropped as an unknown kind, per the rule below. Emitting it early is safe and
> costs nothing; it starts landing when D4 ships.

Each entity type has its own kind namespace — a studio's `logo` kind is unrelated to a
person's `poster` kind, even though the string differs. Holodex maps each kind to one image
**role**; an unknown kind (for that entity type) is **dropped** (never stored under a guessed
role). Holodex does **not** crop to the target aspect (cropping is a separate owner action), so
supply an image already close to the role's aspect to avoid letterboxing.

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
- **Core roles** (person `photo`/`banner`/`poster`; every studio kind — `logo`/`icon`/`poster` —
  is core too, F51/ADR-079): Holodex fetches the first asset of each role it can successfully
  store, then skips the rest of that role. Emit at most one per core kind. A studio has no
  gallery role.
- **`gallery` role** (person only): Holodex fetches all `gallery` assets in order until an
  operator-configured cap is reached. You may include multiple `gallery` entries; they are
  stored as additional photos for the person.
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
| `people[].name` | string | yes | Display name. Holodex sanitizes (strips control chars, caps 4096). **Display only** — it is *not* an identity/match key ([ADR-055](../architecture/ADR-055-enrichment-unique-key-invariant.md)) |
| `people[].role` | string | yes | Credit role. v1 enum: `"actor"`, `"director"`, `"writer"`, `"producer"`, `"composer"`, `"crew"`. An **unknown role is stored generically** (never dropped) — forward-compatible |
| `people[].external_id` | string | **yes** | Namespace-qualified id (`<namespace>:<id>`, [§4.1](#41-external-ids-and-namespaces)). The **stable, deterministic** identity Holodex stores; the same person across films de-duplicates to one record. **Required** ([ADR-055](../architecture/ADR-055-enrichment-unique-key-invariant.md)) — a credit without one is refused, not name-matched. If your source lacks a stable person id, synthesize a deterministic namespaced one |
| `people[].order` | integer | optional | Billing order within a role (0 = top-billed) for display ordering. Holodex caps the list |
| `people[].headshot` | object | optional | A single **asset object** ([§4.3](#43-assets)) — `{ "kind": "photo", "url": "…" }` — for that person's portrait. Subject to **all** the [§4.3](#43-assets)/[§6](#6-security-requirements) asset rules: allowlisted host (your `base_url` host or an operator `asset_hosts` entry), `https` cross-host, no credentials in the URL, raster JPEG/PNG/GIF, ≤16 MiB, ≤4096 px. Omit when you have none |

**How Holodex consumes it.** On the owner's video enrich, Holodex (a) resolve-or-creates a
Person per entry — keyed **by `external_id`** (the identity; a namespaced id that another provider
already emitted converges to the same Person, [ADR-055](../architecture/ADR-055-enrichment-unique-key-invariant.md)) —
and links it to the video with its `role`; (b) downloads each `headshot` through the **same SSRF perimeter**
as every other asset ([§4.3](#43-assets)) and stores it as that person's headshot. The flat
`fields.actors`/`fields.director` text remains the fallback for providers that don't emit
`people`, so emitting both is harmless (Holodex prefers `people` when present).

**Caps & degradation.** Holodex caps `people` (≈50 entries) and applies the per-value caps in
[§5](#5-non-functional-requirements) to `name`. If a response nears the 1 MiB body cap, shed
`people[].headshot` URLs before `fields` (a headshot is recoverable on a later enrich; canonical
text is not) — same precedence as `assets` in [§4.3](#43-assets).

### 4.6 Studio external IDs (`_studio_external_ids`)

> **Status: additive extension** for `video`/`media` enrichment ([ADR-054](../architecture/ADR-054-studio-external-id-dedup.md)).
> **Backward compatible and opt-in:** it is an [internal sidecar](#42-canonical-fields) field, not
> a capability — **do not advertise it in `/describe`**, and a provider that omits it stays fully
> conformant (Holodex just falls back to de-duping studios by name). Emit it only if your upstream
> exposes a stable **company / studio id** for a film's production companies.

When a `video`/`media` `/enrich` response carries the `studio` field (production-company names),
you MAY also emit the reserved sidecar field **`_studio_external_ids`** in the same `fields` map,
carrying each company's provider id so Holodex can converge different spellings of the **same**
company ("Warner Bros." vs "Warner Bros. Pictures") onto one studio entity and refresh it
deterministically:

```json
{
  "fields": {
    "studio": ["Warner Bros. Pictures", "Legendary Pictures"],
    "_studio_external_ids": ["tmdb:174 Warner Bros. Pictures", "tmdb:923 Legendary Pictures"]
  }
}
```

Each `_studio_external_ids` value is a **single self-describing string** pairing one company's id
with its name:

| Part | Rule |
|---|---|
| Format | `"<external_id> <name>"` — the namespace-qualified id ([§4.1](#41-external-ids-and-namespaces)), then **one ASCII space**, then the company name |
| `<external_id>` | `<namespace>:<id>` (e.g. `tmdb:174`). The **id token contains no space**, so Holodex splits on the **first** space; everything after it is the name (which MAY contain spaces/punctuation) |
| `<name>` | The company name **exactly as it appears** in the corresponding `studio` value — this is how Holodex ties the id to the resolved studio |
| When to emit | One entry **per company that has both a non-empty name and a real id**. Omit a company entirely (from this field) if you have no id for it — its name still flows through `studio` and resolves by name |

Rules:

- **Self-describing, not positional.** Each value stands alone (id + name), so Holodex does **not**
  rely on array order matching the `studio` array. Order is not significant.
- **No embedded control characters.** The value rides the normal sanitize/caps ([§5](#5-non-functional-requirements)):
  Holodex strips control chars and collapses newlines/tabs to spaces, so keep the id + name on one
  line. The name may contain spaces — only the first space (after the id token) is the delimiter.
- **One namespace per id.** Use the same `id_namespaces` you declare in `/describe` (e.g. `tmdb`).
  A studio may accumulate ids from multiple providers over time; each provider emits its own.

**How Holodex consumes it.** On the owner's video enrich, Holodex parses `_studio_external_ids`
into a name→id map, and when it derives the video's studio links it resolves each studio **by id
first, then by name** — so a company seen under two spellings (with the same id) converges to a
single studio entity, and a stored id lets a later studio re-enrich re-fetch by id instead of
re-searching by name. A `studio` value with **no** matching id (a custom or id-less name) simply
resolves by name, exactly as before.

**Caps & degradation.** Subject to the per-field caps in [§5](#5-non-functional-requirements)
(≤ 50 values, ≤ 4096 chars each). If a response nears the 1 MiB body cap, you MAY drop
`_studio_external_ids` before dropping `studio` itself — the names still group correctly by name,
you only lose cross-spelling de-dup until the next enrich. Treat your upstream ids as untrusted
like everything else (S4); this field triggers **no** new fetch or host access.

### 4.7 Field render hints (`/describe.field_hints`)

> **Status: additive extension** ([ADR-056](../architecture/ADR-056-provider-field-render-hints.md), Holodex
> F39). **Backward compatible and opt-in:** it is an optional key on the `/describe` manifest; a provider that
> omits it stays fully conformant and renders exactly as before. **No protocol bump** — it rides the
> [§2.2](#22-get-describe--capability-manifest) "unknown keys ignored" rule, so an older Holodex ignores it.

By default a **non-canonical** field key (one outside the recommended vocabulary in [§4.2](#42-canonical-fields))
renders with a **title-cased fallback label** derived from the key, as plain inline text, ordered after the
canonical fields ([§4.2](#42-canonical-fields), "New keys are allowed but coordinate them"). `field_hints` lets
you tell Holodex how to present those keys **without** asking each operator to hand-author a mapping. Add it
alongside `fields` in `/describe`:

```json
{
  "provider": "acme",
  "protocol_version": 1,
  "entity_types": ["person"],
  "fields": ["bio", "birthdate", "gender", "trivia", "credited_as", "home_page"],
  "field_hints": {
    "gender":      { "label": "Gender",           "render": "text",      "group": "attributes", "order": 10 },
    "credited_as": { "label": "Also credited as", "render": "chips",     "group": "attributes", "order": 20 },
    "trivia":      { "label": "Trivia",           "render": "long_text", "group": "extended" },
    "home_page":   { "label": "Home page",        "render": "url",       "group": "extended" }
  }
}
```

`field_hints` is an object **keyed by field key**. Each hint object has these optional keys; **Holodex ignores
any other key** (forward-compat):

| Key | Type | Meaning | Absent / invalid → |
|---|---|---|---|
| `label` | string | Display label | title-cased key. Sanitized (control chars stripped) and capped (~64 chars) |
| `render` | string | Render mode: `text` \| `long_text` \| `chips` \| `url` \| `image_url` | `text`. An unknown mode falls back to `text` |
| `group` | string | Ordering band: `primary` \| `attributes` \| `extended` | `extended` (lowest). Fields sort by group, then `order`, then key — always **after** canonical fields |
| `order` | integer | Secondary sort within a group | `0` |

Rules:

- **Non-canonical keys only.** A hint on a **canonical** key (`bio`, `poster_url`, …) is **ignored** — Holodex's
  own registry owns the canonical vocabulary's label/render, and an **operator** mapping overrides everything.
  You cannot relabel a canonical field.
- **Presence-driven.** A hinted field renders **only when your `/enrich` actually returns a value for it** for
  that entity. Advertising a hint for a key you never populate shows nothing (no empty rows).
- **Advisory + hardened.** Holodex sanitizes `label`, validates `render`/`group` against the enums above, and
  treats the whole block as untrusted input (S4). Omit `field_hints` and nothing changes.
- **`_`-prefixed keys never apply.** Reserved sidecar keys ([§4.2](#42-canonical-fields)) are never displayed,
  so a hint for one is inert.
- **`image_url` needs an allowlisted host.** If you hint `render: "image_url"`, the field's **value** must be
  an image URL on a host the operator allowlisted (`asset_hosts`, [§10](#10-deliverable--operator-wiring)) — the
  same trust gate as `poster_url`/`logo`. A value on a non-allowlisted host renders as **text**, not an image.
  Same posture for `url` (`https`/`http` only). Tell operators which host(s) to allowlist in your provider docs.

**How Holodex consumes it.** Holodex persists your advertised hints when it reads `/describe`, then renders any
stored non-canonical field first-class with the hinted label/mode/order — **with zero per-operator mapping
config**. An operator who wants to curate or re-source such a field can still add a `metadata-mappings.yaml`
entry, which overrides your hint.

---

### 4.8 Provider brand icon (`/describe.brand_icon`)

> **Status: additive extension** ([ADR-059](../architecture/ADR-059-provider-brand-icon.md), Holodex
> HOLODEX-134). **Backward compatible and opt-in:** an optional key on the `/describe` manifest; a provider that
> omits it stays fully conformant. **No protocol bump** — it rides the
> [§2.2](#22-get-describe--capability-manifest) "unknown keys ignored" rule.

Holodex badges each enriched field with its provenance. Rather than spell out "from `<your-name>`" on every
row, it can show **your brand icon**. Advertise it as an [asset object](#43-assets) on `/describe`:

```json
{
  "provider": "acme",
  "protocol_version": 1,
  "entity_types": ["person"],
  "fields": ["bio", "birthdate"],
  "brand_icon": { "url": "https://acme.example/brand-icon.png" }
}
```

| Key | Type | Required | Notes |
|---|---|---|---|
| `brand_icon.url` | string | yes (if `brand_icon` present) | Absolute, directly-fetchable image URL for your brand mark |

This is a **provider-level** asset — one image for your whole provider, unrelated to any entity or
`external_id` (so it is **not** an `asset_kind`; those are per-entity images returned by `/enrich`). Holodex
handles it exactly like every other ingested image:

- **Same asset perimeter as [§4.3](#43-assets)/[§6](#6-security-requirements).** The `url` host must be your
  `base_url` host or a host the **operator** allowlisted in `asset_hosts`; `https` for cross-host; no credentials
  in the URL; the 16 MiB / 4096 px / 8 s-fetch caps apply. A URL on a non-allowlisted host is **refused**, and
  your provider simply shows the monogram fallback (below). Tell operators which host to allowlist in your docs.
- **Downloaded once, normalized, self-hosted.** Holodex fetches the icon when it reads your `/describe`, runs it
  through the same **decode → bound → re-encode-to-JPEG → strip-metadata** ingest as person portraits, and serves
  its **own** copy — viewers never hit your CDN. It re-fetches only when the advertised `url` changes.
- **Raster of any shape.** Any format your host serves is accepted on ingest, but Holodex stores a normalized
  **JPEG** (SVG/animation/transparency are **not** preserved — the decoder rejects SVG/polyglots). Holodex renders
  the icon at a small fixed **height** with automatic width, so both a square mark and a wide **wordmark** read
  correctly (e.g. TMDB self-hosts its rasterized wordmark). Since transparency is dropped, design the icon for a
  light background.
- **Monogram fallback.** Omit `brand_icon`, advertise an un-allowlisted host, or serve an undecodable image, and
  Holodex renders your provider's initial as a themed monogram instead — never a broken image.

---

### 4.9 Preferred search query pattern (`/describe.preferred_search_pattern`)

> **Status: additive extension** ([ADR-080](../architecture/ADR-080-configurable-provider-search-patterns.md),
> HOLODEX-254). **Backward compatible and opt-in:** an optional key on the `/describe` manifest; a provider
> that omits it stays fully conformant, and an older Holodex that doesn't parse it is unaffected. **No
> protocol bump** — it rides the [§2.2](#22-get-describe--capability-manifest) "unknown keys ignored" rule.
> **`video` entity only** — Person/Studio have no studio/year/performer fields to build a pattern from.

By default, `/resolve`'s `hint.query` ([§2.3](#23-post-resolve--identity-match-disambiguation)) carries a
video's plain title — cleaned of bracket/comma punctuation and resolution/quality tokens (`720p`, `4k`, …)
Holodex-side, but otherwise just the title. If your search index matches better on a shaped query — e.g.
studio and year alongside the title — advertise the shape you want:

```json
{
  "provider": "acme",
  "protocol_version": 1,
  "entity_types": ["video"],
  "fields": ["title", "release_date"],
  "preferred_search_pattern": "{studio?} {title?} {performers?} {year?}"
}
```

The grammar is a space-joined list of `{name}` (required) / `{name?}` (optional) tokens, `name` one of
`studio` | `title` | `performers` | `year` — no other punctuation or literal text is allowed. Holodex
renders it entirely on its own side from the video's already-resolved fields before calling `/resolve`; you
still only ever receive the one flattened `hint.query` string ([§2.3](#23-post-resolve--identity-match-disambiguation)) — nothing about the request shape changes.

| Behavior | Detail |
|---|---|
| Token values | `studio`/`title` are the field's top-precedence resolved value (`title` is passed through Holodex's title sanitizer even inside your pattern); `performers` is the top 3 of actors+director, space-joined; `year` is the 4-digit year parsed from the resolved release date |
| Optional token, no value | Dropped from the output — no `"undefined"`, no stray delimiter |
| Required token, no value | The **whole pattern** is skipped for that render — Holodex falls back to its next tier (an operator override, if the operator set one for you, then their fleet-wide default, then the sanitized-title floor). It never sends a query with a gap where your required token would have been |
| Unknown token name | Your whole `preferred_search_pattern` is ignored (logged Holodex-side) — never an error response to you, and it doesn't affect anything else about your provider |
| **Operator override always wins** | If the operator configures their own `search_pattern` for you in `metadata-sources.yaml`, it outranks this key entirely — you may still advertise a sensible default for operators who configure nothing |

**Practical guidance:** advertise this if your search index is meaningfully better with a shaped query than a
bare title — for example, disambiguating common titles by studio or year. If a bare title search already
works well for you, omitting this key is a completely valid, fully conformant choice — the title Holodex
sends is never worse than what it sent before this feature (bracket/resolution cleanup is unconditional
either way).

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
      "disambiguation": "Mathematician · 1815–1852",
      "profile_url": "https://acme.example/people/998211-ada-lovelace"
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

- **`confidence` semantics** for `/resolve` — **resolved (F47 / [ADR-066](../architecture/ADR-066-enrichment-auto-apply-and-dismissal.md)
  D1).** Holodex now thresholds on it client-side: a lone candidate at/above `0.85` auto-applies
  with no picker ([§2.3](#23-post-resolve--identity-match-disambiguation)). Any monotonic 0–1 value
  is still acceptable — there is no required calibration scheme, and the wire shape is unchanged —
  but a provider whose scores cluster near or above `0.85` for genuinely ambiguous matches will now
  cause incorrect auto-applies rather than just a mislabeled badge, so favor conservative scores
  when uncertain.
- **New canonical field keys** ([§4.2](#42-canonical-fields)) — a key outside the recommended set that
  should become part of the shared **canonical** vocabulary (registered label + render, cross-provider
  ordering) still needs a maintainer to add a registry entry; propose it. For a **provider-specific** extra
  attribute you just want rendered well, prefer `field_hints` ([§4.7](#47-field-render-hints-describefield_hints),
  ADR-056) — it needs no maintainer or operator config.
- **`/healthz` upstream signal** ([§2.1](#21-get-healthz--liveness--readiness)) — whether it
  should reflect upstream reachability or stay a pure container-liveness check.
- **Bind-address env var name** ([§7](#7-configuration)) — pick a convention (`HOST` vs
  `BIND_ADDR`).
- **Additional entity types** ([§3](#3-entity-types-and-matching)) — `person`, `video`, and
  `studio` are all live and exercised today (this section previously said only `person` was
  supported; that was stale). A `series` entity type beyond the file-per-video model remains
  designed-in but unexercised; coordinate its canonical field vocabulary before shipping one.
- **Film provider enrichment** — **resolved and shipped**
  ([ADR-086](../architecture/ADR-086-film-provider-enrichment.md)). Film has its own
  `entity_type: "film"` (never a reuse of `video`), its own canonical fields
  ([§4.2c](#42c-canonical-fields--film): `description`, `release_date`), and a portrait poster as
  an `assets[]` entry (the existing `poster` kind, [§4.3](#43-assets)) — never a `fields` entry.
  The TMDB provider advertises `film` and serves it. What remains open is the **breadth** of the
  film field vocabulary, not its existence: a film's title is owner-asserted identity and its
  cast/studio are derived from its attached videos, so a provider's `title`/`studio`/`actors` are
  stored but not applied. See F59 ([film-provider-enrichment-ux.md](film-provider-enrichment-ux.md),
  [ADR-089](../architecture/ADR-089-film-enrichment-field-vocabulary.md)) for the decided
  landing zone of each, and the §4.3 note on the not-yet-built film `banner` kind.
