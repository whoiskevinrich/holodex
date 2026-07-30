# Spec: Tag governance & video enrichment — deny-list, hierarchy, manual tagging, genre writeback (F50)

**Status**: Draft
**Phase**: Phase 3 (Enrichment / curation foundation) — extends F43's tag identity spine with governance
(deny-list, hierarchy) and wires it to video enrichment, manual curation, and file writeback.
**Owner**: Project owner
**Date**: 2026-07-29
**Feature block**: **F50** (working number — verify against `origin/main` immediately before commit; this
repo's F/ADR numbers move fast, see F43's own renumbering history in `entity-identity.md`).

**Depends on** (all shipped):
- F43 tag identity spine ([entity-identity.md](entity-identity.md), [ADR-061](../architecture/ADR-061-unified-entity-name-identity.md)) — `resolveOrCreateByName`, `entity_aliases`, merge/rename, the `/tags` list. This spec adds governance (deny-list, hierarchy) **on top of** that spine; it does not change RD7's "tags are identity-only, no decision model" call.
- F47 enrichment auto-apply + dismissals ([enrichment-review-workflow.md](enrichment-review-workflow.md), [ADR-066](../architecture/ADR-066-enrichment-auto-apply-and-dismissal.md)) — closest structural precedent for a durable negative-assertion table; this spec's deny-list is a different shape (global term block, not per-entity-per-provider) but the same idiom.
- Metadata writeback ([ADR-041](../architecture/ADR-041-metadata-writeback.md), `internal/writeback`) — the `genres` → `Genre`/`QuickTime:Genre` tag mapping **already exists** for every writable container (`internal/writeback/tags.go`); this spec changes what feeds that field, not the plumbing.
- Metadata field mapping ([ADR-013](../architecture/ADR-013-metadata-field-mapping.md), `metadata-mappings.yaml`) — the `genres` canonical field (`multi: true`, sourced from `tmdb:genres`) this spec materializes into real Tag rows.
- TMDB provider sidecar (F26) — the enrichment source whose genre data flows through this pipeline.

**ADR**: [ADR-075](../architecture/ADR-075-tag-governance-and-video-enrichment.md) (Proposed) — the hierarchy
column + application-layer cycle guard, the `denied_tags` table shape and its single enforcement point inside
`resolveOrCreateByName`, the `video_tags` provenance fix to `replaceAssociations` (the correctness-critical
piece), and the materialization pass's placement in the existing `afterEnrichApply` dispatcher.

**Design handoff**: [tag-governance-and-video-enrichment-handoff.md](../design/tag-governance-and-video-enrichment-handoff.md)
— media-page tag chips (P0-8), deny-list placement (P1-1, resolves Q2 below: new `/owner/tags` tab), and
hierarchy curation UI (P1-2/P1-3) are all specified. Companion QA checklist:
[tag-governance-and-video-enrichment-qa-checklist.md](../design/tag-governance-and-video-enrichment-qa-checklist.md).

---

## Problem Statement

Tags have an identity spine (F43) but no governance, no structure, and no write path in either direction:

1. **No deny-list.** Nothing stops a junk or unwanted term from becoming a permanent Tag. This is about to
   matter: the moment anything auto-creates tags (enrichment, below), noise has no backstop.
2. **No hierarchy.** Tags are flat. Searching "dog" does not surface a video tagged "german shepherd" —
   there is no broader/narrower relationship a search or filter can walk.
3. **Video enrichment produces tag-shaped data that never becomes a tag.** `genres` already auto-resolves
   from `tmdb:genres` (multi-value union, display-only) the moment a video is enriched — but it is disconnected
   from the Tag entity system. A video's TMDB genres are visible but not mergeable, aliasable, or curatable
   like a real tag.
4. **The media page cannot add or remove tags.** Today's chips (`web/src/routes/media/[id]/+page.svelte:406`)
   are read-only links. There is no attach/detach-tag-to-video endpoint anywhere in `internal/api` — the only
   way a tag ever lands on a video today is via the scanner reading the file's embedded content tags.
