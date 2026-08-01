# ADR-078: Tag Categories — a deliberately reduced entity, its junction shape, and cross-table name collision

**Status:** Proposed
**Date:** 2026-07-31
**Deciders:** Project owner

**Extends:** [ADR-061](ADR-061-unified-entity-name-identity.md) (the `nameKey` fold, `resolveOrCreateByName`,
`identityQueryByType` — this ADR deliberately keeps Category *outside* that spine; D4 explains why) ·
[ADR-075](ADR-075-tag-governance-and-video-enrichment.md) (D1's "application-layer check because SQLite can't
express it" posture is the comparison point D3 argues *against* — a cross-table name collision **can** be
expressed declaratively, unlike a cycle, and this ADR chooses to do so). **Relates to:** the `video_tags`
junction shape (`internal/db/migrations/0001_init.up.sql`, provenance added in
[ADR-075](ADR-075-tag-governance-and-video-enrichment.md) D3) as the direct precedent `category_tags` mirrors.
**Spec:** [tag-categories.md](../specs/tag-categories.md) (epic
[HOLODEX-240](https://whoiskevinrich.atlassian.net/browse/HOLODEX-240)). **Design:**
[tag-categories-handoff.md](../design/tag-categories-handoff.md) (the `nameKeyExpr`-reuse and facet-expansion
calls made at the design layer, formalized here).

---

## Context

The spec (`tag-categories.md`) asks for a new grouping layer over existing tags — hand-curated, CRUD-only, no
provenance, no alias/merge machinery — that must nonetheless share a name-collision domain with tags ("a
category can't share a name with an existing tag, or vice versa," P0). That single requirement is the crux of
this ADR: every other piece (the entity table, the junction table, the browse-facet expansion) is a direct,
low-risk mirror of an existing pattern in this codebase, but **cross-table name uniqueness has no existing
precedent** — the entity-identity system (ADR-061) deliberately scopes `nameKey` uniqueness *per entity type*,
so a Person, a Studio, and a Tag can today all be named "Fox" simultaneously with zero conflict. This ADR has to
decide, for the first time, how two *different* tables enforce that neither can claim a name the other holds.

### Current state (survey, 2026-07-31)

| Seam | Today | File |
|---|---|---|
| `tags` | `id, name, parent_tag_id` (F50 hierarchy); `name` has both a legacy inline `UNIQUE` and an expression `ux_tags_namekey` index | `internal/db/migrations/0001_init.up.sql:28`, `0022_entity_name_identity.up.sql:167`, `0032_tag_hierarchy.up.sql:12` |
| `nameKeyExpr(entityType, col)` | Go function generating a SQL fold expression — tag variant strips internal spaces too (`replace(lower(trim(x)), ' ', '')`), person/studio fold only case + edge whitespace | `internal/repo/identity.go:47-52` |
| `resolveOrCreateByName` | Entity-generic (`person`/`studio`/`tag`) id→nameKey→create spine — every tag-creation path already routes through it; **checks name-collision only within the entity's own table**, never cross-type | `internal/repo/identity.go:80-146` |
| `EntityType` values | Untyped string constants, no dedicated Go type — `EnrichEntityPerson`, `EnrichEntityStudio`, `EntityTag` (plus `EnrichEntityVideo`, outside the identity set) | `internal/model/model.go:249-254` |
| Cross-entity-type uniqueness | **Does not exist.** ADR-061 states `nameKey` is "unique per entity type... one uniqueness domain, not two" — explicitly per type, not global | `docs/architecture/ADR-061-unified-entity-name-identity.md:88-89` |
| `video_tags` junction | Composite PK `(video_id, tag_id)`, both FKs `ON DELETE CASCADE`, one reverse index on `tag_id` | `internal/db/migrations/0001_init.up.sql:40-45` |
| Browse tag-ID filtering | `VideoFilter.build()`'s `TagIDs` loop: one `EXISTS (... vt.tag_id IN (<subtree>))` clause per selected id, ANDed across selections | `internal/repo/repo.go:417-423` |
| Latest migration | `0032_tag_hierarchy` | `internal/db/migrations/` |

### Forces

- **The collision requirement is the one genuinely new pattern here.** Everything else in the spec (a small
  entity table, a many-to-many junction, a facet that expands to member ids) has a direct, already-shipped
  analog in this codebase. Cross-table name uniqueness does not — this ADR exists almost entirely to decide
  that one thing well.
- **A cross-table invariant, unlike a cycle (ADR-075 D1), is expressible in SQLite.** ADR-075 chose an
  application-layer cycle guard specifically because "SQLite has no native constraint expressive enough" for a
  graph-cycle check. A same-value-in-two-tables check has no such excuse — a `BEFORE INSERT`/`UPDATE` trigger
  can query the other table directly. Reaching for an app-layer-only check here would be reproducing ADR-075's
  posture in a case where the actual justification for it doesn't hold.
- **Category must not inherit alias/merge/near-miss/deny-list machinery it doesn't want.** The spec is explicit
  (Non-Goals): "a small, hand-curated list doesn't have the scanner-driven-duplicate problem that
  [ADR-061's] machinery solves for." Wiring Category into `resolveOrCreateByName`'s `identityQueryByType` would
  pull in `entity_aliases`, near-miss flagging, and merge eligibility for free — none of which the spec wants,
  and all of which would need to be explicitly suppressed per call site if adopted wholesale.
- **The fold used for the cross-table comparison has to be the *same* fold on both sides, or the comparison is
  meaningless.** Category names are being compared against tag names specifically — if Category used the
  person/studio-style fold (case + edge whitespace only) while Tag uses its own space-stripping fold, "Sci Fi"
  (folds to `sci fi` under the generic fold, `scifi` under the tag fold) could pass Category's uniqueness check
  against a tag actually named "SciFi." This was confirmed with the owner at the design layer (handoff §
  Resolved decisions: reuse `nameKeyExpr`'s tag variant) and is formalized here as a correctness requirement,
  not a style preference.

---

## Decision

### D1 — `categories` table: minimal, no identity-spine membership, tag-style fold for its own uniqueness

```sql
CREATE TABLE categories (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL
);
CREATE UNIQUE INDEX ux_categories_namekey ON categories (replace(lower(trim(name)), ' ', ''));
```

Two columns, matching the spec's explicit "no provenance/source, no alias system" framing — this mirrors
`tags`' own original shape (`0001_init.up.sql:28-31`) before F43/F50 added identity and hierarchy columns
neither of which apply here. The unique index reuses the **tag** variant of the fold
(`replace(lower(trim(name)), ' ', '')`, not the person/studio variant) — deliberately, per the Forces section
above: it must match byte-for-byte the fold tags already use, since D3's cross-table check compares against it
directly.

**Category is not added to `resolveOrCreateByName`'s `identityQueryByType`/`canonicalTable` set.** Create,
rename, and delete get three small, dedicated functions in a new `internal/repo/categories.go` — not routed
through the entity-generic identity spine. This is the direct consequence of the third Forces bullet: the
identity spine's value is amortizing alias/merge/near-miss machinery across three entity types that all need
it; Category needs none of it, so joining the spine would mean either (a) silently gaining behavior the spec
explicitly excludes, or (b) threading a fourth `if entityType == model.EntityCategory { skip everything }`
branch through every one of the spine's functions. Three small standalone functions are less code and clearer
intent than either.

### D2 — `category_tags` junction: mirrors `video_tags` exactly, no provenance column

```sql
CREATE TABLE category_tags (
    category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    tag_id      INTEGER NOT NULL REFERENCES tags(id)       ON DELETE CASCADE,
    PRIMARY KEY (category_id, tag_id)
);
CREATE INDEX idx_category_tags_tag ON category_tags(tag_id);
```

Identical shape to `video_tags` (`0001_init.up.sql:40-45`): composite PK, both sides `ON DELETE CASCADE`, one
reverse index on the non-leading PK column. `ON DELETE CASCADE` on `category_id` is what gives the spec's
cascade-delete requirement ("deleting a category unassigns it from every tag... no dependent-tag block") for
free, at the DB layer, with zero application cleanup code — exactly how deleting a `tags` row today silently
clears its `video_tags` rows (`0001_init.up.sql:42`) with no explicit code doing so. `ON DELETE CASCADE` on
`tag_id` handles the inverse (a tag deleted elsewhere drops its category memberships), which the spec doesn't
call out explicitly but is the only non-orphaning choice consistent with this table's own cascade on the other
side. No `source`/provenance column: unlike `video_tags` (ADR-075 D3, which added `source` because file-scan
and manual/provider tagging needed to coexist without one clobbering the other on rescan), every `category_tags`
row is created exactly one way — an owner's explicit assign action — so there is no "which write path owns this
row" question to answer.

### D3 — Cross-table name collision: DB-enforced via paired `BEFORE INSERT`/`UPDATE` triggers, not an app-layer-only check

```sql
CREATE TRIGGER trg_tags_no_category_collision
BEFORE INSERT ON tags
FOR EACH ROW WHEN EXISTS (
    SELECT 1 FROM categories
    WHERE replace(lower(trim(name)), ' ', '') = replace(lower(trim(NEW.name)), ' ', '')
)
BEGIN
    SELECT RAISE(ABORT, 'name collides with an existing category');
END;

CREATE TRIGGER trg_categories_no_tag_collision
BEFORE INSERT ON categories
FOR EACH ROW WHEN EXISTS (
    SELECT 1 FROM tags
    WHERE replace(lower(trim(name)), ' ', '') = replace(lower(trim(NEW.name)), ' ', '')
)
BEGIN
    SELECT RAISE(ABORT, 'name collides with an existing tag');
END;
```

(Plus `UPDATE OF name` variants of both, for tag rename and category rename — same `WHEN` clause, matched
against `NEW.name`.)

**Chosen over an application-layer-only check** (e.g. adding a `nameCollidesWithOtherNamespace` call inside
`resolveOrCreateByName`'s tag path and the new category-create/rename functions) for the reason named directly
in the Forces section: this codebase already has a working precedent for *same-table* name uniqueness enforced
declaratively (`ux_tags_namekey`, `ux_people_namekey`, `ux_studios_namekey` — all real unique indexes, not app
checks), specifically because a unique index catches every insert path, present and future, without relying on
each caller remembering to call a helper. A cross-table version of the identical requirement deserves the
identical posture. An app-layer-only check would be exactly the failure class those indexes already exist to
avoid: one future insert path (a bulk-import script, a fixed-up test helper, a later endpoint) that forgets to
call the helper silently reintroduces the collision the spec explicitly forbids. Triggers close that gap at the
same layer the existing unique indexes do, for a case unique indexes alone can't reach (indexes are
single-table).

Application code still performs the same check **pre-flight**, before attempting the insert/update, purely for
UX: the API handler queries the other table first and returns a clean `409` with the spec's "clear error"
copy, rather than surfacing a raw SQLite `RAISE(ABORT, ...)` message to the owner. The trigger is the
correctness backstop; the pre-flight check is the friendly error path. This two-layer shape (app-level
pre-check for a good error message, DB-level constraint as the actual guarantee) is exactly what
`ux_tags_namekey` already implies for same-table renames — `RenameEntity`'s own conflict `SELECT` before its
`UPDATE` (`internal/repo/identity_ops.go:410-417`) is the same pattern, one layer over.

### D4 — Category stays outside the `EntityType` identity-generic set; no new `model.EntityCategory` identity-spine membership

Only a plain constant is needed for whatever small surfaces do need to name the type (API route grouping, the
pre-flight check's error copy) — not a fourth member of `resolveOrCreateByName`'s `identityQueryByType`/
`canonicalTable` maps. This is the same call as D1, restated as its own decision because it's easy to
default into "every entity gets an `EntityType`" by analogy to Person/Studio/Tag without re-deriving why those
three share a spine (scanner-driven duplicates, near-miss review, merge) that Category's hand-curated,
CRUD-only lifecycle never triggers.

---

## Options Considered

### D1 — where Category's CRUD lives

**A — Dedicated `internal/repo/categories.go`, outside the identity spine (chosen).** Pros: no risk of
inheriting alias/merge/near-miss/deny-list behavior the spec excludes; ~3 small functions, each doing exactly
one thing. Cons: three tiny functions instead of reusing one entity-generic one; the tag-style fold is
duplicated a 5th time (Go: `nameKeyExpr`; SQL: three existing `ux_*_namekey` indexes; now a 4th index plus
this ADR's triggers) — consistent with this codebase's existing practice of hand-duplicating the identical
fold expression per site (research confirmed no shared SQL macro exists today either) rather than a regression
this ADR introduces.

**B — Fourth member of `identityQueryByType`, with alias/merge/near-miss explicitly disabled per call site.**
Pros: one code path, not three. Cons: every future maintainer reading `resolveOrCreateByName` now has to
understand a fourth entity type's worth of "except when it's a category, then skip this" branches scattered
through alias lookup, near-miss flagging, and the deny-list pre-check — more cognitive surface than three
standalone functions, for a spine whose entire value proposition (amortizing shared machinery) doesn't apply
to Category. Rejected.

### D3 — cross-table collision enforcement

**A — Paired DB triggers, app-layer pre-check for UX (chosen).** Pros: correctness holds regardless of which
code path performs the insert, matching this codebase's existing posture for same-table uniqueness; the
friendly `409` still comes from the pre-check in the common (single-owner-UI) path. Cons: four triggers
(insert × 2 tables, update × 2 tables) rather than one function; SQLite trigger errors are marginally more
awkward to unit-test than a Go error path. Accepted — the correctness guarantee is worth the small test-setup
cost, and this is a one-time schema addition, not ongoing maintenance.

**B — Application-layer-only check (a shared `nameCollidesWithOtherNamespace` helper called from both
`resolveOrCreateByName`'s tag path and the new category functions).** Pros: simpler to test (a plain Go
function); no new trigger syntax in the migration. Cons: exactly the gap D3's Forces bullet names — a future
insert path that doesn't call the helper silently violates the invariant, with no error, ever. Rejected: this
is the same trade-off `ux_tags_namekey` et al. already resolved in favor of DB enforcement for the same-table
case; nothing about the cross-table case changes the calculus, and unlike ADR-075's cycle guard, SQLite can
actually express this one.

**C — A single shared `names` table (`entity_type, name`) that both `tags` and `categories` insert into
alongside their own row, with one unique index on `name` doing all the work.** Pros: one index, not four
triggers. Cons: a real schema restructure of the existing, heavily-used `tags` table (every tag-creation path,
every rename, the FTS triggers) to introduce a parallel table that must stay in lockstep with it — solving a
one-off two-table collision with a permanent synchronization obligation. Rejected: same shape as ADR-075 D3's
own rejection of splitting `video_tags` into two tables — the cost of keeping two things in sync forever
outweighs the cost of the smaller, additive fix.

---

## Trade-off Analysis

**Declarative enforcement now vs. four triggers' worth of migration SQL (D3).** The alternative — trusting
every present and future insert path to remember an app-level check — is a bet this codebase has already
declined to make for the *easier* same-table version of this exact problem (`ux_tags_namekey` is a real index,
not a convention). Four triggers is more SQL than one helper function, but it's schema, not behavior — it
doesn't need to be remembered, tested per call site, or kept in sync as new callers are added.

**Duplicating the fold expression a fifth time vs. introducing a shared SQL macro (D1).** This ADR does not
introduce a shared fold macro, even though four now-near-identical expressions exist (three `ux_*_namekey`
indexes plus this ADR's `ux_categories_namekey` and both trigger pairs). SQLite has no macro/function
mechanism that would meaningfully reduce this beyond what already exists — each `CREATE UNIQUE INDEX` and
`CREATE TRIGGER` must independently spell out its expression regardless. Introducing a Go-side shared constant
that migrations copy from wouldn't reduce the SQL duplication either, since migrations are static SQL files,
not templated. Accepted as consistent with the codebase's existing choice (not a new inconsistency this ADR
creates).

**Keeping Category outside the identity spine vs. the spine's amortized value (D1/D4).** The identity spine
(ADR-061) exists to solve one problem well across three entity types that all have it: scanner-driven duplicate
creation needing near-miss review and merge. Category never goes through the scanner and is created exactly
one way (an owner's explicit action) — there's no duplicate-creation problem for the spine to amortize away.
Joining it would import a solution to a problem Category doesn't have, at the cost of every spine function
needing to reason about a type that behaves differently from the other three.

---

## Consequences

**What becomes easier**
- Category delete is a single `DELETE FROM categories WHERE id = ?` — the cascade to `category_tags` (and
  therefore membership loss on every affected tag) is a DB-layer consequence, not application code, exactly
  matching how tag delete already clears `video_tags` today.
- The browse-page category facet needs no new filtering primitive: expanding a selected category to its member
  tag ids is `SELECT tag_id FROM category_tags WHERE category_id = ?`, feeding straight into the existing
  `EXISTS (... vt.tag_id IN (...))` clause shape `VideoFilter.build()` already uses for `TagIDs`
  (`internal/repo/repo.go:417-423`) — one more clause in the same loop, no new query shape.
- The cross-table collision guarantee holds even against paths this ADR didn't anticipate (a future bulk-import
  tool, a fixed-up test seed, a differently-shaped admin endpoint) because it's enforced by the schema, not by
  which code happened to remember to check.

**What becomes harder**
- Four new triggers is more migration surface than a single helper function would have been, and SQLite trigger
  failures surface as a raw driver error if a caller ever bypasses the app-layer pre-check (e.g. a direct SQL
  script) — acceptable because the pre-check is the actual user-facing path and the trigger is explicitly a
  backstop, not the primary UX.
- A future contributor adding a fifth "named" entity type (unlikely, but the identity spine has grown by one
  member roughly once a quarter) needs to re-derive, not copy, whether it belongs in the identity spine or
  should stay standalone like Category — this ADR's D1/D4 reasoning is the citation for that judgment call, not
  a rule that generalizes mechanically.

**What we'll need to revisit**
- **If Category ever needs alias/merge behavior** (e.g. "rename and keep the old name searchable") — not
  requested, explicitly a non-goal today, but if it ever is, joining the identity spine at that point is the
  right move, and D1/D4 would need to be revisited, not just extended.
- **If a fourth "named" table ever needs to join this same collision domain** — the trigger-pair approach
  becomes O(n²) trigger pairs for n tables; at that point the shared-`names`-table option (D3-C, rejected here
  for n=2) becomes the better trade and should be reconsidered.

---

## Action Items

1. [x] Migration `0034_categories.{up,down}.sql`: `categories` table + `ux_categories_namekey` (D1),
   `category_tags` junction (D2), the four collision triggers (D3) — engineering's call whether this is one
   migration or split further; `.down.sql` drops triggers before tables (SQLite trigger-then-table drop order).
2. [x] `internal/repo/categories.go`: `CreateCategory`, `RenameCategory`, `DeleteCategory` — each performs the
   app-layer pre-flight collision check against the other table before its own trigger-backed insert/update
   (D3), returning a typed error the API layer maps to `409` with the spec's required copy.
3. [x] `resolveOrCreateByName`'s tag-creation path (`internal/repo/identity.go`) gains the same pre-flight
   check against `categories` before insert — the tag side of D3's symmetry; every existing tag-creation
   caller (scanner, manual attach, materialization) inherits it for free via the single choke point, matching
   this codebase's established "one choke point, not N call sites" preference (cf. ADR-075 D2).
4. [x] Category CRUD endpoints (`POST/PATCH/DELETE /owner/categories`), owner-gated, per the spec's P0
   checklist.
5. [x] `category_tags` assign/unassign endpoints backing the bulk "Add to category…"/"Remove from category…"
   actions and the `/categories/{id}` member-tag chips (spec P0), both simple junction inserts/deletes — no
   new query shape beyond D2.
6. [x] `VideoFilter` gains a `CategoryIDs` field; `VideoFilter.build()` adds one `EXISTS (...)` clause per
   selected category id following the exact shape described in D2/Consequences, ANDed with existing clauses
   the same way `TagIDs`/`StudioIDs` already are.
7. [ ] `/testing-strategy`: cover the cross-table collision at all four trigger paths (tag-blocks-category,
   category-blocks-tag, both directions × insert/rename), cascade-delete (category delete drops
   `category_tags` rows, tag delete drops `category_tags` rows referencing it), and the facet-expansion query
   against a category with 0/1/many member tags.
8. [ ] `/security-review` before merge: confirm the new mutation endpoints are `requireOwner`-gated and that
   no new externally-influenced input reaches the trigger SQL beyond what the existing tag-name validation
   (`model.MaxNameLen`, ADR-075 item 11) already covers — category names should inherit the same cap.
