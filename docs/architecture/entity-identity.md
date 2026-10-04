# Entity identity

This doc owns what makes a Person, Studio, Tag or Film the same entity: the normalized name key,
the alias spine, provider external ids, the `kind:id` reference handle, and how suspected
duplicates are recorded. Links between entities belong to
[entity-relationships.md](entity-relationships.md); how a displayed value is chosen belongs to
[field-resolution.md](field-resolution.md); the provider contract to
[metadata-providers.md](metadata-providers.md).

## A per-entity normalized name key, unique across canonical names and aliases

Name identity is one uniqueness domain per entity type over canonical names and aliases together:
no key is ever held twice. The key is a per-entity normalization, enforced by unique expression
indexes rather than a stored column:

- **Person, studio:** case and edge whitespace (`lower(trim(name))`; `ux_people_namekey`,
  `ux_studios_namekey`).
- **Tag:** case and all spaces (`replace(lower(trim(name)), ' ', '')`; `ux_tags_namekey`). Tag
  names and tag alias text are stored lowercase.
- **Film:** composite with year (`ux_films_namekey` on `(lower(trim(name)), year)`).

Diacritics and punctuation are never folded into identity; they are a search concern and a
near-miss signal. `nameKeyExpr` (`internal/repo/identity.go`) builds the expression and the alias
key is a generated column, so Go never computes a key SQLite could disagree with. Changing a
normalization is an index rebuild and ships as a migration.

**Rejected:** `COLLATE NOCASE` on the canonical columns — fixes case only, with no per-entity
whitespace rule and no shared canonical ∪ alias domain.

