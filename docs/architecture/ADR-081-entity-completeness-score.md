# ADR-081: Entity Completeness Score — Facet Criticality, Not-Applicable Status, and Score Computation

**Status:** Proposed
**Date:** 2026-08-07
**Deciders:** Kevin Rich
**Extends:** ADR-051 (per-field source-of-truth decisions), ADR-052 (baseline source contract), ADR-063 (derived/computed fields)
**Relates to:** ADR-013 (field mapping), ADR-033 (metadata source plugins)
**Contract:** none new (no HTTP/wire contract beyond the existing decision-API and resolved-field shapes)
**Spec:** [entity-completeness-score.md](../specs/entity-completeness-score.md)
**Issue:** HOLODEX-260

## Context

The F55 spec defines a per-entity completeness score (weighted by facet criticality, tiered by
source trust) plus a separate actionability signal, and explicitly punts four things to an ADR
rather than settling them itself:

1. Where facet criticality (critical vs. nice-to-have vs. excluded) lives in the data model.
2. How the tri-state facet status — resolved / missing / **not-applicable** — persists, given
   not-applicable is an owner-asserted, per-entity, per-facet exclusion from scoring.
3. Where and how the score and actionability numbers get computed.
4. How `imdb_id` generalizes to a provider-agnostic `external_provider_id` (this app's production
   instance does not use IMDb-sourced identifiers, so the field's current name misdescribes what
   it actually holds).

This repo has two directly relevant precedents already in production:

- **ADR-051** built `field_source_decisions(entity_type, entity_id, field_key, source, …)` — one
  row per standing owner decision about *which source wins* for a field's value. `source` is a
  closed grammar (`internal/fieldsource`): `file`, `manual`, or `provider:<name>`.
- **ADR-063** added a `computed:<canonical>` provenance token for derived fields (e.g. Age) and
  deliberately kept it **out of** `fieldsource.Valid()`/`ForNamespace()` — a computed value has no
  underlying store to pin, so it can never be adopted as a decision. The computation itself is a
  pure, clock-free post-pass (`Derive`) over already-resolved fields, with the clock injected only
  at the API handler boundary.

Both precedents resolved a "does this belong in the existing decision grammar, or is it a
structurally distinct concept" question before writing code. This ADR has to answer the same
question for not-applicable, and a parallel one for score computation.

### Forces

- **Compute-on-read is this codebase's established default** for anything derived (ADR-063's
  central trade-off: staleness beats storage). But score/actionability have a consumption pattern
  Age never did — they must drive **sort and filter across a whole entity list** (browse pages,
  remediation queue), not just render on one detail page. A naive port of the Age pattern doesn't
  by itself answer how a list of hundreds of entities gets sorted by a value that only exists after
  resolving each one.
- **`field_source_decisions` has a narrow, specific meaning: which source is true for a field's
  *value*.** Not-applicable doesn't select a winning source — it asserts a facet doesn't apply to
  this entity at all, and should be excluded from both the score's denominator and the
  remediation queue. Overloading `source` with a value that has no value already burned this
  project once: ADR-063 rejected exactly this shape for `computed`, and ADR-051 itself avoided
  overloading `metadata_curation` (a table about *pinning a value*) for the same reason.
  Not-applicable is a third case: not a source, not a computed value — an owner-asserted
  exclusion.
- **This is a single-owner, personal-scale library** (hundreds to low thousands of videos; fewer
  people/studios), not a SaaS tenant with large tables. That scale assumption is load-bearing for
  whether "resolve the whole entity set in Go on every browse/queue request" is acceptable, and it
  is the same assumption ADR-063 and the extraction-queue/F47 review workflow already lean on.
- **`imdb_id` has no dedicated Go struct field or SQL column** (confirmed: it appears only in
  `internal/registry/registry.go`). It flows purely as a canonical `field_key` string through the
  mapping, enrichment-shadow, decision, curation, and review layers. Renaming it is therefore a
  **data migration** (rewrite stored strings), not a schema migration.
