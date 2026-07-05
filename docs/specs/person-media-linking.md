# Spec: Owner-authored person & studio ↔ media links, with file writeback (F39)

**Status**: Draft — decisions locked with the owner 2026-07-04 via question cards. Needs an
ADR + `/security-review` before/at implementation.
**Feature block**: **F39** — let the owner *link a person to a video in a role* (and, the same way,
*link/update a studio*) from owner view, persist that link to the file, and (the load-bearing part)
migrate `video_people` from scan-time raw-extraction to **resolved-value derivation** so
owner-authored links and file truth never disagree. **Person and studio ship together (both P0)** —
studio derivation already exists, so its share is the owner-view picker + surfaced writeback, built on
the same generalized machinery. Actor is the only person role built in v1; role and entity are both
extensible.
**Phase**: F36 fast-follow — the fourth entity slice after `person` (F37) and `studio` (F38),
now closing F37's explicit **no-writeback** gap for people *and* amending F38's deliberate
**no-studio-writeback** non-goal (ADR-053) for the manual-link case.
**Owner**: Project owner
**Date**: 2026-07-04

**Depends on** (all shipped):
- the F36 decision + curation model ([ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md),
  [metadata-curation.md](metadata-curation.md)/[ADR-048]) — a "link" is a **curation add** on a person-typed field
- the entity-agnostic resolver ([ADR-052](../architecture/ADR-052-baseline-source-contract.md), `ResolveFields`)
- **F38 studio's resolved-value link derivation** ([ADR-053](../architecture/ADR-053-studio-entity-and-resolved-link-derivation.md),
  `RelinkVideoStudios`) — the *exact* pattern this spec applies to people, and the code to generalize
- person alias routing ([ADR-036](../architecture/ADR-036-person-alias-search-indexing.md),
  `resolveOrCreatePerson` in [`internal/repo/aliases.go`](../../internal/repo/aliases.go)) — reconcile reuses it, so alias/homonym handling is free
- metadata writeback ([ADR-041], `internal/writeback`, owner-gated `POST /media/{id}/writeback`) —
  **`actors → Artist` is already a mapped, sanitized write target** ([`internal/writeback/tags.go`](../../internal/writeback/tags.go)); this feature adds a new *input* to it, not a new perimeter
- field extraction round-trip: writeback emits `Artist` comma-delimited, and the scanner splits it
  back (`splitMulti`, [`internal/metadata/extractor.go`](../../internal/metadata/extractor.go)) — round-trip **verified closed**
- the owner gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md), `requireOwner`)
- the `EnrichPicker` roving-tabindex control (reused for the person link picker)

**ADR**: [ADR-056](../architecture/ADR-056-person-link-resolved-derivation.md) (Proposed) records the two decisions that rise to ADR level:
(1) **`video_people` migrates to resolved-value derivation** (replacing scan-time raw-extraction,
mirroring ADR-053) with a **`role` column derived from the source person-typed field**, and
(2) the **`entity: person` field marker** that makes derivation + writeback + the link picker
generic over person-typed fields. Extends ADR-053; relates ADR-036 (aliases, now *used* not
deferred), ADR-041 (writeback input), ADR-013 (mapping). Touches a **file-write path fed by
owner-authored data** → `/security-review` before merge.

**Related / supersedes-in-part**:
- **[F32 video-credits-people.md](video-credits-people.md)** (Draft, not built) — **this spec becomes
  F32's data-model foundation.** F32's proposed `video_people.role` column and resolved-derivation
  concerns are absorbed here; F32 **rebases** to its unique remainder: provider-driven **headshots**
  + **person external-id de-dup** + billed **order**, layered on top of the derivation this spec ships.
  The two specs must not each own a `video_people` writer (the two-pattern hazard ADR-053 warns of).
- [people-source-of-truth.md](people-source-of-truth.md) (F37) — the `personBaseline`/`·record`/decision-endpoint precedent
- [studio-entity.md](studio-entity.md) (F38) — the sibling that proved the derivation pattern

---

## Problem Statement

