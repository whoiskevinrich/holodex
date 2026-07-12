# Spec: Queryable fields substrate — Phase 1 (typed registry + Age in Media) (F46 Phase 1)

**Status**: Draft
**Phase**: F46 Phase 1 of 3 (epic HOLODEX-178)
**Owner**: Project owner
**Date**: 2026-07-12
**Feature block**: **F46 Phase 1** — a **typed field registry** (text | categorical | numeric | date) with an
**operator model** (equals, contains, range), piloted end-to-end by **Age in Media**: a new
**relationship-scoped** computed field (person × video) extending the F45 computed-field engine beyond
single-entity scope.

**Issue**: [HOLODEX-176](https://whoiskevinrich.atlassian.net/browse/HOLODEX-176) *(parent epic
[HOLODEX-178](https://whoiskevinrich.atlassian.net/browse/HOLODEX-178) — F46 Queryable person/video
attributes)*
**ADR**: TBD via `/architecture` — extends [ADR-062](../architecture/ADR-062-in-app-field-promotion.md)'s
deferred D-filterable item and [ADR-063](../architecture/ADR-063-derived-computed-fields.md)'s derived-field
genre to relationship scope (two-entity input, new injection boundary)
**Design**: TBD via `/design-handoff` — cast-list age placement (the video page's people section is currently
a bare poster grid with no subtitle text; fitting an age number in needs a UI call)
**Testing**: TBD via `/testing-strategy`

**Depends on** (all shipped):
- the entity-agnostic resolver + canonical registry ([ADR-052](../architecture/ADR-052-baseline-source-contract.md),
  `ResolveFields`, `registry.FieldDef`, `internal/registry`)
- the derived-field genre and `Derive` post-pass precedent ([F45](derived-person-fields.md) /
  [ADR-063](../architecture/ADR-063-derived-computed-fields.md), `internal/resolver/derive.go`) — Age in
  Media is the same computable/absent/non-adoptable contract, extended to a second entity's resolved fields
- the video ↔ person link (`video_people`, `internal/repo/repo.go` `attachAssociations`) that already
  populates the video-page cast list

**Touches**: read-only computed values + a Go-level type/operator taxonomy only. **No migration, no stored
index, no new auth/access/infrastructure surface** in this phase (see Resolved Decisions). Per the ticket,
`/security-review` is not expected to be required — confirm at implementation time.

---

## Problem Statement

Every field in the resolver today is presentation-only text with no notion of *type* (categorical vs. numeric
vs. date) or *comparison semantics* — that gap blocked three distinct asks that got bundled into one vague
ticket ("Field Promotions are searchable, similar to tags") before a brainstorm split them into epic
HOLODEX-178: Nationality search, Age in Media, and "lead actor has blue eyes." Separately, F44's in-app field
promotion hit a dead end on `Filterable` (hard-coded `false`, ADR-062 D-filterable) because there was no typed
operator model to express what "filterable" even means per field type. Concretely today: a video page's cast
list shows names and avatars with no indication of how old each person was when the video was made — a fact a
viewer has to compute by hand, cross-referencing a birthdate on the person page against a release year that
may not even be visible on the video page.

## Resolved Decisions

Three open items from the ticket were resolved with the project owner on 2026-07-12, the last one grounded in
a direct query of the films testbed DB rather than assumption:

| # | Question | Decision |
|---|---|---|
| D1 | Which video-side date feeds Age in Media? | **`release_date` only.** Checked the films testbed DB directly: `recorded_at` (file-native) is a scan/copy timestamp, not the media's real date — e.g. *Aladdin* (1992 film) carries `recorded_at=2016-02-23`, *300* (2006 film) carries `2026-07-01`. Using it, even as a fallback, would show **actively wrong** ages for most of the library, not just missing ones. `release_date` (TMDB-enrichment-sourced canonical) is never wrong, only sometimes absent — same coverage contract F45's Age already has via `birthdate`. |
| D2 | How much of the typed registry/operator model does Phase 1 build? | **Full 4-type/3-operator model now** (text/categorical/numeric/date; equals/contains/range), even though Age in Media itself only exercises numeric+range and no operator is wired to a UI yet. Phase 2 (Nationality) and Phase 3 (role-scoped joins) then extend by registration, not new plumbing — matches the epic's "prove it end-to-end" framing. |
| D3 | Does Phase 1 persist a queryable value index? | **No — compute-on-read only**, mirroring F45's `Derive` pattern exactly (just extended to a two-entity input). Search/filter — the only thing that would actually query a stored index — is explicitly deferred past this phase, so a persisted index here would have zero consumers. Phase 2 (Nationality, which needs a real facet query) is the natural place to size and build the index it actually needs. |

## Goals

1. **Age in Media renders on the video-page cast list** for every cast member with a resolvable birthdate
   *and* a resolved `release_date` — "how old was this actor when this was made," answered without a viewer
   doing arithmetic across two pages.
2. **A reusable typed field registry**, not a one-off: the text/categorical/numeric/date type taxonomy and
   equals/contains/range operator model land as a declared, documented Go-level contract so Phase 2/3 extend
   it by registration.
3. **Prove the relationship-scoped compute path.** F45's `Derive` is entity-intrinsic only (one entity's own
   resolved rows in, computed rows out); this ships the first cross-entity (person × video) computed field in
   the codebase, establishing the injection boundary Phase 2/3 reuse.
