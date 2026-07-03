# ADR-055: Universal enrichment unique-key invariant — every source supplies a namespaced id, and it is the identity

**Status:** Proposed
**Date:** 2026-07-02
**Deciders:** Project owner

**Generalizes:** [ADR-054](ADR-054-studio-external-id-dedup.md) (studio external-id de-dup — promotes its `external_id`-first, namespace-qualified, globally-unique dedup key from a studio-specific mechanism to a **cross-entity invariant**). **Extends:** [ADR-033](ADR-033-metadata-source-plugins.md) (provider contract + `entity_enrichment` shadow store — this ADR tightens the contract: an external id becomes **required and namespace-validated**, not optional). **Relates to:** [ADR-036](ADR-036-person-alias-search-indexing.md) (person name routing/aliases — this ADR draws the line between *owner-curated name identity* (kept) and *provider-supplied identity* (id-only)) · [ADR-047](ADR-047-per-item-metadata-refresh.md) (refresh reads the stored id to re-fetch) · [ADR-051](ADR-051-per-field-source-of-truth-decisions.md)/[ADR-052](ADR-052-baseline-source-contract.md) (entity-generic resolution — the invariant is applied per canonical entity on the same seam). **Spec:** [metadata provider contract](../specs/metadata-provider-contract.md) §4.1/§4.5 (updated here). **Issue:** [HOLODEX-123](https://whoiskevinrich.atlassian.net/browse/HOLODEX-123). **Conformance follow-ups:** [HOLODEX-124](https://whoiskevinrich.atlassian.net/browse/HOLODEX-124) (contract enforcement), [HOLODEX-125](https://whoiskevinrich.atlassian.net/browse/HOLODEX-125) (person id-first identity).

---

## Context

Provider enrichment identifies an upstream record by an **external id** (`<namespace>:<id>`, e.g. `tmdb:603`,
`imdb:tt1160419`). ADR-033 introduced it; ADR-054 made it the **de-dup key** for the studio entity via a
globally-unique `studio_external_ids(external_id PK)` join with external-id-first resolve. But across the
codebase the treatment is a **patchwork**, and where the id is absent the system silently falls back to
matching by **name** — which collides (two different people named "John Smith" merge; "Warner Bros." and
"Warner Bros. Pictures" split).

### Current state (survey, 2026-07-02)

| Layer | Today |
|---|---|
| Contract — `Candidate.external_id` | Present, documented "yes", but **not shape-validated**; `sanitizeCandidates` only trims. An empty id passes the sanitizer. |
| Contract — credits `people[].external_id` (§4.5) | **Optional** — "Omit only if your source has no stable id"; the core then "keys by `external_id` when given, **else by normalized `name`**". The collision path. |
| Contract — `/describe.id_namespaces` | **Advisory only** — never checked against the namespaces a provider actually emits. |
| API — `/enrich` apply | Enforces `external_id != ""` at the HTTP boundary — but not the `<namespace>:<id>` shape, and only on the interactive apply path (not the derived-credits path). |
| Store — `entity_enrichment.external_id` | `TEXT NOT NULL DEFAULT ''` — stored, **can be empty**, and **not part of the uniqueness key** (`UNIQUE(entity_type, entity_id, provider, field_key)`). |
| **Studio** | **Id is identity** — `studio_external_ids(external_id PK)`, external-id-first `resolveOrCreateStudio`, cross-spelling convergence (ADR-054). |
| **Person** | Id **stored but unused for identity**; deduped by name + F23 aliases. Provider credits fall back to name when the id is omitted. |
| **Video/media** | Identified by **file path** (ADR-011); enrichment id = the movie id, used only by `refresh`/`ProviderMatches` (skips empty ids). |
| **Tags** | **No enrichment** exists. |

### Forces

- **Collisions are silent and lossy.** A name-keyed merge of two distinct entities is hard to detect and
  hard to unwind. The owner wants duplicates/collisions *mitigated by construction*, not by later cleanup.
- **One rule, every entity, present and future.** The fix should not be another per-entity special case
  (studio has one, person doesn't); it should be an invariant that Person, Studio, Video, and a future Tag
  entity all inherit — the same way ADR-052 made resolution entity-generic.
- **Providers are untrusted.** The id is external input; enforcing a shape is also an input-validation
  hardening (a bare or malformed id must be refused at the perimeter, ADR-033 §F22.9b).
- **Namespaces are meaningful across providers.** `imdb:tt1160419` denotes the same title whoever reports
  it. A shared namespace is what lets two providers *converge* rather than duplicate — but only if the id
  is required and the namespace is validated.
- **Don't break owner intent.** Owner-curated **name** identity (F23 person aliases/merge, manual studio
  spelling fixes) is a human decision and stays. The invariant governs **provider-supplied** identity.
- **Inherit, don't reinvent** (ADR-054's constraint). Person conformance should mirror the studio join +
  external-id-first resolve, not invent a third pattern.

