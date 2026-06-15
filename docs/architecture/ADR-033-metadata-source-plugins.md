# ADR-033: Metadata source plugins — sidecar providers over a unified resolution layer

**Status**: Accepted (security-review sign-off 2026-06-15: SSRF perimeter — config-only `base_url` + provider allowlist + cross-host redirect refusal; uniform `requireOwner` gating on every enrich route; parameterized SQL; untrusted-response caps/sanitization; no `{@html}` sink; no upstream key or `base_url` serialized — no HIGH/MEDIUM findings)
**Date**: 2026-06-14
**Deciders**: Project owner
**Relates to**: ADR-013 (configurable metadata field mapping — the precedence model generalized here),
ADR-004 (metadata extraction), ADR-007/023 (Docker single-image + compose deployment),
ADR-026 (metrics exposition), ADR-028 (activity surface & job history), ADR-030 (owner gating seam),
ADR-014 (configuration & data layout).
Spec: [Metadata Source Plugins (F22)](../specs/metadata-plugins.md). Detailed [Phase 3](../specs/phase-3-enrichment.md) F16.

---

## Context

Phase 3 needs to enrich local entities (People first; Series/Video later) with data that **isn't in the
files** — bios, birthdates, photos, canonical titles — from external sources like TMDB and IMDB. The
governing requirement (Phase 3 F16, refined as spec F22) is **extensibility: add a new source without
changing core**. Secondary requirements: enrichment is **manual/on-demand only** (F16.3), provider data
sits in a **shadow layer** distinct from file-sourced metadata (F16.4), and the whole thing must be
**mockable in CI** with no network or real API keys.

Three plugin-isolation models were considered:

- **In-process Go interface (compiled-in providers).** Lightest at runtime; but bakes every provider's
  SDK and API-key handling into the single binary, growing `go.mod` (against the ADR-001/022 lean ethos)
  and coupling provider releases to Holodex releases.
- **External-process plugins (subprocess/RPC, e.g. hashicorp go-plugin).** Decouples providers, but adds a
  process-management surface and a binary-distribution story that doesn't fit the Docker-composed model.
- **WASM modules.** Strong sandboxing and language-agnostic, but the heaviest and most novel for this
  codebase — unjustified for a single-user tool.

The owner's preferred deployment shape is **sidecar Docker containers** — which fits the project's
existing "compose a few containers, point at a folder" model (ADR-007/023) better than any in-binary
scheme. The owner was explicitly unsure about the trade-off, so this ADR makes the call **and documents
the cost and an escape hatch honestly.**

Separately, ADR-013 already solved a structurally identical problem: many raw file tag keys normalized
into one canonical field via an **ordered precedence list**. A metadata provider is, at the resolution
layer, *just another source* for that same field. That insight is the spine of this decision.

## Decision

### 1. Providers are sidecar services behind a small HTTP/JSON contract

Each metadata source is a **separately-deployed container** exposing a transport-light contract that core
calls over the internal compose network:

- `GET /healthz` — liveness/readiness (surfaced on `/status`, ADR-028).
- `GET /describe` — capability manifest: name, version, `protocol_version`, supported `entity_types`,
  understood ID namespaces, and the canonical fields it can supply.
- `POST /resolve` — identity match: given embedded external IDs and/or a name query, return ranked
  candidates with provider-stable external IDs and a confidence/disambiguation string.
- `POST /enrich` — given a chosen external ID, return canonical `field → value(s)` plus optional asset
  URLs (e.g. a person photo) that core fetches itself.

**Core owns:** which providers are enabled, per-field precedence, the shadow store, the UI, on-demand
orchestration, and the security perimeter. **The provider owns:** its upstream API key, rate-limiting and
backoff, upstream parsing, and any caching of upstream responses. Core holds **no upstream API keys**.

Core talks to providers through a `ProviderClient` interface whose **default implementation is HTTP**.
The same interface can be satisfied **in-process** (an `httptest`-style fake for CI, and a possible
bundled-default escape hatch) — so "sidecar" is the *deployment default*, not a hard architectural lock.

### 2. One unified field-resolution layer (generalize ADR-013)

ADR-013's mapping is extended so a canonical field's `sources` precedence list may **interleave file tag
keys and providers**:

```yaml
sources: [tmdb, file:Publisher, imdb]   # first present source wins
```

File values resolve from the extracted/`video_metadata` data; provider values resolve from the shadow
store. **File-extracted first-class fields are never overwritten** unless the owner explicitly orders a
provider ahead of `file:` for that field. Resolution stays **pure re-interpretation** of stored data —
changing precedence needs no re-fetch and no re-scan, exactly as in ADR-013.