4. **Zero new storage.** No migration, no persisted index — compute-on-read, exactly like F45's proven
   pattern (D3).
5. **Resolver stays pure.** No `time.Now` inside `internal/resolver`; `now` stays injected at the API handler
   boundary (ADR-051 invariant, carried forward from ADR-063).

## Non-Goals

- **Search-bar / facet UI for Age in Media** — no compelling UX for numeric text search yet (ticket's own
  scope cut); the operator model exists as a contract, unconsumed by any UI this phase.
- **A persisted, queryable value index** — *(Why: D3 — nothing queries a stored index this phase; building
  one with zero consumers is speculative infrastructure. Phase 2 introduces it sized to Nationality's actual
  facet-query need.)*
- **Nationality canonicalization + wiring** (HOLODEX-179), **role-scoped joined queries / TMDB billing-order
  persistence** (HOLODEX-180), **smart playlists** (HOLODEX-181 / HOLODEX-16) — *(Why: separate phases/epics,
  each with its own spec.)*
- **`recorded_at` as an Age-in-Media input, at all** — *(Why: D1 — proven actively wrong for library content,
  not merely sparse; `release_date` is the sole input.)*
- **Role-filtered Age in Media** ("lead actor's age") — *(Why: `video_people` has no `role` column yet —
  ADR-059 is `Proposed`, not implemented. Every linked cast member gets an age, unfiltered by role, in this
  phase.)*
- **Editing / curating a computed relationship value** — *(Why: same non-adoptable convention as F45 Age — no
  `Decision`, no writeback, no promote pill, ever.)*
- **Any field type/operator beyond what proves the taxonomy** (e.g. a real categorical field with `contains`
  wired to a control) — *(Why: D2 builds the *model*, not new consumers of it; Nationality is where
  categorical gets its first real user.)*

## Users & Value

- **Visitor** — reads a cast member's age at the time a video was made directly on the video page, with the
  same "absent rather than wrong" guarantee F45's person Age already established.
- **Owner** — gets Age in Media for free once a video has both a TMDB-enriched release date and a
  birthdate-enriched cast member; nothing to curate or keep fresh.
- **Engineer (future phases)** — Nationality (Phase 2) and role-scoped joins (Phase 3) register new
  types/canonicals against a settled taxonomy instead of re-deriving one.

## User Stories

- As a visitor browsing a video page, I want to see each cast member's age at the time the video was made, so
  that I don't have to cross-reference birthdates and release dates myself.
