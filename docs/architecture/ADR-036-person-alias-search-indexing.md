# ADR-036: Person aliases & merge — an alias is a name→canonical routing rule (search-time and scan-time)

**Status**: Accepted
**Date**: 2026-06-16
**Deciders**: Project owner
**Extends**: ADR-017 (search — global mixed-entity + FTS5), ADR-003 (SQLite + FTS5 + WAL), ADR-030 (owner gate), ADR-018 (scanner change-detection / write path)
**Spec**: [Person Aliases (F23)](../specs/person-aliases.md)

---

## Context

People are auto-created from the person tags embedded in media files, so each person has exactly one
canonical `people.name` (migration 0001, `UNIQUE`). The Person Aliases feature (F23) adds owner-curated
alternate names that must be **findable in global search** — typing any alias should surface the person.

Today search (ADR-017) runs three independent FTS `MATCH` queries — `videos_fts`, `people_fts`,
`tags_fts` — each an **external-content** FTS5 table mirroring one base column, kept in sync by
`ai`/`ad`/`au` triggers (migration 0001). `people_fts` indexes `people.name` only.

A person can have N aliases — a one-to-many relationship. The feature has two faces:

1. **Alternate names** — owner-curated stage names/nicknames that should be **findable in search**.
2. **Merge** — collapsing a duplicate person (the library extracted "J Law" and "Jennifer Lawrence"
   as two people) into one: pick the canonical name, the other becomes an alias, and the duplicate's
   videos move under the canonical person so searching *either* name returns the merged set.

The crux is that **people are auto-created from the person-tags in each file** during scan
(`UpsertVideo` → `replaceAssociations` → `getOrCreateByName`, ADR-018). A merge that only moved
rows would be **silently undone on the next re-scan** — the scanner would re-encounter "J Law" in the
file and re-create the person. So an alias cannot be a mere search string; it must be a
**name→canonical-person routing rule honored at scan time too**.

The first question is how to make aliases searchable while keeping the canonical name authoritative
and re-scan-safe; the second (below) is how merge and scan-time resolution work.

Options considered (search indexing):

1. **Denormalize aliases into a `people` column** (e.g. a newline-joined `aliases` text column) and
   widen `people_fts` to index both `name` and the joined aliases.
2. **Store aliases in their own `person_aliases` table** with its **own external-content FTS mirror**
   (`person_aliases_fts`), and add one more `MATCH` query to search, merging its person hits into the
   existing people results.
3. Store aliases in the F22 `entity_enrichment` shadow layer (reuse the existing table).

## Decision

Take **option 2**: a normalized `person_aliases` table plus a dedicated `person_aliases_fts` external-
content mirror, with `ai`/`ad`/`au` triggers identical in shape to `people_fts`.

```sql
CREATE TABLE person_aliases (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    person_id INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    alias     TEXT    NOT NULL,
    UNIQUE (person_id, alias) ON CONFLICT IGNORE      -- per-person, COLLATE NOCASE
);
CREATE INDEX idx_person_aliases_person ON person_aliases(person_id);
-- alias lookup is on the scan hot path (name→canonical resolution), so index it;
-- COLLATE NOCASE is inherited from the column so the lookup is case-insensitive.
CREATE INDEX idx_person_aliases_alias  ON person_aliases(alias);

CREATE VIRTUAL TABLE person_aliases_fts USING fts5(
    alias, content='person_aliases', content_rowid='id',
    tokenize="unicode61 remove_diacritics 2"
);
-- person_aliases_ai / _ad / _au mirror the people_* triggers.
```

Migration **0007** (next after 0006).

**Search merge.** `Repo.Search` gains one query after the existing people query:

```sql
SELECT DISTINCT p.id, p.name
FROM person_aliases_fts f
JOIN person_aliases a ON a.id = f.rowid
JOIN people p         ON p.id = a.person_id
WHERE person_aliases_fts MATCH ? LIMIT ?
```

