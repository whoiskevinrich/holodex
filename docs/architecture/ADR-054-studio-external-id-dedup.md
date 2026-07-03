# ADR-054: Studio external-id de-dup — provider company id through resolved-value derivation

**Status:** Proposed
**Date:** 2026-07-02
**Deciders:** Project owner

**Extends:** [ADR-053](ADR-053-studio-entity-and-resolved-link-derivation.md) (studio entity + resolved-value link derivation — this ADR realizes the "**Studio external-id de-dup**" item that ADR-053's *What we'll need to revisit* flagged, and answers the design crux it named). **Relates to:** [ADR-033](ADR-033-metadata-source-plugins.md) (shadow-store + provider contract — the id rides `entity_enrichment` unchanged and a new *internal sidecar* field-key convention) · [ADR-036](ADR-036-person-alias-search-indexing.md) (person aliases — the *name*-based routing this ADR's id-based dedup is the deterministic counterpart to; studio aliases/merge stay deferred as F38 P2-1) · [ADR-047](ADR-047-per-item-metadata-refresh.md) (refresh — the stored company id makes a studio re-enrich re-fetch `/company/{id}` without re-searching by name). **Spec:** [Studio as a first-class entity / F38](../specs/studio-entity.md) (P2-5 → in-scope slice). **Issue:** [HOLODEX-122](https://whoiskevinrich.atlassian.net/browse/HOLODEX-122). **Companion:** [HOLODEX-121](https://whoiskevinrich.atlassian.net/browse/HOLODEX-121) (S3 TMDB company enrichment — this ADR makes S3's match + refresh deterministic).

---

## Context

ADR-053 promoted `studio` to a first-class entity whose `video_studios` links are **derived from the
resolved `studio` field** and reconciled by `RelinkVideoStudios` (the sole writer, via
`ReconcileVideoStudios` → `resolveOrCreateStudio`). Identity is exact-name resolve-or-create:
`studios.name UNIQUE`, trimmed. ADR-053 deliberately shipped **no provider id** and named two future
gaps — name-based *aliases/merge* (P2-1) and *external-id de-dup* — and called the id-through-derivation
question "the load-bearing design question." This ADR answers it.

Three things a stable provider id buys, none of which name-only resolve can:

- **Refresh.** A studio re-enrich (F38 S3 / HOLODEX-121) can re-fetch `/company/{id}` deterministically
  instead of re-searching by a name that a decision may since have changed — exactly what
  `entity_enrichment.external_id` + `MatchExternalID` already do for video and person.
- **Cross-enrichment hints.** A video's TMDB `production_companies[].id` can point its derived studio at
  the *right* TMDB company with no fuzzy search.
- **Deterministic de-dup.** "Warner Bros." and "Warner Bros. Pictures" that are the **same** TMDB
  company (id `174`) converge to **one** studio entity — the RD4 duplication problem solved
  deterministically, without the manual aliases/merge that P2-1 defers.

### The crux — the id lives a layer away from what derivation reads

`RelinkVideoStudios` resolves the `studio` **field** to plain **name strings** and hands them to
`ReconcileVideoStudios`. The TMDB company id is **not** in that resolved value; it lives in the
**video's TMDB enrichment**, and it is **per production company**, whereas the one place a provider id
is stored today — `entity_enrichment.external_id` — is **per (entity, provider) row** and already holds
the **movie's** id (`tmdb:<movieId>`), which `refresh` reads to re-pull the video. So there is
currently **no home** for the per-company ids, and the derivation path that would use them never sees
the provider response (it fires on scan/decision/curation, long after enrich). The id must therefore be
**captured at enrich time, persisted at the video layer, and threaded into derivation** — that
threading is what this ADR decides.

### Constraints / forces

- **Inherit, don't reinvent** (ADR-053's own constraint). Prefer riding the existing shadow-store
  write/read/clear machinery over a new video-side table with its own re-enrich/clear semantics.
- **Don't corrupt `external_id`.** `refresh` reads the per-row `external_id` as the movie match id
  (`ProviderMatches`/`ReEnrich`); the company id must not overwrite or masquerade as it.
