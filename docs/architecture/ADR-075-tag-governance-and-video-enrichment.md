# ADR-075: Tag governance & video enrichment — hierarchy column, deny-list table, provenance-aware rescan, write-on-resolve materialization

**Status:** Proposed
**Date:** 2026-07-29
**Deciders:** Project owner

**Extends:** [ADR-061](ADR-061-unified-entity-name-identity.md) (the `resolveOrCreateByName` spine, `entity_aliases`,
`entity_keep_separate` — this ADR's D1 cycle-guard placement and D2 table-shape comparison both cite it
directly) · [ADR-066](ADR-066-enrichment-auto-apply-and-dismissal.md) (`enrichment_dismissals` — the other
comparison point for D2, and the closest existing precedent for "enrichment writes automatically under a
trust rule," relevant to D4). **Relates to:** [ADR-041](ADR-041-metadata-writeback.md) (`internal/writeback/tags.go`
already maps canonical `genres` → the file `Genre` tag for every writable container; this ADR does not touch
that mapping, only what feeds it) · [ADR-013](ADR-013-metadata-field-mapping.md) (the `genres` canonical field,
`multi: true`, sourced from `tmdb:genres`, that D4 materializes from) · [ADR-063](ADR-063-derived-computed-fields.md)
(a partial precedent for D4's "pass chained after resolve" shape — explicitly a **different** posture, see D4).
**Spec:** [tag-governance-and-video-enrichment.md](../specs/tag-governance-and-video-enrichment.md) (F50,
working number). **Issue:** TBD — no Jira issue created yet for this work.

---

## Context

F43 gave tags an identity spine but no governance and no structure; F50's spec adds a deny-list, a hierarchy,
automatic tag-materialization from video enrichment, manual video-tag editing, and a genre writeback source —
all resolved at the product level (see the spec's Resolved Decisions). What remains is the technical shape of
four decisions, one of which (D3) is a correctness fix for a latent bug the other three would otherwise expose.

### Current state (survey, 2026-07-29)

| Seam | Today | File |
|---|---|---|
| `tags` | `id, name` only — no structure beyond F43's identity spine (`nameKey` uniqueness, aliases, merge) | `internal/db/migrations/0001_init.up.sql:28` |
| `video_tags` | `(video_id, tag_id)` composite PK, **no provenance column** | `internal/db/migrations/0001_init.up.sql:40` |
| `replaceAssociations()` | Unconditional `DELETE FROM video_tags WHERE video_id = ?` then reinsert purely from `v.Tags` (the file's embedded tags) on every scan/re-extract | `internal/repo/repo.go:177` |
| `resolveOrCreateByName` | id→nameKey→create, entity-generic (person/studio/tag); the **one** function every tag-creation path already calls | `internal/repo/identity.go` |
| Durable negative-assertion tables | Two exist: `entity_keep_separate` (`entity_type, id_lo, id_hi` — F43) and `enrichment_dismissals` (`entity_type, entity_id, provider` — F47); both are *relationship*-shaped (a pair, or an entity+provider pair) | ADR-061, ADR-066 |
| `fieldsource` grammar | `file` / `manual` / `provider:<name>`, with `Valid()`/`ForNamespace()` gating what counts as an **adoptable decision source** in the F36 per-field model | `internal/fieldsource/fieldsource.go` |
| `afterEnrichApply` | Existing shared post-apply side-effect dispatcher, switched on `entityType`, already called from manual apply, Refresh, and Refresh-all so all three stay in lockstep | `internal/api/enrich_review.go:259` |
| `genres` writeback mapping | Already wired: canonical `genres` → `Genre`/`QuickTime:Genre` across Matroska/WebM/MP4/mp3/flac | `internal/writeback/tags.go:103` (and per-container) |

### Forces

- **`video_tags` has no way to say where a row came from, and the scanner already assumes it doesn't need
  one.** `replaceAssociations()`'s full delete-and-reinsert is *correct* today only because tags have exactly
  one source. The moment this spec ships, that stops being true, and the fix has to land **before or alongside**
  manual tagging and materialization — not after, or the first rescan silently deletes both.
- **A strict tree needs the least structure that can express it.** One parent per tag is exactly a nullable
  self-reference; reaching for a closure table or materialized path buys O(1) ancestor reads at the cost of
  write-time upkeep this codebase's scale (a personal library's tag count, shallow hierarchy depth) doesn't
  need yet.
- **A third durable-negative-assertion table must earn its keep, not just match a vibe.** ADR-061's
  `entity_keep_separate` and ADR-066's `enrichment_dismissals` are both *relationship* records (a pair of
  things). A deny-list blocks a bare *term*, unconditioned on any entity or provider — a different arity, not
  a stylistic variant of either.
- **Denying a term has to stop a row from ever being created, not flag one that already exists.** This rules
  out anything shaped as a column on `tags` — by the time a `tags` row exists, denial already failed.
- **Materialization is a write triggered by a read-path event, which is new for this codebase.** ADR-063's
  `Derive(resolved, now)` is the closest precedent for "a pass appended after resolve" — but `Derive` is pure,
  computes nothing durable, and writes nothing. F50's materialization pass **does** write (`video_tags` rows),
  making it a different kind of thing wearing a similar shape; conflating the two would be a mistake worth
  naming explicitly rather than silently inheriting ADR-063's "just a pass" framing.
- **Whatever hooks materialization in must not fork behavior between manual apply and Refresh/Refresh-all.**
  `afterEnrichApply` exists precisely to prevent that fork for the studio-relink and studio-logo side effects
  it already runs; a new side effect that bypasses it would silently work for manual apply and not Refresh.

---

## Decision

### D1 — Hierarchy: `tags.parent_tag_id`, strict tree, application-layer cycle guard, no denormalized descendant cache

```sql
ALTER TABLE tags ADD COLUMN parent_tag_id INTEGER REFERENCES tags(id) ON DELETE SET NULL;
CREATE INDEX idx_tags_parent ON tags(parent_tag_id);
```

One nullable self-referential column — not a `tag_hierarchy(tag_id, ancestor_id, depth)` closure table, not an
adjacency table with its own PK. A strict tree (spec RD6: one parent per tag, no DAG) is fully expressed by a
parent pointer; a closure table's payoff — O(1) ancestor/descendant reads — is a write-amplification trade
that buys nothing at this scale (a single owner's tag count, hierarchies realistically a few levels deep) and
would need active maintenance on every reparent and merge that a plain column doesn't.

**Cycle prevention is application-layer**, enforced at `POST /tags/{id}/parent`: before committing, walk the
ancestor chain of the proposed `parent_id` (via the same recursive query pattern as descendant-expansion,
below) and reject (`400`) if the tag being reparented appears in it, or if `parent_id == id`. This mirrors
[ADR-061](ADR-061-unified-entity-name-identity.md)'s own posture — the cross-namespace identity guarantee
(a name can't be canonical for one entity and an alias for another) is enforced at the resolve/mutation layer,
not in DDL, because SQLite has no native constraint expressive enough for either invariant.

**Descendant expansion is a query-time `WITH RECURSIVE` over `parent_tag_id`**, run fresh per filter/search
call — no materialized/denormalized descendant set stored anywhere. This matches F43's own precedent: aliases
are not flattened into `tags.name`, they're resolved via `entity_aliases` at query time; hierarchy gets the
same treatment for the same reason (a single source of truth, no drift between a base row and a cache of it).

**Merge reparenting is additive to the existing merge function**, not a new one: the same transaction that
already moves `video_tags` associations and registers the loser's name as an alias
(`entity-identity.md` §Behavior detail, RD6) gains one more statement,
`UPDATE tags SET parent_tag_id = <survivor_id> WHERE parent_tag_id = <loser_id>`, keeping the tree connected
through routine identity cleanup instead of orphaning a merged-away tag's children.

### D2 — `denied_tags`: a new table, a third and genuinely different shape of durable negative assertion

```sql
denied_tags
  term_key    TEXT PRIMARY KEY   -- normalize(term) — same fold as tag nameKey (F43 RD2)
  term        TEXT NOT NULL      -- original casing, display/audit only
  created_at  DATETIME NOT NULL
```

This codebase already has two tables in the "durable negative assertion" family — `entity_keep_separate`
(`entity_type, id_lo, id_hi`: two *specific entities* are deliberately distinct) and `enrichment_dismissals`
(`entity_type, entity_id, provider`: one *specific entity* rejected one *specific provider's* candidates).
Both are relationship records with an entity dimension. A deny-list has neither: it blocks a bare string,
globally, independent of which entity or provider might produce it. Forcing it into either existing shape
would mean inventing a synthetic entity id or provider value to fill a column the concept doesn't have —
worse than a new table with the arity the concept actually needs.

**Enforcement point: inside `resolveOrCreateByName`, gated on `entityType == model.EntityTag`** — a pre-check
before the existing id→nameKey→create order, returning a typed sentinel (`ErrTagDenied`) rather than creating
a row. Every path that can create a tag — the scanner's `replaceAssociations`, the new manual-attach endpoint,
and the new materialization pass (D4) — already calls this one function (F43's own design point: it's the
single spine all three route through). Gating there, instead of at each of the three call sites, means a
denied term is blocked **by construction**, not by three independently-maintained checks that a future fourth
caller could forget to add. Callers translate the sentinel to their own semantics (scanner: skip silently and
continue; manual attach: `422`; materialization: skip silently) — the denial itself is decided in exactly one
place.

### D3 — `video_tags` gains `source`; `replaceAssociations` becomes partial-replace

**This is the highest-risk decision in this ADR** — not because the fix is complex, but because getting it
wrong (or skipping it) silently destroys owner data on the very next scan, with no error and no signal that
anything was lost.

```sql
ALTER TABLE video_tags ADD COLUMN source TEXT NOT NULL DEFAULT 'file';
```

`source` reuses the **existing** `fieldsource` grammar (`internal/fieldsource/fieldsource.go`) — `file` /
`manual` / `provider:<name>` — rather than inventing a second vocabulary for the same concept. Note this is a
plain string column, **not** validated against `fieldsource.Valid()`/`ForNamespace()`: those functions answer
"is this a well-formed **adoptable decision source**" for the F36 per-field model, and tags are explicitly
outside that model (F43 RD7 — "identity-only, not `BaselineSource`"). `video_tags.source` borrows the
grammar's vocabulary for consistency, not its decision-model machinery.

`replaceAssociations()` changes from:
```sql
DELETE FROM video_tags WHERE video_id = ?
```
to:
```sql
DELETE FROM video_tags WHERE video_id = ? AND source = 'file'
```
followed by reinserting the file-derived set exactly as today (`INSERT OR IGNORE`, now scoped to `source='file'`
rows on insert too). Every existing row defaults to `'file'` with zero backfill ambiguity — the migration
needs no data migration step beyond the `ALTER TABLE ... DEFAULT`, because 100% of rows that exist before this
ships are, definitionally, file-derived. `manual` and `provider:*` rows are simply never touched by the delete,
surviving any number of rescans.

Considered and rejected: a **separate `video_tags_curated` table** for non-file associations. This would
split one logical relationship — "does this video have this tag" — into two tables every existing and future
reader of `video_tags` (browse filters, search, the tag-detail item list, writeback assembly) would need to
know to `UNION` or join twice. A single column is additive: every current `SELECT ... FROM video_tags` reader
keeps working unmodified, because the new column doesn't change what rows exist, only which ones a rescan is
allowed to touch.

### D4 — Materialization hooks into the existing `afterEnrichApply` dispatcher; it is a write, not a pure pass

Materialization runs as one more case in the **existing** shared post-apply dispatcher:

```go
// internal/api/enrich_review.go:259
func (h *Handlers) afterEnrichApply(r *http.Request, entityType string, id int64) {
	switch entityType {
	case model.EnrichEntityVideo:
		h.relinkStudios(r.Context(), id)
		h.materializeTags(r.Context(), id) // new
	case model.EnrichEntityStudio:
		h.relinkStudioLogo(r.Context(), id)
	}
}
```

This function already exists precisely to keep manual apply, Refresh, and Refresh-all in lockstep for the
video-entity side effects it runs (its own doc comment: "Shared so Refresh/Refresh-all stay in lockstep with a
manual apply instead of silently skipping these") — hooking materialization in here for free covers all three
trigger paths rather than requiring the materialization author to find and patch each one.

`materializeTags` reads the video's **resolved** `genres` value — the merge-type union across every
enrichment source contributing to that video via the entity-generic `resolver.ResolveFields`/`NewVideoBaseline`
core ([ADR-052](ADR-052-baseline-source-contract.md)), the same path video-detail responses already use to
compute `genres` for display — **not** the raw per-provider fields map `enrich.Enrich()` just returned for
this one apply call. That distinction matters: a second provider might already be contributing to `genres`,
and reading only the just-applied provider's output would materialize an incomplete set. The exact call
(mirroring however video detail assembly invokes the resolver core today) is an implementation detail to
confirm against the current video-response code path, not a new resolver entry point — no resolver change is
needed, only a new caller of the existing one.

Each resolved `genres` value is attached via `resolveOrCreateByName(ctx, tx, model.EntityTag, value, "")` with
`source='provider:<name>'`, `INSERT OR IGNORE` into `video_tags` — idempotent by construction (re-running
against an already-materialized video inserts nothing new).

**Naming the posture change explicitly:** [ADR-063](ADR-063-derived-computed-fields.md)'s `Derive(resolved, now)`
is the closest existing precedent for "a pass appended after resolve," but it is **pure** — no I/O, no store,
values computed fresh on every read and never persisted. Materialization is not that: it **writes** `video_tags`
rows as a side effect of a resolve-triggered event. This is a genuinely different posture — write-on-resolve,
not compute-on-read — and this ADR is where that distinction is made explicit rather than assumed by analogy
to ADR-063. It is closer in spirit to [ADR-066](ADR-066-enrichment-auto-apply-and-dismissal.md)'s auto-apply
(enrichment causing a durable write without an explicit owner click per instance) than to ADR-063's derive
pass, and should be read that way.

---

## Options Considered

### D1 — hierarchy storage shape

**A — `parent_tag_id` column (chosen).** Pros: minimal schema surface, trivial cycle-guard query, zero
write-time maintenance beyond the guard itself. Cons: descendant queries are `O(depth)` recursive CTEs instead
of O(1) closure-table lookups. Accepted: this codebase's scale doesn't need O(1) here, and the codebase's own
precedent (ADR-063 added two struct fields rather than a registry; F43 resolves aliases at query time rather
than flattening them) favors the simplest structure that's correct.

**B — Closure table (`tag_hierarchy(tag_id, ancestor_id, depth)`).** Pros: O(1) ancestor/descendant reads,
no recursive CTE at query time. Cons: every insert/reparent/merge must maintain a transitive-closure set,
real complexity for a single-owner library's tag count. Rejected as premature — no evidence the recursive-CTE
cost is a real problem at this scale; revisit only if it becomes one.

**C — Materialized path (`tags.path = '/1/4/9/'`).** Pros: ancestor reads are a string prefix match, no CTE.
Cons: every reparent (including merge-driven reparenting, D1) requires rewriting the path of every descendant,
not just the reparented tag's row. Rejected: the write cost is worse than B for the same read win, no upside.

### D2 — deny-list storage shape

**A — Dedicated `denied_tags(term_key, term, created_at)` (chosen).** Pros: exactly the arity the concept
needs (a term, nothing else); trivial to query at the one enforcement point. Cons: a fourth entity-typed-ish
table in the family, though this one isn't actually entity-typed at all (RD1: global, no entity dimension) —
arguably the simplest member of the family, not the most complex. Accepted.

**B — Reuse/generalize `entity_keep_separate`'s `(id_lo, id_hi)` shape.** Pros: zero new tables. Cons: a
denied term isn't a pair of entity ids — there's no second entity to pair it with. Forcing a term into an
`id_lo/id_hi` shape would mean inventing a synthetic id, an awkward hack for a one-column saving (the same
rejection ADR-061 itself already gave for a structurally similar mismatch). Rejected.

**C — Reuse/generalize `enrichment_dismissals`'s `(entity_type, entity_id, provider)` shape.** Pros: zero new
tables; superficially similar ("another negative assertion"). Cons: wrong arity again — a term isn't an
entity+provider pair, and conflating "reject this provider's candidate for this specific entity" (which can
later change — "Try again" clears it, ADR-066 D2) with "this term is never a tag, from any entity or
provider, ever" mixes two different lifecycles into one table. Rejected.

**D — A `denied` boolean column on `tags`.** Pros: no new table at all. Cons: fatal — a denied term must never
become a `tags` row in the first place (that's the entire point), so there's no row to flag by the time
denial would need to take effect. Rejected outright, not just on elegance grounds.

### D3 — `video_tags` provenance shape

**A — `source` column on `video_tags` (chosen).** Pros: additive, every existing reader unaffected, zero
backfill ambiguity (100% of pre-migration rows are correctly `'file'`). Cons: `replaceAssociations` gains one
`AND source = 'file'` clause — trivial. Accepted.

**B — Separate `video_tags_curated` table for non-file associations.** Pros: `replaceAssociations` doesn't
need to change at all (it only ever touched the original table). Cons: splits "does this video have this tag"
across two tables — every reader (browse filter, search, tag-detail item list, writeback assembly, this ADR's
own D1 descendant expansion when joined against `video_tags`) now needs to know to check both, forever.
Rejected: the one-column cost of A is strictly smaller than the permanent join-everywhere cost of B.

### D4 — materialization hook point

**A — New case in `afterEnrichApply` (chosen).** Pros: automatically covers manual apply, Refresh, and
Refresh-all — the exact problem this dispatcher exists to solve. Cons: none identified; this is what the
function is for. Accepted.

**B — A separate call added individually to `enrichVideoApply`, the refresh handler, and the refresh-all
handler.** Pros: none over A. Cons: three call sites to keep in sync, the exact failure mode `afterEnrichApply`
was built to prevent (its own doc comment cites this precedent for the studio-relink side effect). Rejected.

---

## Trade-off Analysis

**Simplicity now vs. read-cost later (D1).** A parent-pointer column is the least this codebase could ship
and still have a real tree; its cost is paid at read time (a recursive CTE per filter/search call) rather than
write time. For a single-owner library the read cost is bounded by tag count, not request volume — this is the
right side of the trade for this deployment shape, and revisiting it (Option B) is cheap later precisely
*because* nothing today depends on closure-table semantics.

**A fourth entity-typed-ish table vs. forcing a mismatched arity (D2).** The two existing durable-negative-assertion
tables are relationship-shaped for a reason — they were built for relationship-shaped questions. Reusing their
schema for a question with no relationship in it would save one migration at the cost of a permanently
awkward synthetic key. The family already has two members; a third with an honestly different shape is more
consistent with the pattern's spirit ("the smallest table that fits the question," which is what D2-B and D2-C
both fail) than contorting an existing one.

**One column vs. two tables, permanently (D3).** This is the clearest-cut trade-off in this ADR: D3-A's cost
is a single `WHERE` clause added once; D3-B's cost is every present and future `video_tags` reader carrying a
"did I also check the curated table" obligation forever. There is no read pattern this spec or a plausible
future one needs that the two-table option serves better.

**Reusing `afterEnrichApply` vs. a new hook (D4).** This isn't really a trade-off — `afterEnrichApply` exists
specifically to prevent the failure mode a new, separate hook would reintroduce. The only reason to list it as
"considered" is completeness; there is no scenario where three independent call sites are the better choice.

---

## Consequences

**What becomes easier**
- Hierarchy, deny-list, provenance, and materialization each land as additive schema (one column, one column,
  one table, one dispatcher case) — no existing table is restructured, no existing reader needs to change to
  keep working.
- `afterEnrichApply` proves out as a real extension point beyond its original two side effects — the next
  video-entity post-apply effect has a home already.
- The deny-list and hierarchy are both entity-generic in spirit (global term blocking, a plain parent pointer)
  even though this spec only wires them to tags — a future `person_tags` join table (spec P2-1) needs zero
  change to either.

**What becomes harder**
- `replaceAssociations` now carries an invariant every future contributor must respect: **a rescan only ever
  touches `source='file'` rows.** A future change to scan behavior that reintroduces an unconditional delete
  would silently reopen the exact data-loss bug D3 fixes — this should be asserted by a named regression test
  (spec P0-1's acceptance criterion), not just documented here.
- `resolveOrCreateByName` now has an entity-type-conditional branch (the D2 deny-list check) where it
  previously had none — a future fifth entity type routed through this function inherits a check it may or
  may not want; the branch must stay explicitly gated on `EntityTag`, not fall through by default.
- Materialization is a write triggered by an enrichment resolve — the first instance of that posture in this
  codebase. A future contributor reasoning by analogy to ADR-063's *pure* derive pass could reasonably assume
  no I/O happens here; this ADR's D4 section exists specifically to correct that assumption before it causes
  a bug.

**What we'll need to revisit**
- **Closure table / materialized path (D1-B/C)** — only if recursive-CTE descendant expansion measurably
  underperforms at real hierarchy depth/breadth; no evidence yet.
- **Cross-entity deny-list** (if person tags, P2-1, ever ship) — `denied_tags` is already entity-agnostic by
  construction (RD1), so this is confirmation, not redesign, when the time comes.
- **Materialization for enrichment fields beyond `genres`** — the D4 hook point (`afterEnrichApply`) and the
  deny-list/alias-canonicalization machinery generalize for free; only a new field-to-materialize mapping
  would be new work.

---

## Action Items

1. [ ] Migration: `tags.parent_tag_id` (D1) + `denied_tags` table (D2) + `video_tags.source` (D3) — one
   migration or three, engineering's call; `.down.sql` for each.
2. [ ] `resolveOrCreateByName`: deny-list pre-check gated on `entityType == model.EntityTag`, returns
   `ErrTagDenied`; each of the three callers (scanner, manual attach, materialization) handles it per D2.
3. [ ] `replaceAssociations`: scope the delete to `source = 'file'`; add the regression test asserting a
   `manual`/`provider:*` row survives a rescan (D3 — this is the P0 fix, should land and be tested
   independently of D1/D2/D4).
4. [ ] `POST /tags/{id}/parent`: cycle-guard via ancestor walk before commit (D1).
5. [ ] Tag-merge endpoint: add the `parent_tag_id` reparenting `UPDATE` to the existing merge transaction (D1).
6. [ ] `afterEnrichApply`: add the `model.EnrichEntityVideo` materialization call; confirm and cite the exact
   existing video-resolved-fields call site it reads `genres` from (D4).
7. [ ] Recursive descendant-expansion query wired into tag-based browse filter and global search (D1).
8. [ ] Genre writeback assembly: union tags (ancestor-expanded) with the deny-list-filtered raw `genres`
   union, per the spec's RD9/P0-10 (this ADR does not change `internal/writeback/tags.go`'s mapping, only
   confirms nothing here needs to).
9. [ ] `/testing-strategy`: D3's rescan-preserves-non-file-tags test is the highest-priority case in this ADR;
   also cover deny-list enforcement at all three call sites, cycle rejection, merge reparenting, and
   materialization idempotency (spec Success Metrics already names these).
10. [ ] `/security-review` before merge: confirm the three new mutation endpoints
    (`videos/{id}/tags`, `tags/{id}/parent`, `owner/tags/denylist`) are `requireOwner`-gated; confirm no new
    externally-influenced input beyond what F43 already validates for tag names.
