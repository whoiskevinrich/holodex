# Spec: Unified entity name-identity — merge, alias & case-collision fix (F43)

**Status**: Draft
**Phase**: Phase 3 (Enrichment / curation foundation) — generalizes F23 (person aliases) to all named entities
**Owner**: Project owner
**Date**: 2026-07-05
**Feature block**: **F43** — one **name-identity spine** (a per-entity normalized `nameKey`, unique across
canonical names *and* aliases, plus a shared alias / merge / rename / keep-separate mechanism) across
**Person, Studio, and Tag**. Fixes the `"fox"`/`"Fox"` split where a real name resolves to two entities by
capitalization, and gives Studios and Tags the merge/alias capability People already have (F23).

**Depends on** (all shipped):
- F23 person aliases + merge ([person-aliases.md](person-aliases.md), [ADR-036](../architecture/ADR-036-person-alias-search-indexing.md)) — the pattern this generalizes; `person_aliases` migrates onto the shared spine.
- The studio entity ([studio-entity.md](studio-entity.md), [ADR-053](../architecture/ADR-053-studio-entity-and-resolved-link-derivation.md)) — studio links **derive** from the resolved field, so studio merge must register an alias (RD6).
- Studio external-id de-dup ([ADR-054](../architecture/ADR-054-studio-external-id-dedup.md)) / the enrichment unique-key invariant ([ADR-055](../architecture/ADR-055-enrichment-unique-key-invariant.md)) — provider identity; this spec is the **name** side that ADR-055 explicitly leaves separate (RD3).
- Person link resolved-derivation ([ADR-059](../architecture/ADR-059-person-link-resolved-derivation.md), spec F40) — makes `video_people` **derived** via a generic `RelinkVideoEntity` that routes names through the alias table at derivation time; F43 is the alias/merge spine that derivation routes through, and person merge now survives re-derivation like studio (RD6). The two converge on one `resolveOrCreateByName`; whichever lands second wires to it.
- The owner gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md), `requireOwner`); System Activity job history ([F21](system-activity.md)/[ADR-028](../architecture/ADR-028-activity-surface-and-job-history.md)) for the backfill; the Owner hub ([F35](owner-tooling-hub.md), `/owner`) for the review queue.

**ADR**: [ADR-061](../architecture/ADR-061-unified-entity-name-identity.md) (Proposed) — the shared spine,
per-entity normalize, id→name→create resolve order, two-path handling, keep-separate. Realizes ADR-053's
deferred RD4; generalizes ADR-036/F23; complements ADR-055. **Supersedes** ADR-053's binary-name studio
identity and the tag bare-string identity.

**Design handoff**: [entity-identity-handoff.md](../design/entity-identity-handoff.md) — the review-queue
banner + `/owner` Duplicates tab, tag-list identity actions, the editor near-miss prompt, studio/person alias panels.

**Evidence**: production collision probe (2026-07-05, read-only, anonymized) over **1166 people / 143 studios /
748 tags** — **14 hard case/whitespace collisions** (person 6, studio 3, tag 5; *all* pure-case, 2-way,
canonical) and **56 near-misses** (person 8, studio 7, **tag 41**; ~⅔ internal-whitespace). These numbers set
the P0/P1 split and the backfill safety argument.

---

## Problem Statement

The library's name-identity layer **disagrees with itself about case**. Canonical `name` columns
(`people.name`, `studios.name`, `tags.name`) are `UNIQUE` **case-sensitive**, while `person_aliases.alias` is
`COLLATE NOCASE`. So the scan-time router (`resolveOrCreatePerson`, exact-binary → alias-NOCASE → create) can
route the **same real name to two different entities purely by capitalization** — a person aliased `"Fox"`
and a person canonically named `"fox"` are two records, and file credits split between them by case. The
probe confirms it is live, not theoretical: 14 such pairs exist today. Separately, **Studios and Tags cannot
be merged or aliased at all** — Studios deliberately deferred identity ops (ADR-053 RD4), Tags are bare
strings — so a misspelled or re-spelled studio/tag stays a permanent duplicate. The cost is a library that
quietly fragments identity: two "fox" studios, 41 near-duplicate tags, and no owner tool to fix any of it.