---

## Decision

**Every enrichment source MUST supply a namespaced unique key `<namespace>:<id>` for every record it
resolves or enriches, and that key is the sole identity/de-dup key for the enriched canonical entity.**
Three sub-decisions (owner, via question cards, 2026-07-02):

### D1 — Mandatory: no name fallback

An enrichment identity is **always** the namespaced id. A `/resolve` candidate or `/enrich` (interactive
or derived-credits) whose `external_id` is empty or not of the form `<namespace>:<id>` is **refused** at
the provider perimeter — not accepted and name-matched. **Name becomes display-only**, never an identity
key for provider-supplied data. This closes the §4.5 credits fallback and the `sanitizeCandidates`
empty-id gap.

> Owner-curated **name** identity is a separate system and is untouched: F23 person aliases/merge and a
> manual studio-spelling fix are human intent, not provider identity. The invariant governs the
> provider→entity link only.

### D2 — Shared namespace (cross-provider convergence)

The de-dup key is the **namespaced id itself**, and a namespace is a **shared identity space**:
`imdb:tt1160419` denotes the same entity regardless of which provider reported it, so two providers that
both emit it **converge to one entity**. A provider MUST only emit ids in namespaces it advertises in
`/describe.id_namespaces` (now **validated**, no longer advisory); a provider that can resolve a foreign
namespace (e.g. accepts `imdb:` and maps it internally) advertises it. This is exactly ADR-054's
`studio_external_ids(external_id PK)` behavior, generalized: the PK is global, so the same `namespace:id`
resolves to exactly one entity of that type.

### D3 — Invariant now, conformance as follow-ups

This ADR is the decision + the per-entity **conformance table** below + the provider-contract update
(§4.1/§4.5). It changes **no entity behavior on its own**. Implementation of the gaps lands as
**HOLODEX-124** (perimeter enforcement, entity-agnostic) and **HOLODEX-125** (person id-first identity),
each carrying its own spec/test/security gates.

### Conformance table (the invariant applied per entity)

| Entity | Identity today | Under the invariant | Gap → issue |
|---|---|---|---|
| **Studio** | `external_id` PK, id-first resolve (ADR-054) | **Already conformant** — the reference implementation | none |
| **Person** | name + F23 aliases; id stored, unused | `person_external_ids(external_id PK)` + id-first resolve for **provider-supplied** persons; credits id mandatory; owner name-aliases/merge preserved | **HOLODEX-125** |
| **Video/media** | file path (ADR-011); enrichment id = movie id, used by refresh | File path stays the **file** identity (a local artifact, not a provider record); the **enrichment** carries a required namespaced movie id (already does — `refresh` needs it). Enforcement makes the empty-id case impossible. | covered by **HOLODEX-124** |
| **Tags** | no enrichment | **Pre-decision:** if/when tags gain enrichment, tag enrichment obeys this invariant from day one (a `tag_external_ids(external_id PK)` join, id-first resolve). Nothing to build now. | future |

Note the deliberate distinction for video: a **video** is a local file (path-identified, ADR-011); its
**enrichment** is a provider record (id-identified). The invariant governs the enrichment/provider link,
which is where cross-provider duplicates and refresh live — not the file's own identity.

---

## Options Considered

### D1 — enforcement level

#### A — Mandatory, no name fallback (chosen)

**Pros:** Eliminates the silent name-collision class entirely; identity is deterministic and reversible;
also hardens the untrusted-input perimeter (a malformed id is refused, not stored). **Cons:** A provider
whose source genuinely lacks stable ids cannot enrich until it synthesizes a stable id (e.g. a hash of a
canonical key) and advertises a namespace for it — a real burden pushed onto provider authors. Accepted:
the burden is the point (a source with no stable identity *cannot* be safely deduped, so admitting it
name-only reintroduces the collision the owner is eliminating).

#### B — Preferred, name fallback quarantined

Require the id where available; keep normalized-name as a last-resort identity for id-less providers, but
flag those records low-confidence/mergeable. **Pros:** id-less providers keep working. **Cons:** leaves a
residual collision path and a two-mode identity system (id-keyed vs name-keyed) that every entity must
special-case forever — the exact patchwork this ADR removes. Rejected: softens the headline guarantee.

### D2 — key scope

#### A — Shared namespace, cross-provider convergence (chosen)