Its person hits are appended to `res.People`, **deduplicated by person id** (a person matching both
its canonical name and an alias appears once), and the combined slice is capped to the per-group limit.
Canonical-name matches keep priority (appended first).

**Search returns the matched person's media, not just the chip.** Global search builds people first,
then the video results = **title matches ∪ the media of every matched person** (name or alias),
de-duped by video id and capped at the limit. The person-media half reuses `ListVideos` via a new
OR-semantics `VideoFilter.PersonIDsAny` (the existing `PersonIDs` AND-s, which is wrong for a union).
This is what makes the headline true — *searching either name returns the merged union* — in the
results themselves, not only after clicking through to the person page. (Title-only matches are
unchanged, so searching a word that is only a video title still works.)

**Scan-time resolution (the load-bearing decision).** The people branch of the scanner write path
resolves each extracted name *through the alias table* before creating a person:

1. `people.name = N` (NOCASE)? → use it.
2. else `person_aliases.alias = N` (NOCASE)? → use its `person_id` (the canonical person).
3. else insert a new person `N`.

So a file tagged "J Law" links to "Jennifer Lawrence" once "J Law" is her alias, and **the merge
survives every re-scan**. Only the people branch is alias-aware; the tags branch keeps the plain
`getOrCreateByName` (tag aliases are a future, separate feature).

**Merge** (`MergePersons(canonical, merged)`), in one transaction under the write lock:
1. Move associations — `INSERT OR IGNORE INTO video_people(video_id, person_id) SELECT video_id,
   :canonical FROM video_people WHERE person_id = :merged` then delete the merged person's rows. The
   `OR IGNORE` + composite PK makes it a **de-duped union** (a video credited to both collapses to one).
2. Re-point the merged person's *existing* aliases to canonical (`UPDATE OR IGNORE`, then delete
   leftovers that collided), so a chain of prior merges is preserved.
3. Register the merged person's **name as an alias** of canonical (`INSERT OR IGNORE`).
4. Drop the merged person's shadow enrichment rows (v1 — canonical keeps its own; moving-with-merge
   is a deferred refinement).
5. Delete the merged person row. Its FTS mirror and remaining junction rows clean up via the
   `people_ad` trigger and `ON DELETE CASCADE`.