5. **Genre writeback has no source.** `internal/writeback/tags.go` already maps canonical `genres` → the
   file's `Genre` tag for every writable container — that plumbing is live and unused for this purpose. There
   is no curated source feeding it from the tag system.

**A fifth, load-bearing problem this spec surfaces rather than inherits:** `video_tags` has no provenance
column (`video_id, tag_id` only — `0001_init.up.sql:40`), and `replaceAssociations()` (`internal/repo/repo.go:177`)
**deletes and fully re-populates `video_tags` from the file's embedded tags on every rescan/re-extract.** This
is harmless today because tags only ever come from the file. It stops being harmless the instant this spec
ships: a manually-added tag or an enrichment-materialized tag has no way to survive the next scan — it is
silently deleted and never re-created, because the scanner has no record that it didn't come from the file.
This must be fixed as part of this spec (P0-1), not discovered after ship.

## Goals

1. **A denied term can never become a tag**, from any origin — file scan, manual add, or enrichment —
   enforced at one structural choke point, not per-call-site convention.
2. **Tags form a hierarchy** (broader/narrower); searching or filtering by a tag transitively includes videos
   tagged with any descendant.
3. **Video enrichment materializes into real tags.** The already-resolving `genres` field becomes actual,
   mergeable/aliasable Tag rows on the video — automatically, with no new review UI — the instant a video is
   enriched.
4. **Owners can add and remove a video's tags directly from the media page.**
5. **A video's tags can be written back to the file's `genre` tag** via the existing writeback modal, ancestor-expanded.
6. **Manual and enrichment-derived tags survive a rescan** — closing the `replaceAssociations` gap above.

## Non-Goals

- **A per-field decision/curation model for tags** — unchanged from F43 RD7. Tags remain a single-field
  identity entity; this spec adds governance and structure, not `BaselineSource`/source-chips.
- **Person tags.** Explicitly deferred — no `person_tags` join table, no person-page UI, in this spec. The
  denylist and hierarchy tables are **not** entity-typed (see Data model) precisely so a future `person_tags`
  join table needs zero core-system change — the owner's own call: a shared vocabulary, not separate pools.
- **DAG / multi-parent hierarchy.** v1 is a strict tree — one parent per tag. A tag needing two "broader"
  concepts is out of scope until a real case demands it.
- **Hierarchy-aware or subtree deny-listing.** The deny-list is exact-string match only (case-insensitive,
  not substring — blocking "Gnome" must not block "Garden Gnome"). Blocking an entire subtree in one action
  is out of scope.
- **Automatic/implicit file writeback on tag change.** Attaching or detaching a tag never writes to the file
  by itself. Writeback stays an explicit, owner-triggered action in the existing writeback modal — same
  ADR-041 posture as every other field. DB-side tag changes are cheap and reversible; file mutation is not,
  and the two must not share a trust boundary.
- **Reparenting UI beyond merge.** Merge-driven reparenting (RD-M below) is in scope; a general "move this
  subtree" curation tool is not — hierarchy edits happen one parent-pointer at a time.
- **MCP parity** for hierarchy/deny-list — rides the existing deferred MCP-parity backlog item, not scoped here.

## Resolved Decisions

*(Locked via question cards during the preceding brainstorm and spec-drafting session, 2026-07-29.)*

- **RD1 — One shared vocabulary, no entity-type split.** Tags remain one polymorphic pool (per F43); this
  spec does not introduce a "video tag" vs. "person tag" namespace. Owner's rationale: a person's hair color
  and a video's content tag can be the *same concept* (an actress can dye her hair), and splitting adds
  complexity with no payoff. Hierarchy, alias, and deny-list are keyed by tag id only — never by which entity
  attached it.
