# ADR-053: Studio entity — data model + resolved-value link derivation

**Status:** Proposed
**Date:** 2026-07-02
**Deciders:** Project owner

**Relates to:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (per-field source-of-truth — **realizes its §9 fast-follow ③, promote Studio to an entity**; studio inherits the `(entity_type, entity_id, canonical_field)` decision/curation model) · [ADR-052](ADR-052-baseline-source-contract.md) (`BaselineSource` contract — studio adds a third `BaselineSource` impl, `studioBaseline`, mirroring F37's `personBaseline`) · [ADR-036](ADR-036-person-alias-search-indexing.md) (person aliases — the routing pattern this ADR **deliberately does not** adopt for studios in v1, and the one a P2 would follow) · [ADR-017](ADR-017-search-architecture.md) (global mixed-entity FTS — studio adds a `studios_fts` mirror) · [ADR-013](ADR-013-metadata-field-mapping.md) (field mapping — `studio` is a mapped canonical field whose *resolved* value drives the link). **Spec:** [Studio as a first-class entity / F38](../specs/studio-entity.md).

---

## Context

`studio` is a mapped canonical **video field** (file `Publisher`/`Label` baseline, multi-valued
TMDB `production_companies` candidates, F36 per-field decisions) that never becomes a **thing**.
There is no studio page, no "everything from this studio" navigation, and the browse facet is a
string-match over raw `video_metadata` file values (`?studio=Acme`, `internal/api/handlers.go`
~L283). People and tags have had identity, pages, and FTS since 0001; studio is the last
browse-grade concept still a loose string. ADR-051 §9 committed the decision model to being
entity-generic and named studio promotion as fast-follow ③; ADR-052 built the baseline seam and
F37 proved it on `person`. This ADR records the two decisions that promotion forces to the
architecture level.

**The load-bearing tension — links derive from a *higher* layer than people/tags.** Today a
video's people and tag links are derived from **raw file extraction**, inside the scan
transaction:

```go
// internal/repo/repo.go — replaceAssociations, called from UpsertVideo
for _, p := range people {           // people == ex.People, straight from extraction
    pid, _ := resolveOrCreatePerson(ctx, tx, p.Name)
    INSERT OR IGNORE INTO video_people (video_id, person_id) VALUES (?, ?)
}
```

That works because a person link *is* the raw file credit. A studio link, per F38 **RD1**, must
follow the **resolved** `studio` field — which includes the F36 decision short-circuit,
enrichment candidates, and F30 curation. So the link source is a layer the scan transaction
doesn't have in hand, and — critically — the resolved value **changes without a rescan**: adopting
a TMDB studio (a `PUT …/decision`), typing a custom value, or a re-enrich all move it, and none of
those paths run `UpsertVideo`. If studio links were derived at scan time like people, the browse
facet would group a video under its file value while the page *displays* the adopted value — the
exact display-vs-truth split F36 exists to kill, reintroduced one layer down.

### Constraints / forces

- **Display and grouping must never disagree** (the F36 invariant). The link and the media-detail
  value must come from the *same* resolution, or the facet lies.
- **Resolution stays pure** (ADR-033/052). Derivation resolves via the existing pure resolver over
  pre-loaded baseline + enrichment + curation + decisions; it adds no resolution I/O beyond the
  reads the media-detail path already does.
- **No new writeback surface.** A studio has no file (F37 precedent). Derivation writes only DB
  join rows; it never touches a media file. `/security-review` confirms this.
- **Identity is derived, not authored.** A studio's name comes from field values, so unlike a
  person (whose name is an authored identity with alias/merge, ADR-036) a studio has no stable
  independent identity to protect in v1. That is what makes deferring alias/merge safe — *if*
  orphaned studios are pruned (below).
- **Inherit, don't reinvent.** The decision/curation/enrichment stores are already entity-typed
  (migration 0016 comment reserves `"studio"`); studio must ride them with zero core diffs, as
  `person` did.

---

## Decision

Promote `studio` to a first-class entity with (1) a **`studios` + `video_studios` data model**
(migration 0017) and (2) a **resolved-value link-derivation rule**: `video_studios` is a
*derived index* over the resolved `studio` field, reconciled whenever that value can change.

### 1 — Data model (migration 0017)

The 0001 people pattern, verbatim — a name-keyed entity table, a composite-PK join, and an FTS5
mirror kept current by triggers:

```sql
CREATE TABLE studios (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE video_studios (
    video_id  INTEGER NOT NULL REFERENCES videos(id)  ON DELETE CASCADE,
    studio_id INTEGER NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    PRIMARY KEY (video_id, studio_id)
);
CREATE INDEX idx_video_studios_studio ON video_studios(studio_id);

CREATE VIRTUAL TABLE studios_fts USING fts5(
    name, content='studios', content_rowid='id',
    tokenize="unicode61 remove_diacritics 2"
);
-- studios_ai / studios_ad / studios_au triggers: the people_* triggers verbatim.
```

- **A join table, not `videos.studio_id`** (RD2). One row per **distinct resolved value** — one for
  today's single-valued `studio` mapping, *n* if an operator maps it multi. Schema headroom for
  multi-studio with no re-migration and no multi-studio UI commitment (a Non-Goal).
- **Exact-name resolve-or-create**, `studios.name UNIQUE`, trimmed (no case-folding beyond people's
  binary collation). No `external_id` column in v1 (studio has no provider-id de-dup need until the
  F38 enrichment slice needs one; if it does, it mirrors F32's person-external-id join-table
  approach — out of scope here).
- **Decision/curation/enrichment reuse** the existing tables with `entity_type = "studio"`. No new
  per-entity tables; the 0016 migration already anticipated this.

### 2 — Resolved-value link derivation (RD1)

`video_studios` is **derived**, never authored. A single idempotent reconcile —
`RelinkVideoStudios(videoID)` — is the *only* writer of `video_studios`:

1. Load the video's baseline (`NewVideoBaseline`), enrichment, curation, and decisions — the same
   inputs the media-detail handler assembles.
2. Resolve the `studio` field through the existing pure resolver; take its resolved value(s) (every
   value for a multi-mapped field, else the one value), trim, drop empties.
3. `resolveOrCreateStudio` each name; **replace** the video's `video_studios` rows with exactly that
   set (delete stale, insert missing) in one write transaction.
4. **Prune-on-empty:** delete any studio left with zero `video_studios` rows.

**Call sites** — every path that can change the resolved `studio` value, and no others:

| Trigger | Where | Why |
|---|---|---|
| Scan upsert | after `UpsertVideo` commits | file baseline may have changed |
| Enrich completion | after a video enrich writes `entity_enrichment` | new/changed candidate |
| Decision set/clear | `PUT`/`DELETE …/fields/studio/decision` | the short-circuit winner moved |
| Curation change | `studio` add/suppress/clear (if `studio` is ever curated) | curated value moved |

Derivation **never runs at read time** — the facet and studio pages read `video_studios` directly.
It is invoked in its **own write transaction after the triggering write commits**, not folded into
each call site's transaction (see Trade-off Analysis).

### 3 — `studioBaseline` (the third `BaselineSource`)

Mirrors `personBaseline` (F37) exactly: `name` is the only baseline-backed field; every other
canonical studio field resolves an empty-but-claimed baseline, so undecided enrichment-only fields
keep resolving to the provider value (the RD6 additivity property). The resolver-internal namespace
token stays `"file"`; the studio API presents it as `"record"` at the payload edge (RD4/RD5),
identical to person. **Zero diffs to `ResolveFields`, `resolveField`, the decision store, or the
chip components** — the ADR-052 claim, proven a third time.

### 4 — Facet + search

- Browse studio facet switches from `FacetValues` over mapped source keys to `studios` join counts
  (active, non-soft-deleted videos); filter via `?studio_id={id}`.
- The **legacy `?studio=` string filter (REST + MCP `fields`) is kept unchanged** for back-compat;
  it filters raw file values and may drift from entity grouping *by design* (documented).
- `studios_fts` participates in global mixed-entity search (ADR-017) as a new entity group.

### 5 — One-time backfill

After migration 0017, a startup pass derives links for all active videos via the same
`RelinkVideoStudios`, recorded as a System Activity job run (F21/ADR-028) so it is observable and
idempotent (no-op on re-run). This avoids requiring a manual rescan to populate an existing library.

### What is explicitly *not* in this ADR

Studio **identity operations — rename, aliases, merge — are deferred** (RD4), tracked as a P2 that
would mirror ADR-036. See Trade-off Analysis for why prune-on-empty makes that deferral safe. The
**TMDB company enrichment** slice (F38 S3/RD3) is a provider + registry change, not an architecture
decision — it rides the entity-generic enrichment path this ADR enables.

---

## Options Considered

### Link-derivation source

#### A — Links follow the *resolved* `studio` value (chosen)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium — a new derivation trigger surface (4 call sites) beyond scan |
| Consistency | Exact — display and grouping share one resolution; the facet cannot lie |
| Cost | Low — reuses the pure resolver + pre-loaded reads; reconcile is a bounded diff |
| Familiarity | Medium — a *new* pattern vs. people (raw-extraction links), but the resolver it calls is well-worn |

**Pros:** Kills the display-vs-truth split at the entity layer; an adopted/custom/enriched studio
re-links with no rescan; prune-on-empty keeps the entity set honest. **Cons:** Links must be
re-derived from more triggers than scan alone; a missed call site would silently stale a link (the
testing-strategy derivation matrix is the guard).

#### B — Links follow the raw file value at scan time (people's pattern)

**Pros:** Simplest — fold studio into `replaceAssociations` beside people, one transaction, one
trigger. **Cons:** A video whose studio decision adopts TMDB **displays** one studio but
**groups/filters** under another (or none); re-introduces exactly the split F36 removed, one layer
down. Rejected — it defeats the stated goal (RD1).

#### C — No entity; keep the string facet, add only a studio *view* page keyed by string

**Pros:** No migration, no join. **Cons:** No stable id to hang decisions/curation/FTS/enrichment
on; can't inherit the decision model (the whole point of fast-follow ③); string identity breaks the
moment a decision changes the displayed value. Rejected.

### Data model shape

#### Join table `video_studios` (chosen) vs. single FK `videos.studio_id`

**Join (chosen):** mirrors `video_people`; one row per resolved value; no re-migration if `studio`
becomes multi-valued; TMDB's extra production companies have a home if ever surfaced. **Single FK:**
simplest, but one primary studio per video only, and multi-studio later needs a schema migration
while TMDB's extra companies stay display-only candidates. Chosen the join for headroom at trivial
extra cost (RD2).

---

## Trade-off Analysis

**Derivation transaction boundary — separate tx after commit, not folded in.** Reconcile runs in
its own write transaction *after* the triggering write commits, rather than inside each call site's
transaction. Folding-in would give strict atomicity (the link can never lag its cause) but would
force every call site — scan, enrich, decision handler, curation handler — to carry the full
resolve-and-reconcile logic inside its own transaction and hold the write lock across a resolver
pass. A separate post-commit reconcile keeps each call site's write small and puts *all*
`video_studios` mutation behind one idempotent function. The cost is a narrow window where a link
lags its cause if the process dies between the two writes; because reconcile is idempotent and also
runs at scan and at startup backfill, any such lag is self-healing on the next trigger or restart.
Acceptable for a single-owner media server; the alternative's lock-scope cost is not.

**Deferring identity ops (rename/aliases/merge) — safe *because* of prune-on-empty.** A person
needs alias routing (ADR-036) because its name is an authored identity that must survive rescans and
never silently merge homonyms. A studio's name is *derived from field values*, so the correction
path is different: to "rename" a studio you fix the `studio` field on its videos (a decision or
custom value), the links re-derive, and the old studio — now linked by nothing — is pruned. No
orphan, no ghost in the facet, no alias table needed. This only holds if **prune-on-empty** is part
of the derivation (it is, step 4); without it, deferring identity ops would leave dangling empty
studios and the deferral would be unsafe. The residual gap deferral accepts: two *distinct* file
spellings of one real studio ("Warner Bros." vs "Warner Bros. Pictures") produce two entities until
the owner harmonizes the underlying field values — the P2 (aliases/merge) becomes worth building the
first time that pain is real, and it slots onto this exact model without reopening it.

**A new derivation pattern vs. reusing people's.** This ADR knowingly introduces a *second* link
pattern (resolved-value-derived) alongside people/tags (raw-extraction-derived). The inconsistency
is the price of correctness: people links are correct at raw extraction because a credit *is* the
raw value; studio links are only correct post-resolution because a studio *is* the decided value.
Documenting both patterns explicitly (here + the spec) is cheaper than forcing studio into people's
mold and re-deriving the F36 split.

---

## Consequences

**What becomes easier**
- Studio becomes navigable identity (page, facet, global search) and inherits adopt/custom/blank
  curation with no resolver-core changes.
- The browse facet groups by the *displayed* value; the F36 invariant holds at the entity layer.
- Fixing the last misspelled video prunes the bogus studio automatically — no cleanup tooling.
- F32 (video credits) and any future entity have a second worked example of entity promotion.

**What becomes harder**
- There are now two link-derivation patterns in the codebase; a contributor adding a studio-value
  write path must remember to call `RelinkVideoStudios`. Mitigated by concentrating all
  `video_studios` writes in that one function and by the testing-strategy derivation matrix.
- The legacy `?studio=` string filter and the new `?studio_id=` entity filter can return different
  sets (raw vs. resolved); this is intended but must stay documented so it isn't read as a bug.

**What we'll need to revisit**
- **Studio aliases + merge** (P2) — add `studio_aliases` + routing at derivation time + a merge
  endpoint the first time two real spellings both carry libraries; mirrors ADR-036.
- **Studio external-id de-dup** — if the enrichment slice needs deterministic provider-id matching,
  add it as F32 does for people (join table), not a column.
- **Multi-studio UI** — only if `studio` is ever promoted to a merge field; the schema already
  permits it.

---

## Action Items

1. [ ] Migration 0017: `studios`, `video_studios`, `studios_fts` + triggers (0001 pattern); down migration drops them.
2. [ ] `resolveOrCreateStudio` + `RelinkVideoStudios(videoID)` reconcile with prune-on-empty (single writer of `video_studios`).
3. [ ] Wire the four call sites: scan upsert, enrich completion, decision set/clear, curation change (studio field).
4. [ ] `studioBaseline` (`NewStudioBaseline`) in `internal/resolver`, mirroring `personBaseline`; assert zero resolver-core diffs.
5. [ ] Startup backfill pass over active videos, recorded as a System Activity job run (idempotent).
6. [ ] Facet → `studios` join counts + `?studio_id=`; keep `?studio=` string filter (REST + MCP) unchanged and documented.
7. [ ] Add this ADR to `docs/architecture/README.md`; cross-reference from the F38 spec.
8. [ ] `/testing-strategy`: derivation matrix (scan/enrich/decision/curation × link outcome), prune-on-empty, backfill idempotency, `studioBaseline` additivity, facet-vs-soft-delete.
9. [ ] `/security-review` before merge: owner-gated studio endpoints; untrusted provider company data through the existing sanitize + `asset_hosts` perimeter; confirm no media-file write anywhere in F38.
