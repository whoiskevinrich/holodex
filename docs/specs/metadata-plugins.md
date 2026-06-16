# Spec: Metadata Source Plugins (F22)

**Status**: Draft (Accepted direction — decisions locked via [ADR-033](../architecture/ADR-033-metadata-source-plugins.md))
**Feature block**: F22
**Phase**: 3 (Enrichment foundation) — this is the **keystone** that unblocks the rest of [Phase 3](phase-3-enrichment.md)
**Depends on**: Phase 2 complete · ADR-013 (field mapping — the precedence model this generalizes) · ADR-004 (extraction) · ADR-028 (job history / activity surface) · ADR-030 (owner gating) · ADR-007/023 (Docker compose deployment)
**Refines / supersedes**: [Phase 3](phase-3-enrichment.md) **F16** (Metadata Source Plugins) — that high-level block is detailed and made concrete here. F17 (writeback) and F14.3 (person photo storage) remain in the Phase 3 spec and *consume* this layer.

---

## Objective

Give Holodex an **extensible metadata-source system**: external providers (IMDB, TMDB, …) that enrich local entities with data Holodex cannot read from the files themselves, **without changing core for each new source**. The first slice enriches **People**. The same design generalizes to Series/Title and Video content-metadata later.

The load-bearing idea: **a provider is just another *source* feeding the field-resolution layer that file tags already feed** (ADR-013). File-extracted metadata and plugin-fetched metadata become the same kind of thing at resolution time, merged by per-field precedence and badged by provenance in the UI.

---

## Goals

1. **Add a new source without touching core** — a provider is a separately-deployed sidecar speaking a small HTTP contract; enabling it is a config entry, not a code change or recompile.
2. **One unified, configurable field-resolution layer** — per canonical field, the owner configures an ordered precedence list that may interleave file tag keys *and* providers, plus a display label and provenance.
3. **Prove the seam end-to-end on People** — enrich an existing Person from TMDB/IMDB (bio, birthdate, nationality, website, aliases, photo), on demand, with provenance shown distinctly from any file-sourced value.
4. **Non-destructive & re-scan-safe** — plugin data lives in a shadow layer; it never overwrites file-extracted first-class fields, mirroring ADR-013's capture-then-interpret model.
5. **Safe by construction** — enrichment is owner-gated, on-demand only, never automatic, and treats provider responses (and provider URLs) as untrusted.

## Non-Goals

- **Metadata writeback to source files** — separate decision ([Phase 3 F17](phase-3-enrichment.md), `WRITEBACK_ENABLED`). This spec produces the shadow layer F17 will read; it does not write files.
- **Series/Title and Video enrichment in v1** — the protocol and resolver are designed to generalize, but only **People** ships first. (Why: Person already exists as an entity; no new table/relations/UI needed to prove the seam.)
- **Automatic / background enrichment** — every fetch is user-initiated (F22.6). No crawler.
- **In-process / compiled-in providers as the shipped model** — providers deploy as sidecar containers (ADR-033). The contract is transport-light enough to also be served in-process (used for the CI fake and as an escape hatch), but bundling providers into the binary is explicitly not the v1 deployment model.
- **Third-party plugin marketplace / signing / sandboxing** — the contract is open, but discovery is manual compose wiring; no registry, no plugin sandbox.
- **Multi-user** — enrichment controls reuse the single owner gate (ADR-030).

---

## User Stories

