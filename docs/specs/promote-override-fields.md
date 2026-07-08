# Spec: In-app promote / override affordance for auto-registered fields (F44)

**Status**: Draft
**Phase**: Phase 3 enrichment follow-up
**Owner**: Project owner
**Date**: 2026-07-07
**Feature block**: **F44** — an owner-only, in-app affordance to **promote** an auto-registered
(display-only) enrichment field into a **first-class, curatable field** — setting its label / render mode /
group-order and opting it into the F36 source-decision + F30 curation machinery — **without hand-editing
`metadata-mappings.yaml`**.

**Issue**: [HOLODEX-171](https://whoiskevinrich.atlassian.net/browse/HOLODEX-171)
**ADR**: [ADR-062](../architecture/ADR-062-in-app-field-promotion.md) *(written — the DB-backed override store,
the tier-0 precedence ladder amending ADR-056 §D2, and per-entity candidate-source derivation; settles the
`filterable` deferral and the candidate rule left open here)*
**Design**: [promote-override-fields-handoff.md](../design/promote-override-fields-handoff.md) *(to be written)*

**Depends on** (all shipped):
- provider render hints + presence-driven auto-registration ([F39](provider-render-hints.md) /
  [ADR-056](../architecture/ADR-056-provider-field-render-hints.md), `ResolvedField.AutoRegistered`,
  `AutoRegisterFields`, the four-tier ladder, `provider_field_hints`)
- per-field source decisions ([F36](field-source-of-truth.md) / [ADR-051](../architecture/ADR-051-field-source-of-truth.md),
  `field_source_decisions`, `ResolvedField.Decision/Candidates/InSync`, `SourceSelect.svelte`)
- metadata curation ([F30](metadata-curation.md) / [ADR-048](../architecture/ADR-048-metadata-curation.md),
  `metadata_curation`, `CurationFieldRow.svelte`)
- the entity-agnostic resolver + canonical registry ([ADR-052](../architecture/ADR-052-baseline-source-contract.md),
  `ResolveFields`, `mapping.Field`, `internal/registry`)
- the owner gate ([ADR-045](../architecture/ADR-045-owner-session.md), `requireOwner`, Admin mode /
  `effectiveOwner`)

**Touches** an owner-gated mutation that changes what fields render and how they navigate/curate, and
introduces the application **writing** presentation config that the resolver trusts → a **`/security-review`**
sign-off is required before merge (labelled `needs-security-review`).

---

## Problem Statement

F39/ADR-056 makes a provider's non-canonical fields first-class in the UI with zero mapping config — but only
as **display-only** rows. An auto-registered field renders read-only with a provider-supplied (tier-3) or
title-cased (tier-4) label, and is deliberately kept out of `field_source_decisions` / `metadata_curation` /
writeback.

The **only** way to give such a field a curated label, a chosen render mode, a deliberate order, or actual
value curation (pick source / add / suppress) is to **hand-author a `metadata-mappings.yaml` entry** (tier-1),
which promotes it to a first-class mapped field. ADR-056 named this the "Promotion UX" gap and deferred it:

> **Promotion UX** — surfacing "promote this field to a mapping" as an owner affordance (today it is a manual
> YAML edit). Deferred; capture as a follow-up issue.

Two things make the YAML path unworkable as the *only* path:

1. **Person and studio have no operator-YAML remap surface at all.** Their field sets are synthesized in code
   from a fixed canonical list (`internal/api/person_fields.go`, `studios.go`); `metadata-mappings.yaml`
   governs **video** only. So for two of three entity types there is literally nowhere to write a promotion.
2. **YAML is deploy-time operator config**, gitignored and ephemeral in-container. Editing it from a running
   app — concurrency with hand edits, a required `reload-config`, no per-entity granularity — is a poor and
   partial surface, and the owner of a personal single-user server should not have to drop to a text editor to
   rename a field.

This issue delivers the in-app affordance ADR-056 asked for.

## Resolved Decisions

Three open questions were carried on the ticket; resolved with the project owner on 2026-07-07:

| # | Question | Decision |
|---|---|---|
| D1 | **Where does the override live?** | **New DB-backed override store**, consulted by the resolver as a tier. `metadata-mappings.yaml` stays **operator-only** (the app never writes it). This is also the only option that works for person/studio, which have no YAML path. *(Rejected: app writes YAML.)* |
| D2 | **How far does "promote" go?** | **Full promotion.** A promoted field becomes a first-class **curatable** field with the complete F36 source-decision + F30 curation controls — not merely a presentation relabel. Matches ADR-056's original framing. |
| D3 | **Precedence vs. operator YAML?** | **The in-app override wins over `metadata-mappings.yaml`.** The owner's live promotion is the most-authoritative tier (tier-0), above operator YAML (tier-1). This is a deliberate departure from ADR-056's "operator YAML always wins" — justified by the single-operator context (the owner *is* the operator) and recorded with rationale in ADR-062. |

### Revised precedence ladder (supersedes ADR-056 §D2)

For a field's `(label, render/display, group, order)` and its curatable status, first tier with an answer wins:

```
0. In-app promotion  (new — field_promotions DB store)   ← this feature
1. Operator metadata-mappings.yaml
2. Code registry (registry.Lookup) — canonical keys
3. Provider hint (provider_field_hints) — non-canonical keys only
4. Title-case fallback
```

A promotion may only target a **non-canonical** key (one the code registry does not know) — the same
predicate F39 uses for auto-registration. Canonical keys remain owned by the registry/operator YAML; you cannot
"promote" `bio`.

---

## Goals

1. **No-YAML promotion.** An owner viewing an auto-registered field row can promote it in-app — set its label,
   render mode, and group/order — on all three entity types, with **zero** `metadata-mappings.yaml` editing.
2. **Full curation on promotion.** A promoted field gains the existing F36 source-decision + F30 curation
   controls (`SourceSelect`, `CurationFieldRow`) exactly as a YAML-mapped field would.
3. **Reversible.** An owner can **de-promote** a field back to its auto-registered display-only state; the
   underlying shadow value is never touched.
4. **Owner override is authoritative.** A promotion outranks an operator YAML mapping for the same key (D3).
5. **Bounded, gated blast radius.** Every promote/edit/de-promote is owner-gated and validated; a promotion
   changes render + curation surface but never mutates enrichment values or writes files by itself.
6. **Zero-impact when unused.** No promotions ⇒ behaviour is byte-identical to F39. No protocol change, no
   provider change.

## Non-Goals

- **Promoting canonical fields.** The promotion predicate is strictly "key the registry does not know", mirroring
  F39. Canonical keys are governed by the registry + operator YAML. *(Why: preserves the schema contract.)*
- **Per-entity presentation overrides.** A promotion's label/render/group/order is **global per
  `(entity_type, field_key)`** — renaming "measurements" renames it for every person that has the key, not one
  person at a time. *(Per-entity variance is what F36/F30 already give, on values — not on presentation.)*
- **The app writing `metadata-mappings.yaml`.** Rejected in D1; YAML stays operator-only.
- **New render modes.** Reuse F39's vocabulary: `text` / `long_text` / `chips` / `url` / `image_url`.
- **Bulk / cross-entity promotion management screens.** A per-row affordance + edit/remove is the slice;
  a "manage all promotions" admin table is a follow-up if wanted.
- **Writeback of promoted fields to files.** Out of scope here; a promoted field participates in decisions +
  curation (render/source truth), not file writeback. *(Follow-up if desired.)*

---

## Users & Value

- **Owner** of a rich provider: renames, re-renders, re-orders, and curates the provider's extra attributes
  from the entity page — no shell, no YAML, no `reload-config`.
- **Visitor**: sees the owner's curated label/mode/order and curated values on the same rows — indistinguishable
  from a natively-mapped field.
- **Operator** (deploy-time): `metadata-mappings.yaml` remains the operator surface; unaffected unless the owner
  deliberately overrides a key in-app.

---

## Functional Requirements

### FR1 — Promotion store (`field_promotions`)

Migration `0023_field_promotions` (append-only, with `.down.sql`):

```sql
CREATE TABLE field_promotions (
  entity_type TEXT    NOT NULL,               -- 'video' | 'person' | 'studio'
  field_key   TEXT    NOT NULL,               -- the non-canonical shadow key
  label       TEXT    NOT NULL DEFAULT '',    -- '' ⇒ inherit tier-3/4 label
  render      TEXT    NOT NULL DEFAULT '',    -- '' ⇒ inherit; else text|long_text|chips|url|image_url
  hint_group  TEXT    NOT NULL DEFAULT '',    -- '' ⇒ inherit; else primary|attributes|extended
  ord         INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT    NOT NULL,
  updated_at  TEXT    NOT NULL,
  PRIMARY KEY (entity_type, field_key)
);
```

- **Scope is global per `(entity_type, field_key)`** — one row promotes the key for every entity of that type
  (Non-Goals). Per-entity value curation continues to key off the existing `field_source_decisions` /
  `metadata_curation` tables by `entity_id` — **no new per-entity table**; those already key by `field_key`.
- Empty presentation columns **inherit** from the lower tiers (provider hint → title-case), so a promotion whose
  only purpose is "make this curatable" need not restate the label.
- Writes go through the single-writer `writeMu`; repo lives in `internal/repo/promotions.go`
  (`SetPromotion` / `ClearPromotion` / `PromotionsForEntityType`), mirroring `decisions.go`.

### FR2 — Resolver consults promotions as tier-0

The promotion store materialises into a synthetic `mapping.Field` that the resolve path merges **over** the
YAML-parsed mappings (promotion wins on `canonical` collision — D3), before `ResolveFields` runs:

- A promotion row for `(entity_type, key)` yields a `mapping.Field{Canonical: key, Label, Display, ... }` with
  its candidate **sources derived from the shadow store's provenance** for that key (the supplying
  provider namespace(s) + `manual`) — no source list is stored in the promotion row.
