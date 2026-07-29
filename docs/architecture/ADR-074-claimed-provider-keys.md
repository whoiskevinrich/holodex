# ADR-074: Claimed provider keys — a field's own source list is the sole suppression authority for auto-registration

**Status:** Proposed
**Date:** 2026-07-28
**Deciders:** Project owner

**Amends:** [ADR-056](ADR-056-provider-field-render-hints.md) — narrows its presence-driven auto-registration with a second suppression input: a provider key that already **feeds** a rendered field no longer earns a row of its own, whether or not it wins resolution for the entity being viewed. **Extends:** [ADR-062](ADR-062-in-app-field-promotion.md) (the `field_promotions` store this mirrors; claims merge **after** promotions and are mutually exclusive with them per key) · [ADR-013](ADR-013-metadata-field-mapping.md) (`mapping.Field.Sources` — the YAML half of the same mechanism) · [ADR-052](ADR-052-baseline-source-contract.md) (`ResolveFields` — a claim materializes into the `[]mapping.Field` this core already consumes, on all three entities). **Passes (does not widen):** [ADR-039](ADR-039-provider-asset-urls.md) (asset-host allowlist — unchanged on both the auto-registered and canonical paths) · [ADR-030](ADR-030-access-control-gating-seam.md) / [ADR-046](ADR-046-owner-session-persistence.md) (owner gate). **Relates to:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (per-field source decisions — the instrument that picks a winner, which a claim deliberately does not touch) · [ADR-055](ADR-055-enrichment-unique-key-invariant.md) (untrusted-provider perimeter — a claimed value is still provider data). **Spec:** [Claimed provider keys (F49)](../specs/claimed-provider-keys.md). **Issue:** [HOLODEX-218](https://whoiskevinrich.atlassian.net/browse/HOLODEX-218) · [GH #178](https://github.com/whoiskevinrich/holodex/issues/178). **Security:** `/security-review` **not** required — see Security below, which records the test the spec deferred against.

---

## Context

[ADR-056](ADR-056-provider-field-render-hints.md) (F39) auto-registers any non-canonical provider key that has a value, display-only. It suppresses a key that is "already rendered" — and that suppression is where [GH #178](https://github.com/whoiskevinrich/holodex/issues/178) lives. `appendAutoRegistered` builds its `rendered` set from **canonical names**:

```go
rendered[strings.ToLower(strings.TrimSpace(f.Canonical))] = true   // internal/api/auto_register.go
```

and `AutoRegisterFields` tests that set against the **raw provider key**:

```go
if key == "" || … || registry.IsKnown(key) || rendered[key] { continue }   // internal/resolver/auto_register.go:74
```

Both sides are lowercased and trimmed, so the comparison looks sound. It is comparing two different namespaces. A provider key is suppressed only when it happens to be spelled the same as the canonical it feeds: `tmdb:overview → overview` survives by coincidence; `provA:synopsis → overview` does not, and the same paragraph renders as Overview, Synopsis *and* Comments.

The reporter's instinct — list the key under the field's `sources:` — is the right mental model and does nothing today, because **nothing on the auto-registration path reads a field's sources**. Suppression is decided before the mapping is consulted. The mechanism this ADR backs is that instinct made real: a field's source list *is* the statement "this key is me."

The spec ([F49](../specs/claimed-provider-keys.md)) resolved the product-level questions and deferred two architecture questions here:

- **Q1** — does a claim row carry a precedence position, letting an in-app claim outrank a YAML source?
- **Q2** — what happens to a claim whose target canonical later disappears?

Both are answered below (D3, D4), and the second turns out to be answered *structurally* rather than by policy.

### Constraints / forces

- **Two persistence paths, one meaning.** `metadata-mappings.yaml` governs **video only** — person and studio field sets are synthesized in code ([`person_fields.go`](../../internal/api/person_fields.go), `studios.go`), so for two of three entity types YAML is not a poorer option, it is *no* option. This is the same force that produced [ADR-062](ADR-062-in-app-field-promotion.md) D1, and it lands the same way. But YAML `sources:` must keep working and must mean exactly what a DB claim means, or operators learn two mechanisms.
- **Resolution stays pure** ([ADR-052](ADR-052-baseline-source-contract.md)). A claim is a **pre-loaded input** merged into `[]mapping.Field` before `ResolveFields` — no new per-field query, no conflict arbitration inside the merge core.
- **A claim must never silently move the winner.** Adding a claim is a statement about *identity* ("this key is that field"), not about *precedence* ("this key should win"). Precedence is already an instrument the owner holds per entity ([ADR-051](ADR-051-per-field-source-of-truth-decisions.md) source decisions); a claim that reshuffled winners would make an identity edit look like a data edit.
- **Suppression must never open a black hole.** Hiding a key without giving its value somewhere to go loses it from the UI entirely. Any design where "suppress" and "contribute" are two independent code paths can drift into exactly that.
- **Provider-scoped.** `provA:synopsis` being claimed must leave `provB:synopsis` alone — two providers can use one word for different things.
- **Zero-impact when unused.** No claims and no `sources:`-listed provider keys ⇒ byte-identical output to F39.

---

## Decision

Introduce **claimed provider keys**: a canonical field may **claim** a differently-named provider key, so the key contributes its value as a candidate of that field and stops auto-registering as a separate display-only row. Claims are expressed two ways — operator `sources:` in `metadata-mappings.yaml` (video) and an owner-gated DB store (`field_claims`, all three entity types) — that converge on **one** representation before anything reads them.

### D1 — Claims persist in a new `field_claims` store, mirroring `field_promotions`

```sql
CREATE TABLE field_claims (
  entity_type TEXT NOT NULL,               -- 'video' | 'person' | 'studio'
  provider    TEXT NOT NULL,               -- enrichment namespace (entity_enrichment.provider)
  field_key   TEXT NOT NULL,               -- the non-canonical provider key being claimed
  canonical   TEXT NOT NULL,               -- the field that claims it
  created_at  TEXT NOT NULL,               -- RFC3339 UTC (timeLayout), matching every ts column
  updated_at  TEXT NOT NULL,
  PRIMARY KEY (entity_type, provider, field_key)
);
```

Migration `0029_field_claims` (`.up.sql` + hand-written `.down.sql`); repo `internal/repo/claims.go` mirrors [`promotions.go`](../../internal/repo/promotions.go) end to end — `SetClaim` / `ClearClaim` / `ClaimsForEntityType`, upsert and delete under `writeMu`, stamping `timeLayout`, with the same small per-type cache and reload.

**The PK grain differs from `field_promotions` on purpose.** A promotion is global per `(entity_type, field_key)` because it is about presentation, which is shared. A claim carries `provider` in the key because identity is **not** shared: `provA:synopsis` and `provB:synopsis` are different assertions, and one row must never speak for both.

`metadata-mappings.yaml` stays operator-only — the app never reads or writes it beyond today ([ADR-062](ADR-062-in-app-field-promotion.md) D1).

### D2 — Suppression derives from the merged field set, not from the claims table *(the load-bearing decision)*

A claim **materializes as an appended `mapping.Source{Namespace: provider, Key: field_key}`** on its target field, exactly as `mergePromotions` materializes a promotion. Auto-registration then asks a single question of the **effective `[]mapping.Field` handed to `ResolveFields`**:

```
ClaimedKeys(effective []mapping.Field) map[string]bool     // "<provider>:<key>", lowercased + trimmed
```

Everything else follows from that one derivation:

- **YAML and DB claims are indistinguishable** by the time suppression runs. A `sources:` entry and a `field_claims` row produce the same `mapping.Source`; there is one suppression path, not two, and no possibility of the two halves disagreeing.
- **Every present and future source of fields is covered for free** — parsed YAML (video), the synthesized person/studio sets, and F44 promotions all arrive as `[]mapping.Field` at the same seam. (The ticket proposed deriving from `Mappings.Fields()`, which would have covered video only.)
- **The black hole is structurally impossible, not guarded against.** Because suppression reads the *materialized* field set, a claim that fails to materialize suppresses nothing. There is no code path on which a key can be hidden without its value having somewhere to go — this is what settles Q2 (D4) rather than a policy.

**Only namespaced provider sources claim.** A bare source (`Comment`, `Artist`) and the `file:` namespace are file tags, not provider keys, and must never claim — otherwise one mapping's `Comment` source would swallow every provider's `comment` key.

**`rendered` is retained, unchanged and un-subsumed.** The two checks answer different questions, and neither implies the other:

| Check | Question | Catches |
|---|---|---|
| `rendered[key]` | is this key *the name of* a field we already showed? | `tmdb:overview` when `overview` renders from `file:` baseline alone |
| `claimed["provider:key"]` | is this key already *feeding* a field we already showed? | `provA:synopsis` feeding `overview` |

**Suppression is unconditional.** A claimed key does not auto-register whether or not its provider wins resolution for the entity being viewed. The claim is a config-level statement about identity, not a per-entity outcome; a row that appeared on videos where its provider lost and vanished where it won would be worse than the duplicate it replaced.

**A partially-claimed key still auto-registers.** With `provA:synopsis` claimed and `provB:synopsis` not, the `synopsis` row survives carrying provB's value and provB's provenance only. This falls out of the existing per-`(provider, key)` accumulator loop with no special case, and is pinned by a test.

### D3 — The row carries the target canonical only; claims append at lowest precedence *(settles Q1)*

No precedence column. An in-app claim appends at the **end** of the target field's candidate list, below every YAML source. Three reasons, in order of weight:

1. **Adding a claim must not move the winner** (Constraints). A precedence position would make "this key is the same thing as Overview" also mean "…and it should now be Overview's value," conflating identity with data.
2. **Per-entity precedence already exists and is the better instrument.** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) source decisions let the owner pick a winner *for the entity in front of them*, which is where the judgement actually belongs. A global ordering column would be a second, blunter answer to a question already answered.
3. **It would have no arbiter against ADR-051 §8's global provider trust order.** Two global precedence mechanisms with no stated relationship is exactly the emergent-precedence trap [ADR-062](ADR-062-in-app-field-promotion.md) D3 avoided by writing its ladder down.

**Deterministic ordering among claims.** When several claims target one canonical, they append sorted by `(provider, field_key)` — not by `created_at`. Lexicographic order is reproducible from the table's contents alone and survives a delete-and-re-add; insertion order would make resolution depend on edit history, which is untestable and surprising.

**Claims are not a presentation tier.** [ADR-062](ADR-062-in-app-field-promotion.md) D3 established a tier-0..4 ladder for `(label, render, group, order)`. A claim touches neither end of it: it changes *which sources feed a field* and *whether a key auto-registers*, and never contributes a label. There is no tier −1.

**Claiming grants no new curation surface.** The value joins a canonical field that already carries F36 decisions and F30 curation; nothing becomes curatable that was not already.

### D4 — A dangling claim is inert and survives; nothing is auto-pruned *(settles Q2)*

A claim whose target canonical is absent from the effective field set (mapping edited, promotion cleared, entity type changed) contributes **nothing**: no appended source, and — by D2 — no suppression either. The key simply auto-registers again, which is precisely the pre-F49 behaviour, so the failure mode is *visible* rather than silent. The resolve path logs it; P1.1's claims list marks it broken.

The row is **not** deleted. Target absence is usually transient — a `metadata-mappings.yaml` edit awaiting `reload-config`, a promotion cleared and about to be re-made — and pruning on a transient state destroys owner intent that was never withdrawn. This is the same reasoning as [ADR-062](ADR-062-in-app-field-promotion.md) D-reversible, where decisions and curation survive de-promotion because they are keyed independently of it. *(Rejected: prune-on-read — a config reload race silently eats claims; prune-on-reload — same hazard, just rarer and harder to reproduce.)*

### D5 — Mutual exclusion with promotion is enforced at write time, in one transaction

A key may be promoted (F44) or claimed (F49), never both — the spec's RD3. `PUT /admin/field-claims/...` on a key that currently holds a promotion clears the promotion **in the same transaction**; the affordance names the promotion it will remove before applying (FR5).

Write-time, not resolve-time, for two reasons: resolve must stay pure and free of conflict arbitration ([ADR-052](ADR-052-baseline-source-contract.md)), and only a write-time rule can tell the owner what is about to be destroyed while they can still decline.

**Merge order: claims merge after promotions.** A claim may therefore target a *promoted* field — key `X` promoted to its own field, later joined by `provB:slogan` — which is a legitimate and useful combination, distinct from the same key being both.

### Mechanism (the seams)

1. **Store** — migration `0029_field_claims`; `internal/repo/claims.go` mirroring `promotions.go`.
2. **Merge** — `mergeClaims(ctx, entityType, fields)` runs immediately after `mergePromotions` at all three call sites ([`handlers.go:506`](../../internal/api/handlers.go), [`person_fields.go:129`](../../internal/api/person_fields.go), [`studios.go:124`](../../internal/api/studios.go)), appending each claim's source to its target field and skipping claims whose target is absent (D4).
3. **Derivation** — `resolver.ClaimedKeys(effective []mapping.Field) map[string]bool`, pure, in `internal/resolver/auto_register.go` beside the pass that consumes it.
4. **Suppression** — `AutoRegisterFields` gains `claimed map[string]bool` as a second suppression input alongside `rendered`; `appendAutoRegistered` gains the effective `[]mapping.Field` (which every caller already holds) and derives the set.
5. **API** — owner-gated routes in the `requireOwner` group: `GET /admin/field-claims/{entity_type}`, `PUT|DELETE /admin/field-claims/{entity_type}/{provider}/{field_key}`. Type-global path, not `/{entity}/{id}/...`, for the same reason [ADR-062](ADR-062-in-app-field-promotion.md) chose `/admin/field-promotions` — the URL must not imply per-entity scope. Validation: `entity_type` via the existing `parseEntityType` (400); `field_key` non-canonical and non-`_`-prefixed (422 — you cannot claim `bio`, it is already canonical); `canonical` must be a field of that entity type (422); `DELETE` idempotent (204 on missing).
6. **SPA** — a claim affordance beside F44's promote affordance on an auto-registered row, with a picker over the entity type's canonical fields. Placement and picker scope are a `/design-handoff` concern (spec Q3).

### Security

The write surface is one owner-gated CRUD trio over a keyed lookup table whose three inputs are each validated against a closed vocabulary — `entity_type` against `parseEntityType`, `field_key` against the registry (non-canonical, non-reserved), `canonical` against the entity type's own field set. There is no free-text sink, no new network egress, no filesystem access, and no credential handling.

The [ADR-039](ADR-039-provider-asset-urls.md) image perimeter is untouched. `gateImageURL` still runs on the auto-registered and promoted paths; a claimed value routes through the **canonical** field path, which already applies the canonical image rules. A claim cannot move a value onto an ungated path.

**One asymmetry with [ADR-062](ADR-062-in-app-field-promotion.md), stated plainly.** That ADR's D-filterable forbids a promotion from creating a browse facet. A claim *can* put a provider's values into a canonical field that is filterable — but only into one that the registry or operator **already declared** filterable and that **already accepts provider sources**. The claim adds a candidate to a declared surface; it cannot invent one. That is the same thing a YAML `sources:` entry does today, on the path browse already trusts. The constraint that holds this bound is FR4's 422: `canonical` must be an existing field of that entity type.

Taken together this is the "nothing beyond a keyed lookup table" test the spec's §9 deferral was written against, and it holds — so **no `/security-review` gate**. Recorded here so the deferral is a decision with a stated test, not an omission. If a later slice widens any of the three vocabularies to free text, or lets a claim target a field the entity type does not declare, that trigger fires.

---

## Options Considered

### D1 — where a DB claim lives

#### A — a new `field_claims` table mirroring `field_promotions` (chosen)
| Dimension | Assessment |
|-----------|------------|
| Complexity | Low — one table + a repo file mirroring `promotions.go`; the merge seam already exists |
| Cost | One migration; one small per-type cached read per resolve, beside the promotion read |
| Scalability | Uniform across video/person/studio; provider-scoped grain is correct by construction |
| Team familiarity | High — the third store with this exact shape (`decisions.go`, `promotions.go`) |

**Pros:** the only option serving person/studio; provider in the PK expresses provider-scoping in the schema rather than in code. **Cons:** a third owner-config store — mitigated by it being the same shape as the two beside it, and by D2 collapsing both persistence paths to one representation before anything reads them.

#### B — a nullable `claims_canonical` column on `field_promotions`
**Pros:** no new table; makes RD3's mutual exclusion a column-level invariant. **Cons:** the grain is wrong — `field_promotions` is keyed `(entity_type, field_key)` with no `provider`, so it cannot express `provA:synopsis` claimed while `provB:synopsis` is not, which is a hard requirement. Widening its PK would change the meaning of every existing promotion row. Rejected.

#### C — YAML `sources:` only, no DB store
**Pros:** zero new storage; fixes the reported bug outright. **Cons:** video-only — person and studio have no YAML at all, so two of three entity types would have no path. This is slice A of the spec's phasing, not a resting place. Rejected as the end state (accepted as the first slice).

### D2 — what auto-registration consults

#### A — derive the claimed set from the merged `[]mapping.Field` (chosen)
**Pros:** one suppression path for both persistence mechanisms; covers YAML, synthesized, and promoted fields with no per-source special case; makes the suppress-without-a-home failure unrepresentable. **Cons:** the effective field set must be plumbed into `appendAutoRegistered` — which every caller already has in hand, so this costs a parameter.

#### B — suppress directly from the `field_claims` rows
**Pros:** one fewer parameter; suppression is a direct table lookup. **Cons:** two independent suppression paths (rows for DB claims, nothing for YAML) that must be kept in agreement forever, and — decisively — suppression would no longer be tied to materialization, so a dangling claim could hide a value with nowhere to go. Q2 would need a policy where D2 gives a structural guarantee. Rejected.

### D3 — precedence

#### A — target canonical only, append last, lexicographic tie-break (chosen)
**Pros:** an identity edit stays an identity edit; precedence stays with the per-entity instrument built for it; resolution is reproducible from table contents. **Cons:** an owner who wants a claimed source to win must make one more gesture (the F36 source chip) — which is the gesture that already exists and already shows what it is doing.

#### B — a precedence column on the claim row
**Pros:** one gesture to claim *and* prefer. **Cons:** adding a claim could silently change displayed values across the library; competes with ADR-051 §8's provider trust order with no stated arbiter; global where the useful judgement is per-entity. Rejected.

---

## Trade-off Analysis

**A third owner-config store vs. two mechanisms that must agree.** The cost of D1 is one more table beside `field_promotions` and `field_source_decisions`. The mitigation is D2: because both persistence paths materialize into the same `mapping.Source` before anything reads them, the store is a *second way to write* one mechanism, not a second mechanism. Contributors adding a future third way to author fields get claim behaviour without touching the suppression code at all.

**Structural safety vs. a policy.** The most consequential choice here is small and easy to miss: suppression reads the merged field set rather than the claims table. Both would fix GH #178. Only the first makes "value suppressed with nowhere to go" impossible to express — which converts Q2 from a policy needing a rule, a log line, and a test into a property that holds because there is no code path on which it can fail. When a safety property can be made structural at the cost of one function parameter, it should be.

**Identity vs. precedence, held apart.** D3 refuses the convenience of a precedence column. The immediate cost is real: an owner claiming a key and *also* wanting it to win makes two gestures. The purchase is that a claim can be made freely — on a hunch, to tidy a page — without any risk of quietly changing values across the library. In a personal media server where the owner is also the operator and there is no review step between intent and effect, an edit that cannot break data is worth more than an edit that saves a click.

---

## Consequences

**What becomes easier**
- One paragraph renders once, with combined provenance, on video, person, and studio.
- An operator's `sources:` list finally means what it reads like — the field's statement of which provider keys are it.
- A future field-authoring mechanism inherits claim behaviour by materializing `[]mapping.Field`, with no change to auto-registration.

**What becomes harder**
- **Not purely additive** (spec §8): a key already listed under `sources:` stops auto-registering the day this ships. It was rendering twice, so a duplicate goes away rather than data — but a row the owner is used to seeing will disappear, and a *losing* claimed source is now one click away behind the F36 source chip rather than on the page. Release note required.
- Anyone who exploited the bug to surface a second provider's text as its own row must now use F44 promotion — which is the affordance actually meant for it.
- Two suppression checks (`rendered`, `claimed`) that look redundant and are not. The table in D2 exists so the next contributor does not delete one.

**What we'll need to revisit**
- **Proactive duplicate detection** (spec P1.3 / slice C) — counts must be library-wide, and value equality may only ever prompt, never auto-fold.
- **A claims management surface** (P1.1) and the inverse "unclaim" gesture from the source chip (P1.2), which is also where a dangling claim (D4) gets surfaced.
- **MCP visibility of a claimed key** (spec Q4) — nothing in the MCP surface reads auto-registered rows today; revisit if that changes.
- **The filterable asymmetry** (Security) — if a claim is ever allowed to target a field the entity type does not declare, the security deferral is void.

---

## Action Items

1. [ ] ADR-074 recorded; add to `docs/architecture/README.md`; cross-reference from ADR-056's auto-registration section and ADR-062's Consequences.
2. [ ] **Derivation + suppression (slice A)** — `resolver.ClaimedKeys`; `AutoRegisterFields` second suppression input; `appendAutoRegistered` takes the effective `[]mapping.Field` at all three call sites.
3. [ ] **Store (slice B)** — migration `0029_field_claims` (`.up`/`.down`); `internal/repo/claims.go` mirroring `promotions.go`.
4. [ ] **Merge (slice B)** — `mergeClaims` after `mergePromotions` at all three call sites; dangling targets skipped + logged (D4); lexicographic append order (D3).
5. [ ] **API (slice B)** — owner-gated `GET`/`PUT`/`DELETE /admin/field-claims/...`; 400 unknown entity type; 422 canonical/reserved `field_key`; 422 undeclared target canonical; idempotent delete; RD3 promotion clear in the same transaction (D5).
6. [ ] **SPA (slice B)** — claim affordance beside the F44 promote affordance + canonical picker; three-skin QA. (`/design-handoff`, spec Q3.)
7. [ ] **Tests** — `docs/testing-strategy.md` F49 block: bare-file-tag claims nothing, `file:` namespace claims nothing, same-key-different-provider isolation, unconditional suppression (claimed source both winning and losing), partially-claimed key still auto-registers with the remaining provider only, dangling claim neither suppresses nor appends, claim appends last, promotion cleared on claim, golden no-op with no claims. (`/testing-strategy`.)
8. [ ] **Docs** — the spec §6.5 cookbook lands in `docs/reference/canonical-fields.md` + `metadata-mappings.yaml.example` (spec FR7, ships with slice A).
