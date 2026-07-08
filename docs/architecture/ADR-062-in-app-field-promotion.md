# ADR-062: In-app field promotion — an owner-authored DB override that makes an auto-registered field first-class curatable

**Status:** Proposed
**Date:** 2026-07-07
**Deciders:** Project owner

**Amends:** [ADR-056](ADR-056-provider-field-render-hints.md) — realizes its deferred **"Promotion UX"** consequence, adds a **tier-0** above its four-tier ladder (§D2), and **inverts** its "operator `metadata-mappings.yaml` always wins" rule *for a promoted key* (the owner's live promotion outranks operator YAML — see D3). **Extends:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (per-field source decisions — a promoted field gains them, keyed by the existing `(entity_type, entity_id, field_key)` grain) · [ADR-048](ADR-048-metadata-curation-and-write-queue.md) (F30 value curation — a promoted merge field gains it) · [ADR-052](ADR-052-baseline-source-contract.md) (`ResolveFields` / `BaselineSource` — promotions materialize into the `[]mapping.Field` this core already consumes, on all three entities) · [ADR-013](ADR-013-metadata-field-mapping.md) (`mapping.Field` + `metadata-mappings.yaml`, the operator surface a promotion overlays). **Passes (does not widen):** [ADR-039](ADR-039-provider-asset-urls.md) (asset-host allowlist — a promoted `image_url` is still gated) · [ADR-030](ADR-030-access-control-gating-seam.md) / [ADR-045](ADR-045-owner-session-persistence.md) (owner gate). **Relates to:** [ADR-055](ADR-055-enrichment-unique-key-invariant.md) (untrusted-provider perimeter — promotion values are still provider data). **Spec:** [In-app promote / override affordance (F44)](../specs/promote-override-fields.md). **Issue:** [HOLODEX-171](https://whoiskevinrich.atlassian.net/browse/HOLODEX-171). **Security:** `/security-review` required before merge (owner-authored config the resolver trusts).

---

## Context

[ADR-056](ADR-056-provider-field-render-hints.md) (F39) made a provider's **non-canonical** fields first-class in the UI with zero mapping config — but deliberately **display-only**. An auto-registered field renders read-only with a provider-supplied (tier-3) or title-cased (tier-4) label, and is kept out of `field_source_decisions` (F36), `metadata_curation` (F30), and writeback. ADR-056 named the gap in its own Consequences and punted:

> **Promotion UX** — surfacing "promote this field to a mapping" as an owner affordance (today it is a manual YAML edit). Deferred; capture as a follow-up issue.

Today the **only** way to give such a field a curated label, a chosen render mode, a deliberate order, or actual value curation is to hand-author a `metadata-mappings.yaml` entry (tier-1), promoting it to a first-class mapped field. Two forces make that path unworkable as the *only* path:

1. **Person and studio have no operator-YAML remap surface at all.** Their field sets are synthesized in code from a fixed canonical list (`internal/api/person_fields.go`, `studios.go`); `metadata-mappings.yaml` governs **video** only. For two of three entity types there is literally nowhere to write a promotion.
2. **YAML is deploy-time operator config** — gitignored, ephemeral in-container, requires a `reload-config`, has no per-entity granularity, and races with hand edits. The owner of a single-user server should not drop to a text editor to rename a field.

Crucially, **the owner *is* the operator** on a personal Holodex. ADR-056's "operator authority is sacred" force was written to keep an *untrusted provider* from relabeling `bio`; it was never meant to stop the human running the server from renaming a field live. That reframing is what licenses D3 below.

### Constraints / forces

- **Resolution stays pure** ([ADR-013](ADR-013-metadata-field-mapping.md)/[ADR-052](ADR-052-baseline-source-contract.md)). A promotion must be a **pre-loaded input** merged into the `[]mapping.Field` before `ResolveFields` runs — no new per-field query, no I/O in the merge core.
- **Entity-generic** ([ADR-052](ADR-052-baseline-source-contract.md)). One mechanism for video, person, and studio — each already builds its own `[]mapping.Field`; each now merges promotions first.
- **Reuse F36/F30 wholesale.** A promoted field must gain decisions + curation through the *existing* code paths, not a parallel model. The resolver already derives `Candidates`/merge-union from a field's `ParsedSources` gated by present enrichment (`replaceMarkers`, `resolver.go`), so the entire job reduces to **synthesizing a `mapping.Field` with the right `ParsedSources`**.
- **Non-canonical only.** A promotion may target only a key the code registry does not know (`registry.IsKnown == false`) and that is not `_`-prefixed — the same predicate F39 auto-registration uses. The registry/schema contract stays inviolate: you cannot "promote" `bio`.
- **Zero-impact when unused.** No promotions ⇒ byte-identical resolved output and rendering to F39. No protocol change, no provider change.
- **Owner-gated, bounded blast radius** ([ADR-030](ADR-030-access-control-gating-seam.md)). A promotion changes render + curation surface; it never mutates enrichment values and never writes files by itself.

---

## Decision

Introduce **in-app field promotion**: an owner-gated, **DB-backed** presentation override (`field_promotions`) that the resolver consults as a new **tier-0**, materializing a promoted non-canonical key into a synthetic `mapping.Field` so it becomes a **first-class, curatable** field via the existing F36/F30 code paths — with **zero** `metadata-mappings.yaml` editing, on all three entity types. Three sub-decisions were resolved with the owner (2026-07-07); this ADR records them and settles the two architecture-level open items the spec deferred here.

### D1 — The override lives in a new DB store; the app never writes YAML

A dedicated `field_promotions` table (migration `0023`), consulted by the resolver as a tier. `metadata-mappings.yaml` stays **operator-only** — the app never reads *or writes* it beyond today. This is also the **only** option that works for person/studio, which have no YAML path (Context force 1). *(Rejected: the app editing YAML — concurrency with hand edits, a required `reload-config`, video-only, and a poor surface for a live single-user app.)*

```sql
CREATE TABLE field_promotions (
  entity_type TEXT    NOT NULL,               -- 'video' | 'person' | 'studio'
  field_key   TEXT    NOT NULL,               -- the non-canonical shadow key
  label       TEXT    NOT NULL DEFAULT '',    -- '' ⇒ inherit tier-3/4 label
  render      TEXT    NOT NULL DEFAULT '',    -- '' ⇒ inherit; else text|long_text|chips|url|image_url
  hint_group  TEXT    NOT NULL DEFAULT '',    -- '' ⇒ inherit; else primary|attributes|extended
  ord         INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT    NOT NULL,               -- RFC3339 UTC (timeLayout), matching every ts column
  updated_at  TEXT    NOT NULL,
  PRIMARY KEY (entity_type, field_key)
);
```

- **Global per `(entity_type, field_key)`.** One row promotes the key for *every* entity of that type. **Presentation** (label/render/group/order) is shared; **value curation stays per-entity** on the existing `field_source_decisions` / `metadata_curation` tables (keyed by `field_key` + `entity_id`) — no new per-entity table.
- **Empty presentation columns inherit** from the lower tiers (provider hint → title-case), so a promotion whose only purpose is "make this curatable" need not restate the label.
- **No stored source list.** The row carries presentation only; the field's F36/F30 candidate sources are *derived* at resolve time (D-candidate below).
- Repo `internal/repo/promotions.go` mirrors `decisions.go`: `SetPromotion` / `ClearPromotion` (upsert / delete under `writeMu`, stamping `timeLayout`) and `PromotionsForEntityType`.

### D2 — "Promote" means full promotion, not a relabel

A promoted field becomes a first-class **curatable** field with the complete F36 source-decision + F30 curation controls — not merely a presentation relabel. It matches ADR-056's original framing of the deferred affordance, and it is what makes the feature worth a migration: the owner can pick a source, add, or suppress values on the field, exactly as a YAML-mapped field allows. `ResolvedField.AutoRegistered` is **false** for a promoted field. De-promotion (D-reversible) returns it to the display-only auto-registered state, untouched shadow value intact.

### D3 — The in-app promotion outranks operator `metadata-mappings.yaml` (revised ladder, supersedes ADR-056 §D2)

The owner's live promotion is the **most-authoritative** tier for a field's `(label, render/display, group, order)` and its curatable status. This is a deliberate departure from ADR-056's "operator YAML always wins," justified by the single-operator context (Context) — recorded here so the inversion is explicit and greppable.

```
0. In-app promotion  (new — field_promotions)            ← this feature (tier-0)
1. Operator metadata-mappings.yaml                        (ADR-013)
2. Code registry (registry.Lookup) — canonical keys       (unchanged; still wins for canonical)
3. Provider hint (provider_field_hints) — non-canonical    (ADR-056)
4. Title-case fallback
```

Because a promotion may target only a **non-canonical** key, it never collides with tier-2 (registry-canonical keys keep their contract). Tier-0 sits above tier-1 **only for the promoted non-canonical key**; every un-promoted key resolves exactly as ADR-056 defines. On a `canonical`-string collision between a promotion and a YAML `mapping.Field`, the promotion **replaces** it (rendered once, via the curatable path — see FR3).

### D-candidate (settles spec Open Item 2) — candidate sources are derived per-entity from shadow provenance

The resolver builds a replace field's `Candidates` (and a merge field's union) from the field's `ParsedSources`, keeping only providers that supplied a **non-empty** value in the pre-loaded `Enrichment` map. A promotion stores **no** source list (D1), so at the point each entity assembles its `[]mapping.Field`, the materialized promotion Field gets:

- `Canonical = field_key`, `Label`/`Display` from the tier-0/3/4 fold, and
- **`ParsedSources` = one `provider:<namespace>` per namespace present for `(entity_type, entity_id, field_key)` in that entity's shadow rows** (the `entity_enrichment.provider` column; union when several providers supply it). Baseline (`file`/intrinsic) is always a candidate — empty for a provider-only person/studio key, which is fine (F36 already shows "keep baseline" even when empty). `manual` is always available (the F36 `manual` decision / F30 `manual add` are independent of `ParsedSources`).

This is **entity-specific** even though the promotion row is global: presentation is shared, but *which providers are offered as candidates* follows what each entity actually has — computed at resolve time from data already in hand (`EnrichmentForEntity` / `EnrichmentForVideos`), so purity holds. `ResolveFields` then attaches `Decision`/`Candidates`/`InSync` (replace) or per-value union + curation (merge) with **no change to the merge core**.

**Replace vs. merge is derived from render:** `Multi = (render == "chips")`. `chips` is F39's multi-value render mode, so a chips-rendered promotion is a merge field (per-value F30 curation, union across providers); every other render mode is a scalar replace field (F36 source decision). This avoids a `multi` column on `field_promotions` and reuses F39's vocabulary as the sole multiplicity signal. *(Edge case — an owner wanting a merge field rendered as stacked text rather than chips — is out of scope; chips is the multi surface.)*

