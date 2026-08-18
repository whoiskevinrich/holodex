# ADR-085: Films entity — asserted video links + dynamically-namespaced resolver source

**Status:** Proposed
**Date:** 2026-08-18
**Deciders:** Project owner

**Relates to:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (per-field source-of-truth
decisions — films become the first `provider:<name>` decision source whose namespace is **not**
statically declared in `metadata-mappings.yaml`, amending the static-provider assumption implicit
in §8) · [ADR-052](ADR-052-baseline-source-contract.md) (`BaselineSource` contract — films add a
fourth implementation whose *inherited* fields resolve from a union of other entities, not a file
or a scanned record) · [ADR-053](ADR-053-studio-entity-and-resolved-link-derivation.md) (studio
entity — the **derived**-link precedent this ADR deliberately inverts: `video_studios` is
reconciled by `RelinkVideoStudios`; `film_videos` has no reconciler at all, ever) ·
[ADR-072](ADR-072-person-link-resolved-derivation.md) (person-link resolved derivation —
`video_people` and its 30-day orphan grace are **unchanged**; films read the resolved cast, never
write to it) · [ADR-074](ADR-074-claimed-provider-keys.md) (claimed provider keys — a superficially
similar per-provider mechanism this ADR does **not** reuse: `field_claims` is keyed
`(entity_type, provider, field_key)` with **no precedence column**, "append last, lexicographic";
a film competing for `album`/`title` needs the opposite — a first-class precedence candidate a
decision can pin to) · [ADR-079](ADR-079-studio-image-roles.md) (studio image roles — the
`{entity}_images(entity_id, role, source, …)` shape `film_images` reuses verbatim) ·
[ADR-013](ADR-013-metadata-field-mapping.md) (field mapping — `ParsedSources` is the static
per-field source list this ADR's resolver change bypasses for films) ·
[ADR-030](ADR-030-access-control-gating-seam.md) (owner gating — every new mutation endpoint).
**Spec:** [Films entity / F56](../specs/films-entity.md); epic
[HOLODEX-279](https://whoiskevinrich.atlassian.net/browse/HOLODEX-279).

---

## Context

The spec locks the product behavior: a film is a first-class entity whose membership is a durable
**owner assertion** ("this file is scene 6 of film Y"), never derived or pruned by scan/enrich
machinery, and its name competes as a resolver **source** for a linked video's `album` (every
attachment) and `title` (only the attachment flagged as representing the entire film) — never a
writeback bypass. Two questions were deliberately left open for this ADR (spec Q1/Q2), because
they force real code-level decisions the spec's product framing can't settle on its own.

**Q1 — the decision grammar assumes a small, static provider set; films are neither.** Today's
`provider:<name>` decision source (`internal/fieldsource`) is resolved by `resolveDecided`
(`internal/resolver/resolver.go:491`) by scanning the field's `ParsedSources` — the fixed list a
field's YAML mapping declares (`internal/mapping`) — for an entry whose `Namespace` equals the
decided provider's name, then reading that provider's shadow-store value
(`enrichment[namespace][key]`, from `entity_enrichment`). Every provider that can ever decide a
field is therefore known **at config-load time**: `tmdb`, `imdb`, whatever `metadata-sources.yaml`
declares. A film is not that. A video can attach to **any number of films at runtime**, each
identified only by a database row created through search-attach — there is no YAML entry for
"film 4217" and there never will be, because the whole point is that films are created through the
UI, not configured. `ParsedSources`-based matching structurally cannot see a namespace that isn't
declared in advance, so `provider:film:<id>` as a decision source resolves to nothing under the
current code, decided or not. This is a genuine gap, not a missing config line.

**Q2 — "suspend, not drop" needs an actual mechanism, not just a promise.** The spec's RD7 requires
that disabling `films_enabled` stop a film source from contributing to resolution and from
appearing in the decision UI, **without deleting** the `field_source_decisions` row that names it,
so re-enabling restores the identical prior resolution. Nothing in the decision/resolver layer
today has an "available/unavailable" axis for a source — every `provider:<name>` decision is
resolved as long as *a* row for that namespace exists in the pre-loaded `enrichment` map, and
`enrichment` is always built from whatever's currently enriched. The open question is what "film
source is temporarily unavailable" actually *does* inside `resolveDecided`, concretely.

### Constraints / forces

- **No new resolver I/O.** Whatever answers Q1 must fit the existing purity discipline
  (ADR-013/033/051/052): every input `resolveField` touches is pre-loaded by the caller before the
  resolve pass, never fetched mid-resolution.
- **`RelinkVideoEntity`/prune-on-empty must never see `film_videos`** (spec RD1 — the single most
  load-bearing rule in the spec). Whatever call sites this ADR wires must be a *closed, disjoint*
  set from the four ADR-053/ADR-072 relink triggers (scan, enrich completion, decision set/clear,
  curation change) — films have their own triggers (attach/detach), full stop.
- **A scene file writes `Album` only; a full-film file writes `Title` and `Album`** (spec RD7) —
  this split must fall out of *which (namespace, field) pairs exist as candidates* for a given
  video, not a special case inside the resolver core, or every future per-field writeback rule
  needs its own resolver branch.
- **Multiple films on one video is a normal, resolvable case** (spec RD7), not an error — the
  decision UI must be able to show two distinct "Film A" / "Film B" candidate chips for `album` on
  the same video, exactly like two providers disagreeing on `title` today.
- **Film identity de-dup follows ADR-055's precedent, adapted.** ADR-055 makes a namespaced
  provider id the sole de-dup key for enrichment-sourced entities; a film has no provider id in v1
  (spec RD8) — `(name, year)` is the identity key instead, exact-match resolve-or-create, the same
  posture ADR-053 took for studio names before ADR-054/055 added provider-id de-dup.

---

## Decision

### 1 — Data model (migration 0042)

The ADR-053 pattern (name-keyed entity + join + FTS5 mirror), extended with the two columns the
spec's scene semantics require, plus a film-only roles table:

```sql
CREATE TABLE films (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    year        INTEGER,
    UNIQUE (name, year)                    -- RD8: NOT bare name UNIQUE (studios' mistake for this entity)
);

CREATE TABLE film_videos (
    film_id       INTEGER NOT NULL REFERENCES films(id)  ON DELETE CASCADE,
    video_id      INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    scene_number  INTEGER,                 -- NULL = unnumbered
    is_full_film  INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL,
    PRIMARY KEY (film_id, video_id),       -- one link per (film, video); no ranges (spec Non-Goal)
    UNIQUE (film_id, scene_number)
);
CREATE INDEX idx_film_videos_video ON film_videos(video_id);

-- UNIQUE(film_id, scene_number) deliberately relies on SQLite/ANSI SQL treating NULL as
-- distinct in a UNIQUE constraint, so any number of unnumbered scenes coexist while numbered
-- ones collide (RD5). This is the OPPOSITE fix from migration 0037's video_people, where that
-- same NULL-distinctness was a bug worked around with an empty-string sentinel for `role` — do
-- not "fix" this table by copying that pattern; NULL is the wanted behavior here, not a bug.
-- Note this is a bare UNIQUE constraint, not a composite PRIMARY KEY (0037's situation) — no
-- workaround is needed either way, but the contrast is worth stating for the next reader.

CREATE TABLE film_people_roles (
    film_id       INTEGER NOT NULL REFERENCES films(id)  ON DELETE CASCADE,
    person_id     INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    role          TEXT    NOT NULL DEFAULT '',  -- '' sentinel mirrors migration 0037's video_people.role
    billing_order INTEGER,
    PRIMARY KEY (film_id, person_id, role)
);

CREATE VIRTUAL TABLE films_fts USING fts5(
    name, content='films', content_rowid='id',
    tokenize="unicode61 remove_diacritics 2"
);
-- films_ai / films_ad / films_au triggers: the studios_* triggers verbatim (ADR-053).

CREATE TABLE film_images (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    film_id  INTEGER NOT NULL REFERENCES films(id) ON DELETE CASCADE,
    role     TEXT    NOT NULL,              -- 'poster' | 'thumb' (ADR-079 shape; roles TBD at impl)
    source   TEXT    NOT NULL,              -- 'upload' | 'provider:<name>' (ADR-079/049 provenance lock)
    -- remaining columns mirror studio_images (path, width/height, content hash, created_at) verbatim
    UNIQUE (film_id, role, source)
);
```

- **Join table, not `videos.film_id`**, mirroring ADR-053's reasoning exactly: one row per
  attachment, headroom for a video to belong to multiple films (spec RD7's central case), no
  re-migration if the model ever needs to carry more per-attachment data.
- **`film_people_roles` is separate from `video_people`**, not a role column bolted onto a join —
  it has no `video_id`; it is a property of the *film*, populated by the owner directly on the film
  page, never derived from any video (spec RD3).
- **No `external_id` column in v1** — same posture ADR-053 took for studio before ADR-054/055;
  identity is `(name, year)` exact-match only (RD8), consistent with the spec's Non-Goal on
  identity operations (alias/merge/de-dup deferred to a P2).

### 2 — Asserted-link invariant: zero relink participation (realizes spec RD1/P0-2)

`film_videos` has **no reconciler**. Its only writers are four owner-gated endpoints: attach
(video-side), attach/bulk-attach (film-side), and detach. It is added to **none** of the four
`RelinkVideoStudios`/`RelinkVideoEntity` call sites (scan upsert, enrich completion, decision
set/clear, curation change) and to no prune-on-empty pass. This is a structural absence, not a
flag: there is no `RelinkFilmVideos` function for anything to call. A regression test asserting
that a full scan → enrich → decision-change cycle leaves every `film_videos` row byte-for-byte
unchanged is the guard (spec Success Metrics, leading #1); `/testing-strategy` must make this the
first film-related test written, not a follow-up.

### 3 — `filmBaseline`: the entity whose baseline is other entities (realizes spec RD2)

A fourth `BaselineSource` implementation, `filmBaseline`, mirrors `personBaseline`/`studioBaseline`
for the film's **own** fields (`name` is baseline-backed; `description`/`release_date`/poster
resolve as empty-but-claimed baseline, so undecided enrichment candidates still win — the RD6
additivity property ADR-052 established). Cast and tags are **not** resolved through
`filmBaseline` at all — they are a read-only set union computed by joining `film_videos` →
`video_people`/`video_tags` for the film's attached videos (full-film files included, per RD4),
deduplicated by person/tag id. This is deliberately **not** a `BaselineSource` method: a union
query over other entities' already-resolved links has no "is this a baseline value" question to
answer, so forcing it through the `Baseline(src)` interface would be a fiction. `filmBaseline`
answers only for film-owned fields; the film API assembles the cast/tag union alongside it as a
separate read.

### 4 — Film resolver source: a dynamically-namespaced `provider:<name>` (resolves Q1)

`film_videos` supplies each attached video with **synthetic enrichment-shaped candidates** — not a
real provider, but shaped identically to one so the existing `Enrichment` type
(`map[namespace]map[fieldKey][]string`) carries them with no new resolver parameter type. For an
attached film with id `42`:

- namespace `"film:42"`, field `album` → the film's own resolved `name` — injected for **every**
  attachment.
- namespace `"film:42"`, field `title` → the same value — injected **only** when
  `film_videos.is_full_film = true` for that attachment.

This injection happens at the call site that assembles a video's resolver inputs (the media-detail
handler / writeback-assembly path, wherever `Enrichment`+`Curation`+decisions are pre-loaded today)
— **not** inside `internal/resolver`. It is gated by `films_enabled` (§5) and requires exactly one
new pre-loaded read: the video's `film_videos` rows joined to `films.name`. The `Album`/`Title`
split (spec RD7) therefore falls entirely out of *which candidates get injected*, with zero new
branching for it inside the resolver.

**The one genuine resolver-core diff.** `resolveDecided`'s provider branch
(`internal/resolver/resolver.go:492-502`) matches a decided provider's name against the field's
statically-configured `ParsedSources` to find which mapping key to read — a film namespace has no
such static entry, by design (Context, Q1). `resolveDecided` gains one additional branch: when the
decided source's provider name has the `"film:"` prefix, skip the `ParsedSources` scan and read
`enrichment[name][f.Canonical]` directly (the field's own canonical key doubles as the pseudo-key,
since a synthetic film candidate has no YAML-declared key to look up). This is a deliberate,
narrow, named exception to "providers are always statically declared" — the first one this
codebase has needed — and is worth flagging explicitly rather than folding silently into the
existing loop, so a future reader doesn't mistake it for dead code. `gather` (used for undecided
precedence/candidate listing, line 464) needs the identical treatment for symmetry, guarded the
same way. No other function in `internal/resolver` changes.

