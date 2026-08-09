# ADR-082: `external_provider_id` is a namespace-qualified value, not a plain rename

**Status:** Proposed
**Date:** 2026-08-08
**Deciders:** Kevin Rich

**Supersedes:** [ADR-081](ADR-081-entity-completeness-score.md) **D5** only — the decision that
`imdb_id` → `external_provider_id` is a straight `field_key` rename with a bare-string value.
ADR-081's D1–D4 (facet criticality, not-applicable persistence, score computation, list
consumption) stand unchanged. **Extends:** [ADR-054](ADR-054-studio-external-id-dedup.md) (the
namespace-qualified-value convention this ADR reuses) · [ADR-033](ADR-033-metadata-source-plugins.md)
(declared-not-compiled-in provider registry — the reason a bare id is ambiguous). **Relates to:**
[ADR-074](ADR-074-claimed-provider-keys.md) (a structurally different multi-provider mechanism,
considered and rejected below). **Spec:** [entity-completeness-score.md](../specs/entity-completeness-score.md)
(F55.11, open question "Multi-provider external-ID shape"). **Issue:** HOLODEX-260.

---

## Context

D5 justified a plain `field_key` rename on a specific claim: *"this project's production instance
was never populated under the `imdb_id` name in practice (it uses a different, unnamed
provider)"* — singular. That premise is false. This repo's `providers/` directory ships one
compiled-in sidecar (TMDB), but `metadata-sources.yaml` (ADR-033) is a **declared-not-compiled-in**
provider registry: an operator can and does run additional providers against a live instance that
are not part of this codebase and must never be named in it (the standing constraint ADR-081
already carries). Production enrichment is therefore not limited to what ships here.

That changes what a resolved `external_provider_id` value has to represent. A bare id string
(`"608"`, `"tt0137523"`) carries no information about which provider issued it. With exactly one
possible source, that was recoverable from context; with more than one, it is not — the same
digits-only shape can collide across providers, a UI can't build a link back to the source without
knowing which host `/title/{id}` or `/movie/{id}` means, and D2's not-applicable / dedup logic has
no way to tell "same id, same provider" from "same id, different provider, coincidence."

This is not a new problem for this codebase. It already has a working answer.

### The existing precedent: namespace-qualified scalar values

Three places already solve "one external-id concept, multiple possible providers" the same way —
by prefixing the value, not by adding a column:

- **`entity_enrichment.match_id`** (ADR-033) — already `"tmdb:608"` in production
  (`internal/repo/enrichment_test.go:28`).
- **`_studio_external_ids`** (ADR-054, migration 0018) — `studio_external_ids.external_id` is
  `"tmdb:174"`, explicitly namespace-qualified so the value survives resolver reordering and stays
  self-describing on its own.
- **`person_external_ids`** (migration 0038) — `external_id` is `"tmdb:6384"`, and its own comment
  states the general case: *"a person may carry ids from multiple providers (e.g. tmdb + imdb)."*

All three are a single scalar `TEXT` value carrying `<namespace>:<id>`, not a `(provider, id)`
column pair. `external_provider_id` is the same shape of problem as these three — a single
identifying value for an entity, sourced from a provider that varies — so it should be the same
shape of answer.

This is a **closer analogy** than [ADR-074](ADR-074-claimed-provider-keys.md)'s `field_claims`
table, which does use a dedicated `provider` column. That table's provider column is load-bearing
for a different reason: a claim's *identity* is `(entity_type, provider, field_key)` — `provA:rating`
(an age certificate) and `provB:rating` (a 1–10 score) are different assertions that must never
collapse into one row, because the table's whole job is disambiguating *which provider's raw key
name means what*. `external_provider_id` isn't naming a key — it's holding a value. There is
already a live TMDB implementation bug demonstrating the gap this ADR closes: `providers/tmdb/tmdb.go:539`
emits `fields["imdb_id"] = []string{det.IMDbID}` — a **bare** IMDb id, with no `imdb:` prefix, from
the one provider already in this codebase.

### Forces

- **Reuse the existing convention, don't invent a fourth one.** Three precedents already agree on
  namespace-qualified scalars; a `(provider, id)`-keyed schema change across the nine `field_key`-keyed
  tables D5 named would be a new, fourth shape for the same underlying problem, with no precedent
  and a much larger migration surface.
- **D5's "no live data to protect" premise needs re-examination, not just its provider-count
  claim.** If any provider (in-repo or operator-declared) has been enriching `imdb_id` in
  production, those rows may already hold bare, non-namespaced values — the migration must handle
  that, not just rewrite the `field_key` column.