- **Migrations are append-only with a manual, real down** (project convention,
  `.claude/rules/migrations.md`) — any rename ships as a new numbered migration pair, never an
  edit to a shipped one.
- **Standing constraint:** no spec/ADR/commit may name the real third-party adult-content
  enrichment provider. `external_provider_id` must stay generic in every example.

## Decision

### D1: Facet criticality is static metadata on `registry.FieldDef`

Add `Criticality string` to `FieldDef` (`internal/registry/registry.go`), with values `""`
(excluded from scoring — the zero value, so most fields need no annotation), `"critical"`, and
`"nice_to_have"`. Only P0-scored facets per entity type (per the spec's facet tables) get an
explicit `critical`/`nice_to_have` tag; everything else — including every `Computed: true` field
— stays `""` and is skipped by the scorer without a second flag to keep in sync. A field can never
be both `Computed` and criticality-tagged; the scorer treats `Computed` as an automatic exclusion,
reusing the invariant ADR-063 already established rather than re-deriving it.

**Chosen over:** a parallel `weights.yaml`/DB table keyed by canonical field name. Field metadata
that shapes resolver/display behavior already lives in `FieldDef` (`Display`, `EntityKind`, `Role`,
`Computed`) — criticality is the same kind of static, code-reviewed, per-field fact, not
operator-tunable config. It also keeps weight assignment in the same file as the fields it
weights, so adding a facet and forgetting to weight it is a one-file diff to catch in review.

### D2: Not-applicable persists in a new, dedicated table — not a 4th decision `source`

Add `facet_not_applicable(entity_type, entity_id, canonical_field, created_at)`, composite
`PRIMARY KEY (entity_type, entity_id, canonical_field)`, no other columns. A row's existence *is*
the fact — mirroring `person_image_suppressions` (migration `0012`), the closest existing shape
for "an owner-asserted exclusion with no value of its own." Marking not-applicable is
`INSERT OR IGNORE`; clearing it is a `DELETE`, both owner-gated. The resolver/scorer treats a
present row as "exclude this facet from this entity's score denominator and remediation queue,"
independent of whatever `field_source_decisions` says (or doesn't say) for the same field — the
two tables answer different questions and a row can exist in neither, either, or (in principle,
though the UI should prevent it) both without contradiction, since not-applicable is checked
first and short-circuits scoring before any source decision is consulted.

**Chosen over:** adding `source = 'not_applicable'` to `field_source_decisions`. Rejected because:
(a) every existing `source` value answers "which source's value wins," and not-applicable has no
value to select — the resolver's "decided source's current value" read path would need a special
case for a source that returns nothing, exactly the shape ADR-063 avoided for `computed`; (b) the
table's `UNIQUE (entity_type, entity_id, field_key)` means a field already carrying a `manual` or
`file` decision couldn't also be marked not-applicable without deleting that decision first, an
arbitrary ordering constraint the two-table design doesn't impose; (c) a reader scanning
`field_source_decisions` today reasonably assumes every row names a live source of truth — folding
in a value-less state quietly breaks that assumption for every existing caller, including
writeback, which reads decisions to know what to persist back to the file.

### D3: Score and actionability are computed on read, pure, entity-generic

A new pure function — `resolver.Complete(resolved []ResolvedField, notApplicable map[string]bool) Completeness`
(exact name TBD at implementation time) — runs as a post-pass alongside `Derive`, taking the
already-resolved field set plus the entity's not-applicable set (one query, loaded the same way
decisions/curation are pre-loaded today) and returning the score, actionability, and a per-facet
breakdown. No clock is involved (unlike `Derive`/Age), so there's no clock-injection concern; it
can live directly in `internal/resolver` without violating the package's clock-free contract.

Per-facet **tier** is derived entirely from the already-computed `ResolvedField.WinningSource` —
no new per-field resolution logic:

| `WinningSource` | Tier | Weight |
|---|---|---|
| empty (no resolved value) | missing | 0.0 |
| `provider:<name>` | provider | 0.7 |
| `file` or `manual` | curated | 1.0 |
| `computed:<canonical>` | *(excluded — never reached; `Computed` fields carry no criticality)* | — |

A facet present in `facet_not_applicable` is excluded from both the weighted-sum denominator and
the actionability count, regardless of tier.

**Chosen over:** storing the score as a column, recomputed on writes. Rejected for the same reason
ADR-063 rejected storing Age: staleness. A stored score needs invalidation hooks scattered across
every mutation path that can change a resolved value — decision changes, curation, writeback,
enrich, scan — and a missed hook silently shows a stale number, which is worse for a "should I
trust this score" feature than the general recompute cost.

### D4: List-wide sort/filter resolves the full per-type entity set in Go — no persisted/indexed score

Browse-page sort-by-completeness, the "missing facet" filter, and the remediation queue all share
one backend code path: fetch the full entity set for the type (bypassing SQL `LIMIT`/`OFFSET`),
resolve + score each entity in Go, then sort/filter/paginate in Go before responding. This is the
direct consequence of D3 — a compute-on-read value can't be pushed into a SQL `ORDER BY` or `WHERE`
without either a stored column or a per-request full scan, and D3 already ruled out the stored
column. The full-scan cost is bounded by this app's stated personal-library scale (the same
assumption already load-bearing for existing owner-mode-only full-table reads, e.g. the extraction
review queue), not by a SaaS-tenant row count.