**Decision-UI candidate chips.** Listing "Attach to Film X" as an adoptable chip for an undecided
`album`/`title` field is a handler-layer concern, not a resolver-core one — the same layer that
already assembles per-provider `Adopt` chips from the matched-provider list (ADR-051 §8) enumerates
the video's film attachments alongside real providers when building the field's candidate payload.
No protocol/schema change to the existing chip shape is needed; a film candidate is just another
entry whose `source` is `provider:film:<id>`.

### 5 — `films_enabled` suspend mechanism: reuse the existing "decided source currently unmatched" path (resolves Q2)

No new column, no status flag, no new resolver state. When `films_enabled = false`, the call site
in §4 simply **does not inject** any `film:<id>` candidates — `enrichment` contains no such
namespace. `resolveDecided`'s `film:`-prefixed branch then finds `enrichment[name]` empty (or
`f.Canonical` absent from it) and returns `nil, ""` — **exactly** the code path that already fires
today when a video is decided to a real provider it is no longer matched to (`internal/resolver/
resolver.go:498-502`, `firstNonEmpty(pFields[src.Key]) == ""` → loop exhausts → `nil, ""`). Films
introduce no new "unavailable" concept; they are simply the first case where "currently unmatched"
is *driven by a global flag* rather than *per-item enrichment state*. The `field_source_decisions`
row is never touched — the decision still reads `provider:film:42` in the database the entire time
— so the instant `films_enabled` flips back on and the call site resumes injecting that namespace,
the same decision resolves to the same value with zero owner action, satisfying RD7's restore
guarantee exactly. This is why Q1's answer (candidates as caller-injected, gated enrichment rows)
and Q2's answer (suspension as an injection no-op) are the same mechanism observed at two different
moments — deciding one decided the other.