- **Migrations are append-only with a manual down** (project convention) — this still ships as one
  new numbered migration pair, per D5's original plan; only its `UPDATE` statement's scope changes.
- **Standing constraint carried forward from ADR-081:** no spec/ADR/commit may name the real
  third-party adult-content enrichment provider. Every example here uses `tmdb`/`imdb`/generic
  placeholders only.

---

## Decision

### Value shape: `<namespace>:<id>`, scalar `field_key` unchanged

`external_provider_id` remains a single scalar `field_key`, riding the same nine tables D5 already
enumerated (`field_source_decisions`, `entity_enrichment`, `metadata_curation`,
`provider_field_hints`, `field_promotions`, `metadata_extraction_review`,
`file_writeback_snapshots`, `file_writebacks`, `field_claims`) with **no schema change** to any of
them — `field_key`/equivalent columns are already untyped `TEXT`, exactly as D5 established. What
changes is that every writer of this field must emit `<namespace>:<id>` (e.g. `"tmdb:603"`,
`"imdb:tt1234567"`), never a bare id. Multiple providers supplying different candidate values for
the same video is not a new problem this facet introduces — it's the same multi-candidate,
one-decision-winner shape every other resolved field already has (ADR-051); the decision's
`source` column (`provider:tmdb`, `provider:<other>`) already records which provider's candidate
won. Namespacing the value adds the one thing the decision grammar doesn't carry: a value that is
self-describing when read on its own, outside the decision row that produced it (display, links,
dedup, refresh).

### Migration: rename `field_key` *and* namespace-qualify existing values

The migration D5 planned (`internal/db/migrations/00XX_rename_imdb_id_field_key.{up,down}.sql`)
still runs, but its `up` does two things instead of one:

1. `UPDATE <table> SET field_key = 'external_provider_id' WHERE field_key = 'imdb_id'` — unchanged
   from D5.
2. For every row now carrying `field_key = 'external_provider_id'` whose value does not already
   contain a `:` separator, prefix it with `imdb:` — the only namespace a bare legacy `imdb_id`
   value could have come from, since that was the field's sole historical meaning. Any value
   already containing `:` (from a provider that was already emitting namespace-qualified data, or
   backfilled ahead of this migration) passes through unchanged, so the migration is idempotent if
   re-run against a partially-migrated table.

`down` reverses both: strip a leading `imdb:` (restoring the bare value) and rename the `field_key`
back to `imdb_id`. A value under a different namespace (`tmdb:`, or an operator-declared provider)
has no `imdb_id`-era equivalent to revert to and is left namespace-qualified on down-migration —
documented in the migration's comment, matching how this project already flags irreversible edge
cases in manual-down migrations.

### Provider emission: TMDB sidecar namespaces its own value

`providers/tmdb/tmdb.go:539` changes from:

```go
fields["imdb_id"] = []string{det.IMDbID}
```

to:

```go
fields["external_provider_id"] = []string{"imdb:" + det.IMDbID}
```

reusing the `imdb` token the sidecar already treats as a first-class namespace elsewhere
(`handler.go:38`, `IDNamespaces: []string{"tmdb", "imdb"}`). Any other provider (in-repo or
operator-declared) emitting this field must namespace-qualify its own value the same way; this is
a provider-contract expectation, not new plumbing — no protocol change, since the field already
flows through the existing untyped `fields[canonical][]string` shape.

### Display: strip the namespace for the label, keep it for the link

`registry.FieldDef.Label`/`Description` describe the facet ("External ID"), not its value's
namespace, so the owner-facing render should show the id portion after the `:` and use the
namespace prefix only to build a provider link (`tmdb:603` → `https://www.themoviedb.org/movie/603`,
`imdb:tt1234567` → `https://www.imdb.com/title/tt1234567/`) when the namespace is recognized, or
show the raw `namespace:id` string as inert text otherwise. This is a display concern in the
frontend field-rendering layer, not a registry or resolver change — flagged here so the
`/design-handoff` this ADR's action items require doesn't have to re-derive it.

---

## Options Considered

### Where multi-provider identity lives

#### A — Namespace-qualified scalar value (chosen)

Reuses `entity_enrichment.match_id` / `_studio_external_ids` / `person_external_ids` verbatim.
**Pros:** zero schema change past what D5 already planned; one migration; self-describing value
survives outside its row (display, links, dedup) the same way the three existing precedents do.
**Cons:** the `:` separator is a value-format convention enforced by discipline, not a `CHECK`
constraint — a provider that forgets to namespace its value produces an ambiguous row, same
residual risk the three existing precedents already accept.