Decided in [`28e2540d`](https://github.com/whoiskevinrich/holodex/commit/28e2540d).

## One polymorphic alias store that routes, searches and merges

Aliases for every kind live in `entity_aliases(entity_type, entity_id, alias, alias_key, source)`
with `UNIQUE (entity_type, alias_key)`: an alias key belongs to exactly one entity of its type.
An alias is a name→canonical routing rule, not a search string, so it is read in three places:

- **Resolve.** `resolveOrCreateByName` consults it, so scanning or re-deriving a file that carries
  the alias links the canonical entity.
- **Search.** `entity_aliases_fts` mirrors it by triggers (`unicode61 remove_diacritics 2`, like
  the other FTS tables), filtered by `entity_type` in the query.
- **Merge.** See the next section.

Aliases never live in the `entity_enrichment` shadow store and are not a resolved field. Provider
alternate names (TMDB `also_known_as`, film alternative titles) are written by
`ApplyProviderAliases` straight into `entity_aliases` with `source` set to the provider, and then
behave exactly like owner-typed ones; `source` is provenance, read only for suppression. Deleting a
provider alias writes a per-entity `entity_alias_suppressions` row so re-enrich never re-adds it.
The polymorphic tables have no FK cascade; each canonical table has AFTER DELETE cleanup triggers.

**Rejected:** per-kind alias tables — three tables and three FTS mirrors kept in sync by hand.
**Rejected:** a tombstone row instead of a suppression table — it would hold the globally unique
key hostage against another entity's legitimate claim.

Decided in [`025079d2`](https://github.com/whoiskevinrich/holodex/commit/025079d2).

## Resolve-or-create: provider id, then name key, then alias, then create

Every name-born entity goes through `resolveOrCreateByName(entityType, name, externalID)`, inside
the caller's transaction under `Repo.writeMu`, in a fixed order: (1) external id, when one is in
hand, via `entity_external_ids`, without consulting the name; (2) canonical name key; (3) alias
key; (4) create. The id comes first so a merge or provider link survives a rescan whose spelling
exactly names a different entity (`TestResolvePrecedence_*`). Tags are lowercased and checked against the deny-list
before step 1. Film titles route by year: an alias reaches a film only when the year matches, or
when no year is given and exactly one film has that name key; anything else is queued, never
guessed. The scanner and derivation paths never prompt; a create that only loosely matches an
existing entity is flagged to the review queue (`FlagNearMiss`).

Decided in [`28e2540d`](https://github.com/whoiskevinrich/holodex/commit/28e2540d).

## Merge registers the loser's name as an alias; keep-separate is the durable negative

`MergeEntities` runs in one transaction under the write lock: it moves the loser's links and
external ids to the survivor, re-points its aliases, registers its name as a survivor alias, drops
its enrichment, decisions and curation, and deletes it. The alias is load-bearing: links are
re-derived, so without it the next rescan or relink would resurrect the loser. Merge is one-way.

`entity_keep_separate(entity_type, id_lo, id_hi)` records that two entities are deliberately
distinct. Every duplicate producer honors it, so a dismissed pair is never proposed again. Nothing
ever merges automatically; the same string can name two real people.

Decided in [`6d129b6c`](https://github.com/whoiskevinrich/holodex/commit/6d129b6c).

## Namespaced provider ids in one polymorphic store; the enrichment memo is only a witness

Provider identity is always a namespace-qualified `<namespace>:<id>` (`tmdb:603`,
`imdb:tt1160419`), never a name. A namespace is a shared identity space, so two providers emitting
the same id converge on one entity. Name identity (above) is the separate system for file-authored
and owner-curated input; the two meet only in the resolve order. Enforced today by `sanitizePeople`
(credits without a well-formed, space-free id are refused), `sanitizeStudioExternalIDs`, and
`identityShaped` in front of the store.

`entity_external_ids(entity_type, entity_id, external_id)` with `PRIMARY KEY (entity_type,
external_id)` is the single identity store for person, studio, tag and film: an id owns exactly one
entity of its kind, and an entity may hold ids from several providers. Videos have no row; two files
of one movie legitimately share an id. Writers:

- **Derivation.** The private `attachExternalID` is a silent `INSERT OR IGNORE`, safe because
  step 1 of resolve has already returned any existing owner.
- **Enrich.** `Repo.AttachExternalID` attaches an id to an entity the owner picked. Under `writeMu`
  it re-reads the owner after the insert; if a different entity holds the id it queues a
  `shared-external-id` pair and returns nil. The enrich never fails and never merges.

Studio and credited-person ids reach derivation through the `_studio_external_ids` /
`_person_external_ids` sidecar fields, one `"<external_id> <name>"` per value so resolver
reordering cannot misalign them (the `_` sidecar channel is described in
[metadata-providers.md](metadata-providers.md)).

`entity_enrichment.external_id` remains as the re-enrich memo `MatchExternalID` reads, and the only
one videos have. It is never read as identity, only as evidence (for the shared-id detector) that a
spine write was dropped. Dropping it and re-homing the video memo is open as HOLODEX-457.

**Rejected:** a name fallback for id-less providers — a silent homonym-merge path.
**Rejected:** provider-scoped keys `(provider, external_id)` — one work seen by two providers stays
two entities. **Rejected:** per-kind `*_external_ids` tables — four tables saying one thing.
**Rejected:** a `UNIQUE` memo column — turns a contested id into a failed enrich.

Decided in [`6ecb3c5e`](https://github.com/whoiskevinrich/holodex/commit/6ecb3c5e).

## `kind:id` is the entity reference handle

Every entity payload (person, studio, tag, film, video; detail and list) carries a server-built
`ref` of the form `kind:id` (`film:42`). Every `{id}` route segment and every MCP id argument
accepts a ref wherever it accepts a bare id. `api.ParseRef` in `internal/api/ref.go` is the one
parser for both; a ref whose kind does not match the route is 400, not 404. The SPA never builds a
ref. URLs stay numeric.

**Rejected:** slugs — break on rename or need a redirect table, and turn ambiguous again once two
entities share a name.

Decided in [`28e2540d`](https://github.com/whoiskevinrich/holodex/commit/28e2540d).

## The canonical name column is identity; the display name is a decision

For Person, Studio and Film, `name` is a resolved field (see
[field-resolution.md](field-resolution.md)) whose sources are the canonical column, each provider's
spelling (`<provider>:title` for films) and custom. No decision writes the canonical column, which
alone feeds writeback, identity, alias routing and MCP; only detail headings and search rows
(`display_name` beside `name`) read the resolved value. Tag is excluded: tags carry the identity
spine but not the field-resolution model.

**Rejected:** a `display_name` column — a third name with its own writeback, search and alias
questions.

Decided in [`28e2540d`](https://github.com/whoiskevinrich/holodex/commit/28e2540d).

## Suspected duplicates go to one review queue, never to a merge

`identity_review_queue(entity_type, id_lo, id_hi, variation, …)` holds every suspected pair; every
producer honors `entity_keep_separate` and none ever merges. Producers, by `variation`:

- **Name near-miss** (`internal-whitespace`, `punctuation`, …) — a loose key (lowercase, strip
  whitespace and punctuation) over canonical ∪ aliases. `SeedIdentityReviewQueue` runs at boot as an
  observable job; `FlagNearMiss` adds pairs on create.
- **`same-title`** — films sharing a title under different years (`queueFilmSameTitle`).
- **`shared-external-id`** — two entities of one kind claiming one provider id. The claimant set per
  `(entity_type, external_id)` is the spine owner ∪ every entity whose newest non-empty memo for
  that provider carries it, paired in every combination (a memo⇔spine join misses memo⇔memo pairs).
  Person, studio and film only. Written by the `AttachExternalID` guard and by
  `SweepSharedExternalIDs`, an idempotent boot sweep with its own job kind; it upgrades an existing
  row for the pair in place.
- **`provider-alias`** — a provider alias another entity already holds. Enrich skips the name and
  writes the row as the record of the skip (`SkippedAliasesForEntity` reads it), but
  `ListReviewPairs` does not return it. Any other reader of the table must filter it too.

**Rejected:** auto-merging on a shared id — irreversible, and the colliding data comes from an
owner's mis-pick. **Rejected:** moving the skip record to its own table — a migration and two
repointed readers for no visible difference.

Decided in [`0f534581`](https://github.com/whoiskevinrich/holodex/commit/0f534581).