**Consequence for display while suspended.** The field resolves to *nothing* (falls out of the
resolved payload) while its decided film source is unavailable — it does not silently fall back to
`file`. This matches existing behavior for any other decided-but-currently-unmatched provider and
is the correct behavior per RD7 (the file on disk still carries whatever was last written; Holodex
just stops asserting a value for a source it cannot currently see). The design handoff must give
this an explicit "source unavailable" state in the field UI (distinct from "field genuinely empty")
so it doesn't read as data loss — flagged as a design action item, not a resolver concern.

### 6 — `films_enabled` gate wiring

Follows the `MCPEnabled`/`ThumbnailEnabled`/`ExtractionAutoApplyEnabled` pattern verbatim:
`internal/config.Config.FilmsEnabled bool` (`yaml:"films_enabled"`, default `false`), env override
`FILMS_ENABLED` via `envBool`. Threaded to `Handlers` the same way `cardLayout` is
(`internal/api/handlers.go`), exposed on the ungated `capabilities()` payload
(`internal/api/auth.go:225`) as a new `FilmsEnabled bool` field so the SPA can gate the attach
affordance and the films-row rendering without a round trip. When false: `/films*` routes are not
registered on the chi router (not merely 404'd — absent), `films_fts` is excluded from the global
mixed-entity search assembly (ADR-017), the MCP film tool surface is not registered
(`internal/mcp`), and §4/§5's candidate injection is skipped. Migrations run unconditionally.

### 7 — API surface (owner-gated mutations only; reads public when enabled)

```
GET    /api/v1/films                                        list (public; empty/404-family when disabled)
GET    /api/v1/films/{id}                                   film + resolved[] + full-film file(s) + scenes
POST   /api/v1/films/{id}/fields/{canonical}/decision        (requireOwner; mirrors ADR-051 §7)
DELETE /api/v1/films/{id}/fields/{canonical}/decision        (requireOwner)
POST   /api/v1/media/{id}/films                              { film_id, scene_number?, is_full_film? }  attach (requireOwner)
DELETE /api/v1/media/{id}/films/{film_id}                    detach (requireOwner)
GET    /api/v1/films/{id}/video-candidates?…                 scoped/filtered search, film-side picker (requireOwner)
POST   /api/v1/films/{id}/videos/bulk-attach                 { video_ids[], starting_scene_number? } (requireOwner)
```

Scene-number collision on attach returns `409` naming the current occupant (`film_id`,
`scene_number`, occupying `video_id`) — no silent swap, no auto-bump, per spec RD5.

---

## Options Considered

### Q1 — how a film competes as a resolver source

#### A — Caller-injected synthetic namespace into the existing `Enrichment` map, one new `resolveDecided` branch (chosen)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low — one bounded branch in one function, reusing the existing map shape |
| Consistency | High — a film candidate is indistinguishable from a real provider candidate everywhere except the one lookup branch |
| Cost | Negligible — one pre-loaded join query at the call site, no new resolver I/O |
| Familiarity | Medium — extends a pattern every provider decision already uses; the one new idea is "namespace not statically declared" |

**Pros:** No schema change to `field_source_decisions` (`provider:film:<id>` fits the existing
grammar); the multi-film-conflict UI need is free (two namespaces, two chips, exactly like two
disagreeing providers today); Q2 falls out for free (§5). **Cons:** the one explicit deviation from
"providers are declared in YAML," which must be commented clearly so it isn't mistaken for a bug
or dead code by a future reader extending `resolveDecided`.

#### B — Compile films into `metadata-sources.yaml` as a pseudo-provider

**Pros:** Zero resolver-core changes — films would satisfy the existing `ParsedSources` matching
untouched. **Cons:** `metadata-sources.yaml` declares a *fixed, operator-configured* set of
external providers (ADR-033's "declared, not compiled in" model) with one row per config; films are
**runtime, per-video, unbounded-cardinality** database entities with no operator-authored config
line — there is no YAML shape that expresses "whichever films this video happens to be attached
to." Rejected — the mismatch is categorical, not a matter of degree.

#### C — Direct writeback bypass (film page writes Album/Title straight to the file, no resolver involvement)

**Pros:** Simplest possible implementation. **Cons:** Explicitly rejected by the spec (RD7) — a
video in two films would silent-overwrite rather than surface as a decidable conflict, and
writeback would no longer write the *decided* value (ADR-051's central invariant). Not seriously
considered; recorded because the original feature request's phrasing ("videos attached to a film
write back the film name") reads this way on first pass and a future reader may wonder why it
wasn't done.

### Q2 — suspend semantics for `films_enabled=false`

#### A — Suspension is injection non-participation; reuse the existing "decided source unmatched → empty" path (chosen)

**Pros:** No new schema, no new resolver state, no new code path to test beyond what already
exists for any provider decision on an unmatched video — the flag-off case is provably identical
in shape to a pre-existing case. Restore-on-re-enable is automatic (the decision row was never
touched). **Cons:** the field visibly goes empty while suspended rather than falling back to file
— a real UX cost, mitigated by design handoff (§5) rather than by changing the mechanism.

#### B — A status/availability column on `field_source_decisions` rows

**Pros:** Makes "this decision is currently suspended" an explicit, queryable fact rather than an
emergent property of what got injected. **Cons:** a new column and a new state machine
(active/suspended) that must be kept in sync with `films_enabled` on every read — solving a problem
Option A already solves for free by construction. Rejected — unnecessary state for a boolean the
config already carries.

#### C — Delete `field_source_decisions` rows naming a suspended film source, re-derive on re-enable

**Pros:** No suspended-state concept needed at all. **Cons:** explicitly the failure mode RD7
exists to prevent — the field would re-resolve to a *different* candidate (or `file`) while the
disk still carries the film-written value, with nothing left to explain the mismatch on the next
scan. Rejected outright per spec.

---

## Trade-off Analysis

**Stretching the `provider:<name>` grammar to a dynamic namespace is a deliberate, narrow
exception, not a precedent to generalize further.** ADR-051 §8's inter-provider model assumes a
small, operator-declared provider set; this ADR's central move is recognizing that films need the
same *shape* of precedence competition (a named source a decision can pin to) without needing the
same *cardinality* assumption (a handful of config-declared providers). The alternative — inventing
a wholly separate "which entity source wins" mechanism parallel to decisions — would duplicate the
entire decision UI, the writeback-uses-decided-value invariant, and the sync-state display for no
real gain, since films' actual behavior (pin a source, read its live value, restore on re-decision)
is identical to a provider's. The cost accepted is a single explicit branch in `resolveDecided`
that a future contributor must understand is intentional; the comment at that branch and this ADR
are the mitigation, not a design that avoids the branch entirely.

**Choosing "suspend = don't inject" over "suspend = a stored flag" trades explicitness for
correctness-by-construction.** A stored suspended/active state is more legible in isolation, but it
is also one more place `films_enabled` and stored state can drift (a decision row marked "active"
while the flag is actually off, if some write path forgets to check). Option A cannot drift: there
is no state to fall out of sync, because suspension is not a fact that is stored anywhere — it is
the observable *absence* of an injection that only ever happens when the flag is read fresh at
resolve time. This mirrors the same reasoning ADR-052 used to keep `BaselineSource` pure and
ADR-053 used for prune-on-empty: prefer a property that holds by construction over one that must be
kept in sync by discipline.

**The empty-while-suspended field is a real, accepted UX cost, deliberately not solved here.**
Falling back to `file` while a film decision is suspended was considered and rejected: it would
mean the *displayed* value changes twice in one flag-toggle round trip (decided film value → file
value while off → decided film value again on re-enable) even though nothing about the underlying
decision changed, which is arguably more confusing than a clearly-labeled "source unavailable"
state. This ADR fixes the *mechanism* (§5); the *display treatment* of that empty state is
explicitly deferred to `/design-handoff` as a named action item, not solved by architecture.

---

## Consequences

**What becomes easier**
- A video in N films is not a special case anywhere — N synthetic candidates, N decision chips, the
  existing conflict-resolution UI, no new merge logic.
- Toggling `films_enabled` is provably non-destructive to decisions by construction, not by
  discipline — there is no stored suspended state that a missed code path could leave stale.
- The asserted-link invariant (§2) is enforceable by absence: there is no `RelinkFilmVideos`
  function to accidentally call from a new trigger, unlike a flag-guarded reconciler that a future
  change could silently re-enable.

**What becomes harder**
- `resolveDecided` now has one provider-namespace shape (`film:<id>`) that behaves differently from
  every other provider namespace it handles — a contributor extending provider-decision logic must
  know this branch exists and why. Mitigated by the inline comment + this ADR being linked from the
  code.
- The empty-while-suspended field behavior needs an explicit UI treatment or it will read as a bug
  report the first time an owner toggles the flag — tracked as a `/design-handoff` action item, not
  optional polish.
- Film cast/tags (the union-of-scenes read) is a live query over `film_videos` × `video_people`/
  `video_tags`, not a cached/materialized value — acceptable at this app's personal-library scale
  (consistent with ADR-081's posture on in-Go resolve-at-read for entity lists), but worth revisiting
  if a film's scene count ever grows large enough to matter.

**What we'll need to revisit**
- **Film identity operations** (alias/merge/de-dup) — deferred per spec Non-Goals; if pursued,
  mirrors ADR-061's unified identity spine, extended to a fourth entity.
- **Film provider enrichment's own namespace collision.** If a real metadata provider is later
  wired for films (spec P1-1) and that provider's declared namespace happens to also be used as a
  film-attachment pseudo-namespace shape (unlikely, since real providers never use a `:`-suffixed
  numeric id), the `"film:"` prefix check in `resolveDecided` must stay a prefix match against the
  *reserved* token, not a generic pattern — worth a defensive test once P1-1 ships.
- **Multi-film UI at scale** — if an owner routinely attaches videos to many films (unlikely per
  this app's usage pattern, but not impossible), the candidate-chip list for `album` could grow
  long; no mitigation designed here, revisit if it becomes real.

---

## Action Items

1. [ ] Migration 0042: `films`, `film_videos`, `film_people_roles`, `film_images`, `films_fts` +
   triggers (§1); down migration drops them.
2. [ ] `filmBaseline` (`NewFilmBaseline`) in `internal/resolver`, mirroring `personBaseline`/
   `studioBaseline` for film-owned fields; cast/tag union assembled separately, not through
   `BaselineSource` (§3).
3. [ ] `resolveDecided` + `gather`: add the `"film:"`-prefixed provider-namespace branch bypassing
   `ParsedSources` matching (§4); unit tests covering decided/undecided/suspended/multi-film cases
   explicitly, since this is the one genuine resolver-core diff in this feature.
4. [ ] Call-site injection: video resolver-input assembly gains a `film_videos` join producing
   synthetic `Enrichment` rows for `album` (all attachments) and `title` (full-film attachments
   only), gated by `films_enabled` (§4/§5/§6).
5. [ ] `internal/config.FilmsEnabled` + `FILMS_ENABLED` env override + `capabilities()` payload
   field (§6).
6. [ ] `film_videos` writers: attach (video-side), attach + bulk-attach (film-side), detach —
   confirm **zero** other call sites touch the table (§2); add the scan/enrich/decision/curation
   non-participation regression test first.
7. [ ] Film API endpoints (§7), owner-gated per ADR-030; scene-number collision `409` with occupant
   detail.
8. [ ] Route/FTS/MCP registration gated on `films_enabled` (§6); confirm routes are **absent**, not
   404-guarded, when disabled.
9. [ ] Add this ADR's row to `docs/architecture/README.md`; cross-reference from
   [films-entity.md](../specs/films-entity.md), replacing its Q1/Q2 Open Questions with resolved
   references to this ADR.
10. [ ] `/design-handoff`: the "source unavailable" (vs. "genuinely empty") field state for a
    suspended film decision (§5 Trade-off Analysis) — a named, non-optional action item.
11. [ ] `/testing-strategy`: asserted-link non-participation (item 6), flag-toggle decision
    round-trip idempotency, scene-number collision, multi-film candidate resolution, film cast/tag
    union correctness, `films_enabled` route/FTS/MCP absence.
12. [ ] `/security-review` before merge: new owner-gated mutation surface (attach/detach/bulk-attach,
    decisions), the film-side video-candidate search endpoint's access parity with browse, provider
    data for P1-1 enrichment through the existing ADR-039 asset perimeter.