#### B — `(provider, external_id)`-keyed schema change across the nine tables

Add a `provider` column to every `field_key`-keyed table D5 listed, mirroring `field_claims`'s
primary key. **Pros:** provider identity becomes a queryable column, not embedded string
structure. **Cons:** nine schema migrations instead of one data migration; every reader of every
one of those tables (decision resolution, curation, writeback, extraction review) gains a new
dimension to reason about for a single field, when only one field needs it; no existing precedent
in this codebase does this for a value-holding field — `field_claims`'s provider column exists
because *identity itself* is provider-scoped there, which is not true of `external_provider_id`.
Rejected: solves a problem this field doesn't have, at a much higher migration cost, and
contradicts three already-shipped precedents that chose Option A's shape for the same kind of
problem.

#### C — Leave `external_provider_id` a bare scalar, disambiguate providers elsewhere

E.g. infer the provider from the winning decision's `source` column (`provider:tmdb`) instead of
the value itself. **Pros:** no value-format change. **Cons:** only works while a value is attached
to its decision row; the moment it's read standalone (frontend display, a future export, a link
builder, D2's not-applicable/dedup reasoning), the provider is lost. `entity_enrichment.match_id`
already rejected exactly this shape by choosing to namespace-qualify itself rather than lean on its
sibling `provider` column. Rejected for consistency and because it breaks the "value is
self-describing" property every other external-id-shaped field in this codebase has.

---

## Trade-off Analysis

The throughline is the same one D1–D4 already established for this feature: reuse what the
codebase already proved rather than inventing a parallel shape for a problem it has already
solved. Option B is "more correct" in a type-purity sense — provider as a real column instead of
string structure — but this codebase has three independent, shipped precedents that chose the
value-prefix shape for the identical problem (one external identifier, multiple possible sources),
and none that chose a provider column for a *value*-holding field. Matching that precedent keeps
`external_provider_id` legible to anyone who already understands `match_id` or
`_studio_external_ids`, and keeps the migration to the single pair D5 already scoped, just with a
value-rewrite pass added to its `up`/`down`.

---

## Consequences

- **Easier:** a resolved `external_provider_id` value is self-describing wherever it's read —
  display, provider-link construction, dedup — with no dependency on the decision row that
  produced it. No new schema surface for any of the nine `field_key`-keyed tables to absorb.
- **Harder:** value-format correctness (the `:` separator) is a provider-contract convention, not
  enforced by the database; a misbehaving provider can still write an unnamespaced value. This is
  the same residual risk `match_id`/`_studio_external_ids`/`person_external_ids` already carry, not
  a new one.
- **Revisit:** if a provider ever needs to attach *structured* per-id metadata beyond a namespace
  and an id (e.g. a confidence score, a fetched-at timestamp) the scalar-value shape stops being
  enough and the `field_claims`-style dedicated-column table becomes the right answer — this ADR
  doesn't foreclose that, it just says today's problem (disambiguate which provider, nothing more)
  doesn't need it yet.

---

## Action Items

1. [ ] ADR recorded; add to `docs/architecture/README.md`'s index.
2. [ ] `docs/specs/entity-completeness-score.md`: resolve the "Multi-provider external-ID shape"
       open question against this ADR; fix the Context section's singular-provider framing
       (~line 47–48); update F55.11's acceptance criteria for namespace-qualified values.
3. [ ] `internal/registry/registry.go`: `imdb_id` → `external_provider_id` rename (D5, unchanged),
       updated `Label`/`Description` noting the namespace-qualified value shape.
4. [ ] `providers/tmdb/tmdb.go:539`: emit `"imdb:" + det.IMDbID` instead of a bare value.
5. [ ] Migration `00XX_rename_imdb_id_field_key.{up,down}.sql`: rename `field_key` (D5, unchanged)
       **and** namespace-qualify (`up`) / strip-and-revert (`down`) any value not already
       containing `:`, across the same nine tables D5 enumerated.
6. [ ] `/design-handoff` follow-up (or fold into the existing F55 remediation-queue/breakdown-panel
       handoff): namespace-strip-for-label + provider-link-build display rule.
7. [ ] `/testing-strategy` update: migration up/down round-trip on both bare-legacy and
       already-namespaced rows; TMDB sidecar emits namespace-qualified values; display strips the
       namespace for known providers and shows raw text for unrecognized ones.