The owner can *see* a video's people (the F30 actor/director chips link to person pages) but
cannot **assert** one: if a film's cast is wrong or incomplete, there is no owner-view action to
say "this person is an actor in this video," and nothing writes that assertion back to the file so
it survives outside Holodex. Worse, the plumbing that would carry such an assertion is structurally
unable to: `video_people` is derived from **raw file extraction at scan time**
([`replaceAssociations`](../../internal/repo/repo.go), `ex.People`), so a person the owner curates
into the `actors` field — but that isn't in the file yet — appears on the media detail while
"everything with this person" navigation silently omits them. That is the exact display-vs-truth
split F36 exists to kill, one layer down — the same split ADR-053 fixed for studio and explicitly
flagged people as still exposed to.

## Goals

1. **Owner can link a person to a video in a role, from owner view** — pick an existing person (or
   create one) and attach them as an **Actor** (v1), as a first-class curation action.
2. **Links agree with what the owner sees** — `video_people` derives from the **resolved**
   person-typed fields, so a curated/adopted/enriched person re-links with no rescan and the person
   page, related-shelf, and filters group by the displayed truth.
3. **The link persists to the file** — the owner can write the resolved `actors` field back to the
   media file (`Artist`, canonical name, comma-delimited) via the **existing** writeback action, and
   a later re-scan reads it back to the same person with no duplicate (round-trip closed).
4. **Role-extensible at near-zero cost** — role lives in the canonical *field* via an `entity: person`
   marker; adding "director/writer/…" later is a mapping + marker edit, no entity-layer change.
   Only **Actor** is wired in v1.
5. **One derivation pattern, one writer** — `video_people` has exactly one writer after this
   (`RelinkVideoPeople`), ideally the same generalized reconcile as studio; F32 rebases onto it.

## Non-Goals

- **A new writeback mechanism or perimeter.** `actors → Artist` already writes through the existing
  owner-gated, sanitized `POST /media/{id}/writeback` (ADR-041). This spec adds owner-curated links as
  an *input*; it does **not** add a write target, a new file format path, or auto-writeback. *(Why:
  the mechanism exists and is reviewed; reusing it is the whole point.)*
- **Auto-writeback on link.** Linking is a curation decision; persisting is a **separate, explicit**
  owner action. No link ever mutates a file as a side effect. *(Why: keeps curation cheap and
  reversible; a failed file write never blocks a link — the locked coupling decision.)*
- **Roles beyond Actor in v1.** `director` mapping already exists and will ride the same model, but
  wiring/QA-ing multiple roles is deferred; the marker makes it a data edit later. *(Why: the use
  case needs Actor now; extensibility is designed in, not built out.)*
- **Provider headshots / external-id de-dup / billed order** — that is **F32**, rebased onto this
  spec's derivation. Not duplicated here.
- **Person identity ops beyond what exists** — rename/alias/merge already ship (ADR-036/F23) and are
  *reused*; no new identity surface.
- **Writing role into the file.** The file has no role field; extraction is role-flat by design
  (`Artist`/`Director` both fold into `ex.People`). Role is a Holodex-side derivation from the
  field a name sits in; writeback flattens each role-field to its own tag (`actors→Artist`,
  `director→Director`) but the *role label* is not persisted to the file.

## Resolved Decisions

*(Locked with the owner 2026-07-04 via question cards.)*

- **RD1 — Link and persist are decoupled.** "Link a person" = a **curation add** on a person-typed
  field (the F30 mechanism). "Persist to file" = the **existing** writeback action, invoked
  separately. One never triggers the other.
- **RD2 — `video_people` migrates to resolved-value derivation, *replacing* raw-extraction.** A new
  `RelinkVideoPeople(videoID)` becomes the **sole writer** of `video_people`, unioning the resolved
  value(s) of every person-typed field, resolve-or-creating each via `resolveOrCreatePerson` (alias
  routing free), replacing the video's rows, and **pruning** any person left with zero links. The
  raw-extraction people path in `replaceAssociations` is removed. Reconciled on the same trigger
  surface as studio: scan upsert, enrich completion, person-field decision set/clear, person-field
  curation change. Never at read time.