**Never auto-merge on a name collision.** The same name string can belong to genuinely different
people (two "Chris Evans"; Michael Keaton's legal name is "Michael Douglas"). So adding an alias that
already resolves to a *different* existing person (by name or by another person's alias) does **not**
silently merge or silently route a homonym's files: the add endpoint returns **409 with the
conflicting person's context** (id, name, video count) and the owner decides — merge (they're the
same) or cancel (they're different). Merge itself is always an explicit, informed confirmation
showing both people's video counts. The string-name homonym that two files share with no other
distinguishing metadata is an inherent limitation we surface, not one we can resolve automatically.

**Curation & merge are owner-gated.** Alias add/delete and `POST /people/{id}/merge` mount inside the
existing `requireOwner` group (ADR-030), alongside the F22 enrich endpoints. Reads (the alias list on
the person detail response) are public — aliases are public metadata, like the canonical name.

## Rationale

- **The canonical name stays authoritative and re-scan-safe.** Aliases in their own table mean the
  scanner's write path (`UpsertVideo` → `replaceAssociations`, which clears and rebuilds
  `video_people`) and `people.name` are completely untouched by aliases. A denormalized `people.aliases`
  column would put user-curated data on the same row the scanner upserts — inviting exactly the kind of
  "an unrelated write clobbered the wrong row" class of bug already fixed once in `UpsertVideo`. Option
  2 keeps machine-extracted and human-curated data physically separate.
- **External-content FTS wants one row per indexed string.** FTS5 external-content tables map `rowid`
  to a base-table row. The natural unit for "one alias = one searchable string" is one
  `person_aliases` row, mirrored 1:1 — exactly the `people_fts`/`tags_fts` pattern. Denormalizing N
  aliases into a single joined cell forces the triggers to rebuild the whole person's FTS row on every
  single-alias add/delete and muddies tokenization (the join separator becomes a token boundary
  concern). The normalized mirror reuses the proven trigger shape verbatim.
- **Stable identity for delete.** A `person_aliases.id` gives the UI and `DELETE
  /people/{id}/aliases/{aliasId}` a stable handle, no delete-by-string-in-URL encoding hazard.
- **`DISTINCT` + id-dedup keeps results clean.** Multiple aliases of one person matching the same query
  (or name + alias both matching) must not list the person several times; `SELECT DISTINCT` on the
  query plus an id-set merge in Go guarantees one entry.
- **Not the enrichment shadow layer (option 3).** `entity_enrichment` (ADR-033) is *provider-sourced,
  re-fetchable shadow data* keyed by `(entity, provider, field)` and displayed with "from <provider>"
  provenance. Owner-curated aliases are first-class user data with no provider and a different lifecycle
  (they are authored, not fetched, and are the *search* seam, which enrichment fields are not). Forcing
  them into that table would overload its meaning and its UNIQUE key. Keeping them separate also leaves
  room to *later* promote a provider's `aliases` field into real `person_aliases` rows — a clean future
  bridge, not a merge of two concepts.
- **Consistent tokenizer.** `unicode61 remove_diacritics 2` matches the other FTS tables, so an alias
  "Beyoncé" is found by "beyonce" exactly as names are.

## Consequences

- **One extra FTS query per global search.** Bounded by the per-group `LIMIT` and joined on the
  indexed `person_aliases.id` / `people.id`; negligible at personal-library scale and identical in
  shape to the existing people/tags queries. If search ever fans out to many entity types, the
  per-entity-query pattern is the thing to revisit (noted in ADR-017's scope), not aliases specifically.
- **Search result ordering** within the people group is name-matches-first, then alias-only matches, in
  FTS rowid order. There is no relevance score across the two (ADR-017 doesn't rank today); acceptable
  for v1. Annotating *which* alias matched is a deferred enhancement (spec open question 1) and would
  add a search-only field, not change this schema.
- **Migration 0007 is additive.** New table + index + virtual table + three triggers; the `down`
  drops them in reverse. No change to `people`, `people_fts`, or any existing trigger.
- **Cascade on person delete.** `ON DELETE CASCADE` means aliases never outlive their person. There is
  no hard person-delete feature today; this is correctness insurance for when one lands (and for the
  future person-merge feature, which will decide alias absorption — out of scope here).
- **Scan-time resolution adds one indexed lookup per *new* name.** Step 1 (name hit) is the common
  path and unchanged; the alias lookup (step 2) only runs when a name isn't already a person, backed by
  `idx_person_aliases_alias`. Negligible on a personal library and off the unchanged-file fast path
  (ADR-018 skips re-extraction entirely for unchanged files).
- **Merge is one-way (no automatic un-merge).** Once "J Law"'s videos move under "Jennifer Lawrence"
  and "J Law" becomes an alias, splitting them again is a manual action (delete the alias — which stops
  *future* routing — and re-scan won't retroactively re-split already-merged associations). Acceptable
  for an owner-curated personal library; a dedicated "split" operation is a possible future feature.
- **Homonyms remain fundamentally ambiguous.** Two distinct people whose files use the identical tag
  string are already one person row before any alias feature; merge/alias neither causes nor fixes
  that. The collision prompt prevents *accidental* merges; it cannot separate identical strings.
- **Covered by tests** following the existing repo/handler patterns: alias CRUD + per-person
  case-insensitive uniqueness; search-by-alias (incl. diacritic folding and the name+alias dedup);
  **scan-time resolution routing an extracted alias to the canonical person and the merge surviving a
  re-scan** (the cardinal invariant); merge association-union/de-dup + alias re-pointing + duplicate
  deletion; the collision 409; and the owner-gating (401) + 404/400 paths on the endpoints.
