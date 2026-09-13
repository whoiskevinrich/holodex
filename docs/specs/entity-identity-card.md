# Spec: Entity identity card — reference handle, unified external ids, films in the spine, edition, display name (F60)

**Status**: Draft
**Phase**: Phase 3 (Enrichment / curation foundation) — completes F43's spine and reuses ADR-051's
decision layer; no new subsystem
**Owner**: Project owner
**Date**: 2026-09-12
**Feature block**: **F60** — every entity (Person, Studio, Tag, Film — and Video) carries one uniform,
*surfaced* identity: a copyable `kind:id` **reference**, one **external-id** store consulted before
name, membership in the ADR-061 **name-identity spine**, and (for Person/Studio/Tag/Film) a
**display name** that is an ordinary ADR-051 decision on `name`. Files carry an **edition** as an
ordinary resolved video field.

**Epic**: [HOLODEX-373](https://whoiskevinrich.atlassian.net/browse/HOLODEX-373) ·
stories [374](https://whoiskevinrich.atlassian.net/browse/HOLODEX-374) reference ·
[375](https://whoiskevinrich.atlassian.net/browse/HOLODEX-375) external ids ·
[376](https://whoiskevinrich.atlassian.net/browse/HOLODEX-376) films in the spine ·
[377](https://whoiskevinrich.atlassian.net/browse/HOLODEX-377) edition ·
[378](https://whoiskevinrich.atlassian.net/browse/HOLODEX-378) display name

**Depends on** (all shipped):
- F43 name-identity spine ([entity-identity.md](entity-identity.md),
  [ADR-061](../architecture/ADR-061-unified-entity-name-identity.md)) — `entity_aliases`,
  `entity_keep_separate`, `identity_review_queue`, `resolveOrCreateByName`, `AliasPanel`. F60 adds
  Film to it and moves the external-id branch of the resolve order onto a shared table.
- Per-field source decisions ([field-source-of-truth.md](field-source-of-truth.md),
  [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md)) and the two-layer
  model ([ADR-090](../architecture/ADR-090-two-layer-entity-metadata-management.md)) — edition
  and display name are *precedence-layer* decisions on ordinary resolved fields.
- Two-tier field editing ([two-tier-field-editing.md](two-tier-field-editing.md)) as implemented by
  `SourceBadge` — the only edit affordance this spec adds fields to.
- F48 metadata extraction ([metadata-extraction.md](metadata-extraction.md)) — the `filename:`
  namespace and its auto-apply / review-queue routing; edition's filename source rides it.
- Writeback ([ADR-041](../architecture/ADR-041-metadata-writeback.md),
  the tri-state read-back of [ADR-093](../architecture/ADR-093-writeback-readback-and-tristate-in-sync.md)) — edition
  is one more row in `formatMap`.
- Provider identity ([ADR-054](../architecture/ADR-054-studio-external-id-dedup.md) /
  [ADR-055](../architecture/ADR-055-enrichment-unique-key-invariant.md) /
  [ADR-083](../architecture/ADR-083-provider-link-badge-person-studio.md)) — the two per-kind
  external-id tables this spec folds into one.
- F56/F59 films ([films-entity.md](films-entity.md), [film-provider-enrichment-ux.md](film-provider-enrichment-ux.md)) —
  the entity being brought into the spine.

**ADR**: pending (`/architecture`) — external-id unification, edition-as-field, curation-on-name;
amendment notes on ADR-051 (name was the one excluded field) and ADR-061 (Film, composite nameKey).
**Design**: [entity-identity-card-handoff.md](../design/entity-identity-card-handoff.md) +
[mockup](../design/entity-identity-card-mockup.svg) — ratified 2026-09-12 (OQ1 deep link, OQ2 keep 378).

---

## Problem Statement

Five recurring confusions trace to one gap: an entity's only surfaced identity is its name.
The owner and an agent can't refer to "the Fox studio" unambiguously; the provider id lives in two
tables with two meanings and only for two kinds; films have no aliases, no rename, and no
lookup-by-provider-id, so a file called *Superman II* can land on the wrong of two TMDB movies;
two files of one film (theatrical / Final Cut) are indistinguishable except by whatever is in
`video.title`; and a name can't be *shown* differently from the file's spelling without changing
the file. Each costs a round-trip of "which one do you mean?" — in chat, in review queues, in
enrichment — and the film cases produce wrong data, not just awkward data.

## Goals

1. **One handle.** Every entity payload carries `kind:id`; every entity route and MCP tool accepts
   it. A ref pasted into chat, Jira, or a tool call is unambiguous.
2. **One provider-identity store.** A provider id is consulted *before* name for all four kinds,
   and "which row is `tmdb:603`?" is a query for films and tags, not just people and studios.
3. **Films are ordinary spine members.** Alias, keep-separate, rename, near-miss queue — the same
   verbs Person/Studio/Tag have, with the one rule Film needs (year is part of the key).
4. **Editions are file facts.** A full-film file's edition is read from the container tag or the
   filename, settable by the owner, and written back through the tag — with zero new UI pattern.
5. **Display ≠ canonical, safely.** The owner can show "Robert Downey Jr." while the file keeps
   `robert downey jr`, and the file / aliases / search / writeback never see the display value.

## Non-Goals

- **Slugs / readable URLs.** The ref is strictly better: stable across rename, no redirect table.
  URLs stay numeric.
- **A Release / Version / Edition entity** between Film and Video. Edition is a *property of a
  file*. If grouping files by edition ever matters, group by the label.
- **An "alternate cut of" relation between films.** Superman II and The Richard Donner Cut are two
  films because TMDB says so; linking them is a later, separate feature.
- **Renaming files** to carry `{edition-X}`. Holodex writes tags, never filenames.
- **Reversing tag lowercasing** (migration 0034) — a storage regret, tracked as
  [HOLODEX-379](https://whoiskevinrich.atlassian.net/browse/HOLODEX-379), sibling of the epic.
- **A display-name column.** Display is a decision (RD9), not a field.
- **Refs on list cards / grids.** Detail pages only in v1; cards already link to the detail page.
- **A provider source for edition.** TMDB has no edition concept; `edition` has file sources only.

## Resolved Decisions

- **RD1 — Reference grammar.** `ref = kind ":" id` with `kind ∈ {person, studio, tag, film, video}`
  and `id` the numeric primary key. Produced by the server, never assembled by the client. Any
  route or tool parameter that takes an id also accepts a ref; a ref whose kind doesn't match the
  route is **400** (not 404 — the row may exist, the request is malformed).
- **RD2 — One table: `entity_external_ids(entity_type, entity_id, external_id)`.** Polymorphic,
  shaped like `entity_aliases`; `external_id` is the `"<provider>:<id>"` string and is globally
  unique **per entity_type**. It is the *identity* store: `resolveOrCreateByName` consults it
  before nameKey and alias for **all four** kinds. `person_external_ids` and `studio_external_ids`
  fold in and are dropped. `entity_enrichment.external_id` (the re-enrich memo) is dropped once
  `MatchExternalID` reads the new table; the memo semantics ("which provider id did we last enrich
  against") are recovered from the same row.
- **RD3 — Scan-time precedence survives.** External id → nameKey → alias → create, unchanged from
  ADR-061 §resolve order. A person with an external id and a differently spelled name in the file
  resolves to the existing row on rescan (the F23 lesson; testing invariant).
- **RD4 — Film joins the spine with a composite key.** `filmKey = (lower(trim(name)), year)`. Two
  films with the same name and different years are legal. An alias routes to a film only when its
  year matches the film's, or the alias carries no year **and exactly one** film has that nameKey;
  otherwise the name goes to the near-miss queue, never auto-routed.
- **RD5 — Film rename = the shared `RenameEntity` path**: old spelling → alias (`source='owner'`),
  year unchanged. Year edits stay on HOLODEX-317's control; both share the collision verdict.
- **RD6 — Edition is a canonical video field**, `replace`, single-value, manual allowed, sources in
  precedence order: container tag `Edition` (baseline) → `filename:edition` (F48 candidate). No
  provider source. It renders through the generic field row + `SourceBadge`; no new component.
- **RD7 — Filename grammar is strict Plex** in v1: `{edition-<text>}` anywhere in the basename,
  `<text>` trimmed, case preserved. Looser forms (`(Director's Cut)`, `- Final Cut`) are **not**
  parsed; they may be added to the near-miss queue by a later pattern. An exact match is
  high-confidence and follows the **F48 auto-apply flag** like every other `filename:` candidate —
  no special rule for edition.
- **RD8 — Writeback key = `Edition` on both backends.** Matroska/WebM: `EDITION` in the GENERAL
  tag block via mkvpropedit. MP4/MOV: `QuickTime:Edition` via exiftool (a native ItemList/Keys tag
  — verified against exiftool 13.59). `Subtitle` was considered and rejected: it is already the
  tagline's key (`tags.go:101`). The read-back key is `Edition`; ADR-093's startup WARN names it
  if the live mappings file lacks it.
- **RD9 — Display name = a source decision on `name`.** Lift the per-kind rejection
  (`person_decisions.go:117`, `studio_fields.go:103`, `film_fields.go:236`, tags). `name` becomes an
  ordinary resolved field with sources: file baseline (the canonical column), each provider's
  spelling, custom. The **rendered** name is the resolved value. The **canonical** column is
  untouched by any decision and remains the file / writeback / identity / alias-routing truth.
- **RD10 — Two verbs, two existing affordances.** *Display as* = the name field's `SourceBadge`,
  on an "In files as `<canonical>`" line shown only when resolved ≠ canonical. *Rename in files* =
  `NameEditControl`'s pencil, unchanged, prefilled with the **canonical** value. No new buttons.
  Ratified in design review (handoff OQ2).
- **RD11 — Set edition from the film page is a deep link**, `/media/{id}#field-edition` with the
  `SourceBadge` auto-expanded — one curation mount, one navigation. Inline editing on the film page
  was rejected (handoff OQ1).
- **RD12 — `SourceBadge` renders its badge for any curatable field**, not only multi-source ones.
  A filename-only edition must still offer the custom chip. This applies to every single-source
  curatable field and is recorded in `curation/CLAUDE.md`.

## User Stories

**Owner**
- As the owner, I want every entity page to show a copyable `film:42`-style reference so that I can
  name an entity to an agent, a ticket, or a tool call without a "which one?" round-trip.
- As the owner, I want a film matched to a provider to *stay* that film on rescan and re-enrich
  so that *Superman II* (1980) and *The Richard Donner Cut* (2006) never swap or merge.
- As the owner, I want to alias, rename, and keep-separate films exactly as I do people and
  studios so that a film's file spelling isn't its only name.
- As the owner, I want a full-film file's edition read from its tag or filename, and to set or
  correct it myself, so that two files of one film are told apart on the film page.
- As the owner, I want a typed edition written into the file's tag on request so that it survives
  a database rebuild and is visible to other players.
- As the owner, I want to show a person's name the way the provider spells it — or my own way —
  without renaming the person in the files, so that display fidelity doesn't cost file churn.
- As the owner, I want it to be obvious which control changes the *display* and which *renames
  for real*, so that I never rename by accident.

**Visitor**
- As a visitor, I want the reference chip and the edition pill visible so that I can report
  exactly which entity or file I mean.

**Agent / MCP client**
- As an agent, I want every entity payload to carry `ref` and every entity-taking tool to accept
  it so that I can act on an entity the owner named without a name lookup.

## Requirements

### Must-have (P0)

**374 — Reference**
- [ ] Every entity JSON payload (person, studio, tag, film, video — detail *and* list items) carries
  `ref`.
- [ ] Every `{id}` route segment and every MCP tool id argument accepts a ref; kind mismatch → 400
  with a body naming the expected kind.
- [ ] `RefChip` mounted as the last item of the meta line on all five detail pages (handoff §1a);
  click/Enter copies; "Copied" for 1.5 s; `aria-live`; clipboard-denied fallback selects the text.
- [ ] Given `GET /people/1234` and `GET /people/person:1234`, the bodies are byte-identical.

**375 — External ids**
- [ ] Migration: `entity_external_ids` created; `person_external_ids` + `studio_external_ids`
  folded in and dropped; per-kind AFTER DELETE cleanup triggers; `entity_enrichment.external_id`
  dropped after readers move.
- [ ] `resolveOrCreateByName` / `identityQueryByType` consult `entity_external_ids` for all four
  kinds; `externalIDTable()`'s person/studio special-casing is gone.
- [ ] Film enrichment adoption records the matched provider id; `GetFilmByExternalID` exists and
  re-enrich uses it.
- [ ] Given a person with `tmdb:6384` and a file whose person tag is spelled differently, when the
  library rescans, then no new person row is created and the link lands on the existing person.
- [ ] The ADR-083 external-id badge reads the new table with no visible change.

**376 — Films in the spine**
- [ ] `entity_aliases`, `entity_keep_separate`, `entity_alias_suppressions`,
  `identity_review_queue` accept `entity_type='film'`; cleanup trigger on `films`.
- [ ] `ux_films_namekey` unique index on `(lower(trim(name)), year)`.
- [ ] `RenameEntity` supports film; `canonicalTable('film')` returns `films`; old spelling → alias.
- [ ] Alias routing per RD4; the ambiguous no-year case queues.
- [ ] Provider alternative titles land as `source='provider'` aliases; `AliasPanel` mounts on the
  film page.
- [ ] `NameEditControl` on the film title with `MergeOfferCard` as the verdict.

**377 — Edition**
- [ ] `metadata-mappings.yaml.example` gains `edition` per RD6; the registry labels it "Edition".
- [ ] Extractor surfaces the `Edition` container tag as the `file` baseline for MKV/WebM/MP4/MOV.
- [ ] F48 filename parser recognises `{edition-<text>}` per RD7 and emits `filename:edition`.
- [ ] Media page renders the Edition row via the generic field row; `SourceBadge` renders per RD12.
- [ ] `formatMap` gains `edition` per RD8; `WritebackFormDialog` lists it; read-back reports
  `in_sync` after a write + re-extract.
- [ ] Video summary payload carries resolved `edition`; film page Full-film rows render the pill;
  empty + owner → "+ Set edition" link per RD11; landing auto-expands the badge.
- [ ] Given a file with tag `Edition=Final Cut` and filename `{edition-Theatrical}`, the resolved
  edition is `Final Cut` with provenance `file · tag`.
- [ ] Given a file with neither, when the owner types "Director's Cut" and confirms, then the value
  persists as a curated decision and the film page pill reads "Director's Cut".

**378 — Display name**
- [ ] The rejection of source decisions on `name` is lifted for person, studio, tag, film.
- [ ] The rendered name on each detail page is the resolved `name`; the "In files as" line + badge
  appear only when resolved ≠ canonical (RD10).
- [ ] Search matches canonical, resolved, and aliases.
- [ ] Given a standing display decision, when the owner opens the rename pencil, then the input is
  prefilled with the canonical value; when a writeback batch is built, the payload carries the
  canonical value.
- [ ] Given the owner clears the decision, the header returns to the canonical spelling and the
  "In files as" line disappears.

### Should-have (P1)

- [ ] `--font-mono` token in `app.css` `@theme inline` so the chip renders in a monospace stack
  across skins (374 decides; falls back to `font-ui`).
- [ ] Edition suggestion list in the custom input (Theatrical · Director's Cut · Extended · Unrated
  · Final Cut · Remastered) — a hint, never a constraint.
- [ ] Near-miss pattern for loose edition forms feeding the queue (RD7's second half).

### Future considerations (P2)

- Group a film's Full-film list by edition when a film has more than one file per edition.
- Refs in list-card context menus ("Copy reference").
- Provider `also_known_as` for films drives search the way it does for people (once 376 lands the
  rows, search indexing is the only missing piece).

## Behavior detail

### Reference (RD1)

```
ref        := kind ":" id
kind       := "person" | "studio" | "tag" | "film" | "video"
id         := [1-9][0-9]*
```

Parsing lives in one Go helper (`internal/api/ref.go`) used by every `{id}` route and by the MCP
tool argument decoder. Bare numeric ids remain valid everywhere. The MCP server echoes `ref` in
every entity result.

### Resolve order (RD2/RD3) — one function, four kinds

`resolveOrCreateByName(kind, name, externalIDs...)`:
1. any supplied external id present in `entity_external_ids` for `kind` → that row
2. `nameKey` (film: `filmKey`, RD4) → that row
3. alias key → its canonical row (film: year rule, RD4)
4. create; record supplied external ids

### Edition (RD6–RD8)

| Layer | Key / grammar | Notes |
|---|---|---|
| File baseline | container tag `Edition` | MKV/WebM GENERAL `EDITION`; MP4/MOV `QuickTime:Edition` |
| Candidate | `filename:edition` from `{edition-<text>}` | F48 routing, auto-apply per flag + confidence |
| Decision | curated custom value | ADR-051; DB only |
| Writeback | `formatMap[*]["edition"] = "Edition"` (Matroska/WebM) · `"QuickTime:Edition"` (MP4) | one WriteBatch per file, atomic — non-negotiable |

The film page never resolves edition itself; it renders the value the video summary carries.

### Display name (RD9/RD10)

`name` is resolved like any replace field. The canonical column is the `file` baseline source. The
resolver output for `name` is what headers, cards, and search-result rows render. Nothing else
reads the resolved value: alias routing, nameKey, `RenameEntity`, writeback, and the MCP
`name` field all read the canonical column. Tags: sources are file + custom only. Films: gated on
376 landing `NameEditControl` on the title.

## Data model

Migration numbers are assigned at implementation time against `origin/main` (0045 is the latest
on main at time of writing; main moves fast).

```sql
-- 375
CREATE TABLE entity_external_ids (
    entity_type TEXT    NOT NULL,   -- 'person' | 'studio' | 'tag' | 'film'
    entity_id   INTEGER NOT NULL,
    external_id TEXT    NOT NULL,   -- '<provider>:<id>'
    PRIMARY KEY (entity_type, external_id)
);
CREATE INDEX idx_entity_external_ids_entity ON entity_external_ids(entity_type, entity_id);
-- fold: INSERT ... SELECT 'person', person_id, external_id FROM person_external_ids; same for studio
-- then DROP TABLE person_external_ids, studio_external_ids; AFTER DELETE triggers per kind
-- ALTER TABLE entity_enrichment DROP COLUMN external_id  (after readers move; may be a follow-up migration)

-- 376
CREATE UNIQUE INDEX ux_films_namekey ON films(lower(trim(name)), year);
-- entity_aliases.alias_key CASE gains no film branch (film uses the person/studio normalisation)
-- AFTER DELETE ON films → clean entity_aliases / entity_keep_separate / entity_alias_suppressions / identity_review_queue

-- 377: no schema — the field lives in the mapping + shadow store + curation tables
-- 378: no schema — the decision lives in the existing per-field decision table
```

## API

| Change | Route / tool | Notes |
|---|---|---|
| `ref` on every entity payload | all `GET` entity routes, MCP `get_*` / `search_*` results | additive |
| Ref accepted as id | every `/{kind}/{id}` route, MCP id args | 400 on kind mismatch |
| `GET /films/by-external-id/{ns}:{id}` | owner-gated | re-enrich + adoption lookup |
| Film alias / keep-separate / rename | mirror studio's `entity_identity.go` routes for `film` | owner-gated |
| `edition` in resolved fields | `GET /media/{id}` (`resolved[]`), video summaries carry the value | |
| `edition` in writeback | `POST /media/{id}/writeback` accepts it like any canonical | |
| `name` accepts a decision | the per-kind decision endpoints stop returning the rename hint | owner-gated |

No endpoint changes its auth posture; all mutations stay behind `requireOwner`.

## UI

Fully specified in the [design handoff](../design/entity-identity-card-handoff.md); summary:
`RefChip` (new, §1), Edition row = generic field row + `SourceBadge` (§2), Full-film pill + dashed
Set-edition link (§3), "In files as" line + name `SourceBadge` (§4). Three-skin QA per handoff §8.

## Success Metrics

Personal-server scale — these are checks, not dashboards.

- **Leading:** after 374 ships, agent sessions on this repo refer to entities by ref, not name
  (spot-check the next three worklogs); zero "which one?" clarifications for entity references.
- **Leading:** the F23 precedence test and the film composite-key tests are green on every CI run.
- **Leading:** the read-only production probe for edition-bearing full-film titles
  (`cut|edition|extended|unrated|remaster`) run *before* 377 sizes the parser; run *after* 377,
  every hit resolves to a non-empty `edition`.
- **Lagging:** a full re-enrich of the film library moves **zero** films between TMDB ids.
- **Lagging (378):** the owner uses Display-as at least once without an accidental rename in the
  first month; if a rename is undone within a minute of a display attempt, 378 is cut.

## Open Questions

- **(engineering, non-blocking)** Does exiftool surface an unknown Matroska SimpleTag named
  `EDITION` as `Edition` on read, or does the extractor need an explicit tag-name map entry? Verify
  on the testbed before wiring the mapping; ADR-093's startup WARN is the safety net either way.
- **(engineering, non-blocking)** Whether `entity_enrichment.external_id` is dropped in 375's
  migration or a follow-up once every reader is moved — decide by the size of the reader diff.
- **(owner, non-blocking)** Whether the near-miss queue should get loose edition patterns in this
  epic (P1) or wait for probe evidence.

## Timeline / routing

Slices ship in this order on the epic's Draft PR
([#332](https://github.com/whoiskevinrich/holodex/pull/332)); each is independently mergeable
behind the epic and none depends on a later one:

1. **374** reference — zero schema, unblocks every later conversation
2. **375** external ids — the precedence test first
3. **376** films in the spine
4. **377** edition
5. **378** display name — lowest; kill criterion stands (handoff QA 4.3)

Gates per `.claude/CLAUDE.md`: spec (this doc) → ADR (pending) → design (done) → testing
strategy (per slice) → security review on the implementation diff (new writeback key; new
mutation surface on `name`; new film identity routes). Jira: children are swept by hand with the
epic — In Review when the PR is marked ready, Done on merge.