- *As the library owner, I want to fetch a person's bio, photo, and birthdate from TMDB so that my People pages are rich without my having to type it all in.*
- *As the owner, I want to choose which source wins per field (e.g. prefer the file's own value for `title`, but TMDB for `birthdate`) so that the most trustworthy value shows.*
- *As the owner, I want every enriched value clearly labeled "from TMDB" vs "from file" so that I can tell what the machine added.*
- *As the owner, when a person can't be auto-matched I want to search the provider by name and pick the right record so that I stay in control of identity.*
- *As the owner, I want to add a new metadata source by dropping a container into my compose file and one config line — not by waiting for a Holodex release.*
- *As an operator exposing Holodex, I want enrichment locked behind the owner token and unable to call arbitrary URLs so that it can't be abused as an SSRF vector.*

---

## Functional Requirements

### F22.1 — Provider protocol (the contract)

A provider is an HTTP/JSON service. Core calls it; it owns its upstream API key, rate-limiting, and parsing.

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F22.1a | `GET /healthz` — liveness/readiness | Returns `200` with `{status, provider, version}`; surfaced on the `/status` page (ADR-028) as provider health |
| F22.1b | `GET /describe` — capability manifest | Returns provider name, version, supported `entity_types` (v1: `["person"]`), the canonical fields it can supply, and the ID namespaces it understands (`imdb`, `tmdb`). Core uses this to populate field-config choices and the UI |
| F22.1c | `POST /resolve` — identity match | Given `{entity_type, hint:{external_ids?, query?}}` returns ranked candidates `[{external_id, namespace, label, confidence, disambiguation?}]`. Supports the ID path (deterministic) and the name-search path (fallback) |
| F22.1d | `POST /enrich` — fetch fields | Given `{entity_type, external_id, namespace}` returns `{fields:{<canonical>:[values]}, assets?:[{kind,url}]}`. Core fetches asset URLs itself (e.g. person photo) |
| F22.1e | Protocol versioning | `/describe` reports a `protocol_version`; core refuses providers whose major version it doesn't support, logging a clear error rather than failing silently |

### F22.2 — Provider registry & configuration

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F22.2a | Providers declared in config (`metadata-sources.yaml`, mirroring `metadata-mappings.yaml`) as `{name, base_url, entity_types, enabled}` | Adding a provider entry + restart (or reload-config) makes it available; no core code change |
| F22.2b | **Allowlist enforcement** — core only ever calls a configured provider `base_url`; never a URL derived from provider/file data | A `/resolve` or `/enrich` response cannot redirect core to an unconfigured host (SSRF guard) |
| F22.2c | Disabled/unreachable providers degrade gracefully | A down provider shows "unavailable" on `/status` and is skipped in resolution; it never blocks page loads or other providers |
| F22.2d | Config hot-reload | `POST /api/v1/admin/reload-config` (F20.10) re-reads the provider list atomically, consistent with the mapping store |

### F22.3 — Unified field resolution & precedence (generalizes ADR-013)

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F22.3a | A canonical field's `sources` list may interleave file tag keys and providers, e.g. `sources: [tmdb, file:Publisher, imdb]` | The first present source in order supplies the value; file values come from extracted/`video_metadata`, provider values from the shadow store |
| F22.3b | Per-field `label` and `multi` semantics carry over from ADR-013 | A multi field aggregates across the winning source; single-valued takes the first |
| F22.3c | Provider data **never overwrites** a file-extracted first-class field unless the config explicitly orders the provider ahead of `file:` for that field | With default config (file-first), re-scanning a file does not lose its own title/date to a provider |
| F22.3d | Resolution is pure re-interpretation of stored data | Changing precedence config takes effect without re-fetching from providers or re-scanning files |

### F22.4 — Shadow enrichment store

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F22.4a | Plugin-fetched values persist in an `entity_enrichment` table keyed by `(entity_type, entity_id, provider, field_key)` with `value`, `external_id`, `fetched_at` | A fetched bio survives restart and is re-displayed without re-calling the provider |
| F22.4b | The chosen external match persists per entity+provider so re-fetch is one click | After confirming "this Person = TMDB #287", re-enrich does not re-prompt for identity |
| F22.4c | Stored separately from file metadata; the two are merged only at resolution time | Clearing/disabling a provider removes its contribution without touching file-sourced data |

### F22.5 — People enrichment (v1 slice)

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F22.5a | A Person detail page has an owner-only "Enrich from <provider>" action | Visible only to an owner client (ADR-030 capability flag) |
| F22.5b | **Matching: embedded-ID first, name-search fallback** | If the person carries a known external ID, core auto-resolves; otherwise the owner is shown provider name-search candidates and **picks one** (disambiguation picker — see [design handoff](../design/metadata-enrichment-handoff.md)). *Note:* Person records rarely carry embedded IDs today, so the name-search-confirm path is the dominant one for People v1; the ID-first path becomes primary when the design generalizes to Series/Video, which carry `©imdb`/MKV `IMDB` tags |
| F22.5c | Enrichable Person fields v1: `bio`, `birthdate`, `nationality`, `website`, `aliases`, `photo` | After enrich, configured fields render on the person page with provenance |
| F22.5d | `aliases` from a provider feed Person aliases | Provider-supplied aliases are added to the Person-aliases store and become searchable (ties into the Person-aliases backlog item) |
| F22.5e | `photo` asset stored under the data dir (`DATABASE_PATH/images/people/:id.{jpg,png}`, per Phase 3 F14.3) | Core downloads and stores the asset; size/dimension limits enforced; displayed on the person card and page |
| F22.5f | MCP `get_person` / `list_people` return enriched fields with provenance | Verified via MCP tool call |

### F22.6 — On-demand only

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F22.6a | No automatic/background enrichment | No provider call happens without an explicit owner action; verified by the absence of any enrichment scheduler |
| F22.6b | Enrichment runs are recorded as `job_runs` with `kind=enrich` (reusing ADR-028) | A completed enrich appears in the 30-day activity history with provider, entity, outcome |

### F22.7 — Provenance & display

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F22.7a | Every resolved field shows its provenance ("from TMDB" / "from file") | The winning source is labeled; QA'd in all three skins using semantic tokens (no hardcoded styling) |
| F22.7b | Owner can clear a provider's contribution for an entity | One action removes `entity_enrichment` rows for that provider+entity; field falls back to the next source |

### F22.8 — Provider health & observability

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F22.8a | Provider health (`/healthz`) shown on the `/status` page | Each configured provider lists state (ok/unavailable) and version; no secrets exposed (ADR-028 invariant) |
| F22.8b | Per-provider request count / latency / error metrics via `/metrics` (ADR-026) | Counters/histogram labeled by provider, hand-rolled exposition (no new deps) |

### F22.9 — Security

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F22.9a | All enrichment endpoints behind `requireOwner` (ADR-030) | Token-less/non-owner client gets `401`; controls hidden in the SPA |
| F22.9b | Provider responses treated as untrusted input | Field values length-capped and sanitized before storage/display; asset downloads size-limited and content-type-checked; malformed responses fail the single fetch, not the server |
| F22.9c | SSRF posture | Core calls only allowlisted provider `base_url`s (F22.2b); it does not follow provider-supplied redirects to other hosts |
| F22.9d | No upstream API keys in core | Provider API keys live in the provider container's own env/secret; they never appear in Holodex config, logs, or the read-model |

### F22.10 — Testability / CI

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F22.10a | A **fake provider** implements the same contract for CI/dev | The People-enrichment flow runs end-to-end in CI against the fake — no network, no real API keys |
| F22.10b | The fake is served in-process (httptest) and/or as a tiny stub container | Resolution, precedence, shadow-store, and provenance are unit/integration tested against it |

---

## Data Model (additions)

```
entity_enrichment            -- the shadow layer (F22.4)
  entity_type   string       -- v1: "person" (later: "series", "video")
  entity_id     int64        -- FK into people (v1)
  provider      string       -- registry name, e.g. "tmdb"
  field_key     string       -- canonical field, e.g. "bio", "birthdate"
  value         string
  external_id   string       -- the matched record id (namespace-qualified)
  fetched_at    timestamp
  PRIMARY KEY (entity_type, entity_id, provider, field_key)
  -- a companion (entity_type, entity_id, provider) → external_id mapping (F22.4b)
  -- can be the external_id column with a sentinel field_key, or its own small table.

person_alias                 -- shared with the Person-aliases backlog item
  id          PK
  person_id → person
  alias       string
  source      string         -- "manual" | "<provider>"  (provenance for F22.5d)
```

Provider list lives in config (`metadata-sources.yaml`), not the DB — consistent with ADR-013's `metadata-mappings.yaml`.

---

## Provider protocol sketch

```jsonc
// GET /describe
{ "provider": "tmdb", "version": "0.1.0", "protocol_version": 1,
  "entity_types": ["person"],
  "id_namespaces": ["tmdb", "imdb"],
  "fields": ["bio", "birthdate", "nationality", "website", "aliases", "photo"] }

// POST /resolve   { "entity_type":"person", "hint": { "query": "Hayao Miyazaki" } }
{ "candidates": [
    { "external_id": "tmdb:608", "namespace": "tmdb",
      "label": "Hayao Miyazaki", "confidence": 0.98,
      "disambiguation": "Director · 1941 · Studio Ghibli" } ] }

// POST /enrich    { "entity_type":"person", "external_id":"tmdb:608", "namespace":"tmdb" }
{ "fields": { "bio": ["Japanese filmmaker…"], "birthdate": ["1941-01-05"],
              "nationality": ["Japanese"], "aliases": ["宮崎駿"] },
  "assets": [ { "kind": "photo", "url": "https://…/portrait.jpg" } ] }
```

## Config sketch

```yaml
# metadata-sources.yaml   (path via METADATA_SOURCES_PATH; missing file = no providers)
sources:
  - name: tmdb
    base_url: http://holodex-tmdb:9100   # compose service name (allowlisted)
    entity_types: [person]
    enabled: true
```

```yaml
# metadata-mappings.yaml — sources may now name a provider, not just a file key:
fields:
  - canonical: birthdate
    label: Born
    sources: [tmdb, file:Birthdate]      # provider wins, file is fallback
  - canonical: title
    label: Title
    sources: [file:Title, tmdb]          # file wins; provider only fills gaps
```

---

## Success Metrics

**Leading**

- People-enrichment flow works end-to-end against the fake provider in CI (binary pass/fail; gate on merge).
- Adding the fake/real provider requires **0 core code changes** — config + container only (verified by the PR diff: no `internal/` change to onboard a second provider).
- Enrich → render latency for one person under a few seconds against a live provider (manual check).

**Lagging**

- Second real provider (IMDB alongside TMDB) added later with no core change — validates the "extensible without core changes" goal.
- Generalization to Series/Video reuses `entity_enrichment` + the resolver unchanged (only new `entity_type` values + a new entity table) — validates the keystone claim.

---

## Open Questions

1. **External-ID source for People (engineering/design).** People are derived from file tag *names*, not IDs, so v1 People enrichment is name-search-confirm in practice. Do we also let the owner paste/store a known TMDB/IMDB person ID manually to make re-matching deterministic? (Leaning yes — cheap, stored in `entity_enrichment` external_id.)
2. **Provider request budget (engineering).** Rate-limiting is delegated to each provider container (it owns its upstream key). Does core also need a global concurrency cap on outbound enrich calls, or is per-provider sufficient? (Phase-3 OQ#3 carried forward.)
3. **Asset storage budget (engineering).** Person photos are small, but a future Series/Video generalization with posters/backdrops needs the same budget/pruning question as previews (Phase-3 OQ#5). Pin when generalizing.
4. **Stale enrichment (product).** Provider data can change upstream. Is `fetched_at` + a manual "re-enrich" button enough, or do we want a staleness indicator? (Leaning: manual only, on-demand ethos.)
5. **Confidence threshold for auto-match (design).** When an embedded ID is absent but a single high-confidence name candidate exists, auto-apply or always confirm? (Leaning: always confirm in v1 — owner stays in control.)

---

## Timeline / Phasing

1. **Contract + resolver + shadow store + fake provider** — the seam, proven in CI with no network. (Most of the architectural risk lives here.)
2. **People enrichment slice** — registry config, the owner-gated enrich action, the name-search disambiguation picker (design handoff), provenance display, photo storage, MCP fields.
3. **First real provider container (TMDB)** — packaged + published image (ADR-023/024), its own API key, documented compose wiring.
4. **Generalize** (separate specs/ADRs): Series/Title entity, Video content-metadata, then writeback (F17) as a consumer of the shadow layer.

> **Provider hand-off specs (for external teams).** The provider HTTP contract is also
> published as self-contained, source-neutral hand-off documents so other teams can build
> their own provider images in separate repos without the Holodex source tree:
> - [Metadata Provider Contract](metadata-provider-contract.md) — the generic, source-neutral
>   spec (any upstream, any language).
> - [TMDB Provider](tmdb-provider.md) — a worked example mapping that contract onto TMDB.

> **Routing reminder (CLAUDE.md):** this feature touches infrastructure (new deployable services, outbound network) and access (owner-gated, SSRF surface) → **`/security-review` is required before merge**, and **`/testing-strategy`** must gain the provider-contract + resolution + provenance cases. Frontend (picker, provenance badges) must use semantic tokens and QA all three skins — see the [design handoff](../design/metadata-enrichment-handoff.md).