- **RD2 — Deny-list scope: global, exact-string, case-insensitive.** A denied term is blocked everywhere —
  manual tagging, enrichment materialization, and file-scan tag resolution — not scoped per-provider.
  Match is exact after normalization (lowercase + trim; **not** substring — "Gnome" does not block "Garden Gnome").
  Distinguishes deny-list (unwanted/junk terms) from alias (synonyms of wanted terms, e.g. "azure" → "blue").
- **RD3 — Deny-list enforcement lives inside the shared tag resolver, not at each call site.**
  `resolveOrCreateByName(ctx, tx, model.EntityTag, name, extID)` is the one function the scanner, manual
  attach, and enrichment materialization all call (F43). The deny-list check gates *there* — structurally
  impossible to bypass from any of the three paths, including a denied term embedded in a file's own tags.
- **RD4 — Auto-apply for enrichment, DB-side only.** Enrichment-derived tags attach automatically (no
  suggest/accept picker) — the intentional automation asymmetry vs. Person/Studio identity matching (F47),
  which stays manual-accept because a wrong identity match is higher-stakes than a wrong tag (one un-tag away
  from undone). File writeback is never automatic regardless (see Non-Goals).
- **RD5 — Alias-canonicalization is inherited, not reimplemented.** Because every tag-creation path routes
  through `resolveOrCreateByName`, an aliased term (enrichment-supplied or owner-typed) already resolves to
  its canonical tag — no new logic. This must remain true: no shortcut path may attach a raw string to
  `video_tags` without going through the shared resolver.
- **RD6 — Hierarchy: strict tree, one parent per tag.** No DAG in v1. `tags.parent_tag_id`, nullable,
  self-referential.
- **RD7 — Search/filter is descendant-inclusive.** Filtering or searching by a tag transitively includes
  videos tagged with any descendant (recursive expansion), everywhere tag-based filtering exists today
  (`?tag=`, global search, "More with tag" shelves).
- **RD8 — New tags default to root (no parent).** Whether created via a media-page chip or materialized from
  enrichment, a brand-new tag starts unparented. Hierarchy placement is a separate, later curation action —
  creation never blocks on a tree decision, and materialization (unattended) has no one to prompt anyway.
- **RD-M — Merge reparents children onto the survivor.** When a tag with children is merged away (F43 merge,
  unchanged mechanism), its children move under the surviving tag rather than being orphaned or blocking the
  merge. Keeps the tree connected through routine identity cleanup.
- **RD9 — Genre writeback source: union of curated tags and the raw resolved union, both deny-list filtered.**
  The `genres` writeback field is fed by **both** the video's attached tags (ancestor-expanded, canonical
  names) **and** the raw `genres` resolved-field union, combined — but the raw-union side is filtered through
  the deny-list before being merged in, same as the tag side. This closes what would otherwise be a real gap:
  without this, a denied term could never become a Tag but could still reach the file via the unfiltered raw
  union. Deny-list is a structural guarantee end-to-end, not just within the DB tag vocabulary.
- **RD10 — `video_tags` gains provenance; rescans stop clobbering non-file tags.** `video_tags` gains a
  `source` column reusing the existing `fieldsource` grammar (`file` / `manual` / `provider:<name>`,
  `internal/fieldsource/fieldsource.go`). `replaceAssociations()` changes from delete-all-and-reinsert to
  delete-and-reinsert **only `source='file'` rows**; manual and provider-sourced rows are left untouched
  across a rescan. This is the fix for the Problem Statement's fifth issue and is P0.

## User Stories

**Owner — governance**
- As the owner, I want to permanently block a junk term (e.g. "TV Movie") from ever becoming a tag, so
  enrichment noise never pollutes my tag vocabulary.
- As the owner, I want a term I hand-typed by mistake to be just as blockable as one enrichment suggested, so
  the deny-list is one rule, not two.

**Owner — hierarchy**
- As the owner, I want to tag a video "German Shepherd" and have it show up when I search "Dog" or "Animal",
  so I don't have to remember or apply every level of a category by hand.
- As the owner, I want to set a tag's parent once, so structure I build in pays off across the whole library
  automatically.