**Chosen over:** a denormalized, indexed `completeness_score` column recomputed via triggers or a
background job. Rejected as premature for the current scale — it reintroduces the staleness
problem D3 avoided, plus a background-job/trigger seam this codebase doesn't otherwise have. If the
library grows past personal scale (see Consequences), this is the first thing to revisit.

### D5: `imdb_id` → `external_provider_id` is a straight rename, via a data migration

Rename the `FieldDef.Canonical` value in place (same registry entry, updated `Label`/`Description`
to describe "an external metadata-provider identifier, not specifically IMDb"). Ship a new
migration pair (`internal/db/migrations/00XX_rename_imdb_id_field_key.{up,down}.sql`) that runs
`UPDATE <table> SET field_key = 'external_provider_id' WHERE field_key = 'imdb_id'` (down does the
reverse) against every table found to store a field-keyed string: `field_source_decisions`,
`entity_enrichment`, `metadata_curation`, `provider_field_hints`, `field_promotions`,
`metadata_extraction_review`, `file_writeback_snapshots`, `file_writebacks`, `field_claims`. No
schema change is needed on any of them — `field_key`/equivalent columns are already untyped `TEXT`.
Operators with a custom (gitignored) `metadata-mappings.yaml` referencing `imdb_id` must rename
their own key; call this out as a breaking config change in the migration's comment and in the
release note, the same way this project has flagged prior operator-facing config renames.

**Chosen over:** introducing `external_provider_id` as a new field and leaving `imdb_id` in place,
unused, as a permanently-dead legacy entry. Rejected — this project's production instance was
never populated under the `imdb_id` name in practice (it uses a different, unnamed provider), so
there is no real population to preserve compatibility for, and a dead field with zero live rows is
pure clutter with no migration-avoidance benefit.

## Options Considered

### D2: not-applicable persistence

| Option | Complexity | Consistency with existing model | Cost |
|---|---|---|---|
| **New `facet_not_applicable` table (chosen)** | Low — one small table, mirrors `0012` | High — doesn't conflate "source" with "relevance" | One migration, one owner-gated mutation pair |
| 4th `field_source_decisions.source` value | Low to add, high to consume | Low — breaks the "every row names a live source" invariant every existing reader relies on | Special-casing in resolver, writeback, and the decision API's non-adoptability guard |
| Boolean column on `field_source_decisions` | Medium | Low — allows nonsensical states (`source='manual', not_applicable=true`) | Extra validation to reject the contradictory combination |

