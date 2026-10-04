# Entity relationships

This doc owns how entities link to videos and to each other: derived person and studio links, the
tag join with its hierarchy, deny-list and categories, and films as owner-asserted groupings of
videos with their field vocabulary. What makes two rows the same entity belongs to
[entity-identity.md](entity-identity.md); how a resolved value is chosen, including the sole-linked-film
default, to [field-resolution.md](field-resolution.md); how a value reaches the file to
[writeback.md](writeback.md); image storage to [images.md](images.md).

## Person and studio links are a derived index over resolved entity-typed fields

`video_people` and `video_studios` are never authored. Each is rebuilt from the video's *resolved*
fields, so grouping, navigation and the media detail always read one resolution.

- **The marker.** `registry.FieldDef.EntityKind` (`person` or `studio`) declares which video fields
  name linkable entities; `Role` names the `video_people.role` a person-typed field links as. The
  relink, the extractor's person-key list and the link picker's target set all read the marker; no
  field name is hardcoded outside `internal/registry`.
- **One writer per table.** `Handlers.RelinkVideoEntity` (`internal/api/person_links.go`) loads the
  resolve inputs once and calls `repo.ReconcileVideoStudios` and `repo.ReconcileVideoPeople`, each a
  full replace (insert desired∖current, delete current∖desired) in one transaction under `writeMu`.
  Names go through `resolveOrCreateByName`, so aliases and provider ids route links.
- **Role is derived, not authored.** `video_people` has `PRIMARY KEY (video_id, person_id, role)`, so
  one person can hold two roles on a video. A role-less person field stores the `''` sentinel, not
  `NULL`, because SQLite treats `NULL`s as distinct in a composite key.