**Owner — video tagging**
- As the owner, I want to add or remove a tag directly on a video's page, so correcting or enriching a video's
  tags doesn't require going through `/tags` or a rescan.
- As the owner, when I enrich a video, I want its TMDB genres to become real tags automatically, so I don't
  have to manually re-type genre data that's already been fetched.
- As the owner, I want a manually-added tag to survive the next library scan, so tagging a video isn't
  effort I might lose.

**Owner — writeback**
- As the owner, I want to write a video's tags into the file's genre field alongside its other metadata, in
  the same writeback action I already use, so tagging data travels with the file.

**Visitor**
- As a visitor, I want searching or filtering by a broad tag to surface videos tagged with something more
  specific under it, so I don't need to know the exact tag applied.

## Requirements

### Must-have (P0)

- **P0-1 — `video_tags` provenance + rescan safety (RD10).** Migration adds `source TEXT NOT NULL DEFAULT
  'file'` to `video_tags`. `replaceAssociations()` deletes and reinserts only `source='file'` rows for a
  video; `manual`/`provider:*` rows persist across rescans, deduplicated against the reinserted file set
  (`INSERT OR IGNORE`, matching today's idempotency).
  - Given a video has one file-derived tag and one manually-added tag, When the video is rescanned, Then the
    manually-added tag is still present and the file-derived set matches the file's current embedded tags.
- **P0-2 — Deny-list table + enforcement (RD2/RD3).** New table (see Data model); `resolveOrCreateByName`
  rejects (returns an error the caller surfaces as 422, not a created row) when the normalized name matches a
  denied term. Applies uniformly to scan, manual attach, and materialization.
  - Given "TV Movie" is denied, When a video's file has "TV Movie" embedded as a tag, Then no `tags` row named
    "TV Movie" is created and no `video_tags` row links it.
  - Given "TV Movie" is denied, When the owner types "TV Movie" into the media-page tag input, Then the
    request is rejected with a clear error, not silently ignored.
- **P0-3 — Deny-list management endpoints.** `POST /api/v1/owner/tags/denylist {term}`,
  `DELETE /api/v1/owner/tags/denylist/{term}`, `GET /api/v1/owner/tags/denylist` — `requireOwner`.
- **P0-4 — Hierarchy: `parent_tag_id` + cycle guard (RD6).** `tags.parent_tag_id` nullable self-reference.
  Setting a parent is rejected (400) if it would create a cycle (the candidate parent is a descendant of the
  tag being reparented) or set a tag as its own parent.
  - Given "Dog" is a child of "Animal", When the owner tries to set "Animal"'s parent to "Dog", Then the
    request is rejected.
- **P0-5 — Hierarchy management endpoint.** `POST /api/v1/tags/{id}/parent {parent_id | null}` — `requireOwner`.
- **P0-6 — Descendant-inclusive filter/search (RD7).** Tag-based filtering and global search expand a query
  tag to itself plus all descendants (recursive query over `parent_tag_id`).
  - Given "German Shepherd" is a child of "Dog", When browsing is filtered by tag "Dog", Then videos tagged
    only "German Shepherd" (not "Dog" directly) are included.
- **P0-7 — Video↔tag attach/detach endpoints.** `POST /api/v1/videos/{id}/tags {name}` (resolves via the
  shared spine, `source='manual'`, deny-list enforced, 422 on denied term), `DELETE
  /api/v1/videos/{id}/tags/{tag_id}` — both `requireOwner`.
- **P0-8 — Media-page tag chips gain add/remove UI.** Replace the read-only chip list
  (`media/[id]/+page.svelte:406`) with removable chips (owner-only) plus an add-tag input with the same
  near-miss/collision affordances F43 already gives `/tags`.
- **P0-9 — Enrichment materialization pass (RD4/RD5).** When a video is (re-)enriched, each value in the
  resolved `genres` field is attached via `resolveOrCreateByName(..., model.EntityTag, value, "")` with
  `source='provider:<name>'`, silently skipping denied terms (no error surfaced — enrichment is unattended).
  No new picker UI; this runs as part of the existing enrich-apply path.
  - Given TMDB enrichment returns genres ["Action", "azure"] and "azure" aliases to "blue", When materialization
    runs, Then the video gains tags "Action" and "blue" (not "azure").
- **P0-10 — Genre writeback field (RD9).** The writeback field list includes `genres`, sourced from the union
  of the video's attached tags (ancestor-expanded, canonical names) and the existing raw resolved `genres`
  union, **with the raw-union side deny-list-filtered before merging** — deduplicated. Uses the existing
  `TagForField`/`ResolveForContainer` mapping unmodified.
  - Given a video is tagged "German Shepherd" (child of "Dog", child of "Animal"), When genre writeback is
    triggered, Then the file's `Genre` tag receives "Animal", "Dog", "German Shepherd" (plus any raw-union
    values not already covered).
  - Given "TV Movie" is denied and present in the raw TMDB genres union but not attached as a Tag, When genre
    writeback is triggered, Then "TV Movie" does **not** appear in the file's `Genre` tag.