- Because the key is now a real `mapping.Field`, `ResolveFields` attaches `Decision` / `Candidates` / `InSync`
  (F36) and the field becomes eligible for `metadata_curation` items (F30) — automatically, via the existing
  code paths. `ResolvedField.AutoRegistered` is **false** for a promoted field.
- The four-tier label/render/order helper (ADR-056 FR3) gains tier-0: a promotion's non-empty presentation
  columns win; empty columns fall through to tiers 3→4.
- Entity seams unchanged from F39: person (`person_fields.go`), studio (`studios.go`), video (media-detail
  resolve). Each already builds the `[]mapping.Field` for its resolve; each now merges promotions first.

### FR3 — Auto-registration yields to promotion

`AutoRegisterFields` already excludes keys "already produced by a mapping/synthesized field". Because a promoted
key materialises as a `mapping.Field` (FR2), it is excluded from auto-registration automatically — the field
renders **once**, via the curatable path, not twice. No new predicate needed; add a test to pin it.

### FR4 — Owner-gated promote / edit / de-promote endpoints

New owner-gated routes in the `requireOwner` group (mirror `internal/api/person_decisions.go`):

| Method | Route | Effect |
|---|---|---|
| `PUT` | `/admin/field-promotions/{entity_type}/{field_key}` | Create or update a promotion (body: `label?`, `render?`, `group?`, `order?`). |
| `DELETE` | `/admin/field-promotions/{entity_type}/{field_key}` | De-promote (delete the row); field reverts to auto-registered. |
| `GET` | `/admin/field-promotions/{entity_type}` | List promotions for an entity type (owner tooling / debug). |