- As a visitor, I want the age to simply not show for a cast member when we don't have enough data (no
  birthdate, no release date, or either is unparseable), so I never see a wrong or placeholder number.
- As the project owner, I want the typed field registry to be extensible without new plumbing, so that
  Nationality (Phase 2) and role-scoped joins (Phase 3) are registrations, not rewrites.
- As the project owner, I want Age in Media to follow the same non-adoptable/no-curation convention as person
  Age, so cast-member ages never become a stray writeback or curation surface.
- As an engineer building Phase 2/3, I want a documented operator model (equals/contains/range) settled before
  a UI consumes it, so the type/operator taxonomy isn't re-litigated per phase.

## Requirements

### Must-Have (P0)

**FR1 — Typed field registry.** Extend `registry.FieldDef` (or a parallel structure — exact shape is an
`/architecture` call) with a `FieldType` (`text` | `categorical` | `numeric` | `date`). Register it for at
least the fields Age in Media touches (`birthdate`, `release_date` → `date`; the new `age_in_media` →
`numeric`) plus one representative example of `text`/`categorical` each, so the taxonomy is proven, not just
declared for one type (D2).
- *AC*: `registry.FieldDef` has a `FieldType` field; at minimum `birthdate`, `release_date`, and
  `age_in_media` carry `date`/`date`/`numeric` respectively; a lookup by type is possible (even if unused by
  any caller yet).

**FR2 — Operator model.** Define the equals/contains/range operator taxonomy per type as a Go-level structure
(D2 — full model, not just range). No operator is wired to any UI or query path in this phase; it is the
reusable contract Phase 2/3 consume.
- *AC*: a documented mapping exists from `FieldType` → its valid operator set (e.g. `numeric`/`date` →
  `{equals, range}`, `text` → `{contains}`, `categorical` → `{equals}` at minimum); no runtime behavior depends
  on it yet (compile-time/doc-level contract is sufficient for Phase 1).

**FR3 — Relationship-scoped compute pass.** A new pass (mirrors `Derive`, e.g. a `DeriveRelationship`-shaped
function — precise signature and injection boundary is an `/architecture` decision) combines a resolved
person's `birthdate` with a resolved video's `release_date` into an `age_in_media` value, following the same
computable/absent contract as `deriveAge`: missing or unparseable input yields **no value**, never a
placeholder (mirrors F45 D3).
- *AC*: given a person with `birthdate=1960-01-01` and a video with `release_date=1990-06-15`, the pass
  returns `age_in_media=30`. Given either input absent or unparseable, it returns not-computable — no row, no
  placeholder.
- *AC*: the pass takes `now`/its inputs as parameters — no clock read, no I/O, inside `internal/resolver` (FR6).

**FR4 — Video-page cast list carries age.** Extend the cast-list data path
(`internal/repo/repo.go:590-593` `attachAssociations` people query, `model.Person` / the video detail
payload) to include each linked person's `age_in_media` when computable. Frontend
(`web/src/routes/media/[id]/+page.svelte:379-400`, `PersonPoster.svelte`, or a small new treatment) renders
it on the cast poster.
- *AC*: viewing a video with a resolved `release_date` and at least one linked person with a `birthdate`
  shows that person's age on their cast poster; a linked person without a birthdate shows no age (no
  placeholder) on the same poster grid.
- *AC*: viewing a video with **no** resolved `release_date` shows **no** age for **any** cast member on that
  video, regardless of their birthdate coverage.

**FR5 — Non-adoptable computed provenance.** `age_in_media` rows carry the same `computed:` token convention
as F45 (`fieldsource.ForComputed`) — no `Decision`, not curatable, never written back to a file or the DB.
- *AC*: an `age_in_media` value, wherever it appears in an API payload, carries `computed: true` and nil
  `decision`/`candidates`/`in_sync`, matching F45 Acceptance Criterion 6. No promote pill, no source-select,
  no curation control renders for it.