## Goals

1. **One name can only be one entity.** A per-entity normalized `nameKey`, unique across canonical names and
   aliases, makes the `"fox"`/`"Fox"` split *unrepresentable* — not merely cleaned up once.
2. **Studios and Tags gain merge + alias, like People.** The same add-alias / merge / rename capability and
   UX F23 gave People, generalized to all three via one shared mechanism — a studio merge that **survives
   ADR-053 link re-derivation**, a tag merge that survives re-scans.
3. **Ambiguous near-misses get a human, never a silent merge.** Case/whitespace variants collapse
   automatically; spelling/punctuation near-misses (the 56, ⅔ tags) surface for the owner to confirm or keep
   separate — the homonym rule holds.
4. **Zero core divergence.** Identity becomes entity-generic (one `resolveOrCreateByName`, one polymorphic
   alias/keep-separate store) the way resolution became entity-generic in ADR-052 — so a future entity
   inherits identity for free.

## Non-Goals

- **A per-field decision/curation model for Tags** (RD7). Tags get the identity spine only, not
  `BaselineSource`/source-chips — a tag is a single-field entity; its name *is* the entity. *(Why: the
  decision machinery is ceremony with no payoff for one field.)*
- **A tag detail page** (RD7). Tag identity is operated from **light list actions** + the review queue; no
  `/tags/{id}` page, no `·record` chips. *(Why: card decision — smallest surface that operates identity.)*
- **Undo / split / un-merge** (RD8). Merge is one-way (mirrors F23); deleting an alias stops *future* routing
  but does not retroactively re-split moved associations. A dedicated un-merge is a tracked P2.
- **Provider (external-id) identity changes.** ADR-055's id→entity invariant is untouched; this spec is the
  name-identity companion that runs *after* an id miss (RD3). `tag_external_ids` (ADR-055 future row) is out
  of scope — tags are not enrichable here.
- **Edit-distance / phonetic near-miss detection.** v1 detection is the probe's loose-key (lowercase + strip
  whitespace/punctuation); fuzzier matching is a P2 refinement.
- **MCP parity** — exposing aliases/merge over MCP rides the deferred F22.5f item.
- **Changing search ranking or the `?person=`/`?studio_id=`/`?tag=` id filters** — a merge already makes the
  canonical id own the union; identity does not touch id-based filtering.

## Resolved Decisions

*(RD1–RD7 lock the ADR-061 decisions; RD8–RD11 lock the spec-level question cards, 2026-07-05.)*

- **RD1 — `nameKey` is the identity, across canonical ∪ aliases** (ADR-061 D1). Each entity type has a
  normalized `nameKey`; it is unique across both canonical names and aliases (one logical uniqueness domain).
  Replaces the binary `UNIQUE(name)` + separate NOCASE alias uniqueness that produced the three collision
  modes.
- **RD2 — Normalize scope is per-entity** (ADR-061 D2, evidence-driven). **Person & Studio**: fold case +
  **edge** whitespace only (`lower(trim(name))`) — curated names, internal spacing can be meaningful.
  **Tag**: fold case + **all** whitespace (edge and internal) — labels, high drift; this auto-resolves ~27
  of the 41 tag near-misses safely. Diacritic/punctuation folding stays **out** of the identity key for all
  three (FTS folds diacritics for *search*, not *identity*).
- **RD3 — Resolve order: id → name → create** (ADR-061 D5). Resolve-or-create tries (1) external-id
  (ADR-054/055, when a provider supplied one), then (2) `nameKey` over canonical ∪ aliases, then (3) create
  (scan) or prompt (editor). Provider data stays id-keyed (name display-only, ADR-055); file/owner-authored
  data is name-keyed. One ordered pipeline, no overlap.
- **RD4 — Two-path collision handling** (ADR-061 D3). **Scan** (non-interactive): an exact-`nameKey` match
  routes silently (case/whitespace variants can never create a second entity); a fuzzy near-miss creates the
  entity and **flags the pair to the review queue** — never auto-merges. **Editor** (interactive): an
  exact-`nameKey` hit on a *different* entity returns the F23-style informed collision (409); a fuzzy
  near-miss shows a **soft warning** (RD9).