- **P0-11 — Merge reparenting (RD-M).** F43's existing tag-merge endpoint is extended so a merged-away tag's
  children are repointed to the surviving tag's id as part of the same transaction.
  - Given "Puppy" (parent of "Golden Retriever Puppy") is merged into "Dog", Then "Golden Retriever Puppy"'s
    parent becomes "Dog".

### Should-have (P1)

- **P1-1 — Deny-list management UI.** A new `/owner/tags` ("Deny-list") tab — a dedicated tab, not folded
  into Duplicates: deny-list is exclusion, Duplicates is identity-merge, different owner intent (resolved in
  `/design-handoff`).
- **P1-2 — Hierarchy curation UI on `/tags`.** Set/clear a tag's parent from the tags list (F43 already gave
  tags "light list actions, no detail page" — this is one more row action, not a new page, consistent with
  F43 RD7).
- **P1-3 — Ancestor breadcrumb on tag chips.** Where a tag is shown (media page, `/tags/{id}`), optionally
  surface its parent chain so hierarchy is legible without opening `/tags`.

### Future considerations (P2)

- **P2-1 — Person tags.** New `person_tags` join table + person-page chip UI, reusing this spec's vocabulary,
  hierarchy, and deny-list unchanged (RD1 is what makes this cheap).
- **P2-2 — DAG hierarchy** — only if a real multi-parent case arrives.
- **P2-3 — Subtree deny-listing** — only if exact-string deny-list proves insufficient in practice.
- **P2-4 — MCP parity** for hierarchy/deny-list.

## Behavior detail

### Deny-list enforcement point (RD3)
`resolveOrCreateByName` gains a pre-check for `entityType == model.EntityTag`: normalize the candidate name
(same fold as tag `nameKey`, RD2 of F43 — lowercase + all-whitespace-collapsed), look up in the denylist table,
and return a typed "denied" error if present — **before** the existing id→nameKey→create resolve order runs.
Callers translate that error to their own semantics: the scanner silently skips the tag (matching today's
tolerant-of-partial-data posture), manual attach returns 422, materialization silently skips.

### Materialization pass (P0-9)
Runs wherever video enrichment currently applies resolved fields (the existing enrich-apply path for video
entities) — not a new trigger, not a new endpoint. Reads the just-resolved `genres` values, calls
`resolveOrCreateByName` per value with `source='provider:<name>'`, `INSERT OR IGNORE`s into `video_tags`.
Idempotent — re-enriching a video that already has its TMDB genres as tags is a no-op.

### Descendant expansion (P0-6)
A recursive CTE over `tags.parent_tag_id` from the query tag's id, unioned with the tag itself, feeding
existing `?tag=` filter and search-match logic. No schema change to `video_tags` — expansion happens at query
time, matching the tag identity spine's own "no denormalization" precedent (F43 does not flatten aliases into
the base tables either).