- `entity_type` validated against `{video, person, studio}`; `field_key` validated **non-canonical** (registry
  `IsKnown` ⇒ 422) and **non-`_`-prefixed** (⇒ 422). `render`/`group` coerced to the F39 vocabulary; `label`
  sanitized + capped 64 chars (reuse F39 ingest sanitizer).
- A dedicated `/admin/field-promotions/...` route (rather than `/{entity}/{id}/fields/...`) is chosen because a
  promotion is **type-global**, not tied to the entity the owner is viewing — the URL should not imply per-entity
  scope. The affordance passes the row's `entity_type` + `field_key`.
- Standard `validate → repo → 204` shape; de-promote of a missing row is idempotent (204).

### FR5 — SPA: promote affordance on the auto-registered row; curatable render after promotion

- `AutoFieldRows.svelte` gains an `isOwner` prop (currently takes none). For the owner, each row shows a small
  **"Promote"** control (mirror the `CurationFieldRow` "+ Add" affordance pattern — border-rule button, tokens
  only). Activating it opens a lightweight editor (label, render mode `<select>`, group, order) → `PUT`
  → parent `reloadDetail()`.
- After promotion the field no longer appears in `extraFields` (it is no longer `auto_registered`); it moves to
  the mapped-field partition and renders through `SourceSelect` (replace) / `CurationFieldRow` (merge) exactly as
  a YAML-mapped field — **for free**, via the existing partition logic
  (`replaceFields`/`mergeFields` filter `!f.auto_registered`).
- A promoted field's row gains an owner-only **"Edit label / Remove promotion"** affordance (on the mapped row,
  or in the same editor) → `PUT`/`DELETE` → `reloadDetail()`.
- Visitor view is unchanged in shape (no owner controls); it simply reflects the promoted label/mode/order and
  curated values.

### FR6 — Security (owner-gated config the resolver trusts)

- Every mutation is behind `requireOwner`; a non-owner receives 401 before the handler and sees no affordance.
- `label` / `render` / `group` are **owner-supplied but still sanitized/validated on ingest** (defense in depth;
  reuse the F39 sanitizer): control-char strip, length cap, vocabulary coercion. Labels render as escaped text.