- **RD3 — `video_people` gains a `role`, derived from the source field; unset is a first-class
  value.** PK becomes `(video_id, person_id, role)`, so one person can be both actor and director on
  one video. `role` is set from the person-typed field a resolved name came from (`actors`→`actor`,
  `director`→`director`); a person-typed field that declares **no** role yields an **unset** role
  (not every credit has a meaningful role — a bare `Artist`/music-video link is just "associated").
  Unset is stored as an **empty sentinel `''`, not SQL `NULL`** — SQLite treats `NULL`s as *distinct*
  in a composite PK, which would let two `(v, p, NULL)` rows coexist and break the one-row-per-role
  guarantee; `''` keys cleanly and is presented as "unset" at the edge. This **unifies with F32** and
  is the F32 data-model foundation.
- **RD4 — Person-typed fields are marked, and derivation/writeback/link-picker are generic over
  them.** A field-registry/mapping marker (`entity: person`, carrying the role identity) declares
  which canonical fields hold people. `RelinkVideoPeople`, the link picker's target set, and the
  extractor's `peopleKeys` all read from this marker. **Both `actors` and `director` are marked
  for *derivation* in v1** (lossless cutover — RD9); only the **Actor** link-picker + writeback QA are
  in v1 UI scope. Further roles follow by adding the marker.
- **RD5 — Writeback writes the canonical name, always.** Flattening the resolved `actors` field to
  `Artist` uses each linked person's **canonical** name (not a file-original alias). Deterministic;
  keeps the re-scan round-trip stable (canonical written → re-scanned → `resolveOrCreatePerson`
  matches the same entity). Comma-delimited, matching the verified extractor split.
- **RD6 — One reconcile, generalized (now).** Generalize `RelinkVideoStudios` into a single
  `RelinkVideoEntity` (or a shared core) covering studio + person-typed fields in *this* change, so
  there is one derivation implementation, not two. Studio behavior must be unchanged
  (regression-guarded). *(Decided now, not deferred: a second parallel writer is the exact hazard
  ADR-053 warns of.)*
- **RD7 — One-time backfill.** After the migration, a startup pass derives `video_people` (with
  roles) for all active videos via `RelinkVideoPeople`, recorded as a System Activity job run
  (idempotent), so existing libraries populate without a manual rescan — and so the raw-extraction→
  derivation cutover doesn't drop any existing link.
- **RD8 — Orphaned people prune after a 30-day grace, and never if authored.** A person whose last
  link is removed is **not** deleted immediately: `RelinkVideoPeople` stamps `orphaned_at` (cleared
  the instant any link returns). A periodic System Activity sweep deletes people orphaned **> 30
  days** — *except* people carrying **authored identity** (aliases, a merge history, a curated
  headshot, or any manual field edit/decision), which are **never** auto-pruned and require explicit
  owner deletion. Owner rationale: file maintenance can pull a person's only file offline for a while;
  the grace window plus the authored-identity guard ensure no custom work is lost. *(Studio keeps
  immediate prune — its identity is derived, not authored.)*
- **RD9 — The derivation source set must cover the extraction source set (lossless cutover).** Because
  P0-3 removes the raw-extraction writer, every file tag that used to create a `video_people` row must
  be reachable through a marked person-typed field, or those links vanish at cutover. Therefore:
  (a) `actors` **and** `director` are both marked `entity: person` from day one (derivation, not UI);
  (b) the `actors`/`director` field `sources` are reconciled with the extractor's `peopleKeys` —
  `Artist, Cast, Actor(s), Performer, AlbumArtist` → `actors`; `Director` → `director`; `Producer`
  deferred (tracked, may become its own role/field); (c) the backfill (P0-4) is **guarded to not
  reduce the active link count** vs. pre-migration (modulo intended alias de-dup). Migration order is
  fixed: migrate → backfill → serve; the add-column default is **unset `''`** (RD3), an honest value
  for the pre-derivation moment (old raw-extraction links were role-flat anyway), which the backfill
  then overwrites with field-sourced roles — so there is no "wrong default" to hide.