- **RD5 — `keep-separate` assertion** (ADR-061 D4). A durable "these two ids are deliberately distinct"
  marker, recorded whenever the owner chooses "keep separate" / "create anyway". The detector and the queue
  never re-propose a kept-separate pair. It is the negative of an alias.
- **RD6 — Studio & Tag get full alias + merge + rename; studio merge registers an alias** (ADR-061 D6).
  Merging B→A repoints associations, moves decisions/curation/enrichment, **registers B's name as an alias of
  A**, deletes B. For studios the alias is load-bearing: without it, ADR-053's `RelinkVideoStudios` would
  re-create B from the resolved field on the next reconcile. **Person too, under ADR-059**: once `video_people`
  is derived via `RelinkVideoEntity`, a person merge without the registered alias is undone the same way — so
  the alias is load-bearing for all three, not a studio-only quirk. (`MergePersons` already registers the alias.)
- **RD7 — Tags: identity-only, light list actions** (ADR-061 D7 + card). Tags gain `nameKey` + aliases +
  merge + rename + shared FTS, operated from `/tags` row actions and the review queue. No decision model, no
  detail page.
- **RD8 — Merge is irreversible + informed confirm** (card). Both entry points show each side's video count
  and what moves before committing; merge is one-way. Undo/split is P2.
- **RD9 — Review queue: banner → `/owner` Duplicates tab; editor near-miss = soft warning** (cards). Entity
  lists (`/people`, `/studios`, `/tags`) show a "N possible duplicates" banner linking to a **Duplicates tab
  in the Owner hub** (F35) where the whole queue is worked. The editor near-miss prompt is **non-blocking**
  ("looks like X — merge instead?" with "create/rename anyway" → records keep-separate).