### Writeback assembly (P0-10)
Two inputs are merged before being handed to the existing `FieldValues{Field: "genres", ...}` → `WriteBatch`
path (unchanged): (1) the video's `video_tags`, expanded to each tag's full ancestor chain, canonical names;
(2) today's raw resolved `genres` union, **with any value matching a `denied_tags.term_key` dropped first**
(same normalize/lookup the tag resolver uses, RD3). Deduplicated by case-insensitive value before writeback.

## Data model

```
-- Deny-list (RD2/RD3) — global, not entity-typed (RD1), distinct shape from
-- entity_keep_separate (F43, id_lo/id_hi pair) and enrichment_dismissals (F47,
-- entity+provider) because this blocks a *term*, not a pair or an entity.
denied_tags
  term_key    TEXT PRIMARY KEY   -- normalize(term), same fold as tag nameKey
  term        TEXT NOT NULL      -- original casing, for display/audit
  created_at  DATETIME NOT NULL

-- Hierarchy (RD6) — one column on the existing table, not a new one; a strict
-- tree needs nothing more than a parent pointer.
ALTER TABLE tags ADD COLUMN parent_tag_id INTEGER REFERENCES tags(id) ON DELETE SET NULL;
CREATE INDEX idx_tags_parent ON tags(parent_tag_id);

-- Provenance (RD10) — the correctness-critical addition. Reuses the existing
-- fieldsource grammar (internal/fieldsource) rather than inventing a new one.
ALTER TABLE video_tags ADD COLUMN source TEXT NOT NULL DEFAULT 'file';
-- values: 'file' | 'manual' | 'provider:<name>'
```

Cycle prevention for `parent_tag_id` is enforced at the application layer (walk ancestors of the proposed
parent before committing a reparent; SQLite has no native graph-cycle constraint) — mirrors how F43 enforces
its cross-namespace identity guarantee at the resolve layer rather than in DDL.

## API

All under `/api/v1`. Reads ungated; mutations `requireOwner` (ADR-030), consistent with F43/F47.

```
POST   /videos/{id}/tags              {"name":"…"}        → 200 {tag} | 422 {denied:true} | 409 {conflict:…}
DELETE /videos/{id}/tags/{tag_id}                          → 204 | 404

POST   /tags/{id}/parent              {"parent_id": N|null} → 200 {tag} | 400 {cycle:true}

GET    /owner/tags/denylist                                 → {terms:[{term, created_at}]}
POST   /owner/tags/denylist           {"term":"…"}          → 201 | 409 (already denied)
DELETE /owner/tags/denylist/{term}                           → 204 | 404
```

Existing endpoints, behavior extended (no new routes):
- `?tag=` browse filter and global search — descendant-inclusive (P0-6).
- `POST /media/{id}/writeback` — `genres` becomes a selectable field, sourced per P0-10 (no request-shape change).
- The existing video-enrich apply path — gains the materialization side-effect (P0-9), no response-shape change.

## UI

- **Media page (`web/src/routes/media/[id]/+page.svelte`)**: tag chips gain an owner-only remove affordance
  and an add-tag input (autocomplete against existing tags, same collision/near-miss soft-warning F43 already
  gives `/tags`). Denied-term submission shows an inline rejection, not a silent no-op.
- **`/tags`**: gains a parent-setter row action (P1-2) alongside F43's existing rename/alias/merge actions.
- **`/owner`**: a new **Deny-list** tab (P1-1), added to the existing tab row (`owner/+layout.svelte`) next
  to Duplicates.
- **Writeback modal**: `genres` appears as one more checkable field, no layout change.
- Tokens only; QA all three skins (Cinémathèque / Broadcast / Brutalist) per this project's standing rule.

## Success Metrics

Single-owner correctness + hygiene feature — same posture as F43:
- **Leading:** zero manually-added or enrichment-materialized tags lost across a rescan (the P0-1 fix,
  regression-tested directly).