- **RD11 — Studio parity (P0, ships with people), amending F38's no-writeback non-goal.** The owner gets the *same*
  approach for studios: an owner-view **studio link picker** (search existing `studios` / inline
  create) that curates the `studio` field, and **writeback** of the resolved `studio` → `Publisher`
  (already in the writeback map; canonical name). Studio↔video derivation already exists
  (`RelinkVideoStudios`, folded into `RelinkVideoEntity` by RD6), so this slice is picker + surfaced
  writeback, not new derivation. This **amends [ADR-053](../architecture/ADR-053-studio-entity-and-resolved-link-derivation.md)'s
  explicit "no studio writeback / no write button" non-goal** — recorded in ADR-056. Studios keep
  **immediate** prune (RD8's grace is person-only: studio identity is derived, nothing authored to
  lose). The `entity:` marker generalizes to `entity: studio` for picker/writeback targeting.
- **RD10 — Inline bare-create in the picker.** Typing a name that matches no person creates a
  **name-only** person and links them in one step; enrichment/curation happen later via existing
  tools. No detour to a separate create flow.

## User Stories

**Owner — assert a person**
- As the owner, on a video's page I want to link a person as an actor — searching my existing
  people and picking one (or creating a new one) — so a wrong or missing cast is fixable in place.
- As the owner, after linking, I want the person to appear everywhere that video's people appear
  (person page, related shelf, filters), with no rescan, so the link is "real" immediately.

**Owner — persist to the file**
- As the owner, I want to write the video's cast back to the file so the metadata travels with the
  file into other tools and survives a library rebuild.
- As the owner, I want a re-scan of a written file to re-link the *same* people, not create
  duplicates, so writeback is safe to run repeatedly.

**Owner — trust the grouping**
- As the owner, when I adopt a provider's spelling of an actor (a decision), I want the person page
  and filters to follow the displayed value with no rescan — the same guarantee studio already gives.

**Owner — assert a studio (P1, RD11)**
- As the owner, I want to link/correct a video's studio from owner view the same way I link a
  person — pick an existing studio or create one — so studio curation is a first-class action, not a
  raw-tag edit.
- As the owner, I want to write the corrected studio back to the file (`Publisher`) so it travels
  with the file, and a re-scan re-links the same studio.

**Visitor — unchanged**
- As a visitor, I keep seeing the video's people and can browse "everything with this person"; the
  set is now derived from resolved truth rather than raw file tags (behavior-compatible for
  un-curated videos).

## Requirements

### Must-have (P0)

- **P0-1 — Migration: `video_people` role + PK, and `people.orphaned_at`.** Add `role TEXT NOT NULL
  DEFAULT ''` (`''` = unset, RD3 — never SQL `NULL`, to keep the composite key sound in SQLite); change
  PK to `(video_id, person_id, role)`; keep `ON DELETE CASCADE` and the `person_id` index. Add
  `people.orphaned_at TIMESTAMP NULL` (RD8). Down migration reverses (collapsing duplicate-role rows to
  the composite `(video_id, person_id)`; dropping `orphaned_at`). Append-only, numbered `NNNN`, with a
  matching `.down.sql`.
  - Given a video with the same person as actor and director, Then two rows exist, distinct by `role`.
  - Given a person linked via a role-less person-typed field, Then their row has `role = ''` (unset),
    and re-deriving is stable (no spurious second row).
- **P0-2 — `RelinkVideoPeople` (RD2/RD3/RD6), sole writer.** Resolve the video's fields; for each
  **person-typed** field (RD4), take its resolved value(s), trim, drop empties, `resolveOrCreatePerson`
  each (alias-routed), and upsert `(video_id, person_id, role=field's role)`; delete stale rows; mark
  any person left with zero links as orphaned (deleted only after a grace period — RD8). One
  transaction, invoked **after** each triggering write commits. Uses the generalized `RelinkVideoEntity`
  form (RD6).
  - Given file `Artist="Alice, Bob"` and no curation, When scanned, Then Alice and Bob link as `actor`.
  - Given the owner curates actor "Carol" onto the video (not in the file), Then Carol links as `actor`
    **without** a rescan and appears on her person page.
  - Given the owner clears that curation, Then Carol's link is removed and, if she has no other links,
    she is stamped `orphaned_at` (not deleted); a link returning before the sweep clears the stamp (RD8).
  - Given a soft-deleted video, Then it stops counting toward its people (parity with today).