- **RD10 — Backfill: auto-fold the safe, queue the ambiguous** (ADR-061 migration). The **hard pure-case
  pairs** (survivor = lower `id`; loser's name → alias; associations moved onto the survivor; loser's shadow
  rows dropped, matching `MergePersons`) are folded **in-SQL inside migration 0022, immediately before the
  nameKey unique indexes are built** — the unique-index build cannot fail on residual dupes because the fold
  runs first, in the same migration. *(As-built refinement: bootstrap applies migrations **before** the Go
  one-time backfills — `cmd/holodex/main.go` — so the fold cannot live in a post-migrate boot job; it is
  data-driven, correct for any number of dupes, not just today's 14.)* The **~56 near-misses** are seeded
  into the review queue by the S5 job — an observable, idempotent System Activity job (ADR-028) — which
  **never** auto-merges a near-miss.
- **RD11 — Person conforms to the spine.** F23's `person_aliases` migrates into the shared store; the
  canonical person `nameKey` becomes case-insensitive (RD2); F23's endpoints, search behavior, and
  scan-routing are **preserved**; `person_aliases_fts` is replaced by the shared `entity_aliases_fts`
  (entity_type-filtered), verified at search parity before the old table drops.

## User Stories

**Owner — kill duplicates**
- As the owner, I want `"fox"` and `"Fox"` to be **one** studio/person/tag, so casing never fragments my
  library.
- As the owner, I want a **Duplicates** list showing likely same-thing pairs (especially my messy tags), so I
  can clear them in one place instead of hunting.
- As the owner, when I say two look-alikes are actually **different**, I want the tool to remember, so it
  stops asking.

**Owner — merge & alias any entity**
- As the owner, I want to merge two studios (or tags) into one — keeping the good name, folding the other in
  as an alias — so re-scans and re-enrich don't resurrect the duplicate.
- As the owner, I want to add an alias to a studio/tag (`"WB"` → `"Warner Bros."`, `"scifi"` → `"sci-fi"`),
  so future files spelled that way route to the right entity.
- As the owner, when I rename a tag into something close to an existing tag, I want a heads-up that they might
  be the same — but I want to proceed if I know they're not.

**Owner — safe by default**
- As the owner, I want a merge to show me both video counts and what moves before it commits, because merge
  is one-way.

**Visitor**
- As a visitor, I want search to keep finding a studio/tag by any of its alternate spellings, so a merge or
  alias improves discovery, never hides content.

## Requirements

### Must-have (P0) — the identity fix + merge/alias capability

- **P0-1 — `nameKey` uniqueness (migration 0022).** Per-entity normalized-key uniqueness replaces binary
  `UNIQUE(name)`: a unique expression index on `lower(trim(name))` for `people`/`studios` and on the
  all-whitespace-folded key for `tags` (RD2). The shared spine tables land here too (see Data model).
  - Given a studio `"fox"` exists, When resolve-or-create is asked for `"Fox"` or `" fox "`, Then it returns
    the existing studio — no second row.
  - Given a tag `"sci fi"` exists, When resolve-or-create is asked for `"scifi"` or `"Sci Fi"`, Then it
    returns the existing tag (tag internal-whitespace fold, RD2).
- **P0-2 — Shared name-identity path (RD1/RD3).** One `resolveOrCreateByName(entityType, name, extID?)`
  implementing id→name→create, wired for person, studio, and tag; `resolveOrCreatePerson`,
  `resolveOrCreateStudio`, and the tag `getOrCreateByName` route through it. Scan-time behavior unchanged
  except that case/whitespace variants now converge.
- **P0-3 — Studio alias + merge + rename (RD6).** `POST/DELETE /studios/{id}/aliases`,
  `POST /studios/{id}/merge {from_id}`, `POST /studios/{id}/rename {name}` (all `requireOwner`), plus alias
  routing in `resolveOrCreateStudio` so a merge survives `RelinkVideoStudios`.
  - Given studios `"WB"` and `"Warner Bros."`, When the owner merges `"WB"` into `"Warner Bros."`, Then
    `"WB"` becomes an alias, both studios' videos link to `"Warner Bros."`, and a **re-enrich/rescan does not
    recreate `"WB"`**.
- **P0-4 — Tag alias + merge + rename (RD7).** `POST/DELETE /tags/{id}/aliases`,
  `POST /tags/{id}/merge {from_id}`, `POST /tags/{id}/rename {name}` (`requireOwner`), with alias routing at
  scan time. `/tags` list gains owner row-actions (rename, alias, merge-select). No tag detail page.
- **P0-5 — Exact-collision prompt, generalized (RD4/RD5).** Adding an alias or renaming to a `nameKey` that
  belongs to a **different** entity returns `409 {conflict:{id,name,count}}` (the F23 shape) for all three
  entities; the owner chooses **merge** or **keep separate** (records a keep-separate marker). Never a silent
  merge (homonym rule).
- **P0-6 — `keep_separate` store (RD5).** A durable per-type "these two ids are distinct" record, written on
  every "keep separate" choice; consulted by all near-miss detection.
- **P0-7 — Hard-pair fold + near-miss seed (RD10).** Migration 0022 folds the 14 hard pairs in-SQL before it
  builds the nameKey unique indexes (survivor = lower id; loser name → alias; associations moved; loser shadow
  rows dropped). A later idempotent System Activity job (S5) records the ~56 near-misses as queue rows (P1
  consumes them). No near-miss is auto-merged.
  - Given a library with pure-case duplicates, When migration 0022 applies, Then `/studios`, `/people`,
    `/tags` each drop their pure-case duplicates and every affected association survives on the surviving
    entity; re-applying (a fresh boot) is a no-op.
- **P0-8 — Person conformance (RD11).** `person_aliases` migrates into the shared store; person `nameKey`
  becomes case-insensitive; F23 endpoints/search/scan behavior preserved; search parity verified before the
  old FTS table drops.
- **P0-9 — Search honors aliases for all three (extends ADR-017/036).** Global search matches any entity's
  aliases and returns that entity (once) + its media, via the shared `entity_aliases_fts` — the F23.5
  guarantee, now for studios and tags too.

### Should-have (P1) — the near-miss review queue (tag-hygiene tool)

- **P1-1 — Near-miss detection (RD4).** The loose-key detector (lowercase + strip whitespace/punctuation)
  flags candidate same-thing pairs within an entity type, excluding exact-`nameKey` matches (already
  collapsed) and any `keep_separate` pair.
- **P1-2 — Scan-time flagging (RD4).** When a scan creates an entity that is a fuzzy near-miss of an existing
  one, it records a queue row (idempotent; inside the scan transaction discipline). It does **not** prompt or
  merge.
- **P1-3 — Review-queue API.** `GET /api/v1/owner/duplicates` (grouped by entity type: each pair with both
  names + video counts + variation kind), `POST /api/v1/owner/duplicates/dismiss {entity_type,id_a,id_b}` →
  records keep-separate (RD5); resolving a pair uses the per-entity `merge` endpoint. `requireOwner`.
- **P1-4 — Review-queue UI (RD9).** A **Duplicates tab under `/owner`** (F35) listing pairs grouped by entity
  (tags first — they dominate) with per-pair **Merge** (→ informed confirm, RD8) / **Keep separate**; and a
  **"N possible duplicates" banner** on `/people`, `/studios`, `/tags` linking into it. Tokens only; QA all
  three skins.
- **P1-5 — Editor near-miss soft warning (RD9).** On create/rename/alias where a fuzzy near-miss (not exact)
  is detected, show a non-blocking "looks like *X* — merge instead?" with **Merge** / **Create anyway** (the
  latter records keep-separate). Distinct from the P0-5 exact-collision 409.

### Future considerations (P2)

- **P2-1 — Undo / un-merge** (RD8) — reversible association moves + entity restore.
- **P2-2 — Fuzzy detection upgrade** — edit-distance / phonetic beyond loose-key, if tag drift outpaces it.
- **P2-3 — Tag detail page** — only if tags ever need more than list actions.
- **P2-4 — MCP parity** — aliases/merge over MCP (rides F22.5f).
- **P2-5 — `tag_external_ids`** — if/when tags become enrichable (ADR-055 future row); independent of this spec.

## Behavior detail

### Resolve order (RD3)
`resolveOrCreateByName(entityType, name, extID?)`: **(1)** if `extID` present, match `<entity>_external_ids`
(ADR-054/055) → return that entity, back-fill the name if new; **(2)** compute `nameKey =
normalize[entityType](name)`, match the canonical unique index, else the alias `alias_key` → return that
entity; **(3)** no match → create (scan) or return a create-with-prompt signal (editor). Step 2 replaces
today's binary-exact→NOCASE-alias steps and is the only behavioral change on the scan hot path (case/ws
variants now converge; the unchanged-file fast path, ADR-018, is untouched).