**FR6 — Resolver purity preserved.** `now` (and both entities' resolved input) is injected at the API handler
boundary; `internal/resolver` gains no clock read.
- *AC*: `grep` over `internal/resolver/` finds no `time.Now` attributable to this feature (mirrors F45
  Acceptance Criterion 8).

### Nice-to-Have (P1)

**FR7 — Provenance affordance on the value.** A hover/tooltip treatment mirroring F45's D5 ("calculated from
Birthdate + Released"), for visual consistency with the person-page Age row. Exact copy/placement is a
`/design-handoff` call.

**FR8 — API/MCP surface.** `age_in_media` appears in the video detail payload's structured field data (or an
equivalent) so MCP and other API consumers get it without a dedicated endpoint, matching F45 FR7.

### Future Considerations (P2 — explicitly not built here)

- Persisted, queryable value index (Phase 2 trigger, sized to Nationality's facet-query need).
- Categorical/text operator wired to a real UI control (Nationality, Phase 2).
- Role-scoped join filtering ("lead actor's age") — blocked on the `video_people` `role` column (ADR-059) and
  TMDB billing-order persistence (HOLODEX-180).
- Search-bar / facet surface for Age in Media itself.

## Success Metrics

This is a single-owner personal media server, not a multi-tenant product — there's no adoption funnel to
measure. The metrics that matter are correctness and coverage:

- **Correctness**: zero instances of a wrong (as opposed to absent) age rendered — verified by the "absent
  over wrong" acceptance criteria above and by D1 having already eliminated the one input source that would
  violate it.
- **Coverage**: Age in Media appears on a cast member's poster wherever both inputs are enriched. Expected to
  track existing enrichment coverage (currently sparse in the films testbed — 0/8 videos have `release_date`
  enriched, 1 person has `birthdate` enriched, at spec time) and grow as the owner runs TMDB enrichment,
  exactly like F45's Age did. No separate adoption target — this is a display feature riding existing
  enrichment, not a thing to "adopt."
- **Reusability (leading indicator for Phase 2/3)**: when HOLODEX-179 (Nationality) is scoped, it should need
  zero changes to the `FieldType`/operator taxonomy itself — only new registrations. If it needs taxonomy
  changes, that's a signal Phase 1 under-built the model.

## Open Questions

- **Which existing canonicals get a `FieldType` in this pass** — all of them, or just enough to prove each of
  the four types (the plan assumed in FR1)? *(engineering — confirm during `/architecture`)*
- **Exact shape/injection boundary of the relationship-compute pass** (`DeriveRelationship` vs. folding into
  the video/person handler directly vs. something else) — the ticket itself flagged "where the typed-field
  registry lives" as open; this extends to where the two-entity pass lives. *(engineering — `/architecture`)*
- **Cast poster layout for the age number** — the current poster grid (`PersonPoster.svelte`) has no
  subtitle/caption slot; where a bare integer fits without cluttering a dense grid is a real design question,
  not a copy-paste of F45's `<dl>` treatment (which had a labeled-row layout to work with). *(design —
  `/design-handoff`)*
- **Whether `age_in_media` needs a distinct label/copy from person Age** ("Age" vs. "Age at the time" vs. just
  the bare number with the video's release year in the tooltip) — *(design — `/design-handoff`)*

## Timeline Considerations

- No hard deadline. This is Phase 1 of 3 in the F46 epic; per the ticket's own framing it was sequenced first
  specifically because it has **no external blocker** (Phase 2/Nationality is explicitly backlog-priority per
  owner; Phase 3 depends on a separate TMDB billing-order persistence fix, HOLODEX-180).
- Suggested phasing is already fixed by the epic: this story ships standalone (substrate + Age in Media pilot
  together, since the pilot is how the substrate gets proven) before Phase 2 (Nationality) or Phase 3
  (role-scoped joins) start.