- **P0-3 — Remove the raw-extraction people writer.** `replaceAssociations` no longer writes
  `video_people`; scan calls `RelinkVideoPeople` post-commit instead. Tags remain raw-extraction
  (documented asymmetry, per ADR-053). No path writes `video_people` except `RelinkVideoPeople`.
- **P0-4 — One-time backfill (RD7/RD9), loss-guarded.** Startup pass derives `video_people` for all
  active videos via `RelinkVideoPeople`, as an idempotent System Activity job run. **Guard:** the
  post-backfill active link count must be ≥ the pre-migration count (modulo intended alias de-dup); the
  job records both counts and fails loudly on unexplained shrinkage (catches a source-set gap, RD9).
- **P0-5 — `entity: person` field marker (RD4/RD9).** The registry/mapping marks person-typed
  canonical fields (with role). **Both `actors` and `director` are marked in v1** (derivation
  fidelity). `RelinkVideoPeople`, the extractor's `peopleKeys` source, and the link-picker target set
  derive from the marker — no field name hardcoded. The `actors`/`director` field `sources` are
  reconciled with `peopleKeys` so no extraction source is orphaned (RD9).
- **P0-6 — Owner-view link action.** On the video page, owner-only: add a person to a person-typed
  field via a search-backed picker (existing people by name/alias; option to create). The action is a
  **curation add** on that field (reuses the F30 curation endpoint); the picker only supplies the
  chosen **canonical name** as the curated value. Reuse `EnrichPicker`'s roving-tabindex a11y pattern.
  A name matching no person **inline bare-creates** a name-only person and links them in one step (RD10).
  - Given the owner picks existing person "Alice", Then `actors` is curated to include "Alice" and
    `RelinkVideoPeople` links her as `actor`.
  - Given the owner types a new name and confirms create, Then a person is created and linked.
  - Given a visitor, Then the link action is not rendered/authorized (`requireOwner`).
- **P0-7 — Persist to file (existing writeback, canonical name — RD5).** The owner writes the resolved
  `actors` back via `POST /media/{id}/writeback`; values are the linked people's **canonical** names,
  comma-delimited to `Artist` (per container map). No new endpoint. A subsequent re-scan re-links the
  same people (round-trip).
  - Given actors [Alice, Bob] written to `Artist`, When the file is re-scanned, Then exactly Alice and
    Bob re-link as `actor` (no duplicates, no "Alice, Bob" single person).
- **P0-8 — Studio unchanged.** Generalizing the reconcile (RD6) must not change any F38 studio
  behavior; the derivation matrix regression-guards it.
- **P0-9 — Orphan sweep job (RD8).** A periodic System Activity job deletes people with
  `orphaned_at < now() − 30d` that carry **no** authored identity (no aliases/merge/curated
  headshot/manual edit or decision); authored orphans are skipped and reported, never deleted.
  Idempotent; observable in activity history.
  - Given an orphaned person with a curated headshot, When the sweep runs after 40 days, Then she is
    **kept** (authored-identity guard).
  - Given an orphaned plain person, When the sweep runs after 40 days, Then she is deleted; before 30
    days, kept.
- **P0-10 — Studio link picker (RD11).** Owner-view picker on the `studio` field: search existing
  `studios` (name; inline bare-create), submitting the chosen studio's name as a curation add. Reuses
  the person picker's component/a11y; `RelinkVideoStudios` (via `RelinkVideoEntity`) already re-derives
  `video_studios`. `requireOwner`.
  - Given the owner picks studio "Acme Films", Then the `studio` field is curated to it and the video
    links to that studio entity with no rescan.