The namespaced id is a global identity; two providers emitting `imdb:tt…` converge. **Pros:** actually
de-dups *across* providers (the multi-provider world HOLODEX-118/119 just enabled); reuses ADR-054's proven
global-PK shape. **Cons:** a provider that emits a *wrong* foreign id could merge two distinct entities —
mitigated by validating the namespace against `/describe.id_namespaces` and by the provider being a trusted,
owner-configured sidecar (not arbitrary). Accepted.

#### B — Provider-scoped keys `(provider, external_id)`

**Pros:** zero risk of a bad foreign id merging entities; simplest. **Cons:** the same real-world entity
seen by two providers stays **two** records until a manual merge — i.e. it does **not** mitigate
cross-provider duplicates, which is a primary goal. Rejected.

### D3 — rollout

**ADR + per-entity follow-ups (chosen)** keeps the decision reviewable in one place and lets each entity's
implementation carry its own migration + spec + security review, rather than one giant multi-migration PR.
**Big-bang implement-now** was rejected as an oversized, harder-to-review change for a decision that is
mostly already realized for studio and only truly missing for person.

---

## Trade-off Analysis

**One invariant vs. N special cases.** The codebase already trends toward entity-generic seams
(`BaselineSource`/`ResolveFields`, ADR-052). Identity was the last per-entity special case (studio had a
real dedup key; person leaned on name). Making the namespaced id the *universal* identity collapses that
divergence: `<entity>_external_ids(external_id PK)` + id-first resolve is one pattern, instantiated per
entity, exactly like the resolver is one core instantiated per baseline. The cost is a hard contract
requirement on providers — acceptable because providers are first-party sidecars the owner configures, and
the contract already *documents* namespaced ids (this makes documented intent enforced).

**Provider burden vs. collision safety.** D1's mandatory id is a genuine cost to a hypothetical id-less
provider. But the alternative (name fallback) does not merely inconvenience — it silently corrupts the
library by merging distinct entities, and the corruption is discovered late and hard to reverse. For a
single-owner curated library the correctness guarantee dominates the provider-author convenience.

**Cross-provider power vs. merge risk.** D2's shared namespace is what makes multi-provider enrichment
*additive without duplication*. The merge-two-entities risk is bounded by namespace validation and by the
trust model (configured sidecars, not open input), and is strictly smaller than B's guaranteed
duplication.

---

## Consequences

**What becomes easier**
- Multi-provider enrichment (HOLODEX-118/119) de-dups by construction: two providers on the same record
  converge instead of doubling entities and chips.
- Refresh (ADR-047) and cross-enrichment hints are deterministic for *every* entity, not just studio.
- Person gains the same convergence studio already has; the credits path stops silently merging homonyms.
- One documented identity pattern for any future entity (tags) — no new identity design needed.

**What becomes harder**
- Provider authors MUST supply a stable namespaced id for every record and advertise its namespace; an
  id-less source is non-conformant. Documented in the provider contract (§4.1/§4.5, updated here).
- A new perimeter validation (shape + advertised-namespace) is added to the untrusted-input path.

**What we'll need to revisit**
- **F32 person-external-id capture** — reconcile the structured `people[]` id channel with this invariant
  (HOLODEX-125 picks the single id-capture style).
- **Legacy empty-id rows** — if any `entity_enrichment.external_id = ''` rows exist pre-enforcement, decide
  backfill vs. leave-inert (HOLODEX-124).
- **Tag enrichment** — instantiate the pattern when/if tags become enrichable (no work until then).

---

## Action Items

1. [x] ADR-055 recorded; added to `docs/architecture/README.md`.
2. [x] Provider-contract spec (§4.1/§4.5): external id is **required and namespace-validated**; the
   normalized-name fallback for credits is removed; `/describe.id_namespaces` is enforced, not advisory.
3. [x] `docs/testing-strategy.md`: note the invariant + its per-entity conformance and the perimeter
   validation, pointing at the implementation issues for the concrete tests.
4. [ ] **HOLODEX-124** — enforce mandatory namespaced id across the contract + shadow store (entity-agnostic):
   reject empty/malformed ids in `sanitizeCandidates`/apply/credits; validate namespace; a single
   `ParseExternalID`/namespace helper (generalize `idNamespace`). Gates: `/write-spec`, `/testing-strategy`,
   `/security-review`.
5. [ ] **HOLODEX-125** — person identity by external id: `person_external_ids(external_id PK)` + id-first
   resolve for provider-supplied persons; preserve F23 owner name-aliases/merge; reconcile F32. Gates:
   `/write-spec`, `/testing-strategy`, `/security-review`.