### Normalize registry (RD2)
`normalize` is keyed by entity type: `person`/`studio` = `lower(trim(name))`; `tag` =
`lower(trim(name))` with internal whitespace collapsed. A change to a normalize function is a migration
(index rebuild + near-miss re-scan) and is versioned as such.

### Merge (RD6/RD8) — one function per entity, same shape
Move associations (de-duped union), move decisions/curation/enrichment where they don't conflict with the
survivor's, **register the loser's canonical name as an alias of the survivor**, delete the loser. For
studios the registered alias is what makes the merge survive `RelinkVideoStudios` re-derivation; for
people/tags it makes the merge survive re-scan. Irreversible; the confirm shows both counts.

### Backfill (RD10)
Two parts, split by ordering necessity. **The hard-pair fold runs in-SQL inside migration 0022** (bootstrap
applies migrations before the Go boot backfills, so the fold must precede the unique-index build it enables):
for each entity type, group by `nameKey`; where a group has >1 row (the 14 hard pairs — all pure-case 2-way
canonical), keep the lowest `id`, move associations onto it, register the loser name as an alias, drop the
loser's shadow rows, delete the loser; then build the unique indexes on the now-clean set. **The near-miss
seed is a separate boot job (S5, ADR-028, idempotent):** the loose-key detector inserts queue rows for the ~56
near-misses (no merge).

## Data model (migration 0022 — final DDL per ADR-061 §Shape / engineering)