- **P0-11 — Studio writeback (RD11), amending ADR-053.** Surface writeback of the resolved `studio` →
  `Publisher` (already in the writeback map; **canonical** studio name) via the existing owner-gated
  `POST /media/{id}/writeback`. No new endpoint/perimeter. `/security-review` covers this as the
  studio counterpart of the person writeback (it reverses F38's no-writeback non-goal).
  - Given studio "Acme Films" written to `Publisher`, When the file is re-scanned, Then the video
    re-links to the same studio entity (round-trip).

### Should-have (P1)

- **P1-1 — Role badge on the person page.** Use the new `role` to badge/group a person's videos
  (acted in / directed). Cheap once RD3 lands; the payoff that motivated the role column.
- **P1-2 — `director` link picker + writeback QA.** Director is already *derived* in P0 (RD9); P1
  adds its owner-view link affordance and QAs `director→Director` writeback end-to-end.
- **P1-3 — Link affordance discoverability.** Inline "add person" affordance on the actor chip row,
  not only in a curation panel.

### Future considerations (P2)

- **P2-1 — F32 rebased**: provider headshots + person external-id de-dup + billed `order`, layered on
  this derivation (its own spec/ADR).
- **P2-2 — Bulk link** (link a person across many videos) — a library op, out of scope for the
  per-video flow.
- **P2-3 — Additional roles** (writer, composer, producer) — each a mapping + marker + file-tag entry.
- **P2-4 — MCP parity** for owner-authored links (rides the deferred MCP-parity item).

## Behavior detail

### Link derivation (RD2/RD3)
`RelinkVideoPeople(videoID)` (ideally `RelinkVideoEntity` over person-typed + studio fields): load
the video's baseline + enrichment + curation + decisions (the media-detail path), resolve each
**marked** person-typed field, and for each resolved name emit a desired
`(person_id via resolveOrCreatePerson, role=field.role)` row; **replace** the video's `video_people`
rows with exactly that set (delete stale, insert missing), then **prune** any person with zero
remaining links. One write transaction, post-commit after the trigger. Triggers: `UpsertVideo`
(scan), enrich completion, decision PUT/DELETE on a person-typed field, curation add/suppress/clear
on a person-typed field. Never at read time. `resolveOrCreatePerson` supplies alias routing and the
"never auto-merge same-name" homonym rule unchanged (ADR-036).

### Writeback round-trip (RD5) — verified
Write: resolved `actors` → `Artist` (comma-delimited), values = linked people's **canonical** names,
through the existing sanitized, owner-gated `POST /media/{id}/writeback`
([`internal/api/writeback.go`](../../internal/api/writeback.go), [`tags.go`](../../internal/writeback/tags.go)).
Read: the scanner's `splitMulti` splits `Artist` back into `ex.People` (test-proven:
`"Audrey Tautou, Mathieu Kassovitz"` → 2). Because P0-3 makes derivation the writer, the re-scanned
names flow through the resolved `actors` field → `RelinkVideoPeople` → same entities. Round-trip is
closed **only** if writeback uses canonical names (RD5) and the delimiter matches the extractor's
split — both asserted in tests.

### Resolution / entities
No change to the resolver core or `personBaseline`. `video_people` is a *derived index*, exactly as
`video_studios` is; the person page/related shelves read it directly.

## API

No new endpoints. Reuses:
```
POST   /api/v1/media/{id}/curation           add person (canonical name) to a person-typed field   (requireOwner)
POST   /api/v1/media/{id}/curation/clear     remove a curated person                               (requireOwner)
PUT/DELETE /api/v1/media/{id}/fields/{canonical}/decision   adopt/clear a source for a person field (requireOwner)
POST   /api/v1/media/{id}/writeback          persist resolved fields (incl. actors→Artist) to file  (requireOwner)
GET    /api/v1/people?q=…                     link-picker search (existing; name/alias)              (public)
```
Each curation/decision write on a person-typed field triggers `RelinkVideoPeople` post-commit.

## UI (grounded in real components)

- **Video page, owner view**: a person link picker on person-typed field rows — search existing
  people (name/alias, headshot when present), or create — reusing `EnrichPicker`'s roving-tabindex
  focus model and the F30 `CurationChip`/`SourceSelect` chrome. Chosen person's canonical name is
  submitted as a curation add.
- **Writeback**: the existing writeback control; no change beyond `actors` being among the writable
  fields it already offers.
- **Person page (P1-1)**: role badge/grouping from `video_people.role`.
- Tokens only; QA **Cinémathèque / Broadcast / Brutalist**; verify loading/empty/error states.

## Success Metrics

Single-owner correctness feature:
- **Leading:** curating an actor onto a video links them on the person page with **no rescan**
  (test + QA) — the people display-vs-truth split is dead.
