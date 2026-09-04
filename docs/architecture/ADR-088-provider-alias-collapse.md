# ADR-088: Provider aliases collapse into the canonical alias spine

**Status:** Proposed
**Date:** 2026-09-02
**Deciders:** Project owner

**Extends:** [ADR-036](ADR-036-person-alias-search-indexing.md) (an alias is a name→canonical
routing rule; this ADR takes the "promote a provider's `aliases` field into real rows" bridge that
ADR-036 explicitly reserved, and makes it the only path) · [ADR-061](ADR-061-unified-entity-name-identity.md)
(`entity_aliases`, the polymorphic spine and its global `UNIQUE (entity_type, alias_key)`) ·
[ADR-033](ADR-033-metadata-source-plugins.md) (`entity_enrichment`, the shadow store a provider
alias now leaves on its way to a real row). **Relates to:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md)
(per-field decisions — `aliases` was never eligible, being a merge field) ·
[ADR-081](ADR-081-entity-completeness-score.md) (the scored facet this ADR re-homes).
**Spec:** [provider-alias-collapse.md](../specs/provider-alias-collapse.md) (F58, HOLODEX-306) —
which refines D3 with two decisions taken after this ADR was drafted: provider input is additive,
so a name the provider later drops is kept (spec RD5), and every AKA is imported except
punctuation/spacing near-duplicates of the canonical name (spec RD6).
**Design:** [alias-collapse-handoff.md](../design/alias-collapse-handoff.md).

---

## Context

A person has alternate names in two unrelated places, and only one of them works.

| | Mechanism A — registry field | Mechanism B — identity spine |
|---|---|---|
| Store | `entity_enrichment` (shadow) | `entity_aliases` |
| Source | TMDB `also_known_as` | owner typing, rename, merge |
| Curation | `metadata_curation` add/suppress | add/delete rows |
| Searchable | **no** | yes (`entity_aliases_fts`) |
| Routes scans | **no** | yes (`resolveOrCreateByName`) |
| UI | "Also known as" row, `CurationFieldRow` | Aliases panel, `AliasPanel.svelte` |

Both render on the person detail page, stacked within ~200px of each other
(`web/src/routes/people/[id]/+page.svelte:656` and `:734`), each labelled as alternate names. The
page comments acknowledge the split as deliberate — *"these names drive search + scan routing,
unlike the display-only 'Also known as' chips above"* — but the distinction is invisible to anyone
who has not read the source. The observable behaviour is that Holodex knows a person is also called
宮崎駿, displays it, and then fails to find them when you search it.

ADR-036 anticipated the fix and deferred it, choosing to keep curated aliases out of the enrichment
shadow layer and noting that the separation *"leaves room to later promote a provider's `aliases`
field into real `person_aliases` rows — a clean future bridge, not a merge of two concepts."* That
bridge was never built, so the provider half has sat inert since F22.

### Why now, and why not just build the bridge as a button

An owner-driven promote action (a "+" on each provider name) preserves both mechanisms and adds a
third state — *suggested but not yet promoted*. The suggestion tier has to be modelled, rendered,
dismissed, and re-suppressed on every re-enrich, which is more machinery than the collapse itself,
in service of a review step whose value is low: TMDB `also_known_as` is a curated field, and the
failure it guards against (a wrong name attaching) is already recoverable by deleting the alias.

## Decision

**Aliases have one store. Provider names land in it directly, and behave exactly like owner-typed
ones from the moment they arrive.**

### D1 — `aliases` leaves the field registry

Delete the `aliases` `FieldDef` from `internal/registry/registry.go` and the corresponding block
from `metadata-mappings.yaml.example`. The resolver stops emitting an `aliases` entry in
`resolved[]`; `entity_enrichment` stops being an alias store. `providers/tmdb` keeps reading
`also_known_as` — only its destination changes.

This does not reopen ADR-036's rejection of option 3. Aliases still do not *live* in
`entity_enrichment`; they merely arrive through it, as every enriched value does, and are written
onward into the spine.

### D2 — `entity_aliases` gains a `source` column

`source TEXT NOT NULL DEFAULT ''` — empty for owner-authored (the existing rows, and the default
keeps every current insert correct without touching its call site), otherwise the provider
namespace (`tmdb`). It is **provenance, not privilege**: no query filters on it except the
suppression bookkeeping in D4 and the badge in the UI.

### D3 — A provider alias is fully live on arrival