- **The sidecar must never display or resolve.** A per-company-id channel is plumbing, not a user
  field: it must not surface in the media-detail `enriched[]` list (`FieldsFromRows`) and must be inert
  to the resolver (`ResolveFields` only touches mapped canonical fields).
- **Robust to the resolver.** Decisions and curation reorder, drop, and substitute studio values; a
  name→id mapping keyed by **position** would break. Keying by **name** is stable.
- **No new provider perimeter.** The TMDB provider already dials `/movie/{id}`; capturing a field it
  already receives adds no host, no SSRF surface, no asset. `/security-review` confirms.
- **Derivation stays pure and idempotent.** The id only *narrows* resolve-or-create; it adds no
  resolution I/O and re-running must converge to the same links.

---

## Decision

Capture the TMDB `production_companies[].id`, carry it to the video layer as a **self-describing
internal sidecar enrichment field**, and thread a **name→external_id side-map** into a
`studio_external_ids`-backed **external-id-first** `resolveOrCreateStudio`.

### 1 — Data model (migration 0018)

A join table mirroring the `person_aliases` shape (ADR-036) and the F32 person-external-id idiom — a
separate table, **never a column on `studios`** (RD: studio can carry ids from multiple providers):

```sql
CREATE TABLE studio_external_ids (
    studio_id   INTEGER NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    external_id TEXT    NOT NULL,          -- namespace-qualified, e.g. "tmdb:174"
    PRIMARY KEY (external_id)              -- one company id → exactly one studio (the dedup key)
);
CREATE INDEX idx_studio_external_ids_studio ON studio_external_ids(studio_id);
```

- **`external_id` is the primary key** → globally unique → a company id resolves to **one** studio (the
  convergence guarantee). It is also the lookup index for the external-id-first path.
- **`studio_id` is indexed**, not unique — a studio may hold *n* provider ids (multi-provider headroom;
  v1 writes only `tmdb:`).
- **`ON DELETE CASCADE`** → ADR-053's prune-on-empty (delete a studio with zero links) drops its id
  rows with it; no orphan ids, no cleanup tooling. The id re-attaches on the next derivation from the
  persisted video sidecar.

### 2 — Capture: an internal sidecar enrichment field

The TMDB provider's movie mapping already emits `fields["studio"] = [names…]`. It now **also** decodes
`production_companies[].id` and emits a companion field:

```
fields["_studio_external_ids"] = ["tmdb:174 Warner Bros. Pictures", "tmdb:923 Legendary Pictures", …]
```

- **Reserved `_` prefix = internal sidecar.** A field-key beginning with `_` is provider→core plumbing:
  it is persisted in `entity_enrichment` like any other field (no schema change) but is **never
  displayed and never resolved**. `FieldsFromRows` skips `_`-prefixed keys (one guard, general
  hygiene); the resolver already ignores any key not in a mapping. This establishes a small, documented
  contract extension (ADR-033 / the provider-contract spec), reusable by any future sidecar.
- **Self-describing value: `"<external_id> <name>"`.** The id token (`tmdb:<digits>`) contains no
  space, so the name is the unambiguous remainder after the first space. This makes the mapping
  **self-contained in one value** — it does *not* depend on positional alignment with the `studio`
  field (which the resolver may reorder/curate) and survives `SanitizeValue` (which strips control
  chars but keeps spaces and `:`). Only companies with a non-empty name **and** a positive id are
  emitted.
- **Persistence is free.** The sidecar rides the existing `UpsertEnrichment` (replace-on-re-enrich) and
  `DeleteEnrichmentByProvider` (clear) — so re-enrich refreshes it and clearing a provider removes it,
  with zero new write paths. `external_id` on the sidecar row keeps the movie id, untouched.

### 3 — Thread: name→external_id side-map into reconcile

`RelinkVideoStudios` already loads the video's enrichment rows to resolve the `studio` field. It now
**also** parses the `_studio_external_ids` rows into `map[name]externalID` and passes it alongside the
resolved names:

```
ReconcileVideoStudios(ctx, videoID, names, extIDByName)   // extIDByName: resolved-name → "tmdb:<id>"
    → resolveOrCreateStudio(ctx, tx, name, extIDByName[name])
```

The map is keyed by **name**, so a resolved value produced by a decision/curation in a different order
still finds its id, and a **custom/decided name with no matching company** simply has no entry → id
absent → name-only resolve (unchanged behavior). No signature change ripples past these two functions.