### D-filterable (settles spec Open Item 1) — a promotion is render + curation only; `Filterable` stays false in v1

A promotion **cannot** make a field a browse facet. The synthetic `mapping.Field` sets `Filterable = false` unconditionally, so a promoted field never enters the browse facet / query-param path. This keeps the security surface to render + curation (FR6): there is no way for a promoted, owner-supplied-but-provider-valued field to smuggle an unvalidated value into browse. "Promote to facet" is a deliberate **follow-up** (it needs the browse-facet value-validation review ADR-056's `Filterable` path implies) — captured as a HOLODEX issue, noted in Consequences.

### D-reversible — de-promotion is a row delete; curation survives

`DELETE`ing the `field_promotions` row reverts the key to its F39 auto-registered, display-only state. The underlying shadow value is never touched. Prior `field_source_decisions` / `metadata_curation` rows for the key are **keyed by `field_key`, independent of the promotion row**, so they persist across de-/re-promote and re-apply automatically on re-promotion. Idempotent: de-promoting a missing row is a no-op (204).

### Mechanism (the seams)

1. **Store** — migration `0023_field_promotions` (`.up`/`.down`); `internal/repo/promotions.go` (`SetPromotion`/`ClearPromotion`/`PromotionsForEntityType`), mirroring `internal/repo/decisions.go` (write-lock upsert/delete, `timeLayout`, `placeholders`/`toAnySlice`).
2. **Tier-0 fold** — the ADR-056 label/render/group/order helper gains tier-0: a promotion's non-empty presentation columns win; empty columns fall through to tiers 3→4 (provider hint → title-case).
3. **Materialize + merge** — per entity, before resolve: build the base `[]mapping.Field` (YAML for video; synthesized for person/studio), then for each promotion for that `entity_type`, **replace-or-append by `Canonical`** a synthetic `mapping.Field{Canonical: field_key, Label, Display: render, Filterable: false, Multi: render=="chips", ParsedSources: <derived, D-candidate>}`. Promotion wins the collision (D3).
4. **Auto-registration yields automatically** (FR3) — `AutoRegisterFields` already skips a key `rendered[key]` by a mapping/synthesized field. Because a promoted key is now a real `mapping.Field`, it is excluded from auto-registration with **no new predicate** — the field renders **once**, via the curatable path. Pin it with a test.
5. **API** — owner-gated routes in the `requireOwner` group (`PUT`/`DELETE`/`GET /admin/field-promotions/{entity_type}[/{field_key}]`), mirroring `internal/api/person_decisions.go`. `entity_type ∈ {video, person, studio}`; `field_key` validated **non-canonical** (`registry.IsKnown ⇒ 422`) and **non-`_`-prefixed** (`⇒ 422`); `render`/`group` coerced to the F39 vocabulary; `label` control-char-stripped + capped 64 (reuse the F39 ingest sanitizer). A **type-global** `/admin/field-promotions/...` path (not `/{entity}/{id}/fields/...`) is chosen so the URL does not imply per-entity scope.
6. **SPA** — `AutoFieldRows.svelte` gains `isOwner`; owner rows show a **Promote** control opening a small editor (label / render `<select>` / group / order) → `PUT` → `reloadDetail()`. After promotion the field leaves `extraFields` (no longer `auto_registered`) and renders through `SourceSelect`/`CurationFieldRow` **for free** via the existing `!f.auto_registered` partition. Promoted rows gain an owner-only **Edit / Remove promotion** affordance. Visitor view is unchanged in shape. Editor placement (inline expander vs. popover) is a `/design-handoff` concern.

### Security (owner-authored config the resolver trusts)

- Every mutation is behind `requireOwner`; a non-owner gets 401 before the handler and sees no affordance.
- `label`/`render`/`group` are **owner-supplied but still sanitized/validated on ingest** (defense in depth; reuse the F39 sanitizer): control-char strip, length cap, vocabulary coercion (unknown `render ⇒ text`). Labels render as **escaped text** (Svelte), never HTML.
- A promoted `image_url` field remains **asset-host allowlist-gated** ([ADR-039](ADR-039-provider-asset-urls.md)) exactly as F39 — a value on a non-allowlisted host degrades to text, not `<img>`. Promotion does **not** bypass the image perimeter.
- Promotion cannot target a canonical or `_`-prefixed key (Mechanism 5) — it cannot shadow the schema contract or reach a reserved sidecar key.
- `Filterable` is always false (D-filterable) — a promoted field never enters the browse facet / query path, so there is no unvalidated-value-into-browse surface to review.

This is why the change carries a **`/security-review`** gate before merge.

---

## Options Considered

### D1 — where the override lives

#### A — new DB-backed `field_promotions` store, resolver tier-0 (chosen)
| Dimension | Assessment |
|-----------|------------|
| Complexity | Low — one table + repo mirroring `decisions.go`; the resolver already consumes `[]mapping.Field` |
| Cost | One migration; a small pre-loaded map per entity resolve (like decisions/curation) |
| Scalability | Works for all three entity types uniformly; per-entity curation reuses existing stores |
| Team familiarity | High — mirrors `field_source_decisions` end to end |

**Pros:** the only option that serves person/studio (no YAML there); live, per-entity-type, no `reload-config`; app never mutates operator config. **Cons:** a second presentation source of truth beside YAML — mitigated by the explicit tier-0 ladder (D3) so precedence is testable, not emergent.

#### B — the app writes `metadata-mappings.yaml`
**Pros:** one presentation source of truth. **Cons:** video-only (person/studio have no YAML); races with hand edits; needs `reload-config`; gitignored/ephemeral in-container; makes a running app a YAML editor. Rejected (D1).

### D3 — precedence vs. operator YAML

#### A — promotion outranks YAML (tier-0, chosen)
**Pros:** matches the single-operator reality (the owner *is* the operator); the live in-app action is the most recent, most specific intent. **Cons:** inverts ADR-056's "operator YAML wins" — acceptable and **scoped to the promoted non-canonical key only**, recorded here for grep-ability.

#### B — operator YAML still wins (promotion below tier-1)
**Pros:** literal continuity with ADR-056. **Cons:** an owner who promoted a field in-app would be silently overridden by a stale YAML entry they may not even remember — surprising in a single-user app. Rejected.

### D-candidate — where a promoted field's F36 candidates come from

#### A — derive `ParsedSources` per-entity from shadow provenance; reuse `replaceMarkers` unchanged (chosen)
**Pros:** zero change to the merge core; candidates reflect what each entity actually has; no source list stored (nothing to drift). **Cons:** the materialization step must see the entity's `Enrichment` map — but it already does, at resolve assembly.

#### B — store a source list on the promotion row
**Pros:** explicit. **Cons:** duplicates provenance already in `entity_enrichment`; drifts when a new provider supplies the key; a global row can't express per-entity presence. Rejected.

---

## Trade-off Analysis

**A second presentation store vs. the person/studio gap.** The cost of D1 is a second place (beside YAML) that shapes a field's presentation. But YAML *cannot* shape person/studio at all, so the alternative is not "one store" — it is "no store for two of three entities." The tier-0 ladder (D3) makes the two stores' precedence a single explicit, unit-testable rule rather than an emergent surprise, which is the mitigation that makes the second store safe.

**Inverting operator authority vs. honoring the single-operator reality.** ADR-056 guarded operator authority against *untrusted providers*. D3 keeps that guard exactly where it matters — a provider hint still can't touch a canonical key (tier-2), and a promotion still can't target one — while letting the human who owns the server win over their own stale deploy-time config. The inversion is narrow (one non-canonical key, owner-gated) and loud (recorded here, cross-linked from ADR-056).

**Full promotion vs. a cheap relabel.** Reusing F36/F30 wholesale (D2) looks heavier than a label override, but it is actually the *smaller* diff: the resolver already turns a `mapping.Field` with the right `ParsedSources` into a fully curatable field, so "make it curatable" costs a synthetic field, not a new editing model. A relabel-only option would need its own presentation-but-not-curation path — more surface for less capability.

---

## Consequences

**What becomes easier**
- An owner renames / re-renders / re-orders / **curates** a rich provider's extra attributes from the entity page — no shell, no YAML, no `reload-config` — on video, person, **and** studio.
- The ADR-056 "Promotion UX" gap closes without a per-entity-YAML surface person/studio never had.
- A promoted field is indistinguishable from a natively-mapped one to a visitor: curated label/mode/order + curated values.

**What becomes harder**
- A second presentation source of truth (promotion vs. YAML) — contributors must respect the tier-0 ladder (D3) and never re-derive precedence ad hoc.
- One more owner-mutation surface with untrusted-*value* rendering (`image_url`) — mitigated by reusing the F39 sanitizer + ADR-039 allowlist, behind a `/security-review`.
- The promotion→`mapping.Field` materialization must stay in step with the F36 candidate derivation (D-candidate); a test pins that a promoted key renders **once** and attaches `Decision`/`Candidates`.

**What we'll need to revisit**
- **`filterable` on a promotion** (D-filterable) — making a promoted field a browse facet; deferred, needs a browse-value-validation review. Capture as a HOLODEX follow-up.
- **Writeback of promoted fields to files** — out of scope (F44 Non-Goals); a promoted field participates in decisions + curation, not file writeback. Follow-up if wanted.
- **Bulk / cross-entity promotion management** — a per-row affordance + edit/remove is the slice; a "manage all promotions" admin table is a follow-up.
- **Cross-provider candidate precedence on a promoted merge field** — inherits ADR-051 §8's global provider trust order; confirm when a second real provider exists.

---

## Action Items

1. [ ] ADR-062 recorded; add to `docs/architecture/README.md`; add the tier-0 cross-reference note to ADR-056 §D2.
2. [ ] **Store** — migration `0023_field_promotions` (`.up`/`.down`); `internal/repo/promotions.go` mirroring `decisions.go`.
3. [ ] **Resolver** — tier-0 in the label/render/group/order fold; per-entity promotion materialization + replace-or-append-by-`Canonical` merge with `ParsedSources` derived from shadow provenance (D-candidate), `Multi = render=="chips"`, `Filterable = false`; auto-registration exclusion confirmed by test (FR3).
4. [ ] **API** — owner-gated `PUT`/`DELETE`/`GET /admin/field-promotions/...`; non-canonical/`_` ⇒ 422; render/group coercion; label sanitize+cap; idempotent delete.
5. [ ] **SPA** — `AutoFieldRows.svelte` `isOwner` + Promote/Edit/Remove affordance + inline editor → the partition move to `SourceSelect`/`CurationFieldRow`; three-skin QA. (`/design-handoff`.)
6. [ ] **Tests** — `docs/testing-strategy.md` F44 block (ladder tier-0 precedence, empty-column inherit, canonical/`_` rejection, promoted-key materializes + attaches decision + excluded from auto-register, per-entity curation survives de-/re-promote, `image_url` allowlist gate, golden no-op). (`/testing-strategy`.)
7. [ ] **Security** — `/security-review` sign-off before merge; clear `needs-security-review`.
8. [ ] **Docs** — `docs/reference/configuration.md` notes the in-app promotion tier above operator YAML; F44 spec ADR/precedence cross-refs confirmed.