Searchable via the existing FTS triggers (which fire on insert — no new search work) and
scan-routing via `resolveOrCreateByName`, indistinguishable in behaviour from an owner-typed alias.
There is no confirmation step and no second tier.

The consequence is deliberate and is the point of the change: after enriching a person from TMDB, a
file tagged `H. Miyazaki` routes to them on the next scan. The risk that a junk AKA claims files is
accepted, bounded by D4 (deleting the alias stops it, permanently) and D5 (it can never steal a
name another entity already holds).

### D4 — Deleting a provider alias records a suppression

New table, keyed per-entity so a suppression is never global:

```sql
CREATE TABLE entity_alias_suppressions (
    entity_type TEXT NOT NULL,
    entity_id   INTEGER NOT NULL,
    alias_key   TEXT NOT NULL,
    PRIMARY KEY (entity_type, entity_id, alias_key)
);
```

The enrich path skips any candidate whose key is suppressed for that entity. Suppressing 宮崎駿 on
person 12 leaves person 40 free to hold it. Deleting an *owner-authored* alias writes nothing —
nothing would re-add it.

### D5 — Collisions skip and enqueue for review

`UNIQUE (entity_type, alias_key)` is global per entity type (ADR-061 RD1: one alias key belongs to
exactly one entity), so a provider name already held by another entity cannot be inserted. Enrich
skips it and inserts the pair into the existing `identity_review_queue` with
`variation='provider-alias'`. Enrich never fails on a collision, and never silently merges — the
owner decides, through the near-miss surface that already exists.

### D6 — The migration promotes existing curation, rather than dropping it

`metadata_curation` rows with `field_key='aliases'` carry real owner intent. The migration converts
`add` actions to owner alias rows (`INSERT OR IGNORE`, `source=''`) and `suppress` actions to
`entity_alias_suppressions` rows, then deletes them. An owner who curated the old display row keeps
the result, now searchable.

### D7 — Completeness re-homes to a synthetic facet

`aliases` leaving the registry removes a scored facet (`CriticalityNiceToHave`). Replace it with a
synthetic facet resolved by querying `entity_aliases` directly — structurally identical to the
studio `branding_image` facet (`registry.go:260`), which already exists precisely so a scorer can
weight something the resolver does not produce.

## Consequences

**Good.** One store, one API, one panel, one mental model — an alias is a name that finds this
entity, full stop. Provider names become useful instead of decorative, which is the user-visible
payoff. Search and scan routing get their coverage widened for free, since both already read the
spine. Net deletion of code: a registry field, a mapping block, a `CurationFieldRow` branch, and
the person page's merge-row loop.

**Costs and risks.**

- *Scan routing widens without an owner action.* Accepted per D3; mitigated by D4/D5. Worth
  watching after the first enrichment sweep of an existing library.
- *`metadata_curation` loses its only person-entity multi-value user.* If no other merge field is
  mapped for person, the person page's `mergeFields` loop renders nothing. Keep the loop — an
  operator can map one — but it is dead in the default configuration.
- *Suppression is a third small table on the identity spine.* Judged cheaper than the alternative
  (a soft-deleted tombstone row inside `entity_aliases`), which would hold the globally-unique key
  hostage and block a legitimate claim by another entity.
- *Studio inherits the change.* `AliasPanel` is reused verbatim on studio, and `entity_aliases` is
  polymorphic, so studio gets the same behaviour for free. Not tag (RD7 — tag has no panel).

## Alternatives considered

- **Two-zone panel with promote (rejected).** Confirmed chips above, dashed "suggested from TMDB"
  chips below with per-chip add/dismiss. Honest about the distinction and lower-risk, but it
  *keeps* both mechanisms and adds a suggestion state to model and persist. Rejected as the
  opposite of the goal: it makes the duplication legible rather than removing it.
- **Searchable on arrival, routing on confirm (rejected).** Provider aliases enter FTS immediately
  but carry a `confirmed` flag the scan-resolve predicate filters on. Splits the difference on the
  D3 risk, but reintroduces a two-tier distinction inside the single table being collapsed to, and
  makes `resolveOrCreateByName` — currently one predicate over one spine — source-aware.
- **Silent auto-import with no `source` column (rejected).** Simplest possible collapse. Rejected
  because without provenance a re-enrich cannot tell its own prior output from an owner's typing,
  so it can neither refresh nor respect a deletion.
- **Leave both, relabel the display row (rejected).** A copy fix for a data-model problem. The
  second list still cannot find the person.