- `image_url` render on a promoted field remains **asset-host allowlist-gated** (ADR-039) exactly as F39 —
  promotion does not bypass the image perimeter.
- Promotion cannot target a canonical or `_`-prefixed key (FR4) — it cannot shadow the schema contract or reach
  a reserved sidecar key.
- A promotion changes render + navigation/facet surface (a promoted `filterable`-style field could influence
  browse); the security review must confirm a promoted field cannot smuggle an unvalidated value into the browse
  facet / query-param path. *(If promotion is kept render+curation only — no `filterable` — this is moot; see
  Open Items.)*

---

## Acceptance Criteria

1. An owner on a person/studio/video page with an auto-registered field sees a **Promote** control; a visitor
   does not.
2. Promoting a field with a new label + render mode + order → the field re-renders with those, **and** gains the
   F36 source picker + F30 curation controls, on that entity and every other entity of the type that has the key
   — with **zero** `metadata-mappings.yaml` editing.
3. A promotion **outranks** an operator `metadata-mappings.yaml` entry for the same key (label/render/order and
   curatable status come from the promotion). *(D3 — verified against a video key that also has a YAML mapping.)*
4. Curation on a promoted field is **per-entity**: adding/suppressing a value on person A does not affect person
   B, while the label/render/order (from the promotion) is shared.
5. **De-promote** removes the curatable controls and the field returns to an auto-registered display-only row;
   the shadow value and any prior `field_source_decisions`/`metadata_curation` rows are untouched (and re-apply
   if re-promoted).
6. A promoted key renders **once** (not doubled as both auto row and mapped row).
7. Attempting to promote a **canonical** key (`bio`) or a `_`-prefixed key → 422; no row created.
8. Owner-supplied `label`/`render`/`group` are sanitized/coerced; an over-long or control-char label is capped/
   cleaned; an unknown render mode coerces to `text`.
9. An `image_url` promoted field whose value host is not allowlisted still renders as **text**, not `<img>`.
10. No promotions present ⇒ **byte-identical** resolved output and rendering to F39 (the golden no-op case).
11. All three skins (Cinémathèque, Broadcast, Brutalist) render the promote affordance, the inline editor, and
    the post-promotion curatable row with tokens only, in loading/empty/populated states.

---

## Test Notes (for `/testing-strategy`)

- **Store/repo** — `SetPromotion`/`ClearPromotion` upsert + delete under `writeMu`; `PromotionsForEntityType`.
- **Ladder** — unit-test tier-0 precedence: promotion > YAML > registry > hint > title-case; empty promotion
  columns inherit from lower tiers; promotion cannot target canonical/`_` keys.
- **Resolver integration** — a promoted key materialises a `mapping.Field`, attaches `Decision`/`Candidates`,
  is excluded from auto-registration, `AutoRegistered=false`; de-promote reverts to an auto row. Extend the
  ADR-052 baseline tests for all three entities.
- **Curation interplay** — per-entity `field_source_decisions`/`metadata_curation` on a promoted key resolve
  correctly and survive de-/re-promote (the rows are keyed by `field_key`, independent of the promotion row).
- **API** — owner gate (401 unauth); validation (canonical/`_` ⇒ 422, render/group coercion, label cap);
  idempotent delete; `PUT` upsert.
- **Security** — label sanitize/cap; render/group coercion; `image_url` allowlist gate on a promoted field;
  no canonical/`_` promotion; (if applicable) no unvalidated browse-facet injection.
- **SPA** — owner-only affordance; the inline editor; partition move auto → mapped after promotion and back on
  de-promote; three-skin QA.
- **Backward compat** — golden no-op: no promotions ⇒ identical to pre-F44.

---

## Open Items

*Two prior architecture items are resolved in [ADR-062](../architecture/ADR-062-in-app-field-promotion.md):*
`filterable` on a promotion — **deferred** (`Filterable` stays false in v1; browse-facet is a follow-up,
ADR-062 D-filterable); and candidate-source derivation — **per-entity from shadow provenance** (one
`provider:<ns>` per supplying namespace, union across providers, `manual` always available; no stored source
list), with `Multi = render=="chips"` (ADR-062 D-candidate).

1. **Editor placement/shape** — inline expander on the row vs. a small popover; whether promote + edit share one
   editor. For `/design-handoff`.

---

## Rollout

Single feature block across all three entities, one migration (`0023`), no contract/protocol change, no provider
change. Ship behind the normal owner gate; no flag needed — absence of promotions is a no-op by construction.
Update `docs/reference/configuration.md` (note the in-app promotion tier above operator YAML) and the ADR-056
ladder cross-reference (superseded §D2 → ADR-062).