- **Leading:** write `Artist` → re-scan → identical person set, zero duplicates (round-trip test).
- **Leading:** exactly **one** writer of `video_people` in the codebase after the change (grep/CI
  guard); studio behavior byte-for-byte unchanged (regression suite).
- **Lagging:** F32 lands as headshots+external-id **only**, cleanly on top — confirming this was the
  right foundation and the two-pattern hazard was retired.

## Open Questions

*(Q1 prune-policy, Q2 generalize-now, Q3 cutover-fidelity, and Q4 create-friction were resolved with
the owner 2026-07-04 → RD8, RD6, RD9, RD10.)*

- **Q5 (engineering, non-blocking): sweep cadence + `orphaned_at` clock.** How often the orphan sweep
  runs (lean: daily) and whether the 30-day clock is wall-clock or scan-relative (lean: wall-clock
  `orphaned_at`, simplest). Doesn't change the schema.
- **Q6 (engineering, non-blocking): `Producer` disposition (RD9).** `Producer` is in `peopleKeys` but
  has no canonical field. Lean: give it its own deferred role rather than silently relabel producers
  as actors; until then exclude `Producer` from `peopleKeys` so the loss-guard (P0-4) stays honest.
- **Q7 (design, non-blocking): the "authored identity" predicate (RD8/P0-9).** Enumerate exactly what
  makes a person un-prunable (has alias OR merge-history OR curated headshot OR any manual field
  edit/decision). Lock it in the ADR so the sweep and its tests agree.

## Timeline / routing

No hard deadline. Per the change-routing rules, before/with implementation:
1. ✅ **`/architecture`** — [ADR-056](../architecture/ADR-056-person-link-resolved-derivation.md):
   `video_people` resolved-derivation + `role` column (unset-capable) + `entity:` marker + orphan grace
   + the `RelinkVideoEntity` generalization + the **studio-writeback amendment to ADR-053** (RD11).
   Extends ADR-053; relates ADR-036/041/013/030/052/055. **Update F32's spec** to rebase onto it (pending).
2. ✅ **`/design-handoff`** — [person-media-linking-handoff.md](../design/person-media-linking-handoff.md):
   the owner-view person + studio link picker (combobox popover reusing `EnrichPicker`'s keyboard model;
   states, a11y, 3-skin QA) + person-page role badge (P1-1).
3. ✅ **`/testing-strategy`** — [testing-strategy.md](../testing-strategy.md) §9 (F39 block) + §10
   example cases: derivation matrix (incl. **unset role**), canonical-name **round-trip** (person *and*
   studio), orphan grace + sweep + authored-identity guard (RD8/P0-9), **lossless-cutover loss-guard**
   (RD9/P0-4), backfill idempotency, **studio regression** under the generalized reconcile,
   single-writer CI guard, homonym safety, owner-gating, 3-skin picker + role-badge QA.
4. **`/security-review`** — a **file-write path fed by owner-authored data** (person *and* studio):
   confirm curated person/studio names ride the existing `writeback` sanitize + `requireOwner` gate
   (no new perimeter, RD/Non-Goal), no name injection into the exiftool/mkv write, and the picker's
   create path can't be driven by a non-owner. Note the studio slice **reverses F38's no-writeback
   posture** — call it out explicitly. (Lighter than F32's asset perimeter — no downloads here.)

Slices (person + studio together): **S1** backend (migration, `RelinkVideoEntity` generalization over
person **and** studio fields + `RelinkVideoPeople`, remove raw-extraction writer, backfill,
`entity:` marker) → **S2** owner-view link pickers — person **and** studio, one component (against the
curation endpoint) → **S3** writeback round-trip QA — `actors→Artist` **and** `studio→Publisher`
(P0-7/P0-11) + person-page role badge (P1-1) → **S4** QA + `/security-review` (person + studio
file-write path). Effort: **L** (the data-model migration + writer cutover with backfill is the weight;
studio rides the same machinery, adding a picker target + one surfaced write).

### Before implementation
- File a **HOLODEX** issue for F39 and **rename the working branch to carry its key**
  (`HOLODEX-###-…`) as the first action, per the branch↔Jira linkage rule.
- Apply `needs-adr`, `needs-design`, `needs-security-review` labels; clear each as the artifact lands.
