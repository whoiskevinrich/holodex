# ADR-059: Person link resolved-derivation — role, orphan grace, and a generic `entity: person` reconcile

**Status:** Proposed
**Date:** 2026-07-04
**Deciders:** Project owner

**Amends:** [ADR-053](ADR-053-studio-entity-and-resolved-link-derivation.md) — reverses its explicit
**"no studio writeback / no write button"** non-goal (decision §6): studios gain the same owner-view
link + file-writeback treatment as people. Its data model, derivation rule, and immediate-prune policy
are otherwise unchanged.

**Relates to:** [ADR-053](ADR-053-studio-entity-and-resolved-link-derivation.md) (studio resolved-value
link derivation — the pattern this ADR **applies to people and generalizes**; `RelinkVideoStudios`
becomes one case of `RelinkVideoEntity`) · [ADR-052](ADR-052-baseline-source-contract.md)
(`BaselineSource` / entity-agnostic `ResolveFields` — the resolution derivation reads) ·
[ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (per-field decisions/curation — a "link" is
a curation add on a person-typed field) · [ADR-036](ADR-036-person-alias-search-indexing.md) (person
aliases — `resolveOrCreatePerson` routing, **now used by derivation**, not deferred as studio did) ·
[ADR-041](ADR-041-metadata-writeback.md) (writeback — the existing `actors → Artist` path this reuses,
no new perimeter) · [ADR-013](ADR-013-metadata-field-mapping.md) (field mapping — the `entity: person`
marker and reconciled `sources`) · [ADR-030](ADR-030-access-control-gating-seam.md) (`requireOwner`) ·
[ADR-055](ADR-055-enrichment-unique-key-invariant.md) (enrichment unique-key invariant — person
external-id de-dup, HOLODEX-125; this ADR's name-based `resolveOrCreatePerson` **composes with** it,
see Trade-off Analysis). **Spec:** [person-media-linking.md / F40](../specs/person-media-linking.md).
**Foundation for:** [F32 video-credits-people.md](../specs/video-credits-people.md) (headshots +
external-id + billed order rebase onto this data model).

---

## Context

The owner can *see* a video's people — the F30 actor/director chips render and link to person
pages — but cannot **author** one. There is no owner-view action to assert "this person is an actor
in this video," and no way to persist that assertion to the file so it survives outside Holodex.

The blocker is structural. Today `video_people` is derived from **raw file extraction at scan time**:

```go
// internal/repo/repo.go — replaceAssociations, called from UpsertVideo
for _, p := range people {           // people == ex.People, straight from extraction
    pid, _ := resolveOrCreatePerson(ctx, tx, p.Name)
    INSERT OR IGNORE INTO video_people (video_id, person_id) VALUES (?, ?)
}
```

This is the *exact* situation ADR-053 faced for studio, and it decided (RD1) that a link whose
displayed value can change via a **decision / curation / enrichment without a rescan** must derive
from the **resolved** field, not raw extraction — otherwise the media detail shows one thing while
grouping/navigation shows another (the F36 display-vs-truth split, one layer down). The moment F40
lets the owner curate a person into the `actors` field, people inherit that exposure: a curated actor
is not in `ex.People` (not in the file yet), so raw extraction omits them from `video_people`. ADR-053
foresaw this — *"F32 and any future entity have a second worked example of entity promotion"* — and
deliberately left people on the old pattern until a feature forced the move. F40 is that feature.

Three forces distinguish people from studio and rise to ADR level:

1. **This replaces a *live* writer, not a greenfield add.** `video_people` exists and holds data;
   switching its writer risks silently dropping links at cutover. Raw extraction folds **many** file
   tags into `ex.People` (`Artist, Cast, Actor(s), Performer, AlbumArtist, Director, Producer` —
   `internal/metadata/extractor.go`), while the canonical mapping today sources only `actors`
   (←`Artist`) and `director` (←`Director`). Derivation that reads fewer sources than extraction
   *loses* links. **Lossless cutover is a hard constraint.**
2. **Person identity is *authored*, not derived.** ADR-053 could prune studios the instant their last
   link vanished because a studio's name *is* a field value — nothing is lost. A person carries
   authored identity: aliases, merge history, a curated headshot, manual field edits (ADR-036/038/048).
   Immediate prune-on-empty would destroy owner work — especially during file maintenance, when a
   person's only file is legitimately offline for a while.
3. **Role matters for people.** A person is an actor *in this film* and maybe a director *in that
   one*. Studio has no analogous sub-typing. F32 (unbuilt) already proposed a `video_people.role`
   column; F40 must decide role's home so the two specs don't each grow a `video_people` writer.

### Constraints / forces

- **Display and grouping must never disagree** (the F36 invariant, inherited from ADR-053).
- **Resolution stays pure** (ADR-033/052) — derivation resolves over pre-loaded baseline + enrichment
  + curation + decisions; no new resolution I/O beyond the media-detail path's reads.
- **One writer of `video_people`.** ADR-053 warns that two link-derivation patterns already coexist
  (people/tags raw-extraction vs. studio resolved); adding a *third* (a parallel manual-link table)
  or leaving people half-migrated multiplies the hazard.
- **No new writeback perimeter.** `actors → Artist` is already a mapped, sanitized, owner-gated write
  (ADR-041); F40 adds an *input*, not a target or a format path. No media-file write beyond it.
- **Inherit, don't reinvent.** `resolveOrCreatePerson` (alias routing, homonym safety), the F30
  curation endpoint, `EnrichPicker`, and the ADR-053 reconcile all exist — reuse them.

---

## Decision

Migrate `video_people` from scan-time raw-extraction to **resolved-value derivation**, add a
**role** dimension sourced from the field, gate person-ness behind a **field marker**, and fold the
studio reconcile and this one into a **single generic reconcile**. Four decisions:

### 1 — `video_people` is a derived index over resolved person-typed fields

A single idempotent `RelinkVideoPeople(videoID)` becomes the **sole writer** of `video_people`
(the raw-extraction people branch of `replaceAssociations` is **removed**). It mirrors
`RelinkVideoStudios` (ADR-053 §2):

1. Load the video's baseline + enrichment + curation + decisions (the media-detail path).
2. For each **person-typed** canonical field (decision §3), resolve its value(s), trim, drop empties.
3. `resolveOrCreatePerson` each name (ADR-036 alias routing, homonym-safe) → a desired set of
   `(person_id, role)` rows, where **role = the field's declared role**.
4. **Replace** the video's `video_people` rows with exactly that set (delete stale, insert missing).
5. **Orphan, don't delete** (decision §4): a person left with zero links is stamped, not dropped.

Runs in its own write transaction **after** the triggering write commits (ADR-053's boundary
rationale, unchanged). Call sites — every path that can move a resolved person-typed value: scan
upsert, enrich completion, decision set/clear, curation add/suppress/clear. Never at read time.

### 2 — `role`, derived from the source field (the F32 foundation)

`video_people` gains `role TEXT NOT NULL DEFAULT ''`; **PK becomes `(video_id, person_id, role)`** so
one person can be both actor and director on one video. `role` is **not authored on the link** — it is
the **identity of the field the name resolved from** (`actors` → `actor`, `director` → `director`), and
a person-typed field that declares no role yields an **unset** role: not every credit is a typed role
(a bare `Artist`/music-video link is just "associated"). Unset is the **empty sentinel `''`, not SQL
`NULL`** — SQLite treats `NULL`s as *distinct* in a composite PK, so `(v, p, NULL)` rows would not
dedup; `''` keys cleanly and is presented as "unset" at the edge. The file is never asked to store a
role (extraction is role-flat by design — `Artist` and `Director` both fold into `ex.People`); role is
a Holodex-side projection of *which field* carries the name.

This **subsumes F32's proposed `video_people.role`**. F40 owns the derivation + role data model; F32
rebases to its unique remainder (provider headshots, external-id de-dup, billed `order`).

### 3 — `entity: person` field marker; generic reconcile

A field-mapping/registry marker `entity: person` (carrying the role identity) declares which canonical
fields hold people. **Derivation, the extractor's person-key source, and the link picker's target set
all read the marker** — no field name is hardcoded. Adding a role later (writer, composer) is a
mapping + marker edit, not code.

**Lossless-cutover constraint (hard):** because §1 removes the raw-extraction writer, every file tag
that used to create a `video_people` row must be reachable through a marked person-typed field. So:
- **Both `actors` and `director` are marked from day one** (derivation ≠ UI; the Actor *picker* is the
  only v1 UI surface).
- The `actors`/`director` field `sources` are **reconciled with the extractor's `peopleKeys`**
  (`Artist, Cast, Actor(s), Performer, AlbumArtist` → `actors`; `Director` → `director`; `Producer`
  deferred — see Consequences).
- The backfill (decision §5) is **guarded to not reduce the active link count** vs. pre-migration
  (modulo intended alias de-dup); the job records both counts and fails loudly on unexplained
  shrinkage.

`RelinkVideoStudios` and `RelinkVideoPeople` are unified into one **`RelinkVideoEntity`** (or a shared
core parameterized by the marked fields + entity table) **in this change**. Studio behavior must be
byte-for-byte unchanged (regression-guarded). Decided now, not deferred: a parallel second writer is
the exact hazard ADR-053 named.

### 4 — Orphan grace + authored-identity guard (person prune policy)

Unlike studio's immediate prune, a person whose last link is removed is **stamped `orphaned_at`**
(cleared the instant any link returns), and a **periodic sweep deletes people orphaned > 30 days** —
**except** people carrying **authored identity** (an alias, a merge history, a curated headshot, or any
manual field edit/decision), which are **never** auto-pruned and require explicit owner deletion.

- The **30-day grace** covers the owner's real workflow: file maintenance pulls a person's only file
  offline for a while; the person (and any curated work) must survive the round trip.
- The **authored-identity guard** ensures curated work is never lost even past 30 days.
- Studio keeps immediate prune (ADR-053 §2.4) — its identity is derived, so nothing is lost.

`people.orphaned_at TIMESTAMP NULL` (migration); the sweep is a System Activity job (ADR-028),
idempotent and observable.

### 5 — Migration order + one-time backfill

Fixed order **migrate → backfill → serve**. Migration: add `role TEXT NOT NULL DEFAULT ''` (unset —
an honest value for pre-derivation rows, which were role-flat anyway), change the PK, add
`orphaned_at`. Backfill: a startup pass derives `video_people` (with field-sourced roles, overwriting
the `''` where a field declares a role) for all active videos via `RelinkVideoPeople`, as an
idempotent System Activity job with the §3 loss-guard. There is no "wrong default" to hide — `''` is a
legitimate terminal value for role-less links.

### 6 — Studio parity, amending ADR-053's no-writeback non-goal

The owner gets the **same approach for studios**: an owner-view studio link picker (curate the
`studio` field) and **writeback** of the resolved `studio` → `Publisher` (already in the writeback
map). This is deliberately small because ADR-053 already did the hard part — `video_studios` derives
from the resolved `studio` field, and `RelinkVideoStudios` folds into `RelinkVideoEntity` (§3). The
only *new* thing is **surfacing a studio write action**, which **reverses ADR-053's explicit "a studio
has no file… no write button" non-goal**. The reversal is narrow and honest: a studio *value* lives in
a video's `Publisher` file tag even though the studio *entity* has no file of its own — writing the
resolved value back to `Publisher` is exactly the person-writeback shape (canonical name, existing
sanitized owner-gated path, no new perimeter). **Studios keep immediate prune** (ADR-053 §2.4) — the
person orphan-grace (§4) is person-only, because studio identity is derived and nothing authored is
lost when a studio re-derives on the next scan. **Ships with people (P0)** — one picker component and
one surfaced write over machinery people already build.

### What is explicitly *not* in this ADR

- **Writeback mechanics** — unchanged (ADR-041). F40 asserts one invariant on top: writeback flattens
  `actors → Artist` using each person's **canonical** name always (deterministic round-trip). That is
  a spec/test invariant, not a new architecture decision.
- **Provider headshots / external-id de-dup / billed order** — F32, rebased onto this model.
- **Person identity ops** (rename/alias/merge) — already ship (ADR-036); reused, not extended.
- **A role beyond actor/director in the UI** — data-model-ready via the marker; not wired.

---

## Options Considered

### Link-derivation source

#### A — Derive `video_people` from the resolved person-typed fields (chosen)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium-High — replaces a live writer + backfill + a lossless-cutover constraint |
| Consistency | Exact — display and grouping share one resolution; owner-authored links appear with no rescan |
| Cost | Low at runtime — reuses the pure resolver + pre-loaded reads (ADR-053's cost profile) |
| Familiarity | High — the ADR-053 pattern, generalized; `resolveOrCreatePerson` reused verbatim |

**Pros:** Kills the display-vs-truth split for people; owner curation/decision/enrichment re-link with
no rescan; one writer; becomes F32's foundation. **Cons:** Cutover risk against existing data (the
loss-guard is the mitigation); a contributor adding a person-value write path must call the reconcile.

#### B — Keep raw-extraction; union curated-but-unwritten people into it at scan (hybrid)

**Pros:** Smaller scan-path change. **Cons:** Two writers of `video_people`; a curated actor still
missing until the next scan (re-introduces the split between scans); this is ADR-053's rejected
Option B, one layer over. Rejected.

#### C — Keep raw-extraction; add a separate `manual_video_people` table for owner links

**Pros:** Leaves scan untouched. **Cons:** A *third* link-derivation pattern; every read (person page,
related shelf, filters, counts) must union two tables; writeback and de-dup logic split across them.
Rejected — multiplies the exact hazard the constraints forbid.

### Role model

#### A — `role` column on `video_people`, PK `(video_id, person_id, role)`, derived from field (chosen)

**Pros:** One person → many roles per video; role grouping/badging on the person page; unifies with
F32; role is *derived* (no new authored surface). **Cons:** PK migration; a person in two roles is two
rows (intended).

#### B — Role-agnostic `video_people` (no role); role lives only in the resolved fields

**Pros:** No migration. **Cons:** The person page can't cheaply badge actor vs. director; leaves F32's
role decision as a separate later fork — the two-derivation-writer hazard unresolved. Rejected.

#### C — Dedicated `video_credits` table (person, role, order, headshot-ref)

**Pros:** Richest; F32's order/headshot have a natural home. **Cons:** Heavier than F40 needs now;
duplicates `video_people`'s job. Deferred — F32 may promote to this later; the `role` column is the
minimal step that unblocks F40 and de-risks F32.

### Person prune policy

#### A — 30-day orphan grace + authored-identity guard (chosen)

**Pros:** Matches the owner's maintenance workflow; never loses curated work; observable sweep.
**Cons:** A new nullable column + a sweep job + an "authored identity" predicate to define.

#### B — Immediate prune-on-empty (studio's policy)

**Pros:** Simplest; symmetric with studio. **Cons:** Destroys authored identity the moment a file goes
offline during maintenance — the owner's stated failure mode. Rejected for people.

#### C — Never auto-prune (manual delete only)

**Pros:** Zero data-loss risk. **Cons:** Orphaned bare people accumulate as cruft with no cleanup.
Rejected — the grace + guard gets the safety without the accumulation.

### Reconcile structure

**Generalize to one `RelinkVideoEntity` now (chosen)** vs. ship `RelinkVideoPeople` parallel and merge
later. One-writer-per-table is a stated goal and a stated hazard; generalizing under the studio
regression suite is cheaper than living with two implementations and a merge debt. Chosen.

---

## Trade-off Analysis

**Replacing a live writer — the loss-guard is load-bearing.** Studio derivation was greenfield; person
derivation swaps the writer of a populated table whose *extraction* source set is broader than the
*mapping* source set. If derivation reads fewer tags than extraction, director-/cast-/performer-only
people silently vanish at cutover. Two mitigations, both in the decision: (1) reconcile the field
`sources` with `peopleKeys` so no extraction source is orphaned; (2) a backfill count-guard that fails
loudly on unexplained shrinkage. This is the single biggest risk delta from ADR-053 and the reason
this rises to an ADR rather than riding the spec.

**Role on the link vs. role in the field.** Writeback derives the *target tag* from the field
(`actors→Artist`), so one might argue role need not live on the link at all. But the link table is what
navigation/badging read, and re-resolving every linked video's fields to recover role at read time is
wasteful and couples the person page to the resolver. Storing the field-derived role on the link is a
cheap denormalization of a value the derivation already knows — and it is what F32 needs. Role stays
*derived* (never authored on the link), so it can't drift from the fields.

**Person prune diverges from studio — on purpose.** ADR-053's prune-on-empty is safe *because* studio
identity is derived; reusing it for people would contradict ADR-036/038/048, which treat person
identity as authored and worth protecting. The 30-day grace + authored-identity guard is the minimal
policy that (a) honors the maintenance workflow and (b) never loses curated work, at the cost of one
column and one sweep job. The asymmetry is documented so it isn't read as an inconsistency.

**Composition with ADR-055 (external-id de-dup), not conflict.** ADR-055 makes a namespaced
`<namespace>:<id>` the sole identity/de-dup key for entities (person → HOLODEX-125), *no name
fallback*. This ADR derives links from resolved field **values, which are names** (strings), and
resolves them via `resolveOrCreatePerson`. These compose: for **owner-authored** links the field
carries a name with no id, so name resolution is the only option and remains correct; for
**provider-sourced** names, the id travels as an internal sidecar field (the F32 `_person_external_ids`
analog to ADR-054's `_studio_external_ids`), and `resolveOrCreatePerson` — once HOLODEX-125 upgrades it
to external-id-first — dedups by id before name. `RelinkVideoPeople` calls one resolve function; which
key it consults inside is ADR-055's concern, not this ADR's. No decision here forecloses HOLODEX-125;
this ADR simply keeps name resolution as the authored-link path ADR-055 explicitly cannot cover.

**Transaction boundary — inherited.** Post-commit separate-transaction reconcile (ADR-053's analysis
verbatim): each call site's write stays small; all `video_people` mutation sits behind one idempotent
function; the narrow lag window self-heals on the next trigger/backfill. Acceptable for a single-owner
server.

---

## Consequences

**What becomes easier**
- The owner can author a person link (a curation add) and it appears everywhere with no rescan; the
  people display-vs-truth split is dead.
- One writer of `video_people`; one reconcile (`RelinkVideoEntity`) for studio + people.
- F32 lands as headshots + external-id + order **only**, on a data model already carrying role.
- Writeback of cast to the file works through the existing, reviewed path; a re-scan re-links the same
  people (canonical-name round-trip).
- Studio gains the identical link+writeback treatment (§6) at near-zero marginal cost — one picker
  component and one surfaced write action over machinery people already build.

**What becomes harder**
- A contributor adding any person-value write path must call the reconcile (mitigated: one function,
  the derivation matrix test).
- The field `sources` ↔ `peopleKeys` correspondence is now a maintained invariant; a new person-ish
  file tag added to extraction without a mapped field would drop links (mitigated: the loss-guard).
- A new periodic sweep + an "authored identity" predicate to keep correct as identity surfaces grow.

**What we'll need to revisit**
- **`Producer` (and other unmapped `peopleKeys`) disposition** — give it its own deferred role/field
  vs. drop from extraction; until decided, exclude it from `peopleKeys` so the loss-guard stays honest
  rather than mis-attributing producers as actors.
- **External-id de-dup composition** — when HOLODEX-125 lands, confirm `RelinkVideoPeople` benefits
  from external-id-first resolution for provider names (F32 supplies the sidecar id).
- **Promotion to a `video_credits` table** — if F32's `order`/headshot-ref pressure grows, migrate the
  `role` column into a richer credits model; this ADR is the minimal step toward it.
- **Multi-role UI** and roles beyond actor/director — data-model-ready via the marker.

---

## Action Items

1. [ ] Migration (next `NNNN`): `video_people.role TEXT NOT NULL DEFAULT 'actor'`, PK →
   `(video_id, person_id, role)`, `people.orphaned_at TIMESTAMP NULL`; matching `.down.sql`
   (collapse duplicate-role rows; drop columns).
2. [ ] Generalize `RelinkVideoStudios` → `RelinkVideoEntity` (shared core over marked fields + entity
   table); assert studio behavior unchanged (regression suite).
3. [ ] `RelinkVideoPeople` case: resolve marked person-typed fields → `(person_id, role)` set via
   `resolveOrCreatePerson`; replace rows; **orphan-stamp** (not delete) on zero links. Sole writer.
4. [ ] Remove the raw-extraction people branch from `replaceAssociations`; scan calls the reconcile
   post-commit. (Tags remain raw-extraction — documented asymmetry.)
5. [ ] `entity: person` marker in the mapping/registry (with role); mark `actors` + `director`; source
   derivation, `peopleKeys` source, and the link-picker target set from it.
6. [ ] Reconcile `actors`/`director` `sources` with `peopleKeys`; decide `Producer` (Consequences).
7. [ ] Backfill pass (idempotent, System Activity) with the **loss-guard** (post ≥ pre active links).
8. [ ] Orphan **sweep** job (System Activity): delete `orphaned_at < now()−30d` AND no authored
   identity; define the "authored identity" predicate; skip+report authored orphans.
9. [ ] Owner-view link picker (curation add; `EnrichPicker` a11y; inline bare-create) — reuses the F30
   curation endpoint; no new API.
10. [ ] `/testing-strategy`: derivation matrix (scan/enrich/decision/curation × person-typed field ×
    link+role), canonical-name **round-trip** (write→rescan→same set), **loss-guard**, orphan
    grace + sweep + authored-guard, studio regression, single-writer CI guard, owner-gating.
11. [ ] `/security-review`: owner-authored names through the existing writeback sanitize + gate (no new
    perimeter); no role/name injection into the exiftool/mkv write; picker create-person path
    owner-only.
12. [ ] **Studio parity (P0, ships with people, §6):** studio link picker (curate `studio`; reuse the
    person picker) + surface `studio → Publisher` writeback; note the ADR-053 non-goal reversal;
    studios stay on immediate prune. Covered by `/security-review` (studio counterpart of the person write).
13. [ ] Add ADR-059 to `docs/architecture/README.md`; cross-reference from F40 spec; **update the F32
    spec** to rebase onto this data model.