- **Triggers.** Every path that can move a resolved entity-typed value, and only those: scan (the
  scanner's `SetRelinker`), enrich apply and refresh (`afterEnrichApply`), decision set/clear and
  curation add/suppress/clear. Relink runs in its own transaction after the triggering write commits,
  never at read time; a lag from a crash between the two self-heals on the next trigger.
- **A link is a curation add.** The owner links a person or studio to a video by curating the field,
  and the resolved value flows to the file through the ordinary writeback map (`actors → Artist`,
  `studio → Publisher`). A studio entity has no file, but its value lives in the video's `Publisher`
  tag, so studios get the same link-and-write treatment as people.
- **Pruning differs by kind.** `ReconcileVideoStudios` deletes a studio left with no link in the same
  transaction. Studios now carry authored aliases, ids, images and decisions, so this may lose data;
  it is under review as HOLODEX-535. `ReconcileVideoPeople` never deletes; it stamps `people.orphaned_at` (cleared when a
  link returns), and `SweepOrphanedPeople` (`internal/personorphan`, an observable job) deletes people
  orphaned longer than `GraceDays` (default 30) unless `personHasAuthoredIdentity` finds an alias
  (covering merge history), a person image, a decision or a curation row.

**Rejected:** links from raw file extraction at scan time — an adopted or curated value would display
one entity and group under another until the next rescan. **Rejected:** a separate manual-link table —
every reader would union two tables. **Rejected:** immediate prune for people — a file pulled offline
for maintenance would destroy curated identity.

Decided in [`9ccbdc2f`](https://github.com/whoiskevinrich/holodex/commit/9ccbdc2f).

## A people curation edit relinks inside the curation lock

A people curation edit builds its link set from the current `video_people` rows plus the edited value
instead of re-resolving. Since the reconcile is a full replace, `Repo.SetCurationChecked(…, check,
commit)` runs the collision `check`, the curation write and `commit` under one `writeMu` hold, and
`commit` calls `ReconcileVideoPeopleLocked` (the lock-free core; the `Locked` suffix and doc comment are
the only enforcement). A relink failure is logged, never returned; the two writes are serialized but
are not one SQL transaction.

**Rejected:** re-resolving from source inside `check` — pays the four-query resolve on every edit to
close a narrow race.

Decided in [`44486cf6`](https://github.com/whoiskevinrich/holodex/commit/44486cf6).

## Tag links carry provenance; a rescan replaces only file rows

Tags stay on the scan-time pattern, not the resolved-field derivation above. `video_tags.source`
(`file`, `manual` or `provider:<name>`, default `file`) borrows the `fieldsource` vocabulary but is
not validated as a decision source; tags are outside the field-resolution model.
`replaceAssociations` deletes and reinserts only `source = 'file'` rows, so manual and provider tags
survive every rescan (`TestUpsertPreservesNonFileTags`).

Provider tags are materialized as rows. `afterEnrichApply`, the post-apply dispatcher shared by
manual apply, refresh and refresh-all, calls `materializeTags`, which attaches each value of the
video's *resolved* `genres` (the union across providers) via `AttachMaterializedTags`: a write, unlike
the pure computed-field pass. Genre writeback (`GenreWritebackValues`) is the ancestor-expanded tags
unioned with the deny-filtered resolved `genres`, overriding any client-sent value.

**Rejected:** a separate curated-tags table — every `video_tags` reader would need a second join.

Decided in [`f4ec870e`](https://github.com/whoiskevinrich/holodex/commit/f4ec870e).

## Tag hierarchy is a parent pointer; the deny-list is its own table checked in the resolve spine

- **Hierarchy.** `tags.parent_tag_id` (nullable self-reference, `ON DELETE SET NULL`) is a strict
  tree. `SetTagParent` refuses self-parenting and any descendant (`ErrTagCycle`) by walking the
  subtree in the application, since SQLite cannot express the constraint. Descendants are expanded at
  query time with a `WITH RECURSIVE` CTE (`tagSubtreeQuery`), which tag browse filters use. Nothing is
  cached. Merge repoints the loser's children to the survivor through the generic `idMoves` step.
- **Deny-list.** `denied_tags(term_key, term, created_at)` blocks a bare term with no entity
  dimension. `resolveOrCreateByName` checks it (and the `model.MaxNameLen` cap) for tags only,
  returning `ErrTagDenied` / `ErrTagNameTooLong`. Every tag-creating path (scanner, manual attach,
  materialization) inherits the check and translates the sentinel itself. Denial is forward-only.

**Rejected:** a closure table or materialized path — write upkeep on every reparent and merge for a
read win this scale does not need. **Rejected:** a `denied` column on `tags` — a denied term must never
become a row.

Decided in [`f4ec870e`](https://github.com/whoiskevinrich/holodex/commit/f4ec870e).

## Categories group tags outside the identity spine, with a DB-enforced name collision

`categories(id, name)` carries `ux_categories_namekey` on the tag fold
(`replace(lower(trim(name)), ' ', '')`), and `category_tags(category_id, tag_id)` mirrors
`video_tags`: composite key, both sides `ON DELETE CASCADE`, no provenance, since the owner is the
only writer. Category CRUD lives in `internal/repo/categories.go`, not in `resolveOrCreateByName`, so
categories get no aliases, merge, near-miss or deny-list. A tag and a category may never share a
folded name: paired `BEFORE INSERT` / `UPDATE OF name` triggers on both tables
(`trg_tags_no_category_collision` and siblings) are the guarantee, and the app checks first only to
return a clean `409`. A browse category filter expands to member tag ids (`VideoFilter.CategoryIDs`).

**Rejected:** an app-only check — a future insert path that skips it breaks the invariant silently.
**Rejected:** a shared `names` table — a permanent sync obligation for a two-table rule.

Decided in [`9e586abf`](https://github.com/whoiskevinrich/holodex/commit/9e586abf).

## Film membership is an asserted link with no reconciler

A film groups videos by owner assertion. `film_videos(film_id, video_id, scene_number, is_full_film,
created_at)` has `PRIMARY KEY (film_id, video_id)` and `UNIQUE (film_id, scene_number)`; the
uniqueness relies on `NULL` being distinct, so unnumbered scenes coexist (the opposite of the
`video_people.role` sentinel). Its only writers are the owner-gated `AttachFilmVideo`,
`BulkAttachFilmVideos`, scene renumber and detach in `internal/repo/films.go`. There is no relink
function for films, so scan, enrich, decision and curation paths cannot touch the table, and nothing
prunes it. A video may belong to several films.

With `films_enabled` off, film routes, `films_fts` and the MCP film tools are not registered and no
film candidates are injected; migrations still run and decisions naming a film are kept, so
re-enabling restores the same resolution.

Decided in [`1d83511b`](https://github.com/whoiskevinrich/holodex/commit/1d83511b).

## A linked film reaches its videos only as a synthetic resolver namespace

The caller assembling a video's resolver inputs, not `internal/resolver`, injects each attached film
as enrichment-shaped candidates under namespace `film:<id>`: `collection` for every attachment, and
`title` only for an `is_full_film` link. The value is the film's decided display spelling
(`repo.AttachFilmDisplayNames`). Injection happens on the media-detail read and on list tiles
(`applyBrowseTitles`, one batched `FilmsForVideos` lookup per page). Search, title sort, MCP and the
film's scene list read the file title, because full-film videos are hidden from those surfaces when
films are on.

A decision pins it as `provider:film:<id>`. `resolveDecided` and `gather` match the reserved `film:`
prefix exactly and read the canonical key directly, because a film namespace is never declared in the
mapping. When films are off, nothing is injected and the decision resolves empty through the existing
unmatched-provider path, rather than falling back to the file. How a sole film wins an undecided field
is in [field-resolution.md](field-resolution.md).

**Rejected:** films as a pseudo-provider in `metadata-sources.yaml` — runtime, unbounded rows have no
config line. **Rejected:** the film writing `Album`/`Title` directly — a video in two films would
silently overwrite instead of surfacing two candidates.

Decided in [`46e1d7a4`](https://github.com/whoiskevinrich/holodex/commit/46e1d7a4).

## A film's cast, tags and studios are read-time unions; nothing on a film writes to its videos

A film has no cast, tag or studio column. `FilmStudios` and the film cast and tag reads are live
`SELECT DISTINCT` unions over `film_videos` joined to `video_people`, `video_tags` and
`video_studios`. Provider cast for a film is never stored: the billed-but-absent set is the
enrichment shadow's `actors` minus the scene union, computed per request (`filmBilledCast`), so
clearing the provider empties it. Nothing in film enrichment writes `video_people`, decisions or
writeback jobs on attached videos, and no Person row is created for a billed performer.
`film_people_roles(film_id, person_id, role, billing_order)` is owner-asserted only
(`AddFilmPersonRole` and siblings).

**Rejected:** copying billed cast into `film_people_roles` — forces a Person row per billed name, which
fills `/people` with entities that have no footage.

Decided in [`0a32445a`](https://github.com/whoiskevinrich/holodex/commit/0a32445a).

## The film studio cascade is N independent per-video decisions and one writeback batch

The one film-to-video write path is `POST /films/{id}/studio/cascade` (`cascadeFilmStudio`). For each
`VideoIDsForFilm` video it calls `decideStudioForVideo`, the same helper the single-video studio
decision uses (collision gate, `SetDecisionChecked`, relink), always with the gate on. Each video's
decision commits on its own; a collision or error excludes only that video. The decided names of
every successful video go into one `writequeue.EnqueueMany` batch, polled and reverted through the
existing batch-status endpoints. It is film-scoped on purpose; there is no generic bulk-decide
primitive.

**Rejected:** abort on first failure — earlier videos have already committed.

Decided in [`3e481ab0`](https://github.com/whoiskevinrich/holodex/commit/3e481ab0).

## Films enrich as their own entity type; the year is an identity column filled from `release_date`

- **Entity type.** Film enrichment uses `entity_type: "film"` (`model.EnrichEntityFilm`), never
  `video`, because a film resolves with zero, one or many attached videos. The TMDB sidecar remaps
  its movie payload by entity type (`overview` → `description`, poster and backdrop as assets).
- **Vocabulary.** `description` and `release_date` are resolved film scalars. Poster and banner are
  `film_images` assets (`model.FilmImagePoster`, `FilmImageBanner`); see [images.md](images.md).
  Other provider keys auto-register inert.
- **Year.** `films.year` is half the film's identity key, not a resolved field. `FillFilmYear` fills a
  blank year from the resolved `release_date` on enrich apply/clear and decision set/clear, and never
  overwrites. `SetFilmYear` is the owner's overwrite. Both share the identity collision probe; a
  collision withholds only the year write, never the enrich, which stays additive. A divergent
  `release_date` is never reconciled. The canonical `name` column is never written by a provider
  (see [entity-identity.md](entity-identity.md)).

**Rejected:** overwriting the year from a provider — silently changes identity, and with no stored
prior value clearing the provider could not restore it.

Decided in [`0a32445a`](https://github.com/whoiskevinrich/holodex/commit/0a32445a).