### 4 — Deterministic resolve: external-id first, then name

`resolveOrCreateStudio(name, externalID)`:

1. **`externalID != ""` → look it up in `studio_external_ids`.** Hit ⇒ return that studio
   (**convergence**: a different spelling of the same company links to the studio that already bears the
   id).
2. **Else exact trimmed name** (ADR-053 behavior).
3. **Create if neither matched**; when an `externalID` is in hand, `INSERT OR IGNORE` it into
   `studio_external_ids` for the resolved studio — including the case where step 2 matched a
   name-created studio that had no id yet (**id back-fill onto existing studios**).

Precedence is **id → name** so two spellings converge; the reverse would keep them split. `INSERT OR
IGNORE` on the id keeps the whole thing idempotent under the global `external_id` PK.

### 5 — What this ADR does *not* change

- **No new provider host, asset kind, or SSRF surface** — a field already in the `/movie/{id}` response.
- **No writeback** — studios have no file (ADR-053); this writes only DB join rows.
- **No resolver-core diff** — the sidecar is inert to `ResolveFields`; the ADR-052 zero-core-diff
  property still holds.
- **Studio aliases/merge (P2-1) stay deferred** — id-dedup handles *same-company* spellings; genuinely
  distinct names that are one studio without a shared id still need the name-based alias path.

---

## The RD1 refinement (the one user-visible consequence)

ADR-053 **RD1** states "the link always matches the displayed value." Convergence-by-id **refines**
this: when a provider id proves two spellings are one company, both videos link to a **single** studio
entity carrying **one** canonical name (the first spelling to create it — deterministic, churn-free; a
later re-derivation never renames an existing studio). So a video whose resolved `studio` field reads
"Warner Bros." may link to the studio entity **named** "Warner Bros. Pictures".

- **The common case is unaffected.** With one spelling in the library, the entity name equals the field
  value and RD1 holds verbatim. The divergence appears **only** in the exact dedup scenario the feature
  targets — and there it is the *point*: display and grouping still agree on the **entity** (they
  resolve to the same studio); only the entity's label may differ from a given video's file spelling,
  which is the spelling-harmonization the owner asked for.
- **Full-fidelity is P2-1.** Studio aliases would let the entity surface both spellings (searchable,
  displayable) — the refinement here is the deterministic 80% that lands without an alias table.

This is called out explicitly because it is the only place the feature softens a previously locked
decision; it is recorded as a Resolved Decision in the F38 spec so it is not later read as a bug.

---

## Options Considered

### Where the per-company id lives (the crux)

#### A — Self-describing internal sidecar field on `entity_enrichment` (chosen)

**Pros:** Reuses the entire shadow-store write/read/clear machinery — re-enrich replaces it, clear
removes it, backfill re-reads it — for one provider line + one display guard. Self-describing values are
immune to resolver reordering. No new table on the video side. **Cons:** Introduces a reserved-key
convention on a generic store; a contributor must know `_`-keys are plumbing (documented in the
provider contract + `FieldsFromRows`).

#### B — Dedicated `video_studio_external_ids(video_id, provider, name, external_id)` table

Mirrors F32's structured `people[]` channel more literally. **Pros:** keeps the generic enrichment
store free of reserved keys. **Cons:** re-implements re-enrich-replace and clear-on-provider-delete that
the shadow store already gives A for free; a new provider-contract response shape (`companies[]`) is a
bigger contract change than one extra `fields` entry; more surface for the same result. Rejected on
"inherit, don't reinvent."

#### C — Overload `entity_enrichment.external_id` on the `studio` row with the company id

**Pros:** no new field. **Cons:** `refresh` reads per-row `external_id` as the **movie** match id; a
company id there manufactures a bogus video "provider match" and breaks re-enrich. The `studio` row is
also single-`external_id` while there are *n* companies. Rejected — corrupts a load-bearing column.

### Mapping the id to a studio value

#### By name, via a self-describing side-map (chosen)

Keyed by name, so it survives the resolver's reordering/curation/substitution; an unmatched custom name
falls back to name-only resolve. **Alt — positional alignment** of the `studio` values with a parallel
id list: breaks the moment a decision reorders or drops a value, and `SanitizeValue`'s empty-drop can
shift slots. Rejected as fragile.