```
entity_aliases                         -- polymorphic; person_aliases migrates in (RD11)
  id           PK
  entity_type  TEXT   -- 'person' | 'studio' | 'tag'
  entity_id    INTEGER
  alias        TEXT
  alias_key    TEXT   -- normalize[entity_type](alias)
  UNIQUE (entity_type, alias_key)
  INDEX  (entity_type, entity_id)
  -- polymorphic ref: cascade handled by merge/delete logic (matches entity_enrichment/decision/curation)

entity_aliases_fts   -- shared external-content FTS5 mirror (unicode61 remove_diacritics 2),
                     -- entity_type-filtered in global search; replaces person_aliases_fts (ADR-036/RD11)

entity_keep_separate                   -- RD5
  entity_type  TEXT
  id_lo        INTEGER    -- min(id_a, id_b)
  id_hi        INTEGER    -- max(id_a, id_b)
  PRIMARY KEY (entity_type, id_lo, id_hi)

identity_review_queue                  -- P1; seeded by backfill, appended by scan flagging
  entity_type  TEXT
  id_lo, id_hi INTEGER
  variation    TEXT       -- 'internal-whitespace' | 'punctuation' | …
  PRIMARY KEY (entity_type, id_lo, id_hi)

-- canonical nameKey uniqueness (replaces binary UNIQUE(name)):
CREATE UNIQUE INDEX ux_people_namekey  ON people  (lower(trim(name)));
CREATE UNIQUE INDEX ux_studios_namekey ON studios (lower(trim(name)));
CREATE UNIQUE INDEX ux_tags_namekey    ON tags    (replace(lower(trim(name)), ' ', ''));
```