### 3. Shadow enrichment store

Provider-fetched values persist in an `entity_enrichment` table keyed by
`(entity_type, entity_id, provider, field_key)` with `value`, `external_id`, `fetched_at`, kept **separate
from file metadata** and merged only at resolution time. The confirmed external match persists per
entity+provider so re-fetch is one click and identity is asked once. This makes the future **writeback**
feature (Phase 3 F17) a clean *consumer* of this layer rather than a new mechanism.

### 4. v1 scope: People; matching is embedded-ID-first with name-search fallback

The first slice enriches **People only** (Person already exists — no new entity needed to prove the seam).
Matching tries an embedded external ID first (deterministic), and falls back to provider name-search with
a **manual owner confirmation** picker. Because Person records are derived from file tag *names* and
rarely carry external IDs, the name-search-confirm path is the dominant one for People v1; the ID-first
path becomes primary when the design generalizes to Series/Video, which do carry `©imdb`/MKV `IMDB` tags.

### 5. Security perimeter

Enrichment endpoints sit behind `requireOwner` (ADR-030). Core calls **only allowlisted provider
`base_url`s** and never a URL derived from provider/file data, and does not follow provider redirects to
other hosts (SSRF guard). Provider responses are **untrusted**: values are length-capped and sanitized,
asset downloads are size- and content-type-limited, and a malformed response fails the single fetch, not
the server. All enrichment is **on-demand only** — there is no enrichment scheduler.

### 6. Observability reuse

Provider health appears on the `/status` page (ADR-028, no secrets). Enrichment runs are recorded as
`job_runs` with `kind=enrich` in the 30-day history (ADR-028). Per-provider request/latency/error metrics
use the hand-rolled exposition (ADR-026) — no new dependency.

## Rationale

- **Sidecar matches the deployment model.** "Add a container + one config line" is how Holodex already
  ships (ADR-007/023); a new source needs **no core code change and no Holodex release**, which is the
  primary requirement. Provider crashes/hangs/rate-limits are isolated, and each provider's upstream
  **secret stays in its own container** — never in core's config, logs, or read-model.
- **Provider-as-source is the minimal generalization.** Reusing ADR-013's precedence/label/`multi`
  machinery means file and plugin metadata are *the same kind of thing* at resolution — one mental model,
  one config surface, one UI path, provenance for free.
- **Protocol-first keeps CI offline and hedges the trade-off.** A fake provider implementing the contract
  runs the whole flow with no network or keys; the same `ProviderClient` seam allows an in-process
  implementation, so choosing sidecar today does not foreclose bundling a default later.
- **Non-destructive shadow layer** preserves the file-as-source-of-truth invariant (ADR-003/004/013) and
  gives writeback a well-defined input.

## Consequences

- **New components:** a `ProviderClient` (HTTP default + in-process fake), a provider registry loaded from
  `metadata-sources.yaml` (atomic reload, mirroring the mapping store, reload via F20.10), the
  `entity_enrichment` table + migration, the extended resolver, and owner-gated enrich endpoints. The SPA
  gains a name-search disambiguation picker and provenance badges (semantic tokens; QA all three skins).
- **Operational cost — stated honestly.** The sidecar model is **heavier than compiled-in for the dominant
  single-user local case**: N providers = N containers, N configs, a network hop + serialization per call,
  a versioned HTTP contract to maintain, and each bundled provider (TMDB/IMDB) must be **built and
  published as its own image** (more release surface under ADR-023/024). This is an accepted trade-off,
  bought back partly by the in-process `ProviderClient` escape hatch for tests and any future bundled
  default. If the per-source overhead proves not worth it, the `ProviderClient` seam allows collapsing the
  default providers in-process **without changing the resolver, store, config, or UI** — that would be a
  follow-up ADR, not a rewrite.
- **Security review required before merge** (CLAUDE.md routing — this touches access + outbound network):
  the `requireOwner` coverage, the `base_url` allowlist / no-redirect SSRF posture, untrusted-response
  handling (size/content-type/sanitization), and the no-upstream-keys-in-core invariant are the sign-off
  items.
- **Testing strategy** gains: contract conformance against the fake, precedence/interleaving resolution,
  shadow-store non-destructiveness on re-scan, the ID-vs-search matching paths, and provenance rendering.
- **Generalization is additive:** Series/Title and Video enrichment add a new `entity_type` value (and, for
  Series, a new entity table) and reuse `entity_enrichment` + the resolver unchanged.
- **Supersede, don't edit:** if the isolation model changes (e.g. to in-process bundling or WASM), this ADR
  is superseded. The provider **protocol version** (`/describe.protocol_version`) governs compatibility
  independently of this ADR's status.