- **Leading:** a denied term never appears as a `tags` row regardless of origin (scan/manual/enrichment),
  tested per path.
- **Leading:** filtering by a parent tag returns the same set as filtering by parent + every descendant
  individually (recursive-expansion correctness).
- **Leading:** re-enriching an already-materialized video is a no-op (idempotency).
- **Lagging:** genre writeback coverage — the share of videos whose file `Genre` tag reflects current curated
  tags — trends toward 100% as the owner works through the library.

## Open Questions

- **Q1 (engineering, non-blocking):** exact whitespace-fold for `denied_tags.term_key` — reuse tag `nameKey`'s
  fold (all whitespace collapsed) verbatim, or a separate normalize function? Recommend reusing tag `nameKey`
  so a denied term and its "same tag" variants are denied together without a second fold to keep in sync.
- ~~**Q2 (design):** deny-list panel placement.~~ **Resolved in `/design-handoff`:** a new `/owner/tags`
  ("Deny-list") tab.
- **Q3 (engineering, non-blocking):** should `denied_tags` cascade-delete if a `tags` row with that exact
  name is later legitimately created after being un-denied? (It shouldn't need to — denying doesn't create a
  `tags` row, so there's nothing to cascade — but worth a test asserting un-deny → create works cleanly.)

## Timeline / routing

No hard deadline. Per this project's change-routing rules, before/with implementation:

1. ✅ **`/architecture`** — [ADR-075](../architecture/ADR-075-tag-governance-and-video-enrichment.md) (Proposed):
   the hierarchy relationship (`parent_tag_id`, application-layer cycle guard, query-time recursive expansion),
   the deny-list table shape (why it's not `entity_keep_separate`- or `enrichment_dismissals`-shaped), the
   `video_tags` provenance column + `replaceAssociations` behavior change (flagged as the ADR's highest-risk
   decision), and the materialization pass's placement in the existing `afterEnrichApply` dispatcher.
2. ✅ **`/design-handoff`** — media-page tag chip add/remove, deny-list management surface, `/tags` parent-setter
   action, 3-skin QA. See
   [tag-governance-and-video-enrichment-handoff.md](../design/tag-governance-and-video-enrichment-handoff.md).
3. ✅ **`/testing-strategy`** — rescan-preserves-non-file-tags (P0-1) is the single highest-value test in this
   spec; deny-list enforced on all three paths; cycle rejection; descendant-inclusive filter/search parity;
   materialization idempotency; merge reparenting. See
   [testing-strategy.md](../testing-strategy.md#9-phasing--what-lands-when) (F50 block).
4. ⬜ **`/security-review`** — new mutations (`videos/{id}/tags`, `tags/{id}/parent`, `owner/tags/denylist`) are
   all `requireOwner`; no new externally-influenced input beyond what F43/F47 already validate (tag names go
   through the same sanitize perimeter). Expect a clean design-level sign-off given the shape matches F43/F47
   precedent, but re-run on the implementation diff per standing policy. **Not started.**

**Suggested slices:** **S1** — `video_tags` provenance + `replaceAssociations` fix (P0-1, the correctness
gap; should land first and independently since it's a latent bug fix, not a new feature) → **S2** — deny-list
table + `resolveOrCreateByName` enforcement + management endpoints (P0-2/3) → **S3** — hierarchy (`parent_tag_id`
+ cycle guard + descendant-inclusive filter/search, P0-4/5/6) → **S4** — video↔tag attach/detach endpoints +
media-page UI (P0-7/8) → **S5** — enrichment materialization (P0-9, depends on S2 for deny-list + S1 for
provenance) → **S6** — genre writeback wiring (P0-10, depends on S4) → **S7** — merge reparenting (P0-11,
depends on S3) → **S8** — P1 UI (deny-list panel, hierarchy row action) → **S9** — QA + `/security-review`.
Effort: **L** (comparable to F43's L, with a similar-shaped correctness fix at its core).