The **cross-namespace** guarantee (a name can't be a canonical of X and a canonical/alias of Y) is enforced
at the resolve/mutation layer — the P0-5 collision check + scan routing — exactly as F23 enforces it today
for people (the DB indexes enforce each namespace; the guard enforces the union). Whether the polymorphic
tables are one shared table or per-entity tables sharing code is settled by ADR-061 (polymorphic, matching
the entity-typed enrichment/decision/curation stores); engineering finalizes the DDL + the `.down.sql`.

## API

All under `/api/v1`. Reads ungated; mutations in the `requireOwner` group (ADR-030). Studio/tag endpoints
mirror the F23 person endpoints exactly.

```
POST   /people|studios|tags/{id}/aliases          {"alias":"…"}   → 200 {aliases:[…]} | 409 {conflict:{id,name,count}}
DELETE /people|studios|tags/{id}/aliases/{aliasId}                 → 204 | 404
POST   /people|studios|tags/{id}/merge            {"from_id":N}    → 200 {entity}      (self-merge → 400)
POST   /people|studios|tags/{id}/rename           {"name":"…"}     → 200 {entity} | 409 {conflict:…}
GET    /studios/{id} | /tags?…                                     aliases included in the payload (public)
GET    /owner/duplicates                                           → {pairs:[{entity_type,a,b,variation}]} (owner)   [P1]
POST   /owner/duplicates/dismiss                  {entity_type,id_a,id_b} → 204 (records keep-separate)         [P1]
```

Errors mirror F23: `404` not found; `400` invalid/empty alias or name, missing/self `from_id`; `409` name
belongs to another entity; `401` unauthorized. Alias validation reuses F23.1 (trim, non-empty, ≤200 chars,
`nameKey`-unique per entity).

## UI (grounded in real components)

- **`/people`, `/studios`**: existing "Also known as" alias panel (F23 `PersonPicker`/alias chips) — the
  studio page reuses it verbatim (RD6). Owner-only add/delete/merge controls.
- **`/tags`**: the list gains owner **row actions** — rename, add-alias, and a **merge-select** mode
  ("Keep which name?" dialog, the F23 `/people` multi-select pattern). No detail page (RD7).
- **`/owner` → Duplicates tab** (F35): pairs grouped by entity (tags first), each with both names + counts +
  variation, **Merge** (informed confirm, RD8) / **Keep separate** (RD5). The **"N possible duplicates"
  banner** on each entity list links here (RD9).
- **Editor near-miss**: non-blocking inline prompt on create/rename/alias — "looks like *X* — merge?" /
  "Create anyway" (RD9); distinct from the exact-collision 409 (P0-5).
- Tokens only; QA Cinémathèque / Broadcast / Brutalist. Full layout/states/a11y in the design handoff.

## Success Metrics

Single-owner correctness + hygiene feature:
- **Leading:** after backfill, `"fox"`/`"Fox"`-class duplicates are **0** across all three entities, and every
  affected association survives (tests + probe re-run returns 0 Tier-A groups).
- **Leading:** a studio/tag merge survives a re-scan and a re-enrich (the merged-away name does not reappear)
  — the RD6 alias-routing guarantee, tested per entity.
- **Leading:** the 56 near-misses are all reachable in the Duplicates tab; dismissing one never re-surfaces it
  (keep-separate honored).
- **Leading:** zero resolver/decision-core diffs — identity is generalized without touching resolution
  (the ADR-052 property, held a fourth time).
- **Lagging:** tag near-miss count trends down as the owner works the queue; undo (P2-1) stays unneeded.

## Open Questions

- **Q1 (engineering, non-blocking):** exact tag `nameKey` normalize — collapse only ASCII space, or all
  Unicode whitespace? The probe's cases are all spaces; `replace(…,' ','')` covers them. Pick at
  implementation; it's the tag index expression.
- **Q2 (engineering, non-blocking):** `entity_aliases` as one polymorphic table vs. per-entity tables sharing
  the Go identity package — ADR-061 chose polymorphic; confirm the `person_aliases` migration + FTS re-point
  keeps F23 search parity before dropping the old table (RD11), else keep per-entity tables behind the shared
  code and revisit.
- **Q3 (design, non-blocking):** review-queue ordering within an entity group — by variation kind, by video
  count, or by name — decided in the design handoff.

## Timeline / routing

No hard deadline. Per the change-routing rules, before/with implementation:
1. ✅ **`/architecture`** — [ADR-061](../architecture/ADR-061-unified-entity-name-identity.md) (the spine,
   resolve order, per-entity normalize, keep-separate, two-path handling).
2. ✅ **`/design-handoff`** — [entity-identity-handoff.md](../design/entity-identity-handoff.md): review-queue
   banner + `/owner` Duplicates tab, `/tags` identity row-actions, editor near-miss prompt, studio/person alias
   panels, 3-skin QA. **Ratified (owner, 2026-07-05):** Duplicates tab layout = Option A (dense pair rows); Option B (cards) dropped.
3. ✅ **`/testing-strategy`** — [testing-strategy.md](../testing-strategy.md) §4 invariants + §9 F43 block, and
   the paired [entity-identity-qa-checklist.md](../design/entity-identity-qa-checklist.md): collision matrix (all
   three modes × scan/editor × entity), per-entity normalize scope, backfill auto-fold-vs-queue split, **studio
   merge survives re-derivation**, keep-separate non-nag, FTS parity after the `person_aliases` migration, alias
   search for studio/tag.
4. ◑ **`/security-review`** — **design-level sign-off (2026-07-05): clean.** New mutations (aliases/merge/
   rename/dismiss × 3 entities) are all `requireOwner` (ADR-030) with no new ungated surface; untrusted
   names reuse the F23 validation + existing sanitize perimeter; identity is **DB-only** (no `WriteBatch`,
   no `.holodex-tmp`, zero `/writeback` — F37/ADR-053 precedent); **no new SSRF/provider/asset surface**
   (provider identity stays ADR-055's id-first path); the backfill runs server-side at boot with no user
   input; queue/keep-separate leak no paths/secrets (ADR-028). **Re-run on the implementation diff** before
   merge (the gating + no-file-write assertions are pinned in QA §2.14–2.15).

**Slices:** **S1** identity core (migration 0022 — spine tables, shared FTS, delete-cleanup triggers,
**in-migration hard-pair fold**, `nameKey` indexes; `resolveOrCreateByName` + normalize registry; person
conformance) → **S2** studio + tag alias/merge/rename endpoints + routing + list actions ∥ **S3** search
(shared `entity_aliases_fts`, alias hits for studio/tag) → **S4** *(folded into S1's migration — the hard-pair
auto-fold must precede the unique-index build; the residual near-miss **queue seed** moves to S5)* → **P1:
S5** review queue (detection + near-miss seed, scan flagging, `/owner` tab + banner, editor soft-warning) →
**S6** QA + security. P0 = S1–S3; P1 = S5. Effort: **L**.