### D3/D4: score computation and list consumption

| Option | Complexity | Staleness risk | Fits current scale |
|---|---|---|---|
| **Compute-on-read, full-scan for lists (chosen)** | Low — no new storage, no invalidation | None | Yes, at personal-library scale |
| Stored/denormalized score column | Medium-high — triggers or job, invalidation surface | Real — missed hook shows a wrong score silently | Yes, but buys scale headroom this app doesn't need yet |
| Cache with TTL | Medium | Bounded but nonzero — score can lag a fresh decision for up to the TTL | Marginal win over full-scan at this scale |

### D5: `imdb_id` rename

| Option | Complexity | Operator impact |
|---|---|---|
| **Straight rename via data migration (chosen)** | Low — one migration, no schema change | One-time: rename a key in a gitignored local config file, if set |
| New field alongside old (supersede) | Low to add, permanent clutter | None immediately, but two names for one concept forever |
| Alias layer (public name ≠ stored column) | Medium — new indirection in registry/mapping | None, but adds a translation seam for a field with no real legacy data to protect |

## Trade-off Analysis

The throughline across D2–D4 is the same one ADR-051 and ADR-063 already established: keep each
table/token meaning exactly one thing, and let genuinely distinct concepts (source-of-truth
selection, computed derivation, relevance exclusion) live in genuinely distinct places rather than
overloading a table whose name and existing readers assume a narrower meaning. That costs one more
small table (D2) and non-SQL sort/filter for lists (D4) — both acceptable at this app's scale — in
exchange for not repeating the exact overload mistake ADR-063's `Computed` token was designed to
avoid. D5 is the one place this ADR chooses simplicity over caution (a real rename, not an alias),
justified specifically because there's no live data to protect — that calculus would flip if this
field had real production rows under the old name.

## Consequences

- **Easier:** adding a new critical/nice-to-have facet in the future is a one-line `FieldDef`
  edit, no migration. Not-applicable reads exactly like every other owner-gated exclusion this
  codebase already has (`person_image_suppressions`), so the mutation handler and frontend
  affordance can crib that shape directly.
- **Harder:** the browse/queue endpoints can no longer rely on SQL `LIMIT`/`OFFSET` for
  completeness-sorted or facet-filtered views — they need a Go-side resolve-all-then-paginate path
  distinct from their existing SQL-paginated path, which is genuinely more code than a plain
  `ORDER BY`.
- **Revisit:** if the library's entity counts grow well past personal scale (the spec and this ADR
  both assume hundreds to low thousands), the full-scan-per-request cost in D4 becomes the first
  thing to reconsider — likely a cached/precomputed score at that point, accepting the staleness
  trade-off D3 currently declines.

## Action Items

1. [ ] ADR recorded; add to `docs/architecture/README.md`'s index and update the F55 Phase-specs
       line's "ADR TBD" to link here.
2. [ ] `/design-handoff` for the remediation queue, breakdown panel, and browse filter/sort
       affordances, informed by the tri-state model and tier table above.
3. [ ] Backend: `FieldDef.Criticality` (D1), `facet_not_applicable` migration + owner-gated
       mutation (D2), `resolver` completeness post-pass (D3), Go-side resolve-all list path for
       browse/queue (D4).
4. [ ] Backend: `imdb_id` → `external_provider_id` rename — registry edit + migration pair across
       the nine tables listed in D5, `metadata-mappings.yaml.example` update, release-note callout
       for operators with a custom mapping file.
5. [ ] `/testing-strategy` update covering the tier-classification logic, not-applicable
       exclusion, and the rename migration's up/down round-trip.
6. [ ] `/security-review` on the new not-applicable owner-gated mutation before merge.
