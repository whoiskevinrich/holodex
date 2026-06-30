# ADR-051: Per-field source-of-truth decisions — file-baseline + standing per-item decision over precedence

**Status:** Proposed
**Date:** 2026-06-29
**Deciders:** Project owner

**Relates to:** [ADR-047](ADR-047-per-item-metadata-refresh.md) (**supersedes its deferred F31.11** "per-item / per-field precedence override") · [ADR-013](ADR-013-metadata-field-mapping.md) (field-mapping precedence — this adds an override layer above it) · [ADR-033](ADR-033-metadata-source-plugins.md) (enrichment sidecars + `entity_enrichment` shadow store — the *candidate* layer) · [ADR-048](ADR-048-metadata-curation-and-write-queue.md) (F30 value-level curation — the **orthogonal** value axis, reused for merge fields) · [ADR-041](ADR-041-metadata-writeback.md) (writeback — now writes the *decided* value) · [ADR-030](ADR-030-access-control-gating-seam.md) (owner gating) · [ADR-036](ADR-036-person-alias-search-indexing.md) (person aliases — the performer-entity note) · [ADR-028](ADR-028-activity-surface-and-job-history.md) (job history). **Spec:** [Per-field source-of-truth / F36](../specs/field-source-of-truth.md) (this decision's spec); relates to [Refresh Metadata / F31](../specs/metadata-refresh.md).

---

## Context

Holodex keeps three metadata layers per canonical field — the **file** layer (`videos` + `video_metadata`,
[ADR-004](ADR-004-metadata-extraction.md)/[ADR-018](ADR-018-scanner-change-detection.md)), the **provider**
enrichment shadow store ([ADR-033](ADR-033-metadata-source-plugins.md)), and the **manual** value-level
curation store ([ADR-048](ADR-048-metadata-curation-and-write-queue.md)) — merged at display time by the
resolver (F27). For a scalar field the resolver takes the **first non-empty source in mapping order**
(`internal/resolver/resolver.go`, `resolvePrecedence`), and the shipped example mappings list providers
(`tmdb:title`) **before** `file:`. So a provider silently outranks the owner's own file tags.

**The reported bug.** Editing an MKV externally, then running per-item Refresh ([ADR-047](ADR-047-per-item-metadata-refresh.md)),
**correctly re-reads and persists the file layer** — verified end-to-end — but the displayed value does not
change, because a higher-precedence source masks it. It surfaced on an instance using a **custom, non-film
provider**, where "provider wins by default" is simply the wrong assumption: the owner's file tags are the
authority and the provider is a *suggestion*. A second, sharper edge: writeback ([ADR-041](ADR-041-metadata-writeback.md))
embeds the resolved **winning** value into the file's own tags, so adopting a provider value and writing it
back **overwrites the file with the provider value** — a feedback loop.

**Why F30 curation isn't already the answer.** [ADR-048](ADR-048-metadata-curation-and-write-queue.md) gives
*value-level* actions keyed by normalized value (`add` / `suppress` / `nowrite`). For a **merge** field that
is exactly the per-value control we want. But for a **scalar/replace** field there is no primitive to say
"for this item, the source of truth is the **file**" or "…is **this provider**". `manual add` happens to
override a scalar (it wins precedence), so "type my own" already works — but "keep file when a provider
outranks it" and "adopt this specific provider" do not exist, and faking them by suppressing a provider's
*value* is value-keyed, brittle, and conflates display truth with a tombstone.

**The reframe (agreed with the owner).** The file is the **baseline / default source of truth**; enrichment
is a **candidate** that never auto-wins; the owner makes a **standing per-item, per-field decision** about
which source is true. That one decision drives **both** what is displayed and what writeback commits. This
ADR records how that decision is modeled, stored, defaulted, and reconciled with the existing layers — and
explicitly **supersedes the deferred F31.11 slice** of [ADR-047](ADR-047-per-item-metadata-refresh.md),
which anticipated exactly this but punted on it.

### Constraints / forces

- **Resolution stays pure.** The [ADR-013](ADR-013-metadata-field-mapping.md)/[ADR-048](ADR-048-metadata-curation-and-write-queue.md)
  invariant holds: a decision is **pre-loaded** with the curation + enrichment maps (no new per-field query,
  no I/O), so a decision change re-renders without re-fetching or re-scanning.
- **Non-destructive layering** ([ADR-047](ADR-047-per-item-metadata-refresh.md)). The decision is a thin
  **selector**, never a materialized value: the raw file and provider layers stay intact, so a decision can
  be changed or cleared at any time and the losing layers remain recoverable.
- **Pin the source, not the value** (for file / provider). A `file` or `provider` decision follows the
  **live** layer — a later Refresh file-edit or re-enrich flows straight through to the display. Only a
  `manual` decision is a frozen literal. This source-pin is what actually fixes the bug.
- **Orthogonal to merge/replace.** Augment-vs-overwrite is the field's existing `multi`/`merge` flag, not a
  new axis. Replace fields take a single **source** decision; merge fields keep F30 **per-value** curation
  (union; include/exclude each candidate). Reuse F30 — do not reinvent it.
- **Owner-gated** ([ADR-030](ADR-030-access-control-gating-seam.md)), parity with curation / writeback / refresh.
- **Entity-agnostic by construction.** Every layer keys by `(entity_type, entity_id, canonical_field)`; the
  decision is not video-specific — `video` ships first, `person` / a future `studio` reuse it (§9). The one
  real dependency: the resolver entry point is video-shaped today (`Resolve(v *model.Video, …)`), so non-video
  entities need it generalized first (tracked fast-follow, **not** built here).

---

## Decision

Introduce a **standing per-item, per-field source decision** as a first-class primitive that overrides
mapping precedence for the field it names, defaulting to the **file baseline**, and drives display and
writeback from one place.

### 1 — The decision primitive

A decision is `(entity_type, entity_id, canonical_field) → source`, where `source ∈ { file, provider:<name>, manual }`:

- **`file`** — the file layer is the truth (the default; see §4).
- **`provider:<name>`** — that provider's shadow-store value is the truth. The item must be matched to that
  provider; the decision stores the **provider name only**, never a snapshot, so a re-enrich updates the field.
- **`manual`** — a literal owner value (`manual_value`), frozen until edited.

For a **replace** (scalar) field the decision selects the whole value. For a **merge** field the field-level
decision is unused — its truth is the F30 per-value union — except for the convenience that a per-value
`manual add` / `suppress` *is* the per-value form of the same idea. The two stores are complementary:
**source decisions for replace fields, value curation for merge fields.**

### 2 — Storage: a dedicated `field_source_decisions` table (migration 0016)

```sql
CREATE TABLE field_source_decisions (
    id            INTEGER PRIMARY KEY,
    entity_type   TEXT    NOT NULL,            -- "video" in v1 (generalizes to "person")
    entity_id     INTEGER NOT NULL,
    field_key     TEXT    NOT NULL,            -- canonical field
    source        TEXT    NOT NULL,            -- 'file' | 'provider:<name>' | 'manual'
    manual_value  TEXT    NOT NULL DEFAULT '', -- literal, only when source='manual'
    created_at    TEXT    NOT NULL,            -- RFC3339 UTC, matching every other ts column
    UNIQUE (entity_type, entity_id, field_key)
);
CREATE INDEX idx_field_decisions_entity ON field_source_decisions(entity_type, entity_id);
```

One row per decided field (the `UNIQUE` makes "set decision" an upsert and "clear" a delete → back to
default). Loaded per entity exactly like the curation map, so the resolver consults it in-memory.

### 3 — Resolver consults the decision first

`resolveField` gains a pre-step: if a decision exists for the field, it **short-circuits mapping order** and
returns the decided source's current value (`file` → file layer; `provider:x` → that namespace; `manual` →
`manual_value`). With **no** decision, behavior falls through to §4. No new I/O — the decision map is
pre-loaded alongside `Enrichment` + `Curation`, so the [ADR-013](ADR-013-metadata-field-mapping.md) purity
invariant is preserved. `ResolvedField` gains a `decision` marker (the chosen source + "is a standing
decision") so the SPA can render the control state and a provenance that reads "you chose file/provider/custom".

### 4 — Default-when-undecided is file-first

An undecided **replace** field resolves to the **file** value when the file has one; the provider is shown as
an available **candidate**, not the winner. This is the real default-behavior change and the second half of
the bug fix: nothing masks the file until the owner adopts a provider.

The configured `metadata-mappings.yaml` source order is **demoted to candidate ordering** (which provider to
suggest first / which file tags feed a field), not a display-precedence winner. An optional global escape
hatch — `default_source: file | mapping` (default **`file`**) — lets a film-centric instance opt the whole
library back into "first configured source wins" without per-field decisions. The shipped example mappings
are also reordered file-first (they are currently film-shaped / provider-first).

### 5 — One decision drives writeback too; convergence is explicit (Open Q1)

Writeback ([ADR-041](ADR-041-metadata-writeback.md)/[ADR-048](ADR-048-metadata-curation-and-write-queue.md))
writes the **decided** value, not "whatever currently wins mapping order". After an `adopt provider` + write,
the file now equals the provider value — but the **decision is not auto-flipped**. It stays `provider:<name>`
(so the field keeps following the provider) and the UI shows a per-field **in-sync / out-of-sync** indicator
(decided value vs. the value currently embedded in the file). Rationale: silently flipping to `file` would
sever the provider-follow the owner asked for, and "did my write land?" is better answered by an explicit
sync state than by mutating the decision. Clearing the decision (→ file default) remains the way to "commit
and detach".

### 6 — Performers are entities, not strings (Open Q4)

A decision fixes the **file-tag** value of a field. For person fields (`actors`/`director`) that is distinct
from **person identity** — "Jhon Doe" → "John Doe" is also an alias/merge question ([ADR-036](ADR-036-person-alias-search-indexing.md)/F23).
This ADR scopes the decision to the file-tag value. When the decided field is a person field, the UI **may
offer** (never automatically) to also alias/merge the person via the existing F23 flow. That offer is a
follow-up, deliberately **not** coupled to the decision write.

### 7 — API & auth

```
PUT    /api/v1/media/{id}/fields/{canonical}/decision   { source, manual_value? }   (requireOwner)
DELETE /api/v1/media/{id}/fields/{canonical}/decision                                (requireOwner → file default)
```

Owner-gated alongside `/enrich`, `/writeback`, `/refresh`, `/curation`. The decision is **standing**: it
drives display continuously; the writeback dialog is the **review/confirm** surface (file vs. candidate vs.
decided diff), not a separate, parallel ranking made at write time.

### 8 — Multiple providers per entity

The decision source is `provider:<name>`, never a singular "provider", so multiple enrichment providers
(e.g. `imdb` + `tmdb`) need no model change: storage (`entity_enrichment` keyed `(entity_type, entity_id,
provider, field_key)`, each provider with its own `external_id`), match-listing (`ProviderMatches`, one per
provider), and re-enrich (Refresh loops every linked provider) are already per-provider. The decision layer
absorbs the rest:

- **Replace fields** — the per-field control renders **one `Adopt` option per *matched* provider**
  (`Keep file / IMDB / TMDB / Custom`). When providers supply different values the reserved
  `sources_disagree` flag ([ADR-047](ADR-047-per-item-metadata-refresh.md)) becomes meaningful and drives a
  per-field conflict hint; the decision is how the owner resolves it.
- **Merge fields** — unchanged: `resolveMerge` already unions every configured provider, deduped, with
  combined per-value provenance (`·imdb + tmdb`).
- **Inter-provider default order (new resolved decision).** When a replace field is *undecided* and several
  providers supply it, the winner among providers follows a **global provider trust order** (config), with
  `file` still ahead of all providers per §4 (file-first). A per-field decision overrides it. This replaces
  leaning on `metadata-mappings.yaml` source order as an implicit cross-provider ranking.

The remaining multi-provider work is **UI + per-provider identity matching** (one match / `EnrichPicker` +
enrich/clear per provider; the SPA's single-provider assumption `provider = sources.find(…)` widens to a
per-provider list) — a spec/design concern, not an architecture change.

### 9 — Generalization to People and Studios (entity-agnostic by construction)

The same approach is intended for other canonical entities — **People** and **Studios**, each with their own
detail page and per-field decisions. The model is entity-generic already: all three layers key by
`(entity_type, entity_id, canonical_field)`. Two clarifications:

- **"File" is the *video's* baseline.** Generalize the concept to the **baseline (intrinsic) source** of an
  entity: for a `video` that's the file layer; for a `person`/`studio` it's the scan-derived record (e.g. a
  person's name routed from file tags) — there is no file. Decision values are identical
  (`baseline / provider:<name> / manual`); only the baseline's identity differs per entity.
- **Two dependencies, deliberately not built here** (high-priority fast-follows; tracked in `TASKS.md`):
  1. **Entity-agnostic resolver.** `Resolve(v *model.Video, …)` hard-codes the video baseline; People fields
     don't flow through the unified resolver today. Abstracting "baseline source" behind an interface (as
     enrichment/curation already are) is the prerequisite for any non-video entity.
  2. **Studio promotion.** `studio` is a canonical *field* on video today (entity types are only
     `person`/`video`); there is no `studio` entity. Promoting it to first-class (table, page, enrichment) is
     a separate feature — once it exists as an `entity_type`, it inherits this decision model with no new design.
- **People compound with identity.** A person's baseline includes their *name*, which is also their identity
  key — so "adopt provider for a person's name" is simultaneously a field decision and a potential
  alias/merge ([ADR-036](ADR-036-person-alias-search-indexing.md)/F23). The entity generalization makes that
  first-class, not a footnote (cf. §6).

---

## Options Considered

### Decision storage

#### A — Dedicated `field_source_decisions` table (chosen)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low–Med — one table + a resolver pre-step |
| Cost | Negligible (pre-loaded like curation; resolution stays pure) |
| Scalability | Good — one indexed row per decided field, set-membership at read |
| Team familiarity | High — mirrors the `metadata_curation` pre-load exactly |

**Pros:** Clean separation of *field-level source* (this) from *value-level curation* ([ADR-048](ADR-048-metadata-curation-and-write-queue.md));
natural keying + upsert/delete semantics; generalizes to `person`. **Cons:** A second curation-shaped store and
pre-load path; the SPA must reconcile two stores on one field (only one is active per field, so this is small).

#### B — Extend `metadata_curation` with a value-less `pin_source` row

**Pros:** One store, one pre-load. **Cons:** Overloads a table whose whole model is *normalized-value* keying
(`norm_value`, the `UNIQUE`) with a field-level row that has no value; muddies the F30 contract and its
queries. Rejected — conceptual clarity is worth one more small table.

#### C — Global config precedence only (no per-item decision)

**Pros:** No schema change; flipping the example mappings file-first already helps. **Cons:** Cannot express
"this item's title is the provider's, that item's is the file's" — the core ask and the exact shape of the
bug. Rejected; kept only as the `default_source` escape hatch in §4.

### Default-when-undecided

#### A — File-first default + mapping order demoted to candidate ordering (chosen)

**Pros:** Fixes the bug for every undecided field with no per-field work; matches the agreed mental model
(file = baseline). **Cons:** Behavior change for any instance relying on provider-first auto-display (Open Q5);
mitigated by the `default_source: mapping` escape hatch and reordered example mappings.

#### B — Keep mapping-order default; per-field decision is the only override

**Pros:** Zero surprise for existing instances. **Cons:** Leaves the bug as the default for custom-provider
instances; every masked field needs a manual decision. Rejected — it preserves the wrong default.

---

## Trade-off Analysis

The central tension is **a global default vs. per-item control**, and the design takes both: a per-field
**decision** for precision and a **file-first global default** so the common case needs no decisions at all.
The cost is a genuine default-behavior change (Open Q5) — but the bug *is* that the current default is wrong
for non-film providers, and the `default_source` flag plus reordered example mappings bound the blast radius
for a film instance. Keeping the decision a **source selector** rather than a materialized value is what lets
all of this stay inside the pure resolver: display truth, writeback payload, and "what did I decide" are one
fact, re-derived from intact raw layers, never a snapshot that can drift. The one thing we deliberately accept
is **two curation-shaped stores** (value-level F30 + field-level here); they are orthogonal (merge vs. replace)
and only ever one is active per field, so the SPA reconciliation is cheap and the conceptual win — "exactly
one place decides each field's source of truth" — is worth it.

---

## Consequences

**What becomes easier**
- The owner fixes the reported bug directly: undecided fields show the file (baseline), and a provider only
  wins where explicitly adopted — per item, per field.
- "Preserve file / override with provider / write my own" is a first-class three-way choice that drives both
  display and writeback from one decision; the writeback feedback loop becomes intentional, with a sync state.
- Source-pinning means a later Refresh file-edit or re-enrich flows through with no re-decision.

**What becomes harder**
- A new default (file-first) and a new store + migration **0016**; `ResolvedField` gains a `decision` marker →
  coordinated SPA (`CurationFieldRow`/`CurationChip`/`ProvenanceBadge`) and MCP field-output changes.
- The resolver now consults three pre-loaded maps (enrichment, curation, decisions); tests must cover the
  decision short-circuit for both replace and merge fields, and the file-first default.
- Two curation-shaped concepts (value-level vs field-level) must be kept clearly separated in code and UI.

**What we'll need to revisit**
- **Writeback convergence policy** (Open Q1) — if "out-of-sync" proves noisy, reconsider an opt-in
  commit-and-detach.
- **Person identity coupling** (Open Q4) — whether adopting a provider performer name should routinely offer
  the F23 alias/merge.
- **Default-flip migration** (Open Q5) — confirm no existing instance is surprised; document `default_source`.
- **Bulk decisions** — a library-wide "prefer file / prefer provider X" applied as decisions, riding the
  F31.11 batch seam from [ADR-047](ADR-047-per-item-metadata-refresh.md).
- **Entity-agnostic resolver + Studio promotion** (§9) — the two high-priority fast-follows that let People
  and Studios inherit this model; tracked in `TASKS.md`, scoped after the video slice.
- **Inter-provider trust order + multi-provider matching UI** (§8) — the default ranking among providers and
  the per-provider enrich/match UX, settled in the decision-UI spec slice.

---

## Action Items

1. [ ] `/write-spec`: a decision-UI spec slice (states, default, sync indicator, person-field offer) cross-referenced from [F31](../specs/metadata-refresh.md); capture Open Q1/Q4/Q5 as Resolved Decisions.
2. [ ] Add migration **0016 `field_source_decisions`**.
3. [ ] Resolver: pre-load decisions; short-circuit mapping order for decided fields; implement file-first default + `default_source` config; keep merge-field path on F30. Regression-guard the precedence path.
4. [ ] Extend `ResolvedField` + detail API + MCP output with the `decision` marker and per-field sync state.
5. [ ] Owner-gated `PUT`/`DELETE /media/{id}/fields/{canonical}/decision` ([ADR-030](ADR-030-access-control-gating-seam.md)).
6. [ ] Writeback writes the decided value; compute + surface in-sync/out-of-sync per field.
7. [ ] `/design-handoff`: the per-field source control grounded in `CurationFieldRow.svelte` / `CurationChip.svelte` / `ProvenanceBadge.svelte`; QA all three skins.
8. [ ] Reorder the shipped `metadata-mappings.yaml.example` file-first; document `default_source` in `docs/reference/configuration.md`.
9. [ ] `/testing-strategy`: decision short-circuit (replace + merge), file-first default, source-pin-follows-live-edit, writeback-uses-decided-value + sync state, escape-hatch.
10. [ ] `/security-review` before merge (owner gate, untrusted `manual_value`).
11. [ ] Add the ADR-051 row to `docs/architecture/README.md`; note in [ADR-047](ADR-047-per-item-metadata-refresh.md) that its F31.11 deferral is superseded here.
12. [ ] Multi-provider (§8): decision control renders one `Adopt` per matched provider; add the global inter-provider trust order config + `sources_disagree` conflict hint; widen the SPA single-provider assumption.
13. [ ] Entity generalization (§9): keep all stores + the primitive entity-typed; log the entity-agnostic-resolver and Studio-promotion fast-follows in `TASKS.md` with handoff detail.