### Resolve precedence

**External-id first, then name (chosen)** converges two spellings of one company. **Name-first** would
find the spelling-specific studio before consulting the id and never converge — defeating the dedup
goal. Chosen id → name.

---

## Trade-off Analysis

**Reserved-key sidecar vs. a typed channel.** A per shows this is a small, contained lever: the `_`
convention costs one `HasPrefix` guard in `FieldsFromRows` and one line of provider output, and it buys
the shadow store's replace/clear/backfill semantics wholesale. The alternative (a typed `companies[]`
response + a video-side table) is "more correct" in the type-purity sense but re-derives machinery that
already exists and works, for the same observable behavior. For a single-owner server the reuse wins;
the guard + a provider-contract note keep the convention discoverable.

**Convergence softens RD1 — accepted deliberately.** Analyzed above: the divergence is confined to the
dedup scenario, where converging *is* the goal, and the common single-spelling case is untouched. The
alternative (never converge distinct names until P2-1 aliases) would leave the headline dedup goal
unmet, which the issue explicitly rejects.

**Best-effort id attachment, self-healing like the links.** Id capture and attachment inherit the
ADR-053 property: derivation runs in its own transaction after the triggering write, is idempotent, and
re-runs at scan/decision/curation/enrich/backfill. A missed attachment (e.g. process death mid-relink)
self-heals on the next trigger, exactly as a missed link does.

---

## Consequences

**What becomes easier**
- A studio re-enrich (HOLODEX-121) re-fetches `/company/{id}` deterministically — no name re-search.
- Same-company spellings converge to one entity with no manual merge; the facet stops listing the same
  studio twice.
- Videos deterministically hint their studio's provider identity for cross-enrichment.
- A reusable, documented internal-sidecar convention exists for any future provider→core plumbing that
  must not display (a second worked example beside F32's structured channel).

**What becomes harder**
- A generic store now carries a reserved key class; contributors must know `_`-keys are inert plumbing
  (guarded in `FieldsFromRows`, documented in the provider contract).
- The studio entity's canonical name can differ from an individual video's file spelling once dedup
  fires (the RD1 refinement) — documented so it is not read as a bug.

**What we'll need to revisit**
- **Studio aliases + merge (F38 P2-1)** — the name-based path for one-studio-two-names *without* a
  shared provider id; would also let a converged entity surface its alternate spellings.
- **Multi-provider ids** — the table already permits *n* ids per studio; a second provider just writes
  its own `<ns>:<id>` sidecar and rides the same resolve.
- **F32 person-external-id** — when it lands, reconcile the two id-capture styles (person's structured
  `people[]` vs. studio's sidecar field) or converge them on whichever proves cleaner.

---

## Action Items

1. [ ] Migration 0018: `studio_external_ids` (external_id PK, studio_id FK cascade + index); down drops it.
2. [ ] TMDB provider: decode `productionCompany.id`; emit `_studio_external_ids` = `"<ns>:<id> <name>"` per named, positive-id company (`providers/tmdb/tmdb.go`).
3. [ ] `FieldsFromRows` skips `_`-prefixed keys (internal sidecar); document the convention in the provider-contract spec.
4. [ ] `resolveOrCreateStudio(name, externalID)` external-id-first + `INSERT OR IGNORE` attach/back-fill; `ReconcileVideoStudios(…, extIDByName)` threads the map (`internal/repo/studios.go`).
5. [ ] `RelinkVideoStudios` parses `_studio_external_ids` rows into a name→external_id side-map (`internal/api/studios.go`); `model.StudioExternalIDsField` constant.
6. [ ] Add this ADR to `docs/architecture/README.md`; promote F38 spec P2-5 to an in-scope slice + Resolved Decision (RD1 refinement).
7. [ ] `/testing-strategy`: external-id resolve precedence, dedup-by-id across two spellings, name fallback when id absent, sidecar parse + `_`-key hidden, prune-on-empty cascades the id row.
8. [ ] `/security-review`: confirm no new host/asset/SSRF surface and no media-file write; sidecar rides the existing sanitize perimeter.
