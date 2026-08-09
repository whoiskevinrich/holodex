# ADR-083: Provider-Link Badge — Extending Namespace-Qualified Display to Person and Studio

**Status:** Proposed
**Date:** 2026-08-09
**Deciders:** Kevin Rich
**Extends:** [ADR-082](ADR-082-external-provider-id-namespace-qualified-value.md) (namespace-qualified
value convention — this ADR fulfils and refines its unchecked action item 6, turning "a display
concern in the frontend field-rendering layer" into a backend-declared link-construction contract,
and widens the badge from video-only to person/studio) · [ADR-033](ADR-033-metadata-source-plugins.md)
(`/describe` `Manifest` contract — adds `link_templates`) · [ADR-059](ADR-059-provider-brand-icon.md)
(`BrandIcon` self-declaration — same provider-declares-its-own-presentation pattern this ADR reuses
for link shape).
**Relates to:** [ADR-054](ADR-054-studio-external-id-dedup.md) (`studio_external_ids`) ·
[ADR-055](ADR-055-enrichment-unique-key-invariant.md) (the namespaced-id identity invariant for
person/studio — this ADR treats `person_external_ids`/`studio_external_ids` as an already-conformant
**read** source and changes nothing about identity/dedup) · [F55 spec](../specs/entity-completeness-score.md)
(the badge is explicitly **not** wired into completeness scoring by this ADR — see Consequences →
Revisit).
**Spec:** none new — a display-only surface; see Action Items for the doc that should exist before
implementation.
**Issue:** [HOLODEX-266](https://whoiskevinrich.atlassian.net/browse/HOLODEX-266).

---

## Context

ADR-082 replaced a bare `imdb_id` rename with a namespace-qualified value
(`"tmdb:603"`/`"imdb:tt1234567"`) for video's `external_provider_id` registry facet, and left one
item unchecked: nothing strips the namespace for display or builds the outbound provider link. A
design pass (mockups + `/design-critique`) resolved *how* that should look — not a raw id, not even
a link labelled with the id, but a small clickable **provider badge** ("IMDb") living inline in the
entity header's metadata row (next to resolution/duration/year), that links out to the provider's
own page for the record. The owner then asked for the same treatment on **person** and **studio**
detail pages.

That extension is not a copy-paste of the video badge, because person and studio don't store their
provider ids the way video does:

| Entity | Where the id lives | Shape |
|---|---|---|
| Video | `entity_enrichment` → per-field source decisions → **resolver** → registry facet `external_provider_id` (ADR-051/052) | Scalar — one resolved winner |
| Person | `person_external_ids(external_id PK, person_id FK)` (migration 0038, ADR-055 D2/HOLODEX-125) | Join table — 0..N rows, **identity key**, never resolved |
| Studio | `studio_external_ids(external_id PK, studio_id FK)` (migration 0018, ADR-054) | Join table — 0..N rows, **identity key**, never resolved |

Video's value already flows through the pure resolver and has a single per-field trust-tiered
winner. Person's and studio's ids are the **de-dup identity key** ADR-054/055 rely on — they were
deliberately kept outside the resolver/curation model (an id is not something to "override" the way
a title or tagline can be) and an entity can legitimately hold ids from more than one converging
provider (ADR-055 D2).

### Forces

- **The user's ask is display-only.** "These examples should extend to all entity types" was about
  the badge, not a request to re-open F55 completeness scoring or the identity model for two more
  entities. Scope creep here would answer a question nobody asked.
- **Providers are declared, not compiled in** (ADR-033). A self-hosted deployment can configure a
  provider Holodex's frontend has never heard of. Any "strip the prefix, build a link" logic that
  hardcodes per-namespace URL shapes in Svelte silently breaks for that provider — worse, the video
  badge and a hypothetical hand-rolled person/studio badge would each reinvent this and drift.
- **The link target is exactly the kind of thing that belongs behind the SSRF/asset perimeter**
  `internal/enrich` already owns (`base_url` allowlist, `asset_hosts` allowlist, ADR-033/039).
  Building an `<a href>` to a third party from client-guessed string concatenation is the one thing
  that perimeter exists to keep out of the frontend.
- **Person/studio ids have no "winner."** Unlike video's resolved scalar, the join tables can hold
  multiple rows per entity (cross-provider convergence, ADR-055 D2). A design that assumes "the"
  external id doesn't fit the data.
- **Don't retrofit the resolver onto an identity key.** ADR-055 D1 already forbids the person/studio
  id having ambiguous or overridable trust tiers — it's binary identity, not a curatable field.
  Running it through per-field source decisions to get a badge would contradict that invariant for
  no benefit.

---

## Decision

The provider badge (ADR-082's design: provider name, not id; inline in the header metadata row;
clickable link-out) is extended to person and studio as a **read-only projection of existing
identity data**, with the outbound link **built server-side from a provider-declared template**.
Three sub-decisions:

### D1 — Data source: read-only projection, not a promoted registry facet

Person/studio detail responses gain a small read-only array (e.g. `external_links: []`) populated
directly from `person_external_ids`/`studio_external_ids` — no new write path, no resolver
involvement, no F55 facet-table change. This is presentation of data that already exists and is
already the identity key (ADR-054/055); it is not a new facet, has no per-field source decision, no
curation override, and does not participate in completeness scoring.

### D2 — Link construction: provider-declared template, resolved server-side

The `/describe` `Manifest` (`internal/enrich/enrich.go`, alongside `IDNamespaces`/`BrandIcon`) gains
an optional `LinkTemplates map[string]map[string]string` — outer key is a namespace the provider
already advertises in `IDNamespaces` (validated per ADR-055 D2), inner key is the Holodex
`EntityKind` (`"video"`/`"person"`/`"studio"`), value is a URL template with a single `{id}`
placeholder. This is what lets one provider emit different URL shapes for a title vs. a person
under the same namespace (e.g. `imdb:tt1234567` → `https://www.imdb.com/title/{id}/` vs.
`imdb:nm7654321` → `https://www.imdb.com/name/{id}/`).

Holodex resolves `(namespace, entity_kind, id)` → full URL **in the backend** and returns
`{provider, label, url}` per badge — the frontend never sees a raw id or synthesizes a URL, mirroring
the existing `EnrichCandidate.profile_url` precedent (provider-attested, server-side, client only
re-validates the scheme before rendering as a link). A namespace with no matching template entry, or
an id that fails basic template-safety validation, renders the provider name with **no** href rather
than a best-guess URL.

### D3 — Badge cardinality: one badge per stored id, no "primary" selection

Video shows exactly one badge (the resolver's single winning value). Person/studio render **one
badge per row** in their respective join table — zero, one, or several. No new precedence logic is
invented; this reuses the join table's existing semantics unchanged, and multiple badges is itself
useful signal (an entity is cross-referenced across providers). This is a deliberate, documented
asymmetry from video's single badge, not an inconsistency to reconcile.

---

## Options Considered

### D1 — where person/studio badge data comes from

#### A — Read-only projection of the existing identity tables (chosen)

**Pros:** No schema change; no resolver/curation involvement; can't contradict ADR-055's
"id is not overridable" invariant because it never enters a system that allows overriding; smallest
change that satisfies what was actually asked. **Cons:** The badge can never be marked
not-applicable or scored toward completeness the way video's can — it's a strictly weaker surface
than the video facet. Accepted: that weakness is a feature, not a gap — see D1's Forces.

#### B — Promote to a resolver-backed registry facet (widen F55's Person/Studio tables)

**Pros:** Consistent completeness-scoring treatment across all three entity types; reuses
`CompletenessPanel`'s existing not-applicable UI verbatim. **Cons:** Requires a resolver adapter
that reads a join table instead of `entity_enrichment` rows; requires deciding source-decision
precedence for a value ADR-055 explicitly declared has none; requires reopening the F55 spec's
Person/Studio facet tables and its scoring weights — three separate design decisions bundled into
what the user asked for as a display tweak. Rejected for this ADR; captured as a Revisit item below.

### D2 — where the outbound URL is built

#### A — Provider-declared `link_templates`, resolved server-side (chosen)

**Pros:** Stays inside the "providers declared, not compiled in" model (ADR-033) — a
self-configured provider Holodex has never shipped code for still gets a working badge; keeps the
one link-building code path shared by video/person/studio instead of three; keeps the
SSRF-adjacent "what hosts do we ever link to" concern inside `internal/enrich`, next to
`base_url`/`asset_hosts`, instead of duplicated in Svelte. **Cons:** A new optional field on an
already-large `Manifest` struct; providers that don't declare it simply get no link (acceptable
degradation — see D2 decision text). Accepted.

#### B — Frontend-hardcoded per-namespace URL map

**Pros:** No backend change; ships faster. **Cons:** Reintroduces exactly the "the frontend must
know about every provider" coupling ADR-033 exists to remove; breaks silently for any
operator-configured provider outside the hardcoded set; duplicates the outbound-host allowlist
concern in a second place with no SSRF-review coverage. Rejected.

### D3 — badge cardinality for person/studio

#### A — One badge per stored external-id row (chosen)

**Pros:** Zero new logic — the join tables already hold exactly the rows to render; surfaces real
cross-provider information (ADR-055 D2's whole point) instead of hiding it. **Cons:** A person/studio
with several converged providers shows several badges where video shows one, a visible asymmetry.
Accepted: it reflects a real difference in the underlying data model rather than manufacturing false
parity.

#### B — Pick a single "primary" badge (first-inserted, or a namespace priority order)

**Pros:** Visual parity with video's single badge. **Cons:** Requires inventing a priority order
with no natural default (namespace alphabetical? insertion order? most-recently-refreshed?); throws
away the fact that an entity is known to multiple providers, which is real information the owner
asked to see ("knowing the entity has been enriched" — plural providers is a stronger signal than
one). Rejected.

---

## Trade-off Analysis

**Scope discipline vs. completeness parity.** D1-A intentionally leaves person/studio's completeness
score untouched by this badge, which means video's `external_provider_id` facet and the new
person/studio badges are visually similar but structurally different (one is scored, resolved,
curatable; the others are a raw read of an identity table). That's an accepted asymmetry: chasing
full parity (Option D1-B) would mean re-deciding three separate things ADR-055 and the F55 spec
already settled on purpose (id-as-identity, not id-as-curatable-field; Person/Studio facet
composition). A display feature is the wrong place to relitigate those.

**Provider-declared templates vs. shipping speed.** D2-A costs a `Manifest` field and a small
backend URL-builder the frontend-hardcoded alternative wouldn't need — but the project's whole
provider model (ADR-033) exists to keep exactly this kind of provider-specific knowledge out of
compiled code. A frontend-hardcoded map is the one shortcut that would make the badge stop working
the moment someone points `metadata-sources.yaml` at an unfamiliar provider, which is a realistic
case for a self-hosted, provider-agnostic deployment.

**Multiple badges vs. visual tidiness.** D3-A can, in the multi-provider case, put two or three
badges in a header row that shows one for video. This is judged worth it because the alternative
(D3-B) requires fabricating a priority order that has no principled default and actively discards
signal ADR-055 was written to surface (cross-provider convergence).

---

## Consequences

**What becomes easier**
- Person and studio detail pages get the same "know it's enriched, click through to the source"
  affordance as video, without touching the resolver, F55 scoring, or the ADR-055 identity model.
- Any future provider (declared via `metadata-sources.yaml`, no code change) that advertises
  `link_templates` gets working badges on all three entity types for free.
- The link-building logic lives in exactly one place (`internal/enrich`), reusable by video's
  existing facet value and person/studio's new projection alike.

**What becomes harder**
- Provider authors who want the badge to link out must additionally declare `link_templates`
  (existing providers that only set `id_namespaces` still work — they just render a badge with no
  href, same as today's plain-chip rendering, so this is strictly additive).
- Two different "does this entity have a provider id" surfaces now exist side by side: the scored,
  not-applicable-toggleable video facet, and the unscored person/studio projection. Anyone extending
  completeness scoring to person/studio later must consciously decide whether to fold this
  projection into that work or keep them separate.

**What we'll need to revisit**
- **Completeness scoring for person/studio external ids** (D1-B, rejected here) — if the owner later
  wants person/studio to score an external-id facet the way video does, that's a new ADR against the
  F55 spec's facet tables, not a re-read of this one.
- **Multi-badge layout at scale** — if cross-provider convergence becomes common (more than 2-3
  providers per entity), the header row may need a "+N more" overflow treatment; not a problem with
  today's typical single-configured-provider deployments.

---

## Action Items

1. [x] ADR-083 recorded; added to `docs/architecture/README.md`.
2. [ ] `/write-spec`: a short design-handoff note for the person/studio badge (visual spec already
   exists from the video mockups; this covers the multi-badge and no-link-degradation states) —
   satisfies the project's UX change-routing gate.
3. [ ] Backend: `Manifest.LinkTemplates map[string]map[string]string` (`internal/enrich/enrich.go`),
   sanitized/validated on `/describe` ingest alongside `FieldHints`/`BrandIcon`; a
   `BuildProviderLink(namespace, entityKind, id) (url, ok)` helper shared by video's existing facet
   render path and the new person/studio projection.
4. [ ] Backend: person/studio detail handlers project `person_external_ids`/`studio_external_ids`
   rows into `external_links: [{provider, label, url}]` via the new helper; no resolver/registry
   change.
5. [ ] Frontend: extract the video badge into a shared component consumed by video/person/studio
   headers; person/studio render zero-to-many badges from `external_links`.
6. [ ] `/testing-strategy`: cover template-mismatch degradation (badge renders, no href) and the
   multi-badge case.
7. [ ] `/security-review`: confirm `LinkTemplates` values are validated (single `{id}` placeholder,
   `https://` scheme, no injection via a malicious provider) before ever being interpolated into a
   URL served to a browser.
